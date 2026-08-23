# Changelog

All notable changes to gomemory are documented in this file.

The format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/).

## [2.9.0] - 2026-08-23

### Added

- **Seeded memories for work rules and the constitution.** The first time
  gomemory is used in a project — either through `mem install` or the first
  start of the MCP server — it seeds two memories: the *work rules* (as a
  `preference`) and the *constitution* (as an `architecture` decision). The
  work rules are emitted **in full** in every `get_context()`, in a section of
  their own; they are the only declared exception to the context budget, the
  same treatment unresolved conflicts already had. The constitution is looked
  up on demand instead, so it never costs hundreds of lines per session.
- **`mem docs`** — manage pinned documents: `list`, `show`, `export`, `import`
  and `reset`. What ships with the tool is a **starting point, not doctrine**:
  without a comfortable way to replace it, seeding rules would turn gomemory
  into the author of a team's standards. `mem docs list` derives each
  document's state (`sin sembrar`, `por defecto`, `personalizado`) by comparing
  against the embedded template, so nothing extra is stored. `--topic` imports
  into any topic key, inside or outside the catalog.
- **Pinned documents in the interactive UI** — the configuration screen gained
  one row per catalogued document (`Actualizar Reglas IA`, `Actualizar
  Constitución`), each opening a screen with view, export, import and restore.
  A contract test checks that both surfaces offer the same operations, so they
  cannot drift apart.
- **`mem constitution [--sync]` and `mem rules`** — shortcuts over
  `mem docs show`. `--sync` mirrors the constitution into
  `.specify/memory/constitution.md` when the project uses spec-kit, and never
  creates that structure when it does not.
- **`/constitution` wrapper** for Claude Code and OpenCode. It carries no copy
  of the text: it resolves the constitution from memory at invocation time,
  which is exactly the mistake the removed install step used to make.
- **`mem seed`** — reseeds the default memories. `mem install` invokes it as a
  subprocess in the target directory.

### Changed

- **`mem install` no longer writes instruction files.** `AGENTS.md`,
  `CLAUDE.md` and `speckit-constitution-gen.md` are no longer generated: the
  protocol block was a second copy of the text the MCP server already delivers
  in its `initialize` response, and the copied constitution froze in place and
  diverged from its source as soon as either was edited.
- **Windsurf and Cline left automatic installation.** They created a folder in
  the root of *every* project to hold a single JSON file. Still supported
  explicitly via `mem setup-mcp --agents windsurf,cline`.
- **Legacy artifacts are removed on install and update.** Instruction files are
  **backed up** to `.memory/backups/agent-files/` before being deleted, and if
  the backup cannot be written the original is kept. MCP configs only lose
  their `gomemory` entry: other servers survive, and a JSON that cannot be
  parsed is left untouched.
- **Activation report**: project-scope instruction channels are reported as
  *not applicable*, with the reason, instead of *missing*. A legacy file that
  still holds an old block is still reported as outdated — that is true
  information about a stale duplicate, not a false alarm.

### Fixed

- **`ListMemories` did not return `topic_key`.** Unlike its sibling
  `ListAllMemories`, the query left the column out of its projection, so
  `TopicKey` reached every consumer of that path empty — the context builder,
  the `list_memories` MCP tool, the UI — with no error and no warning.
- **Seeding could publish the constitution to an external ADR document.**
  `architecture` maps to an exportable section, so with `adr_sync_enabled=true`
  installing would have pushed the whole document to the user's external ADR,
  synchronously and unrequested. Seeding, importing and restoring now use an
  inert insert path that skips automatic synapses and external publication.
  Secret redaction stays active on that path — it is a security defense, not a
  side channel.
- **A pinned memory could silently vanish from the context.** Its presence
  depended on the recency window of the memory list; with checkpoints generated
  every turn, it would eventually be buried with no error. It is now resolved by
  topic key, independently of recency.
- **`mem docs export <alias> -o <file>` wrote to stdout and left the file
  empty.** Go's flag parser stops at the first positional argument, so a flag
  placed after the alias was never read. Both orders work now.
- **Flaky integration tests.** A detached background process writing a graph
  snapshot into `.memory/` raced the temporary-directory cleanup, failing a
  different test roughly one run in four without any assertion failing.

## [2.8.0] - 2026-08-20

### Added

- **`mem usage`** — a measured token benchmark per session. Reports baseline
  tokens (what a response would have cost unoptimized), emitted tokens, the
  saved delta and reduction ratio, broken down by operation and by channel
  (`mcp`, `cli`, `tui`). Available as `mem usage [--session ID|--all] [--json]`.
  The header always declares that the counting method is a neutral
  approximation (~4 chars/token), not any provider's tokenizer — figures are
  comparable against themselves, never against anyone's billing. An optional
  reference-window setting (`usage_window_tokens`, off by default) adds an
  estimated "footprint avoided" percentage, clearly labeled `(estimated)`.
  Machine-readable contract: [`docs/USAGE-REPORT-CONTRACT.md`](docs/USAGE-REPORT-CONTRACT.md).
- **Usage screen in the interactive UI** (`u` key) — shows the same session
  report as `mem usage`, plus an on-demand context-optimization snapshot for
  a specific task (same engine as `mem pack build`), which never persists
  between visits.
- **`mem consolidate [--apply]`** — merges redundant memories within a
  project by two criteria: shared topic key, and automatic activity
  checkpoints with byte-identical content. Previews by default (the
  operation is irreversible); no content is lost, distinct text within a
  merged group is concatenated into the row that's kept. Also available from
  the interactive UI's Maintenance screen.
- **`mem get <id>`** — retrieves a memory's full detail by ID from the
  command line, mirroring the `get_memory` MCP tool's capability on the CLI
  channel.
- **Index mode for `get_context`** (`context_index_mode` setting, off by
  default) — emits the working protocol in full plus a one-line index per
  memory (id, type, title), with detail fetched on demand via the existing
  `get_memory` capability. Reversible: toggling it off returns emission to
  byte-identical output.

### Changed

- `charmbracelet/bubbletea` v0.26.1 → v1.3.10, `bubbles` v0.18.0 → v1.0.0,
  `lipgloss` v1.0.0 → v1.1.0. No application code changes were needed; every
  existing screen and test kept its exact behavior.

### Fixed

- **MCP usage-recording middleware never fired.** The fallback middleware
  that records non-self-reporting tool calls (`save_memory`, `get_memory`,
  and others) type-asserted the request params to `*mcp.CallToolParams`, but
  the SDK hands middlewares the *unparsed* params (`*mcp.CallToolParamsRaw`)
  — the assertion silently failed for every call. Found by an end-to-end
  test that drives a real MCP client against the real server, not a
  schema-listing check. Fixed and covered by two new integration tests.
- `search_memories`/`list_memories` usage accounting could report emitted
  tokens higher than baseline tokens for short memories, because the raw
  baseline only summed memory content and ignored the rendered wrapper
  (id/type/title/formatting). Baseline is now derived as emitted + what was
  actually truncated, guaranteeing baseline ≥ emitted.
- `ListAllMemories` never selected the `topic_key` column, so every memory
  loaded through it appeared to have no topic key regardless of what was
  stored — this silently broke topic-key grouping before it was ever used.
- `mem usage`'s "no session" report collapsed with the "all sessions"
  report internally (both used an empty session ID), so an idle project
  could show accumulated historical totals instead of zeros.

## Earlier releases

See [GitHub Releases](https://github.com/Sayoner-000/gomemory/releases) for
v2.7.0 and earlier.
