# Research: Socratica (039)

Evidencia recogida el 2026-10-15 contra el repositorio en `ccd0e7e` y los binarios instalados: Claude Code 2.1.300, codex-cli 0.162.1 y OpenCode v2.0.27.

## R1. Fuente canónica única de la habilidad

- **Decision**: La fuente canónica es `infrastructure/templates/socratica/SKILL.md`, que entra al binario mediante el `go:embed all:templates` existente (`infrastructure/main.go:34`). `skills/socratica/SKILL.md` es la copia publicada en el repositorio para `skills-ref validate` y lectura humana. Una prueba de contrato nueva exige igualdad byte a byte entre ambas. La misma prueba cubre también `adversarial-consensus-review` y `defensive-qa-audit`.
- **Rationale**: `skills/defensive-qa-audit/SKILL.md` y `infrastructure/templates/defensive-qa-audit/SKILL.md` hoy son idénticos (`diff -q` sin salida), pero ninguna prueba lo exige. Es una deriva latente y esta feature la activaría con una tercera copia, así que se cierra en el mismo cambio (regla 7 del proyecto). `go:embed` no admite rutas fuera del paquete ni sigue enlaces simbólicos, por lo que no basta con apuntar a `skills/`.
- **Alternatives considered**: (a) Enlace simbólico `skills/socratica → templates`: `go:embed` no lo sigue y Windows lo rompe. (b) Generar `skills/` en `go generate`: añade un paso que se puede olvidar y no lo verifica CI. (c) Tener solo `templates/`: incumple la ruta canónica `skills/socratica/SKILL.md` que fija el borrador.

**Cuerpo de la habilidad**: se toma como base el texto «Socratica — portable prompt refinement and critical questioning» aportado por la persona (2026-10-15), traducido al español. Se le añaden solo dos piezas propias de gomemory: `socratica_flow` como compuerta cuando el servidor está conectado, y la prohibición de guardar prompts con `save_memory`. Ver `contracts/skill.md`.

## R2. Distribución e invocación por agente

| Agente | Habilidad | `/socratica` | `/socratic-q` | Evidencia |
|---|---|---|---|---|
| Claude Code | `~/.claude/skills/socratica/SKILL.md` | nativo: toda habilidad es invocable como `/<name>` | envoltorio de comando `~/.claude/commands/socratic-q.md` (fino, delega en la habilidad) | `skillTargets` (`agent_skills_setup.go:30`); verificado con el binario instalado |
| Codex | `~/.codex/skills/socratica/SKILL.md` | invocación explícita de la habilidad (`$socratica …`) o lenguaje natural | lenguaje natural o `$socratica socratic-q …` | binario contiene `.codex/skills`; `~/.codex/prompts` no existe → no se asume comando con nombre hasta verificarlo (T de verificación) |
| OpenCode | `~/.opencode/skills/socratica/SKILL.md` | `~/.config/opencode/commands/socratica.md` | `~/.config/opencode/commands/socratic-q.md` | el directorio `commands/` ya lo usa `atomic-decomposition.md` (`globalTargets`) |

- **Decision**: La habilidad se distribuye con `InstallAgentSkill` a los tres agentes (único dueño, sin filtro), igual que `defensive-qa-audit`. Los envoltorios de comando son archivos de dos o tres líneas que solo nombran la habilidad y el modo, y nunca copian su cuerpo. Se registran `socratica` en `globalSkillNames` y los envoltorios en `GlobalGeneratedArtifacts` para la desinstalación (memoria 501: si una habilidad falta en `globalSkillNames`, `TestUninstall_SistemaSinRastros` falla).
- **Rationale**: Se reutiliza el canal ya probado, idempotente y que no crea directorios de agentes ausentes (FR-023). Los envoltorios no duplican la lógica.
- **Alternatives considered**: Una segunda habilidad `socratic-q` con el cuerpo duplicado se descarta porque deriva. Registrar comandos vía MCP se descarta porque un servidor MCP no puede registrar comandos en todos los clientes (lo dice el propio borrador).

## R3. Generación sin credenciales nuevas (aclaración «Ambas con detección»)

- **Decision**: Dentro de un agente, el propio modelo del agente genera preguntas y reescritura siguiendo la habilidad. Fuera de un agente (`mem socratica refine`, TUI), un port `SocraticGenerator` con un adaptador por agente detectado en `PATH`, en este orden de preferencia configurable:
  - Claude: `claude -p --output-format json --json-schema <schema> --tools ""` (sin herramientas, así no puede ejecutar la tarea).
  - Codex: `codex exec --ephemeral --sandbox read-only --skip-git-repo-check --output-schema <file> -`.
  - OpenCode: **no soportado como generador en esta iteración.** `opencode run --help` (v2.0.27) no ofrece ningún modo sin herramientas ni de solo lectura, y sin él la generación podría ejecutar la tarea (FR-005). La tarea T066 comprueba si un agente de solo lectura definido por la persona (`--agent`) lo hace posible; hasta entonces la matriz lo marca «no soportado».
  - Ninguno disponible: error `ErrNoGenerator` con un mensaje en lenguaje llano; el original se conserva.
- **Selección (constitución §4)**: `infrastructure/container.go` construye la lista ordenada de adaptadores concretos según `socratica_generators` y la pasa a `generator.FirstAvailable([]ports.SocraticGenerator)`. Este adaptador compuesto solo recorre la lista llamando a `Available()` y no conoce los adaptadores concretos. Cada adaptador concreto decide su propia disponibilidad con `exec.LookPath`. El timeout sale del ajuste `socratica_generator_timeout_seconds` (0 = 120 s).
- **Rationale**: Reutiliza credenciales y modelo ya configurados en cada agente (FR-009). Los modos sin herramientas o de solo lectura impiden que la generación ejecute trabajo (FR-005). La salida estructurada está verificada en `--help` de los binarios instalados.
- **Alternatives considered**: Llamar directamente a una API de modelo exige credenciales nuevas y se descarta. Solo gestionar estado fue rechazado en la aclaración.

## R4. «Ejecutar» y despacho sin duplicados

- **Decision**: Dentro de un agente, «Ejecutar» llama a la herramienta MCP `socratica_flow` con `action=approve`. Esa llamada, en una sola transacción, pasa la fase a APPROVED y asigna un `dispatch_id` UUIDv7 (reloj y generador inyectados). Si ya existía, la respuesta devuelve `duplicate=true` y la habilidad prohíbe ejecutar de nuevo. Tras ejecutar, el agente confirma con `action=ack`, que lleva la fase a DISPATCHED. Desde la línea de comandos o la TUI, «Ejecutar» lo orquesta el caso de uso `SocraticaFlow.Execute` a través del port `AgentLauncher` (constitución §3: la CLI y la TUI no contienen esta lógica), en **tres pasos separados, sin transacción que abarque el proceso externo**: (1) `approve` en su propia transacción; (2) arranque del agente elegido con el prompt aprobado como mensaje inicial (`claude "<p>"`, `codex "<p>"`, `opencode --prompt "<p>"`) y la variable de entorno `GOMEMORY_SOCRATICA_DISPATCH=<dispatch_id>`; (3) `ack` en su propia transacción **solo si el proceso arrancó**, mediante el callback `onStarted` que `ExecLauncher` invoca tras `cmd.Start()`; después espera con `cmd.Wait()` para que `mem` no compita por la terminal. En la TUI se usa `tea.ExecProcess`, que suspende Bubble Tea mientras el agente corre (U1). Si el arranque falla, el flujo queda `approved` sin ack y vuelve como revisión recuperable. Con `--print`: `approve`, escritura del prompt por stdout y `ack` tras escribirlo.
- **Marca de origen (U1)**: el hook `user-prompt-submit` y el plugin de OpenCode heredan el entorno del agente. Si `GOMEMORY_SOCRATICA_DISPATCH` está definida, el hook llama a `ConsumeDispatch(dispatch_id, texto)`. Este solo devuelve `matched=true` si el flujo está en approved o dispatched, el texto coincide con `approved_prompt` y la marca no se ha consumido; en ese caso la marca se consume en la misma transacción. Solo entonces `DecideInterception` recibe `Origin="socratica-approved"`. Así la variable, que vive toda la sesión, no desactiva la intercepción de los prompts posteriores (H2). Dentro del agente no hay reenvío: el modelo ejecuta el prompt aprobado en el mismo turno, sin pasar por el hook.
- **Rationale**: La atomicidad se garantiza en SQLite con `UPDATE … WHERE phase IN ('review','failed')` (`failed` solo por `send_original`). Una respuesta con 0 filas afectadas es un duplicado. Entre approve y el arranque, `ClaimDispatch` reserva el despacho de forma exclusiva (protegido por un lock exclusivo de archivo por flujo que se retiene durante todo el despacho y libera el sistema operativo si el proceso muere), para que dos Ejecutar concurrentes produzcan un solo arranque. Sin confirmación de envío no hay reenvío automático (FR-014).
- **Alternatives considered**: Deduplicar por hash del texto se descarta porque dos aprobaciones legítimas del mismo texto son flujos distintos.

## R5. Intercepción automática por agente

| Agente | Gancho | Mecanismo de captura | Estado |
|---|---|---|---|
| Claude Code | `UserPromptSubmit` (ya instalado, `claude_code_setup.go:81`) | `hookSpecificOutput.additionalContext` con la orden de aplicar Socratica al prompt y no ejecutar la tarea | soportable; se verifica en sesión real |
| Codex | `UserPromptSubmit` (ya instalado, `codex_setup.go:49`, dialecto JSON) | mismo sobre `additionalContext` (`promptHookJSON`) | soportable; se verifica en sesión real |
| OpenCode | `chat.message` del plugin (`gomemory.ts:219`) | anteponer una parte de texto con la orden; `>` se elimina mutando la parte | soportable; se verifica en sesión real |

- **Decision**: La intercepción no reescribe ni bloquea el envío. Inyecta una directiva de contexto, de modo que el prompt original queda íntegro en la conversación y el agente entra en el flujo de la habilidad. La decisión de capturar o dejar pasar es una función pura `domain.DecideInterception(input) Decision`. El hook solo recoge las señales (texto, adjuntos, si hay un flujo pendiente en REVIEW o QUESTIONS para ese agente, el origen) y emite la decisión. La activación por agente solo se permite si `domain.SocraticaCapabilities[agent].Interception` es verdadero. Ese valor se pone en verdadero únicamente tras la verificación en sesión real que exige FR-037.
- **Señales de exclusión**: comando con barra (`/` inicial), adjuntos o imágenes (campos del payload; si el agente no los expone, se usa el marcador de imagen del texto y, en caso de duda, se deja pasar), flujo pendiente del mismo agente (respuesta de continuación), respuesta de baja información (≤ 3 palabras o coincidencia con opciones o acciones de Socratica), marca de origen `socratica-approved`, y `>` inicial. En OpenCode se elimina exactamente uno mutando la parte (**garantizado**). En Claude y Codex el hook no puede mutar el prompt, así que se inyecta la directiva «trata el mensaje sin el `>` inicial» (**por directiva**, y la matriz lo distingue). Umbral de baja información: ≤ 3 palabras, común a todos los agentes (constante de dominio); solo se ajustaría por agente con evidencia.
- **Procedencia (G1)**: el hook existente `user-prompt-submit` guarda cada prompt con `SetLastPrompt` (`cmd_hook.go:557`) y `InsertMemory` lo adjunta como origen de lo que se guarde ese turno; OpenCode hace lo mismo con `mem hook prompt`. Para no filtrar prompts de Socratica a las memorias generales (FR-026), ambos llaman antes a `SocraticaFlow.ClassifyPrompt`, que marca `SkipProvenance` en invocaciones explícitas (`/socratica`, `/socratic-q`, `$socratica`), en respuestas a un flujo pendiente vigente y en prompts capturados. Los prompts normales conservan su comportamiento.
- **OpenCode (M4)**: el plugin no replica las reglas en TypeScript. Llama a `mem hook socratica-decide` con el payload (texto, adjuntos, entorno) por el mismo `spawn` que ya usa, y aplica la decisión JSON `{capture, forward, directive}`. `DecideInterception` sigue siendo la única fuente de verdad.
- **Continuación (M1)**: un flujo pendiente solo cuenta como continuación si se actualizó hace menos de 30 min (`ContinuationWindow`). `mem socratica status` lista el pendiente para que la persona pueda cancelarlo.
- **Rationale**: Bloquear el prompt en Claude lo borra de la entrada, lo que viola «nunca perder texto». La inyección de contexto funciona igual en los tres agentes y ya está probada por el recordatorio de guardado.
- **Alternatives considered**: `decision: block` se descarta por la pérdida del texto. Reescribir el prompt no es posible en `UserPromptSubmit` de Claude ni de Codex.

## R6. Contrato de preguntas y reparación de payload

- **Decision**: JSON Schema `contracts/generation.schema.json` (preguntas 0–3; opciones 2–4; «Otra» y «Omitir» los añade el sistema, no el modelo). Parser en `domain`: si hay más de 3 preguntas, se recorta a 3 conservando el orden. Si una pregunta tiene más de 4 opciones, se recorta a 4. Si tiene menos de 2, la pregunta se descarta. Si el JSON está roto, se intenta extraer el primer objeto JSON balanceado; si falla, se devuelve un error y se ofrece reintentar.
- **Rationale**: Nunca se muestran controles inválidos (Edge Cases) y el comportamiento es determinista y comprobable con pruebas de tabla.

## R7. Almacenamiento

- **Decision**: Dos tablas nuevas, `socratica_flows` y `socratica_history`, en `mem.db` por proyecto, añadidas a `migrate()` con `CREATE TABLE IF NOT EXISTS`. La política del repositorio es migración solo aditiva (`db.go:161`). Se sigue esa convención y no se crean archivos `V00N__`, porque la constitución (§6) manda usar la convención del runner vigente. `schema_version` va como columna de fila para versionar el JSON de preguntas y respuestas.
- **Rationale**: Las tablas quedan en un espacio propio, separado de `memories` (FR-025/026), y nada se escribe en los directorios de habilidades.

## R8. Ajustes

- **Decision**: Campos nuevos en `ports.SettingsData` **y** en el `Settings` de persistencia: `socratica_interception map[string]bool` (vacío = todos desactivados), `socratica_history_limit int` (0 = 20), `socratica_interception_unverified map[string]bool` (activaciones hechas con `--unverified`), `socratica_private bool` (por proyecto, común a todos los agentes), `socratica_generators []string` y `socratica_generator_timeout_seconds int` (0 = 120).
- **Rationale**: El comentario de `SettingsData` advierte que `Write` reconstruye todo desde esa struct, así que un campo que no esté ahí se pierde en silencio (incidente de la feature 028).
- **Flujos pendientes (F2)**: como máximo **uno** por proyecto y agente. `start` con otro pendiente lo retoma (`recovered=true`), así que no hace falta límite configurable. Los no terminales con más de 7 días (`PendingMaxAge`) se purgan en `mem gc`, **excepto** `approved` sin ack, que sigue recuperable (FR-014). Los terminales (dispatched, cancelled) se podan a `socratica_history_limit` por (project, client) al terminar y `mem gc` borra los de más de 24 h (`TerminalRetention`); se conservan ese tiempo para la detección de duplicados y `ConsumeDispatch` (G2). Ajustes inválidos: falla rápida con `ErrInvalidSocraticaSettings` en CLI, TUI y MCP (constitución §9); los hooks no bloquean y tratan la intercepción como apagada. Timeout por consulta configurable `socratica_query_timeout_ms` (constitución §5).

## R9. Privacidad

- **Decision**: Toda escritura pasa por `domain.ProtectText` / `RedactPrivate`, y toda lectura también (lección H-01, memoria 510: depurar al leer además de al escribir). `mem sanitize` **no** puede reutilizar `ledgerColumns`: ese mecanismo asume ids `int64`, alcance por `reviews.project`, una `SanitizeCategory` por columna y listas JSON de strings. Las tablas de Socratica tienen id TEXT, columna `project` propia y JSON de objetos. Se añade un lector propio, `readSocraticaCandidates`, con su `Apply` (`UPDATE … WHERE id=? AND <col>=? AND project=?`). `domain.SanitizeCandidate` ya tiene `RecordID string`. Como `sanitizeFields` asocia **un campo por categoría** (`domain/privacy_maintenance.go`), se añade una categoría por columna: `socratica_flow_{original,proposed,approved,questions,answers,error,scrutiny}` y `socratica_history_{original,approved,additions,scrutiny}`. Los valores string dentro de los JSON de objetos se depuran de forma recursiva. Con `socratica_private=true` no se escribe historial, y la fila del flujo se borra al llegar a un estado terminal (dispatched o cancelled).
- **Rationale**: Se evita reabrir la familia de fugas cerrada en las features 037 y la auditoría de 2026-10-12.

## R10. Presentación nativa en el anfitrión

- **Decision**: Esta iteración no implementa paneles nativos dentro de los agentes. La presentación enriquecida es la TUI de `mem` (Bubble Tea, `adapters/primary/tui`). Los paneles de Claude Code (mods) se registran como trabajo posterior y opcional, condicionado a inspeccionar sus tipos reales. La matriz marca «panel: no — flujo de texto» en los tres agentes.
- **Rationale**: FR-010 prohíbe prometer controles que no puedan funcionar y el borrador exige inspeccionar los tipos TSX reales antes de escribir un panel. Dejarlo fuera mantiene el MVP verificable.

## R11. Octopus y Speckit

- **Decision**: Octopus no forma parte del flujo. La habilidad solo menciona que, si el agente consulta `octopus_route_task` antes de delegar la generación, debe respetar su ruta. No hay código nuevo. Speckit: la habilidad ofrece la acción explícita «Convertir en entrada de especificación», que muestra el texto listo para `/speckit-specify` y no lo ejecuta.
- **Rationale**: FR-008 y FR-030 se cumplen sin dependencias nuevas.
