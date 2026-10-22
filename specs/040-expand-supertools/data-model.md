# Data Model: 040 mem expand

Los tipos de dominio son puros (`domain/`). La persistencia es aditiva. Las referencias R* remiten a [research.md](research.md).

## 1. Dominio

### Supertool (`domain/expand.go`)

| Campo | Tipo | Notas |
|---|---|---|
| `Name` | `SupertoolName` | `rules` \| `shield` \| `flow` \| `instincts` (registro cerrado `domain.Supertools`) |
| `Events` | `[]ExpandEvent` | eventos que la pueden disparar (documental; el router es la fuente de verdad) |

`ExpandEvent` ∈ {`session-start`, `prompt`, `pre-tool`, `post-tool`, `stop`}. `ParseExpandEvent(s)` devuelve `ok=false` ante un valor desconocido, y en ese caso el hook permite sin emitir nada.

### ExpandSignals

| Campo | Tipo | Origen |
|---|---|---|
| `Event` | `ExpandEvent` | argumento del hook |
| `Source` | `string` | `startup\|resume\|clear\|compact` (solo session-start) |
| `Tool` | `ToolKind` | `shell\|file\|delegate\|other`, normalizado desde `tool_name` por `domain.ClassifyTool` (Bash/bash/shell/exec_command → shell; Write/Edit/MultiEdit/NotebookEdit/write/edit/patch/apply_patch → file; Task/Agent/task/collaborationspawn_agent → delegate; resto, incluido collaborationwait_agent → other. Nombres de Codex verificados en `evidence/codex-probe.md`) |
| `Command` | `string` | `tool_input.command` (truncado a 8 KiB) |
| `FilePath` | `string` | `tool_input.file_path` / `path` |
| `Prompt` | `string` | `prompt` (truncado a 16 KiB) |
| `ToolCount` | `int` | contador del episodio (R6) |
| `CompactThreshold` | `int` | ajuste; 50 por defecto |
| `ConfigHash`, `LastShieldHash` | `string` | R10 |
| `HasRules` | `bool` | hay al menos una regla válida (propia o de serie) |

### ExpandSwitches

`Disabled bool` y `Off map[SupertoolName]bool`, construidos desde `SettingsData` (R15).

### SupertoolAction

| Campo | Tipo | Valores |
|---|---|---|
| `Kind` | `ActionKind` | `rules.eval`, `shield.scan`, `instincts.observe`, `instincts.fold`, `suggest`, `counter.reset`, `episode.reset` (la inyección de instincts la hace `build_context`, no el router: R9) |
| `Target` | `string` | para `rules.eval`: `bash\|file\|prompt\|stop`; para `suggest`: el nombre de la skill |

`RouteSupertools(ExpandSignals, ExpandSwitches) []SupertoolAction` es pura, determinista y con un orden estable: primero las acciones bloqueantes, luego las de contexto y al final las silenciosas.

### Rule (`domain/rules.go`)

| Campo | Tipo | Validación |
|---|---|---|
| `Name` | `string` | no vacío, ≤ 64 caracteres, `[a-z0-9-]`; por defecto, el nombre del archivo sin `.md` |
| `Event` | `RuleEvent` | `bash\|file\|prompt\|stop\|any` |
| `Match` | `MatchKind` | `contains\|regex` (por defecto `contains`) |
| `Pattern` | `string` | no vacío, ≤ 512 caracteres; si es `regex`, debe compilar con RE2 |
| `Action` | `RuleAction` | `warn\|block` |
| `Enabled` | `bool` | por defecto `true` |
| `Overrides` | `string` | nombre de una regla de serie (opcional) |
| `Message` | `string` | cuerpo saneado con `StripTerminalControls`, ≤ 2 KiB |
| `Origin` | `string` | `builtin\|project` |
| `Source` | `string` | ruta relativa (solo `project`) |

Una regla inválida produce un `RuleLoadError{Source, Reason}`, que `mem expand status` muestra, y no se carga.

### RuleVerdict

`Decision` (`allow|warn|block`), `Reason string`, `Rule string`. `EvaluateRules(target, input, rules) RuleVerdict` es pura y aplica la precedencia de R7. La regla de serie `destructive-shell` es `domain.BuiltinRules[0]`, con un evaluador tokenizado (R8) en lugar de patrón.

### ExpandOutcome

Es el agregado que entrega el caso de uso al adaptador: `Verdict RuleVerdict`, `Context []string` (avisos y sugerencias ya filtrados por episodio), `Fired []string` (claves para `recordGuard`). Por defecto vale `allow` y lleva `Context` vacío.

### SuggestionEpisode (estado del adaptador, archivo)

Es un conjunto de claves `suggest:<skill>` ya emitidas en `.memory/.expand-episode-<sid8>`. Un archivo ilegible se trata como «ya emitidas todas» (sesgo al silencio, el mismo criterio que `planEpisodeDenied`). Se reinicia con `episode.reset` y se purga a los 7 días.

### ShieldFinding (`domain/shield.go`)

| Campo | Tipo | Notas |
|---|---|---|
| `File` | `string` | ruta con `~` en lugar del home |
| `Line` | `int` | 0 si no aplica |
| `Category` | `string` | `secret\|remote-exec\|hook-outside-home\|mcp-inline-credential\|broad-permission` |
| `Severity` | `string` | `critical\|high\|medium` |
| `Detail` | `string` | descripción redactada; **nunca** contiene el valor del secreto |

`ShieldReport{Hash, ScannedAt, Findings, Skipped []string}`.

### Instinct

| Campo | Tipo | Validación / transición |
|---|---|---|
| `Slug` | `string` | `kind:` + clave normalizada, ≤ 80 caracteres |
| `Kind` | `string` | `cmd\|sequence\|correction` |
| `Description` | `string` | redactada, ≤ 200 caracteres |
| `Observations` | `int` | ≥ 1 |
| `Confidence` | `float64` | `InstinctConfidence(obs, corrections)` ∈ [0.3, 0.9] |
| `Scope` | `string` | `project` → `global` cuando `len(projects) >= 2` |
| `Status` | `string` | `proposed` → `confirmed` \| `rejected` (las transiciones son finales; `rejected` no vuelve a `proposed` aunque se observe de nuevo) |

## 2. Persistencia

### `mem.db` del proyecto: migración aditiva en `migrate()`

```sql
CREATE TABLE IF NOT EXISTS instinct_stats (
  project      TEXT NOT NULL,
  slug         TEXT NOT NULL,
  kind         TEXT NOT NULL,
  description  TEXT NOT NULL,
  observations INTEGER NOT NULL DEFAULT 0,
  corrections  INTEGER NOT NULL DEFAULT 0,
  confidence   REAL NOT NULL DEFAULT 0.3,
  scope        TEXT NOT NULL DEFAULT 'project',
  status       TEXT NOT NULL DEFAULT 'proposed',
  first_seen   TEXT NOT NULL,
  last_seen    TEXT NOT NULL,
  PRIMARY KEY (project, slug)
);
```

```sql
CREATE TABLE IF NOT EXISTS instinct_folded (
  project   TEXT NOT NULL,
  obs_id    TEXT NOT NULL,   -- UUIDv7 de la observación
  folded_at TEXT NOT NULL,
  PRIMARY KEY (project, obs_id)
);
```

`instinct_folded` hace el plegado idempotente por observación (research R18, H2). Se inserta en la misma transacción que `instinct_stats` y se purga a los 7 días.

La actualización se hace con `INSERT … ON CONFLICT(project, slug) DO UPDATE` y SQL parametrizado, dentro de una transacción por plegado. `mem sanitize` incluye `description` como columna con categoría propia (memorias 510 y 599).

### `DataHome()/instincts.db`: registro global

```sql
CREATE TABLE IF NOT EXISTS instinct_global (
  slug        TEXT PRIMARY KEY,
  kind        TEXT NOT NULL,
  description TEXT NOT NULL,
  projects    TEXT NOT NULL DEFAULT '[]',  -- JSON de ProjectKey
  confidence  REAL NOT NULL DEFAULT 0.3,
  updated_at  TEXT NOT NULL
);
```

Usa la misma apertura que los `mem.db` (WAL, `busy_timeout`) y permisos `0600`, como `HardenGlobalStore`.

### `hook_guard_events` (existente)

Las activaciones se registran con `recordGuard(deps, agent, kind, detail)` y `kind` ∈ {`expand.block`, `expand.warn`, `expand.suggest`, `expand.shield`, `expand.timeout`, `expand.error`}. La base se abre de forma perezosa, solo cuando hay algo que registrar (R5). `detail` lleva el nombre de la regla o la skill, nunca contenido del usuario. `mem expand status` las lee con `GuardSince(7)`.

### Archivos bajo `.memory/` (R6, R16)

Todos se escriben en modo `0600` y son best-effort: un fallo de escritura nunca bloquea un hook.

| Archivo | Dueño | Acceso |
|---|---|---|
| `.expand-episode-<sid8>`, `.expand-tools-<sid8>` | adaptador primario `hook_expand_state.go` | directo (estado del adaptador) |
| `.expand-rules-cache.json` | `adapters/secondary/rulesfs` | puerto `RuleSourcePort` |
| `expand-observations.ndjson` | `adapters/secondary/expandstate` | puerto `ObservationQueueRepository` |
| `.expand-shield-hash`, `expand-shield-last.json` | `adapters/secondary/expandstate` | puerto `ShieldStateRepository` |

### Observation (cola)

`{ID string (UUIDv7), T time, Kind "cmd"|"correction", Key string, Sample string}`. `Sample` está redactado y tiene como máximo 200 caracteres. Una línea ilegible se ignora al leer.

### Ajustes (`.memory/settings.json`, `ports.SettingsData`)

| Clave | Tipo | Por defecto |
|---|---|---|
| `expand_disabled` | bool | `false` |
| `expand_off` | []string | `[]` |
| `expand_compact_threshold` | int | `50` (se valida en el rango 10–1000; fuera de rango se usa 50) |

## 3. Relaciones

```text
hook payload ──► ExpandSignals ──► RouteSupertools ──► []SupertoolAction
                                                         │
             ┌───────────────┬───────────────┬───────────┴──────────┬───────────────┐
         rules.eval      shield.scan    instincts.*            suggest           resets
             │               │               │                     │
   Rule[] (caché) ─► RuleVerdict   ShieldReport   cola NDJSON → instinct_stats   SuggestionEpisode
                                                 ↘ instinct_global
                         └────────────── ExpandOutcome ──► renderExpandOutput(dialecto)
```
