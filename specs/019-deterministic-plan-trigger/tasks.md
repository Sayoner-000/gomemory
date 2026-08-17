---

description: "Tareas de implementación — Activación determinista del modo plan atómico"
---

# Tasks: Activación determinista del modo plan atómico

**Input**: Documentos de diseño en `/specs/019-deterministic-plan-trigger/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS. No es una elección de esta feature: el Principio III de la constitución
(«Testing First — NO NEGOCIABLE») exige escribir el test primero, verlo fallar y solo entonces
implementar. Cada tarea de implementación va precedida por su tarea de test. Los tests existentes son
intocables.

**Neutralidad de agente (INV-6)**: el contrato que manda es
[contracts/agent-integration.md](./contracts/agent-integration.md), que no pertenece a ningún agente.
Los formatos de Claude Code y de OpenCode son **traducciones**. Ninguna tarea de dominio puede
contener nombres de eventos, matchers ni formatos de un agente concreto: eso vive en el traductor de
dialectos y en el adaptador de setup.

**Organization**: agrupadas por historia de usuario, para poder implementar, probar y entregar cada
una por separado.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (archivos distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1..US5)
- Cada tarea lleva su ruta de archivo exacta

## Path Conventions

Arquitectura hexagonal ya vigente en el repositorio: `domain/`, `application/{ports,usecases}/`,
`adapters/primary/{cli,setup,tui}/`, `adapters/secondary/persistence/`, `infrastructure/`,
`tests/{contract,integration}/`. Los tests unitarios van junto a su paquete (`{paquete}_test.go`).

---

## Phase 1: Setup — verificación en vivo del canal (BLOQUEANTE)

**Purpose**: resolver V1..V4 de [research.md](./research.md) antes de escribir una línea de código.
La lección de campo n.º 2 del proyecto y el bug de la memoria 346 nacieron de asumir capacidades del
agente sin comprobarlas contra el sistema en ejecución.

- [X] T001 Montar la sonda de payloads y registrar los dos hooks temporales según [quickstart.md](./quickstart.md) §1, guardando copia de `.claude/settings.json` en `/tmp/planprobe/settings.backup.json`
- [X] T002 Ejecutar la sonda con el agente (entrar en modo plan, pedir un plan, presentarlo) y anotar V1 (`PreToolUse(ExitPlanMode)` dispara y trae `tool_input.plan`) y V2 (el motivo del rechazo llega al agente) en la sección «Estado de la evidencia» de `specs/019-deterministic-plan-trigger/research.md` — **V1 y V2 confirmadas positivas**, ejecutado en vivo dentro de esta sesión con `EnterPlanMode`/`ExitPlanMode`
- [X] T003 Anotar en `specs/019-deterministic-plan-trigger/research.md` V3 (`PostToolUse(EnterPlanMode)` acepta `additionalContext`) y V4 (¿hay señal al entrar en modo plan por atajo de teclado?), con la evidencia observada — **V3 confirmada positiva**; V4 no evaluada (no reproducible desde una llamada a herramienta), sin bloquear nada
- [X] T004 Retirar la sonda, restaurar `.claude/settings.json` desde la copia y borrar `/tmp/planprobe`

**⚠️ Puerta de decisión**: si V1 o V2 resultan negativas, **detenerse y replantear la traducción a
Claude Code** — el contrato neutral y el resto de agentes no se ven afectados, pero el agente que la
persona usa a diario se quedaría en el nivel 3 y eso hay que decidirlo, no descubrirlo. Si solo V3/V4
son negativas, continuar: la Historia 2 se cubre por el recordatorio por turno (T033) y su canal se
reporta como no aplicable.

**Checkpoint**: capacidades del canal conocidas y documentadas; entorno limpio.

---

## Phase 2: Foundational (prerequisitos bloqueantes)

**Purpose**: piezas compartidas por varias historias. Incluye las dos que sostienen la neutralidad
—registro de capacidades y traductor de dialectos— y la corrección del gestor de bloques de
protocolo, que **debe** ir antes de cualquier subida de versión: hoy una subida borra el contenido
posterior al bloque, así que subir a v8 sin este arreglo destruiría archivos de la persona.

**⚠️ CRITICAL**: ninguna historia puede empezar hasta que esta fase esté completa.

- [X] T005 [P] Test: `hookCommandIsGomemory` reconoce `hook plan-guard` y `hook plan-entered` en `adapters/primary/setup/claude_code_setup_test.go`
- [X] T006 Implementar el reconocimiento de ambos subcomandos en `hookCommandIsGomemory` en `adapters/primary/setup/claude_code_setup.go`
- [X] T007 [P] Test: el helper de estado de episodio arranca en 0, incrementa, se reinicia, y trata contenido inválido como «ya devuelto» (permitir) en `adapters/primary/cli/plan_episode_test.go`
- [X] T008 Implementar el helper de estado de episodio (lectura/escritura del marcador bajo `.memory/`, sin BD) en `adapters/primary/cli/plan_episode.go` según [data-model.md](./data-model.md) §2
- [X] T009 [P] Test: `PlanGuardDisabled` se lee del JSON, tiene default seguro (activo) y no rompe settings antiguos en `adapters/secondary/persistence/settings_test.go`
- [X] T010 Añadir el campo `PlanGuardDisabled` con etiqueta `json:"plan_guard_disabled,omitempty"` en `adapters/secondary/persistence/settings.go` — completo también en `application/ports/settings_repository.go` y el mapeo de `adapters/secondary/persistence/repositories.go` (necesario para el roundtrip completo TUI/MCP, mismo patrón que `AtomicPlanDisabled`)
- [X] T011 [P] Test de integración: actualizar un bloque de protocolo legado preserva el contenido propio anterior **y posterior**, no deja restos de la versión vieja y es idempotente, en `tests/integration/protocol_block_test.go`
- [X] T012 Implementar en `composeAgentFile`/`protocolStart` el marcador de fin y la regla de límite para bloques legados (hasta el siguiente encabezado de nivel 2 o EOF) en `adapters/primary/cli/cmd_install.go`, según [data-model.md](./data-model.md) §6
- [X] T013 [P] Test del registro de capacidades en `domain/agents_test.go`: toda entrada declara `text_floor`; un agente ausente del registro se atiende en `neutral` y no se rechaza; añadir una fila ficticia la hace visible sin tocar otros archivos
- [X] T014 Implementar el registro único de capacidades por agente en `domain/agents.go` según [data-model.md](./data-model.md) §5, **sin nombres de eventos ni matchers de ningún agente** — poblado con `claude` (guard+entry+text_floor) y `opencode` (entry+text_floor, sin guard: degradación declarada); el resto de agentes queda fuera de alcance (research.md §13.3)
- [X] T015 [P] Test del traductor de dialectos en `adapters/primary/cli/hook_dialect_test.go`: sin envoltura reconocible → `neutral`; envoltura de eventos reconocida → dialecto de ese agente; `--emit` fuerza cualquiera de los cuatro; el mismo veredicto produce las cuatro formas documentadas; texto plano por stdin equivale a `{"plan":"…"}`
- [X] T016 Implementar la detección y la traducción de dialectos (`neutral`, `json`, `claude`, `text`) en `adapters/primary/cli/hook_dialect.go` según [contracts/agent-integration.md](./contracts/agent-integration.md)

**Checkpoint**: idempotencia de hooks garantizada, estado de episodio disponible, interruptor
presente, actualizar el bloque de protocolo ya es seguro, y existe un contrato neutral con sus
traducciones.

---

## Phase 3: User Story 1 — Un plan que no cumple el contrato no llega a la persona (P1) 🎯 MVP

**Goal**: cuando un agente —cualquiera— vaya a presentar un plan sin forma de árbol para una
solicitud no trivial, el sistema lo devuelve con el motivo antes de que la persona lo vea: una vez
por episodio, apagable y sesgado a permitir.

**Independent Test**: pedir un cambio no trivial en modo plan forzando una respuesta en prosa; la
prosa no llega a la persona y el plan finalmente presentado es un árbol. Sin agente:
[quickstart.md](./quickstart.md) §2 contra el binario. Con un agente que gomemory no conoce: el
cliente de prueba de T025.

- [X] T017 [P] [US1] Test de tabla de la heurística en `domain/plan_shape_test.go`: árbol con glifos → `ShapeOK`; identificadores jerárquicos → `ShapeOK`; marcadores del método (`✓`, `⚠`, `dep:`, `∥`) → `ShapeOK`; prosa larga → `ShapeMissing`; plan corto → `ShapeNotApplicable`; vacío o solo espacios → `ShapeNotApplicable`; árbol en inglés → `ShapeOK`; prosa larga con una sola flecha `→` y sin estructura → `ShapeMissing`
- [X] T018 [US1] Implementar la función pura y el umbral como constante única en `domain/plan_shape.go` según [data-model.md](./data-model.md) §1, sin I/O ni imports de infraestructura
- [X] T019 [P] [US1] Test de contrato del **dialecto neutral** en `tests/contract/plan_guard_neutral_test.go`: `{"plan":"…"}` y texto plano por stdin producen el mismo veredicto; devolver → código ≠ 0 con el motivo por stderr y stdout vacío; permitir → código 0 sin salida; sin `--emit` y sin envoltura, la respuesta es neutral y **no** la de ningún agente concreto
- [X] T020 [P] [US1] Test de contrato del **dialecto de Claude Code** en `tests/contract/hook_plan_guard_test.go`: prosa larga → `deny` con motivo; segunda invocación idéntica → `{}`; tras `plan-approved` vuelve a `deny`; árbol → `{}`; plan trivial → `{}`; payload en forma `tool_input.plan` y en forma `plan` de nivel superior → mismo resultado; JSON inválido → `{}`; interruptor apagado → `{}` sin escribir estado; código 0 en todos los caminos de este dialecto
- [X] T021 [US1] Implementar el case `plan-guard` en `adapters/primary/cli/cmd_hook.go`: extraer el plan, consultar veredicto y estado de episodio, y delegar la salida al traductor de dialectos
- [X] T022 [US1] Redactar el motivo de la devolución, común a todos los dialectos, en `adapters/primary/cli/cmd_hook.go` según [contracts/hook-plan-guard.md](./contracts/hook-plan-guard.md): qué falta, qué hacer, mención de `get_plan_context()` y aviso de que no se repetirá
- [X] T023 [US1] Registrar `PreToolUse` con matcher `ExitPlanMode` → `mem hook plan-guard` en `claudeHookEvents` en `adapters/primary/setup/claude_code_setup.go`, derivando el registro del nivel `guard` declarado en el registro de capacidades
- [X] T024 [P] [US1] Test de contrato: escribir hooks en un `settings.json` que ya contiene entradas ajenas en `PreToolUse` las **preserva**, y dos escrituras consecutivas no duplican las propias, en `tests/contract/claude_hooks_merge_test.go`
- [X] T025 [P] [US1] Test de integración del **agnosticismo** en `tests/integration/foreign_agent_test.go`: un cliente que imita a un agente desconocido (script del ejemplo mínimo de [contracts/agent-integration.md](./contracts/agent-integration.md), solo stdin + código de salida) obtiene devolución con plan en prosa y permiso con plan en árbol, sin que exista ninguna entrada suya en el registro (SC-A1)
- [X] T026 [US1] Reinstalar en el propio repositorio y validar la Historia 1 contra el binario con [quickstart.md](./quickstart.md) §2, dejando constancia del resultado — los 5 escenarios coinciden con el diseño; hallazgo operativo documentado en research.md (binRefFor resuelve `mem` global por PATH, no el build de desarrollo — no bloquea, se resuelve solo tras un release real)

**Checkpoint**: la garantía determinista funciona por contrato neutral, con Claude Code como primera
traducción y sin tocar nada del brazo extensor.

---

## Phase 4: User Story 2 — Método e historial al entrar en modo plan, en cualquier momento (P1)

**Goal**: el agente dispone del método de descomposición y del historial antes de redactar, entre en
modo plan en el turno que sea; y el documento cabe en el canal sin perder el final del método.

**Independent Test**: gastar tres o más turnos y entonces entrar en modo plan; el plan referencia al
menos un elemento del historial que no se mencionó. Sin agente: [quickstart.md](./quickstart.md) §3.

- [X] T027 [P] [US2] Test de la función de ajuste al presupuesto en `domain/plan_budget_test.go`: con un contexto de 24 000 caracteres el resultado no pasa del tope; el método aparece íntegro (primera y última línea); el corte cae en límite de párrafo o de línea, nunca a mitad de frase; si el método solo ya no cabe, se emite el método y se omite el historial con el puntero; el tope es un parámetro, no una constante escondida
- [X] T028 [US2] Implementar la función pura de ajuste (método > historial > puntero) en `domain/plan_budget.go`
- [X] T029 [P] [US2] Test de contrato en `tests/contract/hook_plan_entered_test.go`: salida ≤ tope en los tres dialectos (`neutral`, `claude`, `json`); método completo presente; `--budget` cambia el tope; sin proyecto resoluble → silencio y código 0; con `atomic_plan_disabled` → silencio; segunda invocación consecutiva → salida corta; el contador de episodio queda en 0
- [X] T030 [US2] Implementar el case `plan-entered` en `adapters/primary/cli/cmd_hook.go`: componer el documento con el caso de uso existente, ajustarlo al presupuesto, reiniciar el estado de episodio y delegar la salida al traductor de dialectos
- [X] T031 [US2] Registrar `PostToolUse` con matcher `EnterPlanMode` → `mem hook plan-entered` en `claudeHookEvents` en `adapters/primary/setup/claude_code_setup.go` — V3 confirmada positiva en T001-T004, registro derivado de `AgentLevelEntry`
- [X] T032 [P] [US2] Test: en un prompt que **no** es el primero de la sesión, la salida de `user-prompt-submit` incluye el recordatorio de modo plan de una línea, y sigue incluyendo el de guardado cuando corresponde — cubierto como `computePlanModeReminder` en `adapters/primary/cli/nudge_test.go` (función pura; `hookUserPromptSubmit` no es unit-testeable en proceso por su `os.Exit`) + verificación contra el binario real
- [X] T033 [US2] Añadir el recordatorio de modo plan de una línea al camino de cada turno en `adapters/primary/cli/nudge.go` y enlazarlo en `hookUserPromptSubmit`/`hookNudge` en `adapters/primary/cli/cmd_hook.go`, manteniendo el coste en una sola línea — sin debounce, a diferencia del nudge de guardado
- [X] T034 [P] [US2] Test: `mem hook nudge` —el canal universal para agentes sin sistema de eventos— incluye la línea de modo plan con las mismas condiciones que el camino de Claude Code, en `adapters/primary/cli/nudge_test.go`
- [X] T035 [US2] Validar la Historia 2 contra el binario y con el agente según [quickstart.md](./quickstart.md) §3 — verificado contra el binario real que un prompt posterior al primero ya incluye el recordatorio (el hueco original reportado por el usuario). Se detectó y corrigió un conflicto con `tests/integration/hook_marker_integration_test.go::TestHookMarkerResetsPerSession` (asumía "sin additionalContext en el segundo prompt", ahora desactualizado por FR-003 — corregido para distinguir bootstrap completo (aún gateado) de recordatorio por turno (nuevo, sin gate), sin debilitar lo que el test protegía

**Checkpoint**: el disparador ya no caduca dentro de la sesión, por los dos caminos (evento y turno),
y el documento respeta el presupuesto de cada canal.

---

## Phase 5: User Story 3 — Cualquier agente, presente o futuro, con las diferencias declaradas (P2)

**Goal**: los agentes soportados se comportan igual; un integrador puede conectar un agente
desconocido leyendo un contrato publicado; y lo que no se puede igualar queda declarado.

**Independent Test**: entrar en modo plan a mitad de sesión en dos agentes y comparar; consultar
`mem doctor`; y conectar un tercer agente siguiendo solo el contrato publicado, sin cambios en
gomemory. [quickstart.md](./quickstart.md) §4.

- [X] T036 [P] [US3] Test del catálogo de canales y sus estados en `domain/activation_test.go`: `not_applicable` reservado a agente ausente o canal no soportado, `duplicated` como estado propio, orden determinista
- [X] T037 [US3] Implementar el catálogo `ActivationChannel` y sus enums en `domain/activation.go` según [data-model.md](./data-model.md) §3
- [X] T038 [P] [US3] Definir el puerto del inspector de canales en `application/ports/activation.go`
- [X] T039 [P] [US3] Test del inspector con archivos temporales en `adapters/primary/setup/activation_inspect_test.go`: bloque en versión anterior → `outdated`; entrada de hook duplicada → `duplicated`; agente ausente de la máquina → `not_applicable`; agente declarado solo con `text_floor` → canales deterministas en `not_applicable` sin degradación oculta; brazo extensor ausente → sin canales de ese brazo
- [X] T040 [US3] Implementar el inspector, **recorriendo el registro de capacidades** en vez de listas propias, en `adapters/primary/setup/activation_inspect.go` (solo lectura para el brazo extensor)
- [X] T041 [P] [US3] Test del caso de uso del reporte en `application/usecases/activation_report_test.go`: `problems` no cuenta degradaciones declaradas; orden estable; una fila ficticia añadida al registro aparece en el reporte sin tocar el caso de uso
- [X] T042 [US3] Implementar el caso de uso del reporte en `application/usecases/activation_report.go`
- [X] T043 [P] [US3] Test de contrato de `mem doctor` en `tests/contract/doctor_report_test.go`: `--json` idéntico en dos ejecuciones consecutivas; `--strict` con exit distinto de cero cuando hay problemas y 0 cuando no; brazo extensor ausente sin avisos; agente ficticio del registro enumerado sin modificar el reporte
- [X] T044 [US3] Implementar `mem doctor [--json] [--strict]` en `adapters/primary/cli/cmd_doctor.go` y registrar el case `doctor` en `adapters/primary/cli/dispatcher.go`
- [X] T045 [P] [US3] Test: el bloque de integración y las instrucciones MCP enuncian exploración y forma de salida **secuenciadas**, y ya no reclaman el modo plan como mandato rival, en `tests/contract/integration_block_test.go`
- [X] T046 [US3] Aplicar la redacción compuesta de [research.md](./research.md) §10 en `buildIntegrationBlock` y `memoryProtocolReminder` en `adapters/primary/cli/cmd_install.go` y `adapters/primary/cli/cmd_hook.go`, y subir `integrationVersionMarker` a `<!-- gomemory-protocol-v8 -->` con su marcador de fin (T012 ya lo hizo seguro)
- [X] T047 [US3] Aplicar la misma redacción compuesta en las instrucciones del servidor MCP en `adapters/primary/cli/cmd_mcp.go`
- [X] T048 [P] [US3] Aplicar la misma redacción compuesta al bloque de modo plan del plugin en `infrastructure/plugin/opencode/gomemory.ts`, sin quitarle al grafo su papel en la exploración
- [X] T049 [US3] Declarar la degradación de OpenCode (sin punto de decisión antes de presentar el plan) como consecuencia de su fila en el registro, no como caso especial escrito a mano, en `domain/agents.go` y `adapters/primary/setup/activation_inspect.go`
- [X] T050 [US3] Publicar el contrato de integración fuera de `specs/`, en `docs/AGENT-INTEGRATION.md`, con los tres niveles, los cuatro dialectos y el ejemplo mínimo ejecutable, y enlazarlo desde `README.md`
- [X] T051 [P] [US3] Test de contrato documental en `tests/contract/agent_integration_doc_test.go`: `docs/AGENT-INTEGRATION.md` menciona los cuatro dialectos y los tres niveles que el código implementa de verdad, para que la publicación no se desincronice del comportamiento

**Checkpoint**: la capacidad está definida fuera de todo agente, publicada para terceros y reportada
desde una sola fuente.

---

## Phase 6: User Story 4 — Habilitar una vez y quedar cubierto en todos los proyectos (P2)

**Goal**: el ámbito de usuario deja de ser texto suelto: escribe también los hooks de los agentes que
lo soporten, queda en la versión vigente del protocolo y actualizarlo no destruye nada.

**Independent Test**: en un directorio sin instalación de gomemory, entrar en modo plan y verificar
que el método se aplica; y actualizar un archivo con contenido propio antes y después del bloque sin
pérdidas. [quickstart.md](./quickstart.md) §5.

- [X] T052 [P] [US4] Test: el ámbito global escribe los hooks de cada agente que declare ese ámbito, preservando entradas ajenas y sin duplicar en la segunda ejecución — implementado en `adapters/primary/cli/cmd_mcp_setup_hooks_test.go` (no `cmd_mcp_setup_test.go`, para no tocar el archivo de tests existente de la feature 005)
- [X] T053 [US4] Escribir los hooks en el ámbito de usuario desde `runGlobalScopeSetup` en `adapters/primary/cli/cmd_mcp_setup.go`, recorriendo los ámbitos declarados en el registro de capacidades — vía `setup.WriteClaudeHooksGlobal`, envoltorio exportado de `writeClaudeHooks`
- [X] T054 [P] [US4] Test: el archivo de instrucciones de usuario queda en la versión vigente al habilitar el ámbito global, y el agente sin directorio de configuración se omite sin error — en `adapters/primary/cli/cmd_mcp_setup_hooks_test.go`, usando `domain.ProtocolVersionMarker` como fuente única de "vigente"
- [X] T055 [US4] Verificar en vivo que la cobertura no depende de la instalación accidental de `$HOME` como proyecto: ejecutar el ámbito global, comprobar `mem doctor` en un proyecto temporal recién creado y anotar el resultado en `specs/019-deterministic-plan-trigger/research.md` — ejecutado con autorización explícita del usuario; proyecto temporal nuevo cubierto por el ámbito de usuario. Esta misma verificación destapó 2 bugs reales en `activation_inspect.go` (ver quickstart.md, sección "Dos bugs reales"), corregidos con TDD y reverificados
- [X] T056 [US4] Actualizar el archivo de instrucciones de nivel usuario a la versión vigente ejecutando el ámbito global en esta máquina, y comprobar con `diff` que no se perdió contenido propio en `~/.claude/CLAUDE.md` — `~/.claude/CLAUDE.md` subió de v4 a v7, diff confirma 0 pérdida de contenido

**Checkpoint**: «habilitar una vez cubre todos los proyectos» es cierto, y por el canal previsto.

---

## Phase 7: User Story 5 — Detectar por adelantado que algo se rompió, en cualquiera de los dos brazos (P3)

**Goal**: la verificación falla cuando un canal del modo plan falta, está desactualizado o duplicado,
y también cuando la activación del brazo extensor deja de producirse.

**Independent Test**: el script pasa en el repositorio; al degradar a mano un canal —o al retirar la
activación del brazo extensor— falla señalando brazo, agente y canal.
[quickstart.md](./quickstart.md) §6.

- [X] T057 [P] [US5] Añadir al script las comprobaciones de los canales de modo plan consumiendo `mem doctor --json`, **sin listas de agentes propias**, en `scripts/test-codebase-memory-activation.sh`
- [X] T058 [P] [US5] Añadir al script la comprobación de no-regresión del brazo extensor: su activación sigue produciéndose, y su ausencia no genera avisos, en `scripts/test-codebase-memory-activation.sh`
- [X] T059 [US5] Añadir al script la comprobación de doble instalación sin duplicados y sin pérdida de entradas ajenas en `scripts/test-codebase-memory-activation.sh`
- [X] T060 [US5] Añadir al script la comprobación del contrato neutral con un cliente de agente desconocido (el ejemplo mínimo del contrato publicado), en `scripts/test-codebase-memory-activation.sh`
- [X] T061 [US5] Actualizar el encabezado y el propósito del script para reflejar que cubre los dos brazos y los canales de modo plan, en `scripts/test-codebase-memory-activation.sh`
- [X] T062 [US5] Ejecutar `scripts/test-codebase-memory-activation.sh`, degradar a mano un canal de cada brazo y confirmar que falla en ambos casos identificando brazo, agente y canal

**Checkpoint**: la clase de regresión que originó esta feature ya no puede vivir varias versiones sin
detectarse, y el agnosticismo tiene su propia comprobación.

---

## Phase 8: Polish & Cross-Cutting

- [X] T063 [P] Añadir el interruptor del guard a la pantalla de configuración de la TUI en `adapters/primary/tui/` siguiendo el patrón de la feature 016
- [X] T064 [P] Documentar `mem doctor`, el guard de modo plan, los dialectos y el interruptor nuevo en `docs/MANUAL.md`
- [X] T065 [P] Documentar en `docs/architecture.md` los dos hooks nuevos, el registro de capacidades, el traductor de dialectos y la relación de solo lectura con el brazo extensor
- [X] T066 [P] Añadir a `README.md` el comando `mem doctor` en el árbol de CLI, `plan_guard_disabled` en la tabla de configuración y el enlace a `docs/AGENT-INTEGRATION.md`
- [X] T067 [P] Registrar en `docs/lessons.md` las lecciones de esta feature: el determinismo se ancla donde el mecanismo está documentado; el presupuesto de canal (10 000) era menor que el documento que se quería inyectar; y definir una capacidad en el formato del primer agente reproduce la asimetría que se venía a corregir
- [X] T068 Verificar cobertura ≥ 80 % con `go test ./... -cover` en los paquetes tocados (`domain/`, `application/usecases/`, `adapters/primary/cli/`, `adapters/primary/setup/`, `adapters/secondary/persistence/`) y completar los huecos
- [X] T069 Ejecutar la validación completa de [quickstart.md](./quickstart.md) §2 a §6 y anotar el resultado de cada criterio SC-001..SC-011 y SC-A1..SC-A3 — todos ✅, tabla completa en quickstart.md "Resultado de la validación (2026-08-17)"

---

## Dependencies

```text
Phase 1 (T001-T004)  ── puerta de decisión (solo afecta a la traducción a Claude Code) ──┐
                                                                                        ▼
Phase 2 (T005-T016)  ── bloqueante para todo ─┐
                                              ├──► US1 (T017-T026)   MVP
                                              ├──► US2 (T027-T035)
                                              ├──► US3 (T036-T051)   T012 antes de T046
                                              ├──► US4 (T052-T056)   requiere T006, T012, T014
                                              └──► US5 (T057-T062)   requiere T044 y T050
                                                        │
                                                        ▼
                                              Phase 8 (T063-T069)
```

**Dependencias que no son evidentes**:

- **T014 y T016 antes de US1**: si el hook se escribe primero y el traductor después, el dialecto de
  Claude Code se filtra al motor y el siguiente agente hereda su forma. Es el orden que protege INV-6.
- **T012 antes de T046**: subir el protocolo a v8 sin el marcador de fin borraría el contenido
  posterior al bloque en los archivos de la persona. El arreglo va primero, siempre.
- **T006 antes de T023 y T031**: registrar un hook cuyo subcomando no reconozca el filtro de
  idempotencia hace que cada reinstalación añada otra copia.
- **T044 antes de T057** y **T050 antes de T060**: el script consume `mem doctor --json` y el ejemplo
  del contrato publicado; sin ellos reimplementaría reglas en shell y perdería las pruebas.
- **T003 condiciona T031**: si el canal de entrada no acepta contexto, ese registro no se hace y se
  documenta el motivo.

**Historias independientes**: US1 y US2 no se necesitan entre sí (una garantiza la forma, la otra la
calidad). US3 depende de US1/US2 solo para tener qué reportar y qué publicar. US4 y US5 se apoyan en lo
anterior pero no lo modifican.

## Parallel Execution Examples

**Fase 2**: T005, T007, T009, T011, T013 y T015 son seis tests en seis archivos distintos → en
paralelo. Sus implementaciones también, salvo T014/T016 que conviene revisar juntas por ser el par que
sostiene la neutralidad.

**US1**: T017, T019, T020, T024 y T025 en paralelo (cinco archivos de test distintos); T021-T023
secuenciales por tocar los mismos dos archivos.

**US2**: T027, T029, T032 y T034 en paralelo.

**US3**: el bloque de tests T036, T039, T041, T043, T045 y T051 en paralelo; T048 (plugin de OpenCode)
y T050 (documento publicado) en paralelo con cualquier tarea de Go.

**US5**: T057 y T058 en paralelo si se escriben como funciones separadas del script; T059-T061 después,
porque tocan las mismas secciones.

**Fase 8**: T063-T067 son cinco archivos distintos → todas en paralelo.

## Implementation Strategy

**MVP = Fase 1 + Fase 2 + US1** (T001-T026). Con eso, ningún plan sin forma de árbol llega a la
persona **en ningún agente**: el contrato neutral y su traducción a Claude Code entran juntos, y T025
demuestra con un cliente ajeno que un agente desconocido obtiene la misma garantía.

**Incremento 2 = US2**: el plan además llega informado por el historial, entre en modo plan en el turno
que sea, por evento o por recordatorio de turno.

**Incremento 3 = US3 + US4**: contrato publicado, reporte desde una sola fuente y cobertura real de
nivel usuario.

**Incremento 4 = US5 + Polish**: la red de seguridad y la documentación.

**Regla de parada**: si la puerta de decisión de la Fase 1 sale negativa en V1 o V2, se detiene la
**traducción a Claude Code**, no la feature: el contrato neutral sigue en pie y cualquier agente que
pueda cumplirlo obtiene la garantía. Lo que no se hace es dar por supuesta una capacidad que no se
comprobó, que es el error que esta feature corrige.
