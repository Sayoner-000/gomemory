# Universal Agent Instructions

GoMemory installs a portable operating baseline in the user-level instruction
files of Claude Code, Codex, and OpenCode. Its canonical, embedded source is
[`infrastructure/templates/universal-agent-instructions.md`](../infrastructure/templates/universal-agent-instructions.md).

The baseline is deliberately small: it establishes authority, evidence,
progressive context, minimal tools, intentional delegation, proportional
validation, safety, and clear reporting without assuming a vendor or runtime.

It is not the GoMemory protocol. The runtime-specific GoMemory block provides
MCP tools, persistence, session handling, privacy, and optional Octopus routing.
Project-specific lessons remain a pinned `Reglas IA` document retrieved through
`get_context`; they override the baseline only where they carry concrete local
evidence.

Hooks and the OpenCode plugin inject only dynamic context. They do not repeat
the universal baseline on every turn.
