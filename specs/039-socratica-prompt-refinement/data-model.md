# Data Model: Socratica (039)

Identificadores de código en inglés; narrativa en español (constitución §18).

## Dominio (`domain/socratica*.go`, sin dependencias de infraestructura)

### `SocraticaMode`
`refine` (`/socratica`) | `scrutiny` (`/socratic-q`).

### `SocraticaPhase` y transiciones

Fases **estables** (un punto seguro desde el que se puede reanudar): `captured`, `questions`, `review`, `approved`. `rewriting` es transitoria. `failed` no es terminal. Terminales: `dispatched`, `cancelled`.

```text
captured ──► questions ──► rewriting ──► review ──► approved ──► dispatched
   │  └────(0 preguntas)──────►│            │  ▲ edit      │
   │             │  (skip_rest)┘            │  └───────────┘ resume (approved sin ack ni reclamo → review, recovered)
   ▼             ▼             ▼            ▼
 failed ◄─── model_error ──── failed     cancelled
   │
   ├─ retry ─► captured (regenera preguntas)  si last_stable_phase = captured
   ├─ retry ─► rewriting (regenera reescritura) si last_stable_phase = questions
   ├─ send_original ─► approved (approved_prompt = original; elección explícita)
   └─ cancel ─► cancelled
cancel desde cualquier fase no terminal ─► cancelled
```

| Desde | Evento | Hacia | Guarda / efecto |
|---|---|---|---|
| captured | `questions_generated` (1–3) | questions | `last_stable_phase=questions` |
| captured | `questions_generated` (0) | rewriting | — |
| questions | `answer` / `skip` (quedan preguntas) | questions | — |
| questions | `answer` (última) / `skip` (última) / `skip_rest` | rewriting | — |
| rewriting | `rewrite_ready` | review | propuesta no vacía; `last_stable_phase=review`; `EffectRecordHistory` |
| review | `edit` | review | `revision`+1; `EffectRecordHistory` (actualiza) |
| review | `approve` | approved | asigna `dispatch_id` si no existe; `last_stable_phase=approved`; `EffectRecordHistory` |
| approved | `approve` | approved | **duplicado**: sin cambios, `Duplicate=true`, `Acked=false`, mismo `dispatch_id`. No prohíbe ejecutar: dirige al `ack`, que es la compuerta (I8) |
| dispatched | `approve` | dispatched | **duplicado**: sin cambios, `Duplicate=true`, `AlreadyDispatched=true` (I3) |
| dispatched | `force_resend` | approved | solo por elección explícita de la persona cuando la autorización se consumió pero la tarea no llegó a empezar (el receptor cayó tras el ack, I10). Nuevo `dispatch_id`, `dispatch_ack=false`, `ack_nonce` y reclamo vacíos. Exige que no haya otro flujo pendiente del mismo `(project, client)` (si lo hay: «cancela primero el pendiente») y que el texto no se haya redactado (modo privado: «vuelve a ejecutar /socratica») |
| approved | `ack` | dispatched | exige `dispatch_id` coincidente y `owner` coincidente (desde MCP `owner` va vacío y exige que no haya reclamo local); solo quien obtiene el ack ejecuta; si `private`: `EffectRedactFlow` (vacía los textos y conserva `dispatch_id`, `approved_hash` y `dispatch_consumed` hasta consumirse o hasta `TerminalRetention`, I2) |
| captured / rewriting | `model_error` | failed | guarda `last_error`; `last_stable_phase` sin cambios |
| failed | `retry` | captured o rewriting | según la tabla del diagrama; nunca otra fase |
| failed | `send_original` | approved | `approved_prompt = original_prompt`, asigna `dispatch_id`; `EffectRecordHistory` |
| no terminal | `cancel` | cancelled | borra `proposed_prompt`; `EffectRestoreOriginal`; si `private`: `EffectDeleteFlow` (sin `dispatch_id`, no hace falta conservar nada) |
| review (`recovered`) | `cancel` | cancelled | libera el único pendiente del agente para que un `start` nuevo cree otro flujo (G1) |
| approved (sin ack, **sin** reclamo) | `resume` | review | `recovered=true`; **nunca** reenvía (el lanzamiento nunca se reclamó, así que no pudo arrancar) |
| approved (sin ack, con reclamo y `DispatchLock` **retenido**) | `resume` | approved | `in_flight=true`: un `Execute` sigue en curso; `mark_dispatched`, `force_resend` y `cancel` se rechazan con «envío en curso» sin cambios (U2) |
| approved (sin ack, con reclamo y `DispatchLock` **libre**) | `resume` | approved | `uncertain=true`: el proceso propietario terminó o murió sin confirmar y el agente pudo haber arrancado. Solo admite `mark_dispatched`, `force_resend` o `cancel`; nunca vuelve a review ni reenvía solo (U2, U3) |
| approved (`uncertain`) | `mark_dispatched` | dispatched | la persona confirma que el envío ocurrió; `dispatch_ack=true`; si `private`: `EffectRedactFlow` |
| approved (`uncertain`) | `force_resend` | approved | elección explícita de reenviar: nuevo `dispatch_id` y reclamo vaciado. El `dispatch_id` anterior deja de valer, así que el ack tardío de un propietario anterior no afecta a ninguna fila |
| captured | `resume` | captured | `EffectGenerateQuestions` (idempotente) |
| rewriting | `resume` | rewriting | `EffectGenerateRewrite` (idempotente) |
| questions / review | `resume` | igual | sin efecto |

`Reduce(flow RefinementFlow, ev Event, now time.Time, newID func() string) (RefinementFlow, Effect, error)`: función pura. Devuelve `ErrInvalidTransition` ante cualquier transición no listada. `Effect` ∈ {`EffectNone`, `EffectRecordHistory`, `EffectGenerateQuestions`, `EffectGenerateRewrite`, `EffectRestoreOriginal`, `EffectDeleteFlow`, `EffectRedactFlow`}; `edit` actualiza también `proposed_prompt` del historial. El estado del lock (`LockHeld`) llega a `Reduce` como dato del evento, así que la función sigue siendo pura. La capa de aplicación ejecuta el efecto. `EffectGenerate*` solo llama a `SocraticGenerator` cuando el caso de uso se invoca con `OriginLocal` (CLI/TUI); con `OriginMCP` se traduce en una instrucción `next` para el agente activo y **nunca** lanza un subproceso (FR-009, U2, I3).

### Garantía de despacho (I10)

Tres hechos distintos, de los que el sistema solo garantiza los dos primeros:

1. **Autorización consumida** (`dispatch_ack=true`): ocurre una sola vez por `dispatch_id`. Es lo que significa `dispatched`.
2. **Entrega**: el único receptor que consumió la autorización recibe `acked=true`. Si pierde la respuesta, repite el `ack` con el mismo `ack_nonce` y la recibe de nuevo, sin consumir otra autorización.
3. **Ejecución**: gomemory no la observa y nunca la afirma.

Si el receptor cae después del ack, otra sesión ve `already_dispatched=true` y **no ejecuta**. La recuperación es `force_resend` desde dispatched, que crea una autorización nueva solo por elección explícita de la persona. Desde la CLI y la TUI el receptor es el agente lanzado: la autorización se consume en `onStarted`.

### `RefinementFlow`

| Campo | Tipo | Regla |
|---|---|---|
| `ID` | string (UUIDv7) | generado por puerto inyectado |
| `SchemaVersion` | int | `1` |
| `Project` | string | resuelto por `ProjectRepository` |
| `Client` | string | `claude` \| `codex` \| `opencode`. Un flujo creado desde CLI/TUI se guarda bajo el agente **destino** (`--client`, o el primer generador disponible si se omite); no existe el valor `cli` |
| `Mode` | SocraticaMode | — |
| `OriginalPrompt` | string | inmutable tras `captured`; depurado de `<private>` |
| `Questions` | []Question | 0–3 |
| `Answers` | []Answer | `{QuestionIndex, Choice \| Custom \| Skipped}` |
| `ProposedPrompt` | string | vacío fuera de review/approved |
| `Scrutiny` | *ScrutinySynthesis | solo modo `scrutiny`: premisas, supuestos (hechos vs. hipótesis), mejor argumento a favor, objeción más fuerte, alternativas, evidencia faltante, criterios de decisión |
| `ApprovedPrompt` | string | fijado en `approve` |
| `Phase` / `LastStablePhase` | SocraticaPhase | estable = captured, questions, review, approved (única definición: `SocraticaPhase.IsStable`) |
| `Revision` | int | +1 por edición |
| `DispatchID` | string? | único; índice único parcial |
| `DispatchAck` | bool | — |
| `ApprovedHash` | string | SHA-256 de `approved_prompt` tras `TrimSpace`; `ConsumeDispatch` compara contra él, así funciona aunque el texto se haya vaciado en modo privado |
| `DispatchClaimOwner` | string? | token único (UUIDv7) de la ejecución de `Execute` que tiene el reclamo; `Ack` y `ReleaseDispatch` exigen que coincida (U2) |
| `DispatchClaimedAt` | time? | momento del reclamo (informativo). No caduca: lo que decide si un reclamo sigue vivo es el `SocraticaDispatchLock` del flujo. Un reclamo sin ack con el lock libre es `uncertain` y exige conciliación explícita (U1, U2) |
| `AckNonce` | string? | valor aleatorio que envía quien hace el `ack`. Un `ack` repetido con el mismo nonce devuelve de nuevo `acked=true` (entrega repetible si se perdió la respuesta); con otro nonce o sin él, `already_dispatched=true` (I10) |
| `DispatchConsumed` | bool | `true` cuando el hook ya reconoció una vez el prompt aprobado como origen `socratica-approved` (H2) |
| `LastError` | string? | texto llano, sin secretos |
| `CreatedAt` / `UpdatedAt` | time | reloj inyectado |

### `Question`
`{Text string, Options []string /*2–4*/}`. «Otra», «Omitir» y «Omitir el resto» son acciones del sistema, no opciones del modelo.

### `RefinementHistory`

| Campo | Regla |
|---|---|
| `ID`, `Project`, `Client`, `Mode` | — |
| `OriginalPrompt`, `ProposedPrompt`, `ApprovedPrompt?` | depurados. `ProposedPrompt` es la última propuesta de revisión (se actualiza en `edit`), para que `history --show` y la TUI la muestren aunque el flujo se haya cancelado y purgado (I6) |
| `Scrutiny?` | copia de la síntesis (modo `scrutiny`), depurada |
| `Additions []string` | adiciones sustantivas declaradas por la reescritura |
| `WordsBefore`, `WordsAfter` | `strings.Fields` |
| `CompletedAt` | — |

Se registra en `review` (primera vez que se alcanza; las ediciones lo actualizan) y en `approved`. No se registra si `socratica_private=true` ni para pasos sin captura. Tras insertar, se podan las filas más antiguas por encima de `socratica_history_limit` (por defecto 20) para el mismo `(project, client)`, dentro de la misma transacción.

### `InterceptionInput` → `InterceptionDecision` (función pura `DecideInterception`)

Entrada: `{Enabled, AgentIdle bool; Text string; HasAttachments, HasImages bool; Origin string; PendingFlowPhase SocraticaPhase}`. `Origin="socratica-approved"` **solo** cuando se cumplen las tres condiciones: el entorno trae `GOMEMORY_SOCRATICA_DISPATCH` con un `dispatch_id` conocido, el texto coincide (tras `TrimSpace`) con `approved_prompt` y `DispatchConsumed=false`. Al reconocerlo, el hook marca `DispatchConsumed=true`. Los prompts posteriores de la misma sesión se evalúan con normalidad (research R4, H2). `PendingFlowPhase` solo cuenta como continuación si el flujo pendiente se actualizó hace menos de `ContinuationWindow` = 30 min (M1).
Salida: `{Capture bool; Forward string; Reason string}`. Orden de evaluación: deshabilitada → origen `socratica-approved` → agente ocupado → adjuntos/imágenes → `/` inicial → `>` inicial (`Forward` = texto sin exactamente un `>`) → flujo pendiente → baja información (≤ `LowInfoMaxWords` = 3 palabras, constante común) → captura.

### `SocraticaCapabilities`
Tabla por agente `{NamedCommand, Interception, Clipboard, InputRestore, Pane bool}`. Solo se pone `true` lo verificado según `contracts/compatibility-matrix.md`.

## Persistencia (`mem.db`, migración aditiva en `migrate()`)

```sql
CREATE TABLE IF NOT EXISTS socratica_flows (
  id TEXT PRIMARY KEY,
  schema_version INTEGER NOT NULL DEFAULT 1,
  project TEXT NOT NULL,
  client TEXT NOT NULL,
  mode TEXT NOT NULL,
  original_prompt TEXT NOT NULL,
  questions_json TEXT NOT NULL DEFAULT '[]',
  answers_json TEXT NOT NULL DEFAULT '[]',
  proposed_prompt TEXT NOT NULL DEFAULT '',
  approved_prompt TEXT NOT NULL DEFAULT '',
  scrutiny_json TEXT,
  phase TEXT NOT NULL,
  last_stable_phase TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 0,
  dispatch_id TEXT,
  dispatch_ack INTEGER NOT NULL DEFAULT 0,
  dispatch_consumed INTEGER NOT NULL DEFAULT 0,
  dispatch_claimed_at TEXT,
  dispatch_claim_owner TEXT,
  ack_nonce TEXT,
  approved_hash TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_socratica_dispatch ON socratica_flows(dispatch_id) WHERE dispatch_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_socratica_flows_scope ON socratica_flows(project, client, updated_at);
-- Un solo flujo no terminal por (project, client) (F2):
CREATE UNIQUE INDEX IF NOT EXISTS idx_socratica_one_pending ON socratica_flows(project, client) WHERE phase NOT IN ('dispatched','cancelled');

CREATE TABLE IF NOT EXISTS socratica_history (
  id TEXT PRIMARY KEY,
  flow_id TEXT NOT NULL UNIQUE,
  project TEXT NOT NULL,
  client TEXT NOT NULL,
  mode TEXT NOT NULL,
  original_prompt TEXT NOT NULL,
  proposed_prompt TEXT NOT NULL DEFAULT '',
  approved_prompt TEXT,
  scrutiny_json TEXT,
  additions_json TEXT NOT NULL DEFAULT '[]',
  words_before INTEGER NOT NULL,
  words_after INTEGER NOT NULL,
  completed_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_socratica_history_scope ON socratica_history(project, client, completed_at DESC);
```

Las columnas de texto de ambas tablas (`original_prompt`, `proposed_prompt`, `approved_prompt`, `questions_json`, `answers_json`, `scrutiny_json`, `last_error`, `additions_json`) quedan cubiertas por `mem sanitize` mediante un lector propio, `readSocraticaCandidates`, con una categoría por columna (research R9). **No** se usa `ledgerColumnFor`.

## Puertos (`application/ports/`)

- `SocraticaRepository`: `CreateFlow`, `GetFlow`, `ActiveFlow(project, client)`, `SaveTransition(ctx, before, after RefinementFlow) error` (control optimista por `revision`/`phase`; error `ErrConcurrentUpdate`), `Approve(ctx, id, dispatchID, approvedPrompt string) (flow, duplicate bool, err)` (`UPDATE … SET phase='approved', approved_prompt=?, approved_hash=? WHERE id=? AND phase IN ('review','failed')`; `failed` solo llega aquí por `send_original`, ya validado por `Reduce`, I1), `Ack(ctx, id, dispatchID, owner, nonce) (acked, alreadyDispatched bool, err)` (`UPDATE … SET dispatch_ack=1, ack_nonce=? WHERE id=? AND dispatch_id=? AND dispatch_claim_owner IS ? AND dispatch_ack=0`, con `owner` NULL desde MCP; si afecta 0 filas y el `ack_nonce` guardado coincide con `nonce`, es una repetición y devuelve `acked=true`; si no, `alreadyDispatched=true` o `ErrAckRejected`; dos ack concurrentes con nonces distintos: gana uno), `MarkDispatched(ctx, id, dispatchID) error`, `ClaimDispatch(ctx, id, dispatchID, owner, now) (claimed bool, err)` (`UPDATE … SET dispatch_claimed_at=?, dispatch_claim_owner=? WHERE id=? AND dispatch_id=? AND phase='approved' AND dispatch_ack=0 AND dispatch_claim_owner IS NULL`; solo quien afecta 1 fila lanza; un reclamo existente **no** se vuelve a tomar: su vigencia la decide el `SocraticaDispatchLock`, U1/U2), `ReleaseDispatch(ctx, id, owner) error` (`… WHERE id=? AND dispatch_claim_owner=?`; nunca libera un reclamo ajeno), `ForceResend(ctx, id, oldDispatchID, newDispatchID) error` (vacía el reclamo y cambia `dispatch_id` en una sentencia condicionada al id anterior), `DeleteFlow(ctx, id) error`, `FindByDispatchID(ctx, dispatchID) (flow, found bool, err)`, `ConsumeDispatch(ctx, dispatchID, text) (matched bool, err)` (compara `sha256(TrimSpace(text))` con `approved_hash`), `RedactFlow(ctx, id)` (vacía los textos y conserva `dispatch_id`, `approved_hash` y `dispatch_consumed`, I1), `UpsertHistory`, `ListHistory(project, client, limit)`, `PurgeStale(project, olderThan) (int, error)`. `PurgeTerminal(project, olderThan) (int, error)` borra dispatched/cancelled con más de `TerminalRetention` (24 h); al terminar un flujo se podan los terminales por encima de `socratica_history_limit` para ese (project, client) (G2). `PurgeStale` borra solo flujos en captured, questions, rewriting, review o failed. **Nunca** borra `approved` sin ack, que debe seguir recuperable (FR-014, M2).
- `SocraticaUnitOfWork` (constitución §5, mismo patrón que `PrivacyMaintenanceUnitOfWork`): `WithTransaction(ctx, fn func(SocraticaRepository) error) error`. Confirma solo si `fn` termina sin error y hace rollback completo en otro caso. El caso de uso aplica cada evento dentro de **una** transacción: `SaveTransition` + `UpsertHistory`/poda o `DeleteFlow`. Ninguna transacción abarca un proceso externo ni una llamada al generador. Toda consulta del repositorio se ejecuta con `context.WithTimeout(ctx, socratica_query_timeout_ms)` (constitución §5, C2).
- `AgentLauncher` (constitución §3, C2): `Launch(ctx, req LaunchRequest, onStarted func() error) error`, **bloqueante**. Llama a `onStarted` justo después de un arranque correcto y luego espera a que el proceso termine. Si el arranque falla, devuelve un error que envuelve `ErrLaunchFailed` sin llamar a `onStarted`. Si `onStarted` devuelve error, **no** mata el proceso (la tarea puede estar ya en curso): sigue esperando y al terminar devuelve un error que envuelve `ErrConfirmFailed` (U3). Implementaciones: `ExecLauncher` (`cmd.Start()` → `onStarted` → `cmd.Wait()`, stdio heredado) y `PrintLauncher` (escribe → `onStarted`). En la TUI, `ExecLauncher` se envuelve en un `tea.ExecCommand` y se ejecuta con `tea.ExecProcess`, que cede la terminal mientras el agente corre (U1).
- `SocraticaDispatchLock` (U2): `TryAcquire(flowID) (release func(), acquired bool, err error)`. Es un lock exclusivo de archivo por flujo en `.memory/socratica/locks/<flow_id>.lock` (flock en Unix, LockFileEx en Windows, igual que `codegraph/refresh_lock_*`). Se libera con `release` o cuando muere el proceso. No añade dependencias.
- `SocraticGenerator`: `Name() string`, `Available() bool`, `GenerateQuestions(ctx, req) (GenerationResult, error)`, `Rewrite(ctx, req) (RewriteResult, error)`. La composición «primero disponible» es un adaptador que recibe la lista ya ordenada del composition root (constitución §4).

## Caso de uso `SocraticaFlow` (application/usecases)

- `Apply(ctx, flowID, Event, origin Origin)`: una transacción por evento. `origin` ∈ {`OriginMCP`, `OriginLocal`} decide si `EffectGenerate*` llama al generador (solo `OriginLocal`) o devuelve `next` (I3).
- `Execute(ctx, flowID, target string, launcher AgentLauncher) (ExecuteResult, error)`: (1) `approve` en una transacción (si es duplicado y ya tiene ack, devuelve «ya enviado» sin lanzar); (0) `SocraticaDispatchLock.TryAcquire(flowID)`: si está retenido devuelve «envío en curso» sin lanzar. El lock se mantiene durante **todo** `Execute` (reclamo, `Launch`, `onStarted`, `Wait` y el reintento del ack) y se libera al final. (1b) `ClaimDispatch` con un `owner` nuevo: si ya existe un reclamo (con el lock libre, es decir, su propietario murió) devuelve `ErrDispatchUncertain` sin lanzar. `force_resend`, `mark_dispatched` y `cancel` sobre un flujo reclamado exigen el mismo lock, así que no pueden invalidar el reclamo entre la comprobación y el arranque. Un propietario pausado conserva el lock y bloquea esas acciones; uno muerto ya no puede lanzar. Dos `Execute` concurrentes producen un solo arranque (U1, U2); (2) `launcher.Launch` con `GOMEMORY_SOCRATICA_DISPATCH=<dispatch_id>`, fuera de cualquier transacción; (3) `ack` en otra transacción **dentro de `onStarted`**, es decir, en cuanto el agente arrancó y antes de que la persona interactúe con él. Si el arranque falla (`ErrLaunchFailed`), no hay ack y se llama a `ReleaseDispatch(owner)`, para que un reintento pueda reclamar. Si el arranque tuvo éxito pero el ack falla (`ErrConfirmFailed`), el reclamo **no** se libera: tras `Wait` se reintenta el ack una vez con el mismo `owner`; si vuelve a fallar, devuelve `ErrDispatchUnconfirmed` y el flujo queda `uncertain` hasta que la persona elija `mark_dispatched`, `force_resend` o `cancel` (U3). Ningún camino reenvía de forma automática. La CLI y la TUI solo llaman a `Execute`; ninguna orquesta los pasos.
- `Resume(ctx, flowID)`: aplica el evento `resume`.
- `ClassifyPrompt(ctx, project, client, text, env) PromptClassification{SkipProvenance, Capture, Forward, Directive}`: única entrada de los hooks (`user-prompt-submit`, `mem hook prompt`, `mem hook socratica-decide`). Combina `IsSocraticaInvocation`, flujo pendiente vigente, `ConsumeDispatch` y `DecideInterception`; el handler no contiene reglas (constitución §20).
- `PurgeStale(ctx, project)`.
- Se reutilizan `Clock`, el generador de IDs y `SettingsRepository`.

## Ajustes (`.memory/settings.json`)

| Clave | Tipo | Ausente |
|---|---|---|
| `socratica_interception` | `map[string]bool` | todos desactivados. El hook captura solo si `interception[agente]` y (`Capabilities[agente].Interception` o `interception_unverified[agente]`) |
| `socratica_interception_unverified` | `map[string]bool` | vacío; marca activaciones hechas con `--unverified` |
| `socratica_history_limit` | int | 20 |
| `socratica_private` | bool | false |
| `socratica_generators` | []string | `["claude","codex"]` (orden de preferencia; `opencode` se ignora hasta tener evidencia de un modo de solo lectura, research R3) |
| `socratica_generator_timeout_seconds` | int | 120 |
| `socratica_query_timeout_ms` | int | 5000 (timeout por consulta del repositorio vía `context.WithTimeout`, constitución §5) |

Validación (constitución §9, falla rápido): al cargar los ajustes, un valor fuera de rango (`history_limit` < 1 o > 1000, `timeout` < 1, `query_timeout_ms` < 100, generador desconocido, agente desconocido en los mapas) produce `ErrInvalidSocraticaSettings` con la clave y el valor. CLI y TUI salen con código 1 y el mensaje; MCP devuelve error. Los hooks nunca bloquean el turno: tratan la intercepción como apagada y escriben el error por stderr. Ausente = valor por defecto documentado; presente e inválido nunca se sustituye por el default (C1).
