<p align="center">
  <img src="assets/gomemory_light.png" alt="gomemory logo" width="200">
</p>

<p align="center">
  <strong>Persistent, local and portable memory for AI coding agents</strong>
</p>

[![CI](https://github.com/Sayoner-000/gomemory/actions/workflows/ci.yml/badge.svg)](https://github.com/Sayoner-000/gomemory/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/Sayoner-000/gomemory?style=flat&color=blue)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-15_tools-blueviolet)](https://modelcontextprotocol.io/)

gomemory gives AI coding agents persistent memory across sessions.
It stores project context, architectural decisions, bug fixes, learnings and checkpoints in a local SQLite database — so your agent can remember what happened, why it happened, and what was decided without polluting your repository with memory files.

Works with Claude Code, Cursor, OpenCode, Windsurf, Cline and Codex through the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP).

```
┌──────────────────────────────────────────────┐
│              AI Coding Agent                 │
│                                              │
│ Claude Code · Cursor · OpenCode · Codex      │
│ Windsurf · Cline                             │
└──────────────────────┬───────────────────────┘
                       │ MCP
                       ▼
┌──────────────────────────────────────────────┐
│                  gomemory                    │
│                                              │
│  Context · Decisions · Bugfixes · Learning   │
│  Checkpoints · Architecture · Patterns       │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ Local SQLite DB │
              │   Persistent    │
              │    Portable     │
              └─────────────────┘
```

No cloud service. No API key. No database server. No files added to your project.

## Why gomemory?

AI coding agents are powerful, but their context is often temporary.
When a session ends, important information can disappear:

- Why was this architecture chosen?
- What bug was fixed and how?
- Which approach was rejected?
- What did we learn from the previous implementation?
- What decisions should the next session know about?

gomemory turns that temporary context into persistent project memory.

## Quick Start

### 1. Install

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/main/scripts/install.ps1 | iex
```

Verify:
```bash
mem --help
```

### 2. Connect your coding agent

**For agents with global MCP configuration:**
```bash
mem setup-mcp --scope global --agents claude,codex,opencode
```

**For project-scoped configuration:**
```bash
cd /path/to/your/project
mem setup-mcp --scope project --agents cursor,windsurf,cline --target .
```

That's it. Your memory is stored outside the repository:

- **Linux / macOS:** `~/.local/share/gomemory/`
- **Windows:** `%LOCALAPPDATA%\gomemory\`

> **`mem setup-mcp` vs `mem setup`:** `setup-mcp` registers the **MCP tools** for all 6 supported agents. The **auto-checkpoints** and **plan capture** features additionally require the hooks/plugin from `mem setup <agent>`, currently available for `opencode` and `claude-code`. In Cursor/Windsurf/Cline/Codex you get memory via MCP, but without that automatic per-turn capture.

### 3. Try it

```bash
# Save a decision
mem save -t "API routing" -y decision "Use Fiber for HTTP routing"

# Search your project memory
mem search "API routing"

# View recent memories
mem list

# Get the current project context
mem context

# Open the interactive terminal UI
mem
```

## What it remembers

gomemory supports structured memory instead of treating everything as an undifferentiated text dump.

| Type | Purpose |
| :--- | :--- |
| `architecture` | Architectural decisions and structure |
| `decision` | Important technical decisions |
| `bugfix` | Bugs, causes and solutions |
| `pattern` | Reusable implementation patterns |
| `learning` | Lessons learned |
| `discovery` | Findings and investigations |
| `preference` | User/project preferences |
| `checkpoint` | Session progress |

## Key Features

**Persistent memory**
Memory survives agent sessions and is stored locally in SQLite.

**Relevant retrieval**
Memory search uses FTS5 + BM25 ranking, with an automatic LIKE fallback when FTS5 is unavailable.

**Connected memory**
Related memories are automatically linked through synapses, forming a persistent knowledge graph that is re-injected into each `get_context`.

**Context-aware retrieval**
`get_context` is budget-limited and `get_memory` provides full details on demand — minimizing token usage.

**Privacy by design**
Memory stays local. Sensitive information is automatically redacted (AWS credentials, GitHub tokens, AI provider keys, Slack tokens, JWTs, PEM private keys). You can also explicitly mark content as private:

```
<private>This information should never be persisted.</private>
```

**Automatic checkpoints**
With Claude Code and OpenCode, active turns are captured automatically as checkpoints without consuming additional agent tokens.

**Plan memory**
Agents can retrieve project history and atomic decomposition guidance before planning:

```bash
mem plan-context
```

**Code graph integration**
Optional integration with an external code graph (via [`codebase-memory-mcp`](https://github.com/DeusData/codebase-memory-mcp)) enriches memory with modules, symbols, dependencies, hotspots and callers. Non-blocking and agnostic to the agent. Controlled via `mem settings --code-graph=true|false`.

**ADR synchronization**
Architecture and decision memories can optionally synchronize with external ADR documents. Controlled via `mem settings --adr-sync=true|false`.

**Portable memory**
Export project memory as a self-contained JSON bundle:

```bash
mem export    # → project-memory.json
mem import    # → import with dedup, preserving timestamps and relationships
```

**Automatic backups**
Local snapshots are created at session end. Do not synchronize `mem.db` directly — SQLite uses WAL mode and partial synchronization can corrupt the database. Use the exported JSON backup instead.

## MCP Tools

| Tool | Description |
| :--- | :--- |
| `save_memory` | Store structured project memory |
| `search_memories` | Search relevant memories |
| `list_memories` | List recent memories |
| `get_memory` | Retrieve a complete memory |
| `get_context` | Retrieve project context |
| `get_plan_context` | Retrieve planning context |
| `start_session` | Start a working session |
| `end_session` | Close a working session |
| `forget_memory` | Remove a memory |
| `judge_memories` | Resolve conflicting memories |

**Resources:** `mem://context` · `mem://memory/{id}`

## CLI

```
mem
├── save          Save a memory manually
├── search        Search project memory
├── list          List recent memories
├── context       Show current context
├── plan-context  Atomic planning context
├── capture       Guided memory form
├── project       Show project info
├── index         Index project code graph
├── export        Export memories to JSON
├── import        Import memories from JSON
├── update        Update the binary
├── uninstall     Remove gomemory
├── purge         Delete memories
├── gc / compact  Cleanup and optimize
├── settings      Configure gomemory
└── help          Show help
```

Run `mem help` for the complete command reference.

## Architecture

```
gomemory/
├── domain/          # Core domain models and rules
├── application/     # Use cases and application services
├── adapters/        # CLI, MCP, TUI and SQLite adapters
├── infrastructure/  # Agent integrations and orchestration
├── scripts/         # Installation scripts
├── tests/           # Tests
└── docs/            # Extended documentation
```

**Storage:** SQLite embedded via `modernc.org/sqlite` (no CGO required). Lives in a global user store, not inside your repository.

**MCP transport:** `stdio` + JSON-RPC. The agent launches `mem mcp` as a subprocess. No TCP server or exposed network port.

**Code graph:** Provider-based architecture (`CodeGraphProvider` port in hexagonal style) allows integrating external code graphs without coupling the core. Hot path reads a cached snapshot; background refresh never blocks saves or context.

For the complete architecture, see [`docs/architecture.md`](docs/architecture.md).

## Configuration

Main settings (via `mem settings` or the interactive TUI):

| Setting | Default | Description |
| :--- | :--- | :--- |
| `budget` | `24000` | Max characters returned by `get_context` |
| `compact_threshold` | `48000` | Context size that triggers compaction guidance |
| `dedup_window_days` | `7` | Deduplication window |
| `synapse_disabled` | `false` | Disable automatic memory relationships |
| `atomic_plan_disabled` | `false` | Disable atomic planning |

See [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) for all configuration options.

## Build from Source

Requirements: Go 1.25+

```bash
git clone https://github.com/Sayoner-000/gomemory.git
cd gomemory
go build -o mem ./infrastructure/
./mem install .
```

Run tests:
```bash
go test ./...
```

## Supported Agents

| Agent | MCP | Automatic hooks |
| :--- | :---: | :---: |
| Claude Code | ✅ | ✅ |
| OpenCode | ✅ | ✅ |
| Cursor | ✅ | — |
| Windsurf | ✅ | — |
| Cline | ✅ | — |
| Codex | ✅ | — |

MCP provides persistent memory across all supported agents. Automatic checkpoint and plan capture currently depend on the agent integration.

## Security & Privacy

gomemory is designed as a local-first memory system.
By default:

- Data remains on your machine
- No external database is required
- No network service is opened
- Sensitive credential patterns are redacted
- Database permissions are restricted
- Memory can be exported or deleted by the user

For security details and limitations, see [`docs/MANUAL.md`](docs/MANUAL.md).

## Documentation

| Document | Description |
| :--- | :--- |
| [`docs/MANUAL.md`](docs/MANUAL.md) | Complete user guide: multi-agent, troubleshooting, security, portability |
| [`docs/architecture.md`](docs/architecture.md) | Internal architecture deep dive |
| [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) | Memory protocol technical reference |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | How to contribute |
| [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) | Community guidelines |

## Contributing

Contributions are welcome. Before opening a pull request:

```bash
go test ./...
go vet ./...
```

Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

If you find a bug or have an idea, [open an issue](https://github.com/Sayoner-000/gomemory/issues) with enough context to reproduce or evaluate it.

## Roadmap

The project is actively evolving. Areas of interest include:

- Additional coding-agent integrations
- Improved memory ranking and retrieval
- More code-graph providers
- Better visualization of memory relationships
- Performance improvements
- Additional portability and synchronization options

See [GitHub Issues](https://github.com/Sayoner-000/gomemory/issues) for current work.

---

**Author:** Sayoner ([@Sayoner-000](https://github.com/Sayoner-000))
**License:** MIT · See [`LICENSE`](LICENSE)
**Inspiration:** Inspired by the architecture of [Engram](https://github.com/Gentleman-Programming/engram).

*Built with Go, SQLite and the [Model Context Protocol](https://modelcontextprotocol.io/).*

If gomemory is useful to you, consider giving the project a ⭐ on GitHub.
