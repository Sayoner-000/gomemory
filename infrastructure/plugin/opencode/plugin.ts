import { PluginContext } from "opencode";

let serverStarted = false;
let port = 9735;

export function activate(context: PluginContext) {
  const cfg = context.config || {};
  port = cfg.serverPort || 9735;

  ensureServer();
  ensureSession(context);
  injectMemoryProtocol(context);
  registerLifecycleHooks(context);
}

export function deactivate() {
  void 0;
}

function ensureServer() {
  if (serverStarted) return;
  try {
    // Attempt to start the gomemory HTTP server as a subprocess
    const { execSync } = require("child_process");
    execSync(`{{BIN_PATH}} serve --port ${port} &`, {
      stdio: "ignore",
      detached: true,
    });
    serverStarted = true;
  } catch {
    // Server already running or can't start — graceful degradation
    serverStarted = true;
  }
}

function ensureSession(context: PluginContext) {
  context.on("session.start", async () => {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/session/start`, {
        method: "POST",
      });
      const data = await res.json();

      // Inject context from previous sessions
      const ctxRes = await fetch(`http://127.0.0.1:${port}/context`);
      const ctx = await ctxRes.json();
      if (ctx.recentMemories?.length > 0 || ctx.recentSessions?.length > 0) {
        context.chat.system.transform((systemMsg: string) => {
          const contextBlock = formatContextBlock(ctx);
          return systemMsg + "\n\n" + contextBlock;
        });
      }
    } catch {
      // Server not available — continue without context injection
    }
  });
}

function injectMemoryProtocol(context: PluginContext) {
  context.chat.system.transform((systemMsg: string) => {
    return systemMsg + "\n\n" + MEMORY_PROTOCOL;
  });
}

function registerLifecycleHooks(context: PluginContext) {
  context.on("compact.after", async () => {
    try {
      const ctxRes = await fetch(`http://127.0.0.1:${port}/context`);
      const ctx = await ctxRes.json();

      const prevCtx = formatContextBlock(ctx);
      context.chat.system.transform((systemMsg: string) => {
        return (
          systemMsg +
          `\n\n**AFTER COMPACTION — FIRST ACTION REQUIRED**\n` +
          `1. Call end_session(summary) with the compacted summary content — this persists what was done before compaction\n` +
          `2. Call get_context() to recover previous session state\n` +
          `3. Only THEN continue working\n\n` +
          prevCtx
        );
      });
    } catch {
      // Compact recovery without server
      context.chat.system.transform((systemMsg: string) => {
        return (
          systemMsg +
          `\n\n**AFTER COMPACTION** — Call get_context() to recover previous state before continuing.`
        );
      });
    }
  });

  context.on("session.end", async () => {
    try {
      await fetch(`http://127.0.0.1:${port}/session/end`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
    } catch {
      // Session end best-effort
    }
  });
}

function formatContextBlock(ctx: any): string {
  const lines: string[] = ["## Previous Session Context\n"];
  if (ctx.activeSession) {
    lines.push("- There is an active session for this project.");
  }
  if (ctx.recentSessions?.length > 0) {
    lines.push("\n### Recent Sessions");
    for (const s of ctx.recentSessions.slice(0, 3)) {
      const summary = s.summary || "no summary";
      lines.push(`- ${s.created_at}: ${summary}`);
    }
  }
  if (ctx.recentMemories?.length > 0) {
    lines.push("\n### Recent Memories");
    for (const m of ctx.recentMemories.slice(0, 5)) {
      lines.push(`- [${m.type}] ${m.title}`);
    }
  }
  return lines.join("\n");
}

const MEMORY_PROTOCOL = `
## Memory Protocol — Persistent Memory System

You have access to gomemory, a persistent memory system for this project.
This protocol is MANDATORY and ALWAYS ACTIVE.

### WHEN TO SAVE (mandatory — do NOT wait for the user to ask)
Call save_memory() or ./mem save IMMEDIATELY after:
- Architecture or design decision made
- Bug fix completed (include root cause)
- Team convention or workflow change agreed
- Tool or library choice with tradeoffs
- Non-obvious discovery about the codebase
- Pattern established (naming, structure, convention)
- User preference or constraint learned

Self-check after EVERY task: "Did I make a decision, fix a bug,
discover something, or establish a convention?"

### WHEN TO SEARCH
Call search_memories() REACTIVELY when:
- User says "remember", "recall", "what did we do"
- User references past work in any language

Call search_memories() PROACTIVELY when:
- Starting work on something that might overlap with past sessions
- The task description mentions something you have no context on

### PROGRESSIVE DISCLOSURE (3 LAYERS)
Token-efficient memory retrieval:
1. search_memories(query) → compact results (~100 tokens each)
2. get_memory(id) → full untruncated content only when needed
3. Never dump all memory — search first, drill in only if necessary

### SESSION CLOSE PROTOCOL (mandatory)
Before ending a session or saying "done", call end_session() with:
## Goal
[What we were working on this session]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done]

## Relevant Files
- path/to/file — [what it does or what changed]

This is NOT optional. If you skip this, the next session starts blind.

### AFTER COMPACTION
If you see a compaction message or "FIRST ACTION REQUIRED":
1. IMMEDIATELY call end_session(summary) with the compacted content
2. Call get_context() to recover previous session state
3. Only THEN continue working

Do not skip step 1. Without it, everything done before compaction
is lost from memory.
`;
