# Research: Integridad de los hooks, avisos por conversación y compresión que no destruye

**Feature**: 035-hook-integrity-compression · **Fecha**: 2026-10-04

Todas las decisiones parten de evidencia medida sobre el binario instalado (v2.27.1) y del código vigente. No quedan `NEEDS CLARIFICATION`.

## R1. Identidad de conversación: qué señal usar

- **Decisión**: la identidad de la conversación es el `session_id` que el host entrega en el payload de `session-start`. Una conversación nueva = un `session_id` distinto del guardado en `.memory/.conversation`. Si el payload no trae `session_id`, se genera uno local y se trata como conversación nueva.
- **Rationale**:
  - Claude Code y Codex ya enrutan el arranque (`startup|resume|clear`) a `mem hook session-start` y la compactación (`compact`) a `mem hook post-compact` mediante matchers (verificado en `~/.codex/config.toml` y `.claude/settings.json`). Por eso `session-start` nunca ve una compactación.
  - Una reanudación (`resume`) conserva el mismo `session_id`, y con este criterio no reinicia el estado: es la misma conversación que continúa.
  - OpenCode emite `session.created` con `info.id`.
- **Alternativas descartadas**:
  - Leer el campo `source` del payload: redundante con los matchers, y no todos los hosts lo envían.
  - Reiniciar en cada `session-start` sin comparar el id: una reanudación perdería su estado legítimo.

## R2. Rotación de la sesión de memoria

- **Decisión**: al iniciar una conversación, si la sesión activa lleva más de 4 h sin actividad, se cierra con el resumen «cerrada automáticamente por inactividad» y su respaldo, y se abre otra. La actividad es `MAX(created_at)` de las memorias de esa sesión, incluidos los checkpoints; si no hay ninguna, el `created_at` de la sesión.
- **Rationale**:
  - Codex no tiene hook de fin de sesión y OpenCode solo cierra en `dispose`.
  - `SecondsSinceLastSave` excluye a propósito los checkpoints (mide guardados reales para el recordatorio), así que no sirve como medida de actividad.
  - La tabla `memories` ya guarda `session_id`; basta una consulta nueva.
- **Implementación mínima**: un puerto nuevo `SessionActivityReader` (`LastActivity(sessionID) (string, bool, error)`) implementado por el repositorio de sesiones de persistencia. No se amplía `SessionRepository`, para no obligar a cambiar sus mocks.
- **Alternativas descartadas**:
  - Rotar cuando cambia el id de conversación: Claude y Codex trabajando a la vez en el proyecto se cerrarían la sesión el uno al otro.
  - Añadir una columna `last_activity_at` a `sessions`: obliga a escribirla en cada turno y requiere una migración, sin ninguna ventaja frente a la consulta.

## R3. Presupuesto de salida de los hooks

- **Decisión**: una función pura de dominio `domain.FitHookOutput(sections []HookSection, maxRunes int) (texto string, recortes []Recorte)`. Las secciones llevan prioridad y una marca de recortable. Se arma por prioridad; lo recortable que no cabe se trunca por párrafos y termina con «… contexto recortado: llama get_context() para el resto». El tope es `domain.HookInlineContextMaxChars` (10 000 runas), medido sobre el texto de `additionalContext` (el sobre JSON no cuenta, porque el host aplica el tope al contexto).
- **Prioridades**: carga de tools (100, no recortable) > protocolo (90, no recortable) > política Octopus (80) > documento de plan (70) > memoria del proyecto (50) > avisos menores (40).
- **Rationale**:
  - Medido: session-start emitió 17 777 caracteres y el primer prompt 12 944; Claude mostró al modelo solo 2 KB.
  - `entregaContextoDeArranque` ya detectaba el exceso pero emitía igual.
  - Al ser pura, cumple la constitución (dominio sin I/O) y se prueba en tablas.
- **Invariante probada**: la suma de las secciones no recortables es ≤ 4 000 runas, de modo que siempre cabe con margen.
- **Alternativas descartadas**:
  - Comprimir más el contexto: no garantiza el tope.
  - Mover la memoria a una tool: ya existe (`get_context`); el recorte simplemente apunta a ella.

## R4. Doble registro de hooks entre ámbitos

- **Decisión** (aclaración Q1):
  - La limpieza ocurre en `mem install` y en `mem update`, reutilizando `setup.filterOutGomemoryHooks`/`IsGomemoryHookEntry` (Claude), el removedor global de la feature 034 (`globalRemover` con `domain.MatrixCell`) para Codex y el plugin de OpenCode.
  - Se conserva el ámbito de `agent_scope`; si no hay ninguno, el global.
  - En `session-start` solo se detecta la duplicación (lectura) y se emite el aviso a la persona por `systemMessage`. Se reutiliza `renderSessionStart(…, avisos)`, que ya existe para los avisos de versión y de copia local.
- **Guarda de reentrada (FR-002)** (endurecida tras los hallazgos LOW de la revisión):
  - Cada invocación de `session-start`, `user-prompt-submit`, `turn-end` y `subagent-*` abre `.memory/.hook-lock-<evento>-<sha256(stdin)[:12]>` y adquiere un lock exclusivo no bloqueante del SO. `.hook-lock-mutex` serializa creación, purga y borrado.
  - Un lock retenido durante el handler descarta la copia concurrente; el SO lo libera si el proceso termina incluso mediante `os.Exit`. Una espera mínima de 250 ms cubre handlers muy rápidos. Un archivo viejo o vacío sin lock activo no indica un dueño y se purga tras 60 s solo cuando está libre.
  - En `user-prompt-submit`, si Claude Code aporta `prompt_id`, el dueño escribe un recibo vacío al completar. Durante 1 h, una copia tardía con el mismo stdin se descarta; otro `prompt_id` se procesa aunque el texto sea idéntico. Si el dueño muere antes de completar, no hay recibo y se puede reintentar.
  - La implementación usa `flock` en Unix y `LockFileEx` en Windows, sin cgo. Sin `prompt_id` (o en los demás eventos), un gemelo posterior al cierre sigue siendo indistinguible de una ejecución legítima con el mismo stdin. No se aplica el recibo a Stop ni a subagentes: varios eventos legítimos pueden compartir un `prompt_id`.
- **Alternativas descartadas**:
  - «Mismo hash en 5 s» a secas: descarta turnos secuenciales legítimos.
  - Deduplicar por PID del padre: el host puede usar procesos distintos.

## R5. Compresión de salidas segura

- **Decisiones**:
  1. **Go por patrones de línea** en `detectCode`: `^\s*func `, `if err != nil`, `:=`, `^\s*return\b`, `^\s*\}\s*$`, `^\t`. Al empatar, Go va antes que TS/Java.
  2. **Heurística de indentación** en `classify`: si ≥ 40 % de las líneas no vacías empieza con tabulador o con ≥ 2 espacios **y** ≥ 20 % contiene `{`, `}`, `;`, `(`, `:=` o `=>`, el bloque es `ContentCode` (por patrones o `compressBraces`).
  3. **Nunca colapsar espacios iniciales**: `compressBlock` solo aplica la limpieza estructural a prosa sin indentación significativa (ninguna línea con tabulador inicial y < 30 % con espacios iniciales).
  4. **Modo «salida de herramienta»**: una opción nueva `ports.CompressionOptions.ToolOutput bool`. Con ella activa:
     - la prosa sale intacta;
     - `JSONMinItems` = 51 (los arrays de hasta 50 elementos se conservan completos a cualquier profundidad);
     - la limpieza estructural se desactiva.
  5. **Exclusiones** (FR-013): `mcp__codebase-memory-mcp__get_code_snippet` y Bash cuyo `tool_input.command`, sin espacios iniciales, empiece por `sed `, `cat `, `head `, `tail `, `git diff`, `git show` o `mem doctor`. Grep **no** se excluye (ver abajo).
  6. **`ToolOutputMinTokens`** sube de 400 a 2 000.
- **Evidencia de compatibilidad con los tests existentes (intocables)**:
  - `tests/contract/hook_tool_output_test.go` (T057) exige que `claude-bash.json`, `claude-grep.json` y `claude-mcp.json` se reduzcan. Medido:
    - bash y opencode son salida de `go test -v` → log, siguen comprimiendo;
    - grep son 400 líneas `ruta:línea` → log o listado, siguen comprimiendo; por eso Grep no se excluye;
    - mcp son 16 objetos con arrays anidados de 313 (`Deps`) y 60 (`GoFiles`) elementos → siguen recortándose con `JSONMinItems`=51.
  - Unos ~8 000 tokens por fixture quedan por encima del umbral de 2 000.
  - `TestRewriteToolOutputClaudeBashShape` no trae `tool_input`, así que no se excluye y sigue pasando.
- **Evidencia del daño** (reproducido 4 veces en esta sesión):
  - `sed` sobre `compression_stats.go` y sobre `channel_activity.go` llegó sin indentación y con líneas omitidas;
  - `sed` sobre `hook_tool_output_test.go` quedó en «×50 filas»;
  - `search_code` llegó con «omitidos 23 de 30»;
  - `mem doctor` llegó con una línea omitida.
- **Alternativas descartadas**:
  - Desactivar por completo el hook de salidas: pierde el 81 % de ahorro en JSON masivo.
  - Excluir Grep: rompe T057 y pierde el caso con más ahorro.

## R6. Compresor de listados

- **Decisión**: `domain.ContentListing`, detectado cuando hay ≥ 30 líneas no vacías y ≥ 70 % tienen forma `ruta:línea[:col]:` o son una ruta sola (sin espacios, con `/` o extensión). Va antes que log y prosa.
  - `native/listing.go` conserva literales las 15 primeras y las 5 últimas líneas, más toda línea que contenga `error`, `FAIL`, `panic` o `fatal` (sin distinguir mayúsculas).
  - El medio se resume en una marca ⟦mem⟧ con la ref y un conteo por archivo o directorio (los 5 más frecuentes y «+N otros»).
  - Las líneas conservadas pasan `checkLiteral` por construcción.
- **Rationale**: es el patrón dominante en las salidas de agentes (grep, rg, find, ls) y hoy cae en prosa con 0,3 % de ahorro.
- **Alternativas descartadas**: muestreo uniforme, porque pierde la localidad que el agente necesita para decidir qué abrir.

## R7. Estado por conversación sin corrupción

- **Decisión**: `writeFileAtomic(path, data, perm)` (temporal en el mismo directorio + `os.Rename`) para `.footprint`, los debounces, `.conversation` y el aviso pendiente. `footprintRead` borra el archivo ante un error de `Atoi` (incluido el desbordamiento) y devuelve 0. Las actualizaciones perdidas por carrera se aceptan: la huella es un proxy.
- **Rationale**: el valor medido `5078217904191461789312457` muestra escrituras entrelazadas; `rename` es atómico en el mismo sistema de archivos.
- **Alternativas descartadas**: llevar la huella a la BD (más I/O por cada llamada MCP, sin necesidad).

## R8. Recordatorios por turno (aclaración Q2)

- **Decisión**:
  - `computePlanModeReminder` queda **sin cambios** (función pura, cubierta por tests intocables). La decisión de emitirlo pasa a quien lo llama. En el dialecto `claude`, `hookUserPromptSubmit` lo emite solo en el primer prompt de la conversación, tras compactar (marcador ya borrado) o cuando `permission_mode` = plan llega por primera vez en la conversación (marcador `.plan-reminder-emitted`). `hookNudge` (OpenCode) y el dialecto `json` (Codex) lo siguen emitiendo en cada turno.
  - La política Octopus se emite en el primer prompt, tras compactar o cuando `octopus_enabled` difiere del último valor emitido (`.memory/.octopus-emitted`), en todos los agentes.
- **Conflicto con un test existente**: `tests/integration/hook_marker_integration_test.go::TestHookOctopusEncendido_SiguePresenteEnTurnosPosteriores` exige la regla Octopus en el segundo turno sin ningún cambio de estado. Contradice FR-008a, decidido con la persona. Según la constitución III, modificarlo **requiere autorización explícita**: se solicita en el plan (Complexity Tracking). El objetivo original del test (ACR 029 C-002: activar Octopus a mitad de sesión debe llegar al agente) se conserva con el disparo por cambio de estado, y el test se reescribiría para cubrir exactamente eso.

## R9. Observabilidad (aclaración Q3)

- **Decisión**: una tabla nueva `hook_guard_events(project, agent, kind, day, count, last_detail, updated_at)` con clave primaria `(project, agent, kind, day)` y *upsert* que suma. La gestiona el mismo `ChannelActivityRepository` mediante un puerto `HookGuardRecorder.Record(agent, kind, detail)`.
  - Tipos: `duplicate_dropped`, `budget_trimmed` (detalle `orig=N,emit=M`), `tool_output_excluded` (detalle: nombre de la herramienta).
  - `mem doctor` suma los últimos 7 días.
- **Rationale**: `channel_activity` guarda solo la última marca por canal (un *upsert* de una fila), así que no puede contar. Agregar por día mantiene la tabla diminuta y sin contenido (privacidad). Es la extensión mínima del «registro de canales existente» que se acordó. La migración es idempotente (`CREATE TABLE IF NOT EXISTS`, constitución II).
- **Fire-and-forget**: un fallo al registrar nunca altera la salida del hook (constitución V.6).

## R10. Codex y OpenCode

- **Decisión**:
  - Se añade al bloque de protocolo de Codex una línea que orienta a usar `search_code`/`get_symbol` de gomemory, porque ahí no hay reescritura de salidas. El texto se resuelve en el renderer por agente que ya existe para el bloque de integración.
  - `mem doctor` ya imprime «hook de salidas (codex): no soportado»; se cambia a «solo en origen (Codex no permite reescribir salidas)».
  - La plantilla del plugin de OpenCode envía también `args` (el comando) a `mem hook tool-output opencode` como `{"tool","output","command"}`, para aplicar FR-013.
