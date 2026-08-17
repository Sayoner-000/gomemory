// gomemory — plugin de OpenCode (API real @opencode-ai/plugin).
//
// `import type` se borra en runtime: el plugin no tiene dependencias de módulo,
// solo usa los tipos para validación. OpenCode auto-descubre este archivo en
// ~/.config/opencode/plugins/gomemory.ts y ejecuta los hooks declarados.
//
// El guardado de memorias lo hace el agente vía las tools MCP (save_memory, …),
// que se exponen al configurar el server `gomemory` en opencode.json. Este
// plugin se encarga de lo que el MCP no cubre: gestionar la sesión de trabajo,
// inyectar el protocolo + el contexto histórico en el system prompt, y recuperar
// el estado tras una compactación. Todo es best-effort: si el binario `mem` no
// está disponible, los hooks degradan en silencio.
import type { Plugin } from "@opencode-ai/plugin";

// {{BIN_PATH}} lo sustituye el instalador por la referencia portable a `mem`
// (normalmente "mem" en el PATH).
const BIN = "{{BIN_PATH}}";

export const GomemoryPlugin: Plugin = async ({ $, directory, client }) => {
  const root = directory;

  // Ejecuta `mem <args>` en la raíz del proyecto y devuelve stdout (trim).
  // Nunca lanza: ante cualquier fallo devuelve "" para no romper la sesión.
  const mem = async (args: string[]): Promise<string> => {
    try {
      return (await $`${BIN} ${args}`.cwd(root).quiet().text()).trim();
    } catch {
      return "";
    }
  };

  // Igual que `mem`, pero pasando `input` por stdin (usado por turn-end para
  // mandar {files, commands} sin depender de un transcript en disco, a
  // diferencia de Claude Code que sí lo tiene).
  const memWithStdin = async (args: string[], input: string): Promise<string> => {
    try {
      const proc = $`${BIN} ${args}`.cwd(root).quiet();
      const writer = proc.stdin.getWriter();
      await writer.write(new TextEncoder().encode(input));
      await writer.close();
      return (await proc.text()).trim();
    } catch {
      return "";
    }
  };

  // Último messageID de sesión ya inspeccionado para checkpoint, para no
  // reprocesar el historial completo en cada session.idle.
  const lastCheckpointedMessage = new Map<string, string>();

  // Equivalente en OpenCode del hook "Stop" de Claude Code: dispara cuando la
  // sesión queda idle (el asistente terminó de responder). Recolecta,
  // determinísticamente y sin gastar tokens del agente, qué archivos se
  // editaron/escribieron y qué comandos de shell corrieron desde el último
  // checkpoint, y se lo pasa a `mem hook turn-end` (misma lógica de guardado
  // que usa Claude Code, ver adapters/primary/cli/cmd_hook.go).
  const handleTurnEnd = async (sessionID: string): Promise<void> => {
    try {
      const res = await client.session.messages({ path: { id: sessionID } });
      const messages: Array<{ info: any; parts: any[] }> = (res as any)?.data ?? [];
      if (messages.length === 0) return;

      const lastSeen = lastCheckpointedMessage.get(sessionID);
      let startIdx = 0;
      if (lastSeen) {
        const idx = messages.findIndex((m) => m.info?.id === lastSeen);
        if (idx >= 0) startIdx = idx + 1;
      }
      const newMessages = messages.slice(startIdx);
      if (newMessages.length === 0) return;
      lastCheckpointedMessage.set(sessionID, messages[messages.length - 1].info?.id);

      const files = new Set<string>();
      const commands: string[] = [];
      const planTexts: string[] = [];
      for (const msg of newMessages) {
        // Turno en modo plan: el asistente redacta un plan sin tocar archivos ni
        // correr comandos, así que el checkpoint de turn-end lo descartaría por
        // vacío. Capturamos su texto como decisión vía `mem hook plan-approved`
        // (mismo contrato Go que usa Claude Code con ExitPlanMode), para que las
        // decisiones del plan no se pierdan y se acumulen entre sesiones.
        if (msg.info?.role === "assistant" && msg.info?.mode === "plan") {
          const text = (msg.parts ?? [])
            .filter((p: any) => p.type === "text")
            .map((p: any) => p.text ?? "")
            .join("\n")
            .trim();
          if (text) planTexts.push(text);
        }
        for (const part of msg.parts ?? []) {
          if (part.type !== "tool" || part.state?.status !== "completed") continue;
          const input = part.state.input ?? {};
          if (part.tool === "bash" && typeof input.command === "string") {
            commands.push(input.command);
          } else if (part.tool === "edit" || part.tool === "write") {
            const path = input.filePath ?? input.path ?? input.file;
            if (typeof path === "string" && path) files.add(path);
          }
        }
      }

      for (const plan of planTexts) {
        await memWithStdin(["hook", "plan-approved"], JSON.stringify({ plan }));
      }

      if (files.size === 0 && commands.length === 0) return;

      await memWithStdin(["hook", "turn-end"], JSON.stringify({ files: [...files], commands }));
    } catch {
      // best-effort: un checkpoint fallido nunca debe romper la sesión.
    }
  };

  return {
    // Arranca una sesión de gomemory cuando OpenCode crea una sesión nueva;
    // dispara el checkpoint de turno cuando la sesión queda idle.
    event: async ({ event }) => {
      if (event.type === "session.created") {
        await mem(["session", "start"]);
      }
      if (event.type === "session.idle") {
        await handleTurnEnd(event.properties.sessionID);
      }
    },

    // Cierra la sesión cuando el plugin se descarta (OpenCode termina).
    dispose: async () => {
      await mem(["session", "end"]);
    },

    // Se dispara una vez por mensaje del usuario, antes de que lo vea el LLM
    // (equivalente a UserPromptSubmit de Claude Code). Persiste el prompt del
    // turno en la sesión activa vía `mem hook prompt` para que `mem` lo adjunte
    // como provenance a lo que se guarde (misma lógica en Go). El texto real
    // viene en output.parts (type "text").
    "chat.message": async (_input, output) => {
      const prompt = (output.parts ?? [])
        .filter((p: any) => p.type === "text")
        .map((p: any) => p.text ?? "")
        .join("\n")
        .trim();
      if (prompt.length > 0) {
        await memWithStdin(["hook", "prompt"], JSON.stringify({ prompt }));
      }
    },

    // Inyecta el protocolo de memoria y el contexto histórico en el system
    // prompt de cada turno, para que el agente sepa que debe usar las tools y
    // arranque con la memoria previa cargada.
    "experimental.chat.system.transform": async (_input, output) => {
      output.system.push(MEMORY_PROTOCOL);
      const ctx = await mem(["context"]);
      if (ctx) {
        output.system.push("## Project Memory (gomemory)\n\n" + ctx);
      }
      // Recordatorio de guardado transversal: misma decisión (umbral + debounce)
      // que el hook de Claude Code, resuelta en Go. `mem hook nudge` devuelve el
      // texto solo cuando el agente lleva rato sin guardar nada real; si no toca,
      // devuelve "" y no se inyecta nada.
      const nudge = await mem(["hook", "nudge"]);
      if (nudge) {
        output.system.push(nudge);
      }
    },

    // Antes de compactar, empuja al `output.context` retenido (sobrevive a la
    // compactación) las instrucciones de recuperación + el contexto histórico.
    // Reusa `mem hook post-compact` para que el texto de recuperación tenga una
    // sola fuente en Go, compartida con el hook SessionStart(compact) de Claude
    // Code — misma lógica transversal que `mem hook nudge`.
    "experimental.session.compacting": async (_input, output) => {
      const recovery = await mem(["hook", "post-compact"]);
      if (recovery) {
        output.context.push(recovery);
      }
    },
  };
};

// OpenCode expone las tools MCP con el prefijo "<servidor>_" (un solo guión
// bajo), no el "mcp__<servidor>__" de Claude Code. Estos nombres van literales
// y ya prefijados aquí mismo — en un solo lugar por servidor — para no repetir
// el bug ya visto del lado de Claude Code (get_plan_context quedó mencionada
// con un nombre que el agente no podía invocar porque no coincidía con el
// real). tests/contract/mcp_tool_sync_test.go verifica que esta lista siga
// cubriendo domain.MCPAllTools() / domain.CodebaseMemoryMCPDiscoveryTools.
const [
  T_GET_CONTEXT,
  T_GET_PLAN_CONTEXT,
  T_SAVE_MEMORY,
  T_SEARCH_MEMORIES,
  T_LIST_MEMORIES,
  T_GET_MEMORY,
  T_FORGET_MEMORY,
  T_JUDGE_MEMORIES,
  T_START_SESSION,
  T_END_SESSION,
] = [
  "gomemory_get_context",
  "gomemory_get_plan_context",
  "gomemory_save_memory",
  "gomemory_search_memories",
  "gomemory_list_memories",
  "gomemory_get_memory",
  "gomemory_forget_memory",
  "gomemory_judge_memories",
  "gomemory_start_session",
  "gomemory_end_session",
];

const [T_SEARCH_CODE, T_GET_SYMBOL, T_LIST_DEPENDENCIES, T_GRAPH_STATUS, T_INDEX_PROJECT] = [
  "gomemory_search_code",
  "gomemory_get_symbol",
  "gomemory_list_dependencies",
  "gomemory_graph_status",
  "gomemory_index_project",
];

// Context Optimization Engine (feature 015): domain.MCPContextPackTools.
const [T_PACK_BUILD, T_PACK_SHOW, T_PACK_COMPRESS, T_PACK_STATS] = [
  "gomemory_pack_build",
  "gomemory_pack_show",
  "gomemory_pack_compress",
  "gomemory_pack_stats",
];

const [
  T_EXT_SEARCH_GRAPH,
  T_EXT_TRACE_PATH,
  T_EXT_GET_CODE_SNIPPET,
  T_EXT_QUERY_GRAPH,
  T_EXT_GET_ARCHITECTURE,
  T_EXT_SEARCH_CODE,
] = [
  "codebase-memory-mcp_search_graph",
  "codebase-memory-mcp_trace_path",
  "codebase-memory-mcp_get_code_snippet",
  "codebase-memory-mcp_query_graph",
  "codebase-memory-mcp_get_architecture",
  "codebase-memory-mcp_search_code",
];

const MEMORY_PROTOCOL = `## Memory Protocol — gomemory (MANDATORY, ALWAYS ACTIVE)

You have a persistent memory system for this project via MCP tools
(${T_GET_CONTEXT}, ${T_GET_PLAN_CONTEXT}, ${T_SAVE_MEMORY}, ${T_SEARCH_MEMORIES},
${T_LIST_MEMORIES}, ${T_GET_MEMORY}, ${T_FORGET_MEMORY}, ${T_JUDGE_MEMORIES},
${T_START_SESSION}, ${T_END_SESSION}). The project's own code graph is
available too (${T_SEARCH_CODE}, ${T_GET_SYMBOL}, ${T_LIST_DEPENDENCIES},
${T_GRAPH_STATUS}, ${T_INDEX_PROJECT}). Do NOT wait for the user to ask.

PLAN MODE: when you enter plan mode — or when the request asks for a plan, an
approach, or a strategy before touching code — call ${T_GET_PLAN_CONTEXT}()
BEFORE drafting the plan. It returns the atomic decomposition method plus the
project history; apply that method when drafting. In plan mode, deliver the
task tree and STOP — do not execute.

SAVE immediately after: an architecture/design decision, a bug fix (include
root cause), a convention or pattern established, a tool/library choice with
tradeoffs, or a non-obvious discovery about the codebase. Routine activity
(which files changed, which commands ran) is already captured automatically as
a checkpoint — don't duplicate it by hand.
Self-check after every task: "Did I decide, fix, discover, or establish
something? If yes → ${T_SAVE_MEMORY} now."

SEARCH (progressive disclosure): ${T_SEARCH_MEMORIES}(query) for compact hits,
then ${T_GET_MEMORY}(id) only when you need full content. Search reactively
when the user references past work, and proactively when starting something
that may overlap.

IMPARTIAL JUDGE: if two memories contradict each other (shown under "Conflictos
sin resolver" in the context, or noticed while searching), don't assume the
newer one is correct. Re-read the current code/source to verify which one
reflects reality, then record the verdict with
${T_JUDGE_MEMORIES}(id_a, id_b, verdict, confidence, reasoning) — explain in
reasoning what you verified.

PRIVACY: if content to save includes a secret, token, or credential, wrap that
part in <private>...</private> — it is never persisted.

CONTEXT PACKS: when a task needs only a slice of the project's memory instead
of the full get_context() dump, call ${T_PACK_BUILD}(task, max_tokens, ...) to
get a token-budgeted ContextPack (relevant memories, deduplicated, non-critical
content compressed, critical content never dropped silently). ${T_PACK_SHOW}
and ${T_PACK_STATS} reformat/report on an already-built pack; ${T_PACK_COMPRESS}
compresses arbitrary text on its own, no retrieval involved.

EXTERNAL CODE GRAPH: if the codebase-memory-mcp MCP server is connected, use it
ALWAYS for code exploration instead of reading files by hand: ${T_EXT_SEARCH_GRAPH},
${T_EXT_TRACE_PATH}, ${T_EXT_GET_CODE_SNIPPET}, ${T_EXT_QUERY_GRAPH},
${T_EXT_GET_ARCHITECTURE}, ${T_EXT_SEARCH_CODE}. Use the graph's tools to
explore; use the atomic task tree to deliver a plan. What you find with the
graph feeds the tree's leaves. If it is not connected, this guidance does not
apply.

SESSION CLOSE: before saying "done", call ${T_END_SESSION}(summary) with Goal /
Discoveries / Accomplished / Next Steps / Relevant Files.`;
