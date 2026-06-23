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

export const GomemoryPlugin: Plugin = async ({ $, directory }) => {
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

  return {
    // Arranca una sesión de gomemory cuando OpenCode crea una sesión nueva.
    event: async ({ event }) => {
      if (event.type === "session.created") {
        await mem(["session", "start"]);
      }
    },

    // Cierra la sesión cuando el plugin se descarta (OpenCode termina).
    dispose: async () => {
      await mem(["session", "end"]);
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
    },

    // Antes de compactar, persiste el contexto y deja una instrucción explícita
    // para recuperar el estado después de la compactación.
    "experimental.session.compacting": async (_input, output) => {
      const ctx = await mem(["context"]);
      if (ctx) {
        output.context.push(
          "## gomemory — persist across compaction\n\n" + ctx,
        );
      }
      output.context.push(
        "AFTER COMPACTION — FIRST ACTIONS: call end_session(summary) to persist what was done, " +
          "then get_context() (or run `mem context`) to recover prior memory before continuing.",
      );
    },
  };
};

const MEMORY_PROTOCOL = `## Memory Protocol — gomemory (MANDATORY, ALWAYS ACTIVE)

You have a persistent memory system for this project via MCP tools
(save_memory, search_memories, get_memory, list_memories, get_context,
start_session, end_session). Do NOT wait for the user to ask.

SAVE immediately after: an architecture/design decision, a bug fix (include
root cause), a convention or pattern established, a tool/library choice with
tradeoffs, or a non-obvious discovery about the codebase.
Self-check after every task: "Did I decide, fix, discover, or establish
something? If yes → save_memory now."

SEARCH (progressive disclosure): search_memories(query) for compact hits, then
get_memory(id) only when you need full content. Search reactively when the user
references past work, and proactively when starting something that may overlap.

SESSION CLOSE: before saying "done", call end_session(summary) with Goal /
Discoveries / Accomplished / Next Steps / Relevant Files.`;
