# Configuration reference

Settings are per project. Change them with `mem settings` or from the interactive
TUI; `mem settings --show` prints the live values for the current project.

| Setting | Default | Description |
| :--- | :--- | :--- |
| `budget` | `24000` | Max characters returned by `get_context` |
| `compact_threshold` | `48000` | Context size that triggers compaction guidance |
| `compact_agent_notice` | `false` | Remind the agent to persist pending decisions when the compaction threshold is reached |
| `dedup_window_days` | `7` | Deduplication window |
| `synapse_disabled` | `false` | Disable automatic memory relationships |
| `atomic_plan_disabled` | `false` | Disable atomic planning |
| `plan_guard_disabled` | `false` | Disable the deterministic plan-shape guard (`mem hook plan-guard`) — see [`AGENT-INTEGRATION.md`](AGENT-INTEGRATION.md) |
| `code_graph_disabled` | `false` | Disable the optional external code-graph provider entirely |
| `code_graph_providers` | *(none)* | Ordered list of external code-graph provider commands (priority fallback) |
| `code_impact_annotation_disabled` | `false` | Disable annotating saved memories with code-graph hotspot impact |
| `adr_sync_enabled` | `false` | Opt-in bidirectional sync of architecture memories with the external provider's ADR document |
| `review_max_fix_rounds` | `2` | Round budget for `mem review` fix/re-judgment cycles |
| `review_auto_fix_severities` | `["CRITICAL","HIGH"]` | Severities eligible for automatic correction without explicit authorization |
| `update_check_disabled` | `false` | Turn off the new-version notice and its background check (no network calls). `GOMEMORY_NO_UPDATE_CHECK=1` does the same for every project |
| `octopus_enabled` | `false` | Opt-in **Octopus AAR** adaptive agent router — decides inline vs. delegate. Off means zero footprint: no MCP tools, no protocol text, no telemetry |
| `theme` | *(unset)* | TUI appearance: `dark`, `light`, `matrix` or `auto` — see [`CONSOLE-THEMES.md`](CONSOLE-THEMES.md) |
| `auto_approve` / `auto_approve_tools` | `false` / *(derived)* | Auto-approve gomemory's MCP tools in the agent; `forget_memory` is never included |
| `speckit_context_disabled` | `false` | Disable the memory-history summary injected into `/speckit-specify` |
| `code_graph_command` | *(none)* | Legacy single provider binary; superseded by `code_graph_providers` |
| `context_index_mode` | `false` | Emit context as an index (protocol + one line per memory); details on demand with `get_memory(id)` |
| `context_compression_level` | `structural` | `none`, `structural` or `max`. `mem install` writes `max` only in projects without prior settings. The legacy `context_compression_disabled: true` means `none` when the level is unset |
| `tool_output_compression` | `false` | Opt-in hook that compresses the agent's tool outputs (run `mem install` after changing it) |
| `concise_output_directive` | `false` | Add a concise-answers directive to the volatile part of the context |
| `compression_min_tokens` · `compression_originals_ttl_days` · `compression_originals_max_mb` · `compression_adaptive_threshold_pct` | `64` · `7` · `256` · `20` | Compression engine limits (`0` = factory value; the originals store never goes below 8 MB) |
| `usage_window_tokens` | *(unset)* | Reference window for the estimated percentage in `mem usage`; you provide it, it is never inferred |
| `review_fix_authorized` | `true` | Whether new reviews may record corrections at all; frozen into each review when it starts |
| `octopus_max_subagents` · `octopus_max_parallel` · `octopus_max_depth` · `octopus_max_retries` | `4` · `3` · `1` · `1` | Octopus AAR limits (`0` = factory value) |
| `octopus_main_agent_pct` · `octopus_delegation_pct` · `octopus_validation_pct` | `55` · `30` · `15` | Session budget split; if they do not add up to 100 the factory split applies |
| `socratica_interception` / `socratica_interception_unverified` | *(none)* | Per-agent automatic interception (`claude`, `codex`, `opencode`), and whether it was enabled with `--unverified`. Changed only via `mem socratica on`/`off` or the TUI, never via MCP |
| `socratica_private` | `false` | Private mode: no Socratica history for the project |
| `socratica_history_limit` | `20` | Refinements kept per agent (1–1000) |
| `socratica_generators` | `["claude","codex","opencode"]` | Enabled harnesses; each new CLI/TUI flow asks you to choose one for refinement and approved execution, without automatic fallback |
| `socratica_generator_timeout_seconds` · `socratica_query_timeout_ms` | `120` · `5000` | Generator timeout and per-query store timeout (minimum 100 ms) |
| `hcd_heartbeat_timeout_ms` | `200` | HCD turn-end heartbeat timeout in milliseconds (minimum 50) |
| `hcd_grant_timeout_hours` | `24` | HCD execution grant lifetime in hours (minimum 1) |
| `hcd_inactive_days` | `7` | HCD inactivity threshold in days (minimum 1) |

`agents` and `agent_scope` are written by `mem install` to remember your selection. The `context_default_budget`, `context_min_relevance`, `context_max_items` and `context_dedup_disabled` keys are reserved but not read yet: `mem pack build` always takes `--task`/`--max-tokens` explicitly. An invalid `socratica_*` value fails fast with the key and value instead of being replaced by a default.
