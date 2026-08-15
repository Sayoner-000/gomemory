# Research: Reindexado dual de grafos de código + edición de huella de contexto en TUI

No quedó ningún `NEEDS CLARIFICATION` en `plan.md` — el brief original del usuario ya traía las
decisiones de diseño cerradas y verificadas en vivo. Este documento consolida esas decisiones
en el formato Decision/Rationale/Alternatives para dejar registro del porqué.

## 1. Interfaz separada `ports.CodeGraphIndexer` (no ampliar `CodeGraphProvider`)

- **Decision**: nueva interfaz, independiente de `ports.CodeGraphProvider`.
- **Rationale**: `CodeGraphProvider` tiene un contrato documentado y deliberado de no-bloqueo
  (`Snapshot`, `MaybeRefresh` con `probeTimeout` de 2s, pensado para hooks por turno). Mezclar un
  método bloqueante de minutos en esa misma interfaz rompería esa garantía para todo el código
  que ya depende de ella. Mismo patrón ya usado para `ports.ADRSyncProvider` sobre el mismo
  `codebasememory.Provider` — separar por capacidad, no por adaptador.
- **Alternatives considered**: agregar `IndexRepository` directo a `CodeGraphProvider` — rechazado
  porque obligaría a todo implementador (incluyendo mocks/fakes de test del hot path) a soportar
  una operación bloqueante que nunca deberían invocar.

## 2. `IndexRepository` no llama a `resolveProject` primero

- **Decision**: `index_repository` recibe `repo_path` directo; el registro del proyecto en el
  proveedor externo es un efecto secundario del indexado, no un prerequisito.
- **Rationale**: a diferencia de `fetchArchitecture`/`GetDocument` (que necesitan que
  `list_projects` ya conozca el proyecto), depender de `resolveProject` haría fallar el caso más
  importante: la primera vez que se indexa un proyecto nuevo, cuando por definición el proveedor
  externo todavía no lo conoce.
- **Alternatives considered**: registrar primero vía una llamada separada — rechazado por
  redundante; verificado en vivo que `index_repository` ya registra el proyecto como parte de su
  propia ejecución.

## 3. Timeout propio (`indexTimeout` = 10 min), no reutilizar `probeTimeout`

- **Decision**: constante nueva junto a `probeTimeout`/`snapshotTTL`; `runCLI` se refactoriza para
  delegar en `runCLIWithTimeout(ctx, timeout, tool, argsJSON)` parametrizable.
- **Rationale**: `probeTimeout` (2s, hardcoded) está diseñado para sondeos rápidos del hot path.
  Un reindexado completo puede tardar minutos — reutilizar ese timeout cortaría la operación a
  mitad de camino. El refactor de `runCLI` evita duplicar la construcción del comando `exec.CommandContext`.
- **Alternatives considered**: timeout configurable por el usuario — rechazado por YAGNI; no hay
  necesidad expresada de ajustarlo, y agregar un flag/setting para esto es complejidad sin
  demanda real.

## 4. Sentinel `ports.ErrIndexerNotInstalled` vive en `ports`, no en `codebasememory`

- **Decision**: el error centinela se declara en la capa de aplicación (`application/ports`).
- **Rationale**: así `cmd_index.go` y `tui.go` (adaptadores primarios) solo importan `ports` para
  distinguir "no instalado" (línea informativa) de un fallo real (advertencia), sin acoplarse al
  paquete del adaptador secundario concreto — coherente con la regla de dependencia de la
  arquitectura hexagonal (adaptadores primarios no deben conocer adaptadores secundarios).
- **Alternatives considered**: error centinela en `codebasememory` con re-export — rechazado,
  viola la regla de capas y es indirecto sin necesidad.

## 5. `deps.CodeProviders[0]` sin filtrar por `FirstAvailable`

- **Decision**: tanto `cmd_index.go` como la TUI usan el proveedor configurado directamente,
  sin pasar por `FirstAvailable` (`application/usecases/provider_selection.go`).
- **Rationale**: `FirstAvailable` descarta proveedores con `Snapshot().Available == false` — que
  es exactamente el estado de un proyecto recién creado o nunca indexado, el caso que más
  necesita el reindexado. Filtrar por disponibilidad excluiría al proveedor justo cuando hace
  falta invocarlo.
- **Alternatives considered**: ninguna — usar `FirstAvailable` aquí sería contradictorio con el
  propósito de la función.

## 6. Modo siempre `"full"`, sin selector

- **Decision**: hardcodeado en CLI y TUI, sin flag ni opción de modo parcial/incremental.
- **Rationale**: YAGNI — no hay demanda expresada de indexado incremental desde estas dos rutas;
  agregar el selector ahora sería especular sobre una necesidad futura no confirmada.
- **Alternatives considered**: exponer `--mode` — rechazado explícitamente por el usuario en el
  brief original.

## 7. Primer comando asíncrono real de la TUI (patrón `tea.Cmd` + mensaje de resultado)

- **Decision**: `reindexExternalGraphCmd() tea.Cmd` + `externalReindexDoneMsg{nodes, edges, err}`,
  manejado en el `Update()` de nivel superior junto a `tea.WindowSizeMsg`.
- **Rationale**: hoy toda la TUI es síncrona porque las operaciones son locales de SQLite/JSON.
  `IndexRepository` puede tardar minutos — bloquear `Update()` congelaría la interfaz. El patrón
  `tea.Cmd` es el mecanismo idiomático de Bubble Tea para trabajo asíncrono sin bloquear el bucle
  de eventos.
- **Alternatives considered**: goroutine manual con canal propio fuera del ciclo de Bubble Tea —
  rechazado, reimplementaría lo que el framework ya ofrece de forma idiomática y sería más difícil
  de testear con el patrón de mensajes ya usado en el resto del archivo.

## 8. Guardia de reindexado concurrente en la TUI (FR-011 del spec)

- **Decision**: mientras un `externalReindexDoneMsg` no haya llegado, una segunda invocación de la
  acción "Reindexar grafo externo" no dispara un segundo `tea.Cmd`; el `statusMsg` indica que ya
  hay uno en curso.
- **Rationale**: sin esta guardia, dos reindexados concurrentes contra el mismo proyecto podrían
  competir por el mismo proceso externo sin beneficio — el usuario no gana nada disparando dos a
  la vez y sí puede confundir el estado final que ve reflejado en pantalla.
- **Alternatives considered**: permitir concurrencia y mostrar el último resultado que llegue —
  rechazado, generaría confusión sobre qué corrida produjo qué conteo.

## 9. Pantalla única `screenEditSetting` parametrizada (no 3 pantallas ni un formulario multi-campo)

- **Decision**: una sola pantalla nueva con un `textinput.Model`, parametrizada por
  `editSettingField` (`editFieldBudget`, `editFieldCompactThreshold`, `editFieldDedupDays`).
- **Rationale**: reutiliza el molde ya existente de `screenImport`/`m.importPath` ("un solo input
  a la vez"), consistente con el resto de la TUI. Una pantalla con 3 inputs navegables
  reimplementaría la lógica de cursor que ya vive en `screenConfig`, sin ganar nada.
- **Alternatives considered**: formulario con los 3 campos a la vez — rechazado por duplicar
  lógica de navegación ya resuelta en otro lugar del mismo archivo.

## 10. Validación de settings: entero requerido; cero y negativo se aceptan tal cual

- **Decision**: `strconv.Atoi` falla → error "Debe ser un número entero", no guarda. Cualquier
  entero (positivo, cero, negativo) se acepta y persiste sin bloqueo adicional.
- **Rationale**: verificado en `adapters/secondary/persistence/settings.go` — el patrón
  0 → default / negativo → opt-out explícito ya está normalizado en `ReadSettings`, no en el
  punto de escritura. La TUI no debe reinventar esa semántica, solo respetarla.
- **Alternatives considered**: bloquear negativos en la UI "por seguridad" — rechazado, iría en
  contra de una semántica ya documentada y en uso (negativo = desactivación explícita e
  intencional).
