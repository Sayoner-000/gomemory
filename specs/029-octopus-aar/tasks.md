---
description: "Lista de tareas para la implementación de Octopus AAR"
---

# Tareas: Octopus AAR — Enrutador Adaptativo de Agentes

**Entrada**: documentos de diseño de `/specs/029-octopus-aar/`

**Prerrequisitos**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`

**Pruebas**: **SÍ se generan tareas de prueba**. No es opcional aquí por dos razones concurrentes: la especificación las exige (§64 del documento de entrada, criterios AC-001 a AC-020) y la constitución del proyecto declara TDD como principio NO NEGOCIABLE — la prueba se escribe primero, falla, y solo entonces se implementa.

**Organización**: por historia de usuario, en orden de prioridad. Cada fase es un incremento entregable y verificable por su cuenta.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: puede ejecutarse en paralelo (archivo distinto, sin dependencias pendientes)
- **[Historia]**: a qué historia de usuario pertenece (US1…US8)
- Toda descripción lleva ruta de archivo exacta

## Convenciones de rutas (arquitectura hexagonal ya vigente)

- Dominio puro: `domain/octopus_*.go`
- Puertos: `application/ports/`
- Casos de uso: `application/usecases/`
- Adaptadores: `adapters/primary/{cli,tui}/`, `adapters/secondary/persistence/`
- Composition root: `infrastructure/container.go`
- Pruebas unitarias: junto al código (`*_test.go`), como ya hace todo el repositorio
- Pruebas de contrato: `tests/contract/` · Pruebas de integración: `tests/integration/`

---

## Fase 1: Preparación

**Propósito**: fijar una línea base honesta antes de tocar nada.

- [X] T001 Registrar la línea base ejecutando `go build -o mem ./infrastructure && go test ./... -count=1` y anotar el resultado (verde/rojo y duración) al final de este archivo, en la sección Registro de ejecución
- [X] T002 [P] Verificar que `gofumpt -l .` y `golangci-lint run` salen limpios ANTES de empezar, para que cualquier hallazgo posterior sea atribuible a esta funcionalidad

---

## Fase 2: Base (prerrequisitos bloqueantes)

**Propósito**: tipos y configuración que TODAS las historias necesitan.

**⚠️ CRÍTICO**: ninguna historia puede empezar hasta que esta fase esté completa.

- [X] T003 Escribir prueba de que un `settings.json` sin la clave deja el módulo APAGADO y que `true` se preserva, en `adapters/secondary/persistence/settings_test.go` (debe fallar antes de T004/T005)
- [X] T004 [P] Añadir el campo `OctopusEnabled bool \`json:"octopus_enabled,omitempty"\`` con su comentario de polaridad en `application/ports/settings_repository.go`
- [X] T005 Añadir el mismo campo y su lectura/escritura en `adapters/secondary/persistence/settings.go`, verificando que T003 pasa
- [X] T006 [P] Escribir prueba de tabla de los tipos base y sus reglas de validación (objetivo vacío es inenrutable, clase desconocida es válida) en `domain/octopus_workunit_test.go`
- [X] T007 [P] Implementar `WorkUnit`, `TaskClass`, `Level`, `Scope`, `ContextNeed` y `OutputSpec` en `domain/octopus_workunit.go` según `data-model.md` §1
- [X] T008 [P] Escribir prueba de la normalización conservadora de capacidades (estructura vacía ⇒ sin subagentes; `MaxParallel<=0` con paralelo ⇒ 1) en `domain/octopus_capability_test.go`
- [X] T009 [P] Implementar `RuntimeCapabilities` y su normalización en `domain/octopus_capability.go`
- [X] T010 [P] Escribir prueba de que el catálogo de razones tiene códigos únicos y ningún texto vacío en `domain/octopus_route_test.go`
- [X] T011 [P] Implementar `Route`, el catálogo cerrado de `Reason`, `RouteDecision`, `ParallelGroup` y `RoutingPlan` en `domain/octopus_route.go` según la tabla de `data-model.md` §1
- [X] T012 [P] Escribir prueba del invariante de reparto del presupuesto (`MainAgentMax + DelegationPoolMax + ValidationReserve <= TotalTokens`) en `domain/octopus_budget_test.go`
- [X] T013 [P] Implementar `Budget`, `CostEstimate` y el reparto configurable en `domain/octopus_budget.go`
- [X] T014 Declarar las constantes de política con nombre en `domain/octopus_policy.go` (`DefaultMaxDelegationDepth = 1`, `DefaultMaxDelegationRetries = 1`, tope de agentes por plan, tope de concurrencia y porcentajes del reparto), en un solo lugar y sin repetirlas en ningún sitio de uso

**Punto de control**: los tipos compilan, sus pruebas pasan y el ajuste del módulo se lee y escribe. Las historias pueden empezar.

---

## Fase 3: Historia 1 — Decidir si una tarea se delega (P1) 🎯 MVP

**Objetivo**: dada una unidad de trabajo, su presupuesto y las capacidades del runtime, devolver exactamente una ruta con una razón explicable, sin modelo y sin historial.

**Prueba independiente**: `./mem octopus route "<objetivo>" --json` devuelve ruta y razón; con las mismas entradas, dos veces, devuelve lo mismo.

### Pruebas de la Historia 1 ⚠️ (escribir primero, deben fallar)

- [X] T015 [P] [US1] Escribir prueba de tabla del orden de evaluación de las 13 reglas de `contracts/routing-policy.md`, una fila por regla, verificando ruta y código de razón, en `domain/octopus_policy_test.go`
- [X] T016 [P] [US1] Escribir prueba de AC-001 (cambio trivial ⇒ `INLINE` con `ReasonTrivial`) y AC-003 (sin subagentes ⇒ `INLINE` con `ReasonNoSubagents`) en `domain/octopus_policy_test.go`
- [X] T017 [P] [US1] Escribir prueba de AC-002 (investigación aislable y de solo lectura ⇒ `DELEGATE` con presupuestos de contexto y salida mayores que cero) en `domain/octopus_policy_test.go`
- [X] T018 [P] [US1] Escribir prueba de dependencia sin resolver ⇒ `WAIT` con `BlockedBy` poblado, en `domain/octopus_policy_test.go`
- [X] T019 [P] [US1] Escribir prueba de reproducibilidad (SC-006): enrutar la misma entrada 100 veces y comparar la serialización completa, en `domain/octopus_policy_test.go`
- [X] T020 [P] [US1] Escribir prueba de pureza: `RouteTask` produce decisión válida con `Evidence == nil` y sin ninguna dependencia inyectada (AC-015, arranque en frío), en `domain/octopus_policy_test.go`
- [X] T021 [P] [US1] Escribir prueba del caso de uso: mide con un `TokenCounter` falso, rellena las cifras de `WorkUnit` y delega la decisión al dominio, en `application/usecases/octopus_route_task_test.go`

### Implementación de la Historia 1

- [X] T022 [US1] Implementar `RouteTask` con las 13 reglas en el orden exacto del contrato, en `domain/octopus_policy.go`, hasta que T015–T020 pasen
- [X] T023 [US1] Implementar la aritmética de `CostEstimate` (beneficio contra costo) sobre cifras ya medidas y constantes con nombre, en `domain/octopus_policy.go`. **El dominio no mide texto**: no importa `ports` ni cuenta tokens
- [X] T024 [US1] Implementar el caso de uso `RouteTaskUseCase` en `application/usecases/octopus_route_task.go`: mide contexto, contrato y salida esperada con `ports.TokenCounter`, rellena `WorkUnit` y llama al dominio
- [X] T025 [US1] Implementar `mem octopus route <objetivo>` con sus banderas y salida `--json` en `adapters/primary/cli/cmd_octopus.go`, según `contracts/cli-octopus.md`
- [X] T026 [US1] Añadir `case "octopus"` en `adapters/primary/cli/dispatcher.go` y la entrada correspondiente en la ayuda de `adapters/primary/cli/cli.go`

**Punto de control**: `./mem octopus route` decide y explica. El módulo todavía no tiene interruptor: en este punto responde siempre, y eso se cierra en la Historia 2 antes de considerar la funcionalidad entregable.

---

## Fase 4: Historia 2 — Encender o apagar el módulo (P2)

**Objetivo**: el módulo aparece como fila "Octopus AAR" en la configuración de la TUI, nace apagado, persiste su estado, y apagado significa huella observable cero.

**Prueba independiente**: abrir la TUI, ver la fila en `off`, alternarla, reiniciar y comprobar que sigue en `on`; con el módulo apagado, `tools/list` no contiene ningún nombre de Octopus.

### Pruebas de la Historia 2 ⚠️ (escribir primero, deben fallar)

- [X] T027 [US2] **Extender** (no relajar) `tests/contract/mcp_tool_sync_test.go`: conservar íntegra la aserción actual como el caso "módulo apagado" y añadir un caso "módulo encendido" que escriba `octopus_enabled: true` en el directorio temporal del servidor y compare `tools/list` contra `domain.MCPToolsFor(true)`. **Cambio de test existente autorizado y justificado en `plan.md`, nota del principio III**
- [X] T028 [P] [US2] Escribir prueba de que el bloque de protocolo generado con el módulo apagado no contiene la cadena "octopus" (comparación insensible a mayúsculas) en `tests/contract/integration_block_test.go`
- [X] T029 [P] [US2] Escribir prueba de que la fila "Octopus AAR" existe, muestra `off` por defecto y que alternarla persiste el ajuste, en `adapters/primary/tui/tui_test.go`
- [X] T030 [P] [US2] Escribir prueba de integración del apagado extremo a extremo: ejecutar un flujo completo con el módulo apagado y verificar cero filas en `octopus_executions` y cero salida atribuible a Octopus en `get_context`, en `tests/integration/octopus_module_off_test.go`

### Implementación de la Historia 2

- [X] T031 [US2] Añadir `ToolOctopusRouteTask` y la lista `MCPOctopusTools` en `domain/mcp_tools.go`, más `MCPToolsFor(octopusEnabled bool) []string`, dejando `MCPAllTools()` con su significado actual (módulo apagado)
- [X] T032 [US2] Hacer que el bloque de protocolo, el bootstrap de ToolSearch y las listas de auto-aprobación consuman `MCPToolsFor(s.OctopusEnabled)` en `adapters/primary/cli/cmd_install.go` y `adapters/primary/cli/cmd_mcp_setup.go`, verificando que con el módulo apagado el texto es byte a byte el actual
- [X] T033 [US2] Crear `registerOctopusTools` con registro de `octopus_route_task` en `adapters/primary/cli/cmd_mcp_octopus_tools.go`, según `contracts/mcp-octopus-tools.md`
- [X] T034 [US2] Invocar `registerOctopusTools` **solo** cuando el ajuste está encendido, en `newMCPServer` de `adapters/primary/cli/cmd_mcp.go`
- [X] T035 [US2] Añadir la constante `configRowOctopus` al final de las filas y el caso de alternado con su mensaje de estado, en `adapters/primary/tui/tui.go`, siguiendo el patrón de `configRowPlanGuard`
- [X] T036 [US2] Añadir la fila `"Octopus AAR: " + onOff(s.OctopusEnabled)` a la lista de filas de la vista de configuración en `adapters/primary/tui/tui.go`
- [X] T037 [US2] Hacer que todo subcomando de `mem octopus` responda "módulo desactivado", indique cómo activarlo y termine con código distinto de cero cuando el ajuste está apagado, en `adapters/primary/cli/cmd_octopus.go`

**Punto de control**: el módulo se enciende y se apaga desde la TUI, persiste, y apagado no deja rastro. Historias 1 y 2 funcionan de forma independiente.

---

## Fase 5: Historia 3 — Enrutar un plan completo (P3)

**Objetivo**: plan de enrutamiento con dependencias respetadas, grupos paralelos acotados y topes de fan-out aplicados.

**Prueba independiente**: `./mem octopus plan --file <plan.json>` produce un plan donde ninguna tarea dependiente comparte grupo con su dependencia y ningún grupo excede el límite de concurrencia.

### Pruebas de la Historia 3 ⚠️ (escribir primero, deben fallar)

- [X] T038 [P] [US3] Escribir prueba de entrada inválida: ciclo en el grafo, dependencia inexistente e identificador duplicado o vacío devuelven error señalando la causa, en `domain/octopus_plan_test.go`
- [X] T039 [P] [US3] Escribir prueba de AC-005 (T002 depende de T001 ⇒ nunca comparten grupo paralelo) en `domain/octopus_plan_test.go`
- [X] T040 [P] [US3] Escribir prueba de AC-004 (dos independientes con `parallel` y `max_parallel>=2` ⇒ mismo grupo) y del recorte a `max_parallel = 2`, en `domain/octopus_plan_test.go`
- [X] T041 [P] [US3] Escribir prueba de AC-009 (20 tareas independientes con tope de 4 ⇒ como máximo 4 delegaciones, el resto con `ReasonFanOutLimit`) en `domain/octopus_plan_test.go`
- [X] T042 [P] [US3] Escribir prueba de AC-010 (profundidad máxima 1 ⇒ el contrato del hijo no autoriza delegar) en `domain/octopus_plan_test.go`
- [X] T043 [P] [US3] Escribir prueba de que dos unidades con alcances de escritura que se intersecan no comparten grupo aunque no exista dependencia declarada, en `domain/octopus_plan_test.go`
- [X] T044 [P] [US3] Escribir prueba de orden determinista: decisiones ordenadas por identificador de tarea y grupos por identificador de grupo, en `domain/octopus_plan_test.go`
- [X] T045 [P] [US3] Escribir prueba del escenario extremo a extremo de `spec.md` §66 (T001/T002 inline, T003+T004 en un grupo, T005 en espera, T006 delegada) en `tests/integration/octopus_route_plan_test.go`

### Implementación de la Historia 3

- [X] T046 [US3] Implementar la validación del grafo (ciclos por recorrido en profundidad, dependencias inexistentes, identificadores duplicados) en `domain/octopus_plan.go`
- [X] T047 [US3] Implementar `RoutePlan`: decidir cada unidad ordenada por identificador, aplicar el tope de agentes y devolver el plan, en `domain/octopus_plan.go`
- [X] T048 [US3] Implementar la formación de grupos paralelos con las cinco reglas de `contracts/routing-policy.md`, en `domain/octopus_plan.go`
- [X] T049 [US3] Implementar el caso de uso `RoutePlanUseCase` en `application/usecases/octopus_route_plan.go`
- [X] T050 [US3] Implementar la lectura del grafo desde Spec Kit vía `ports.SpecKitReader` con degradación silenciosa cuando no hay funcionalidad activa, en `application/usecases/octopus_route_plan.go`
- [X] T051 [US3] Implementar `mem octopus plan` con `--file`, `--budget`, `--max-parallel`, `--max-agents` y `--json` en `adapters/primary/cli/cmd_octopus.go`
- [X] T052 [US3] Registrar la tool `octopus_route_plan` y añadir su nombre a `MCPOctopusTools` en el mismo cambio, en `adapters/primary/cli/cmd_mcp_octopus_tools.go` y `domain/mcp_tools.go`
- [X] T053 [P] [US3] Crear el plan de ejemplo del escenario §66 en `specs/029-octopus-aar/ejemplo-plan.json`, que `quickstart.md` §6 ya referencia

**Punto de control**: un plan real se enruta con dependencias, paralelismo y topes respetados.

---

## Fase 6: Historia 4 — Contexto mínimo y contrato acotado (P4)

**Objetivo**: lo delegado recibe solo el contexto que necesita y un contrato con objetivo, alcance, permisos y forma del resultado.

**Prueba independiente**: pedir el contrato de una tarea que requiere dos archivos y una memoria, y comprobar que el paquete no contiene conversación ni memorias ajenas.

### Pruebas de la Historia 4 ⚠️ (escribir primero, deben fallar)

- [X] T054 [P] [US4] Escribir prueba de AC-006: el paquete de una tarea que necesita dos artefactos y una memoria no incluye historial de conversación ni memorias no relacionadas, en `tests/integration/octopus_context_pack_test.go`
- [X] T055 [P] [US4] Escribir prueba de que todo contrato lleva objetivo, alcance, restricciones, permisos, presupuesto de contexto y forma del resultado, en `domain/octopus_contract_test.go`
- [X] T056 [P] [US4] Escribir prueba de AC-020: una investigación de solo lectura nunca produce un contrato con permisos de escritura, y ningún contrato excede los permisos del flujo principal, en `domain/octopus_contract_test.go`
- [X] T057 [P] [US4] Escribir prueba de que credenciales, tokens y claves quedan fuera del contexto delegado, en `application/usecases/octopus_pack_contract_test.go`
- [X] T058 [P] [US4] Escribir prueba de AC-013: un resultado que excede el presupuesto de integración se reduce preservando conclusiones, evidencia, artefactos y pendientes, y descartando relleno, en `domain/octopus_result_test.go`

### Implementación de la Historia 4

- [X] T059 [P] [US4] Implementar `ExecutionContract`, `Permissions` y la validación de no elevación de privilegios en `domain/octopus_contract.go`
- [X] T060 [P] [US4] Implementar `DelegatedResult`, `ResultStatus` y la compactación estructurada en `domain/octopus_result.go`
- [X] T061 [US4] Implementar el caso de uso que arma paquete de contexto y contrato en `application/usecases/octopus_pack_contract.go`, reutilizando `BuildContextPack` con `MaxTokens` igual al presupuesto de contexto de la unidad
- [X] T062 [US4] Aplicar `domain.RedactSecrets` al contenido antes de empaquetarlo, en `application/usecases/octopus_pack_contract.go`, verificando T057
- [X] T063 [US4] Incluir el contrato de ejecución en la salida de las rutas delegadas de `octopus_route_task` y `octopus_route_plan`, en `adapters/primary/cli/cmd_mcp_octopus_tools.go`

**Punto de control**: lo delegado sale con contexto mínimo, contrato acotado y sin secretos.

---

## Fase 7: Historia 5 — Respetar el presupuesto de la sesión (P5)

**Objetivo**: reparto configurable entre agente principal, fondo de delegación y reserva de validación, sin desbordes silenciosos.

**Prueba independiente**: agotar el fondo de delegación y comprobar que las delegaciones opcionales pasan a `INLINE` o `REJECT` sin tocar la reserva.

### Pruebas de la Historia 5 ⚠️ (escribir primero, deben fallar)

- [X] T064 [P] [US5] Escribir prueba de AC-007: presupuesto restante menor al costo estimado ⇒ `INLINE` o `REJECT` con `ReasonBudgetExhausted`, nunca desborde, en `domain/octopus_budget_test.go`
- [X] T065 [P] [US5] Escribir prueba de AC-008: los tokens restantes solo existen en la reserva y la unidad es opcional ⇒ la reserva queda intacta con `ReasonValidationReserveProtected`, en `domain/octopus_budget_test.go`
- [X] T066 [P] [US5] Escribir prueba de la distinción `REJECT` frente a `INLINE` bajo presión de presupuesto según la regla 9 del contrato, en `domain/octopus_policy_test.go`
- [X] T067 [P] [US5] Escribir prueba de que el reparto por porcentajes es configurable y que sin conteo exacto toda cifra sale marcada como estimada (FR-033), en `domain/octopus_budget_test.go`

### Implementación de la Historia 5

- [X] T068 [US5] Implementar las reglas 9 y 10 (agotamiento del fondo y protección de la reserva) dentro del orden de evaluación, en `domain/octopus_policy.go`
- [X] T069 [US5] Implementar el reparto configurable y su normalización (`ausente o 0 ⇒ valor de fábrica`) en `domain/octopus_budget.go`
- [X] T070 [US5] Añadir los ajustes de topes y reparto a `application/ports/settings_repository.go` y `adapters/secondary/persistence/settings.go`, con la misma semántica de ausencia que `Budget` y `CompactThreshold`
- [X] T071 [US5] Propagar la marca `Estimated` a la salida legible y a `--json` en `adapters/primary/cli/cmd_octopus.go`

**Punto de control**: el presupuesto se respeta y la reserva de validación está protegida.

---

## Fase 8: Historia 6 — Simular y medir (P6)

**Objetivo**: simulación que no ejecuta nada y telemetría que permite contrastar lo estimado con lo real.

**Prueba independiente**: `./mem octopus plan` no crea ningún proceso; tras reportar una ejecución, `./mem octopus usage` muestra estimado frente a real.

### Pruebas de la Historia 6 ⚠️ (escribir primero, deben fallar)

- [X] T072 [P] [US6] Escribir prueba de contrato del esquema: `octopus_executions` no tiene ninguna columna de texto libre alimentada por contenido (SC-011, INV-AAR-013), en `tests/contract/octopus_schema_test.go`
- [X] T073 [P] [US6] Escribir prueba del repositorio con base de datos real: insertar decisión, completarla con el reporte y leer agregados, en `adapters/secondary/persistence/octopus_test.go`
- [X] T074 [P] [US6] Escribir prueba de que un reporte para una tarea sin decisión previa se ignora sin error (fire-and-forget), en `application/usecases/octopus_report_test.go`
- [X] T075 [P] [US6] Escribir prueba de AC-019: la simulación no inicia ningún subagente ni proceso, en `tests/integration/octopus_dry_run_test.go`
- [X] T076 [P] [US6] Escribir prueba de los agregados de telemetría: conteos por ruta, estimado contra real, reducción de contexto, éxitos, fallos, reintentos, repliegues y ancho de paralelismo, en `application/usecases/octopus_report_test.go`

### Implementación de la Historia 6

- [X] T077 [US6] Añadir `CREATE TABLE IF NOT EXISTS octopus_executions` y su índice, de forma aditiva, en `migrate()` de `adapters/secondary/persistence/db.go`, con el esquema exacto de `data-model.md` §3
- [X] T078 [P] [US6] Definir el puerto `OctopusRepository` en `application/ports/octopus_repository.go`
- [X] T079 [P] [US6] Crear el mock del puerto junto a los adaptadores, siguiendo la convención de mocks del proyecto
- [X] T080 [US6] Implementar el repositorio SQL con parámetros bind y sin exponer `*sql.DB`, en `adapters/secondary/persistence/octopus.go`
- [X] T081 [US6] Cablear el repositorio nuevo en `infrastructure/container.go`
- [X] T082 [US6] Implementar el caso de uso de ingesta de reportes y cálculo de agregados en `application/usecases/octopus_report.go`
- [X] T083 [US6] Registrar las tools `octopus_report` y `octopus_status` y añadir sus nombres a `MCPOctopusTools` en el mismo cambio, en `adapters/primary/cli/cmd_mcp_octopus_tools.go` y `domain/mcp_tools.go`
- [X] T084 [US6] Implementar `mem octopus status`, `usage` e `history` en `adapters/primary/cli/cmd_octopus.go`
- [X] T085 [US6] Añadir el desglose legible de la simulación (qué queda inline, qué se delega, qué corre en paralelo, presupuestos y razón de cada ruta) a `mem octopus plan`, en `adapters/primary/cli/cmd_octopus.go`

**Punto de control**: se puede simular sin ejecutar y medir lo que se ejecutó.

---

## Fase 9: Historia 7 — Sobrevivir a delegaciones fallidas (P7)

**Objetivo**: política acotada de reintento, una única expansión de contexto y repliegue a ejecución inline.

**Prueba independiente**: simular un fallo y comprobar que se recomienda como máximo un reintento y luego repliegue, conservando el resultado parcial.

### Pruebas de la Historia 7 ⚠️ (escribir primero, deben fallar)

- [X] T086 [P] [US7] Escribir prueba de AC-011: con tope de un reintento, un segundo fallo no produce otro reintento automático, en `domain/octopus_result_test.go`
- [X] T087 [P] [US7] Escribir prueba de AC-012: `INSUFFICIENT_CONTEXT` permite una única ampliación acotada; la segunda vez va a repliegue, en `domain/octopus_result_test.go`
- [X] T088 [P] [US7] Escribir prueba de que el repliegue conserva el resultado parcial útil cuando es seguro entregarlo, en `domain/octopus_result_test.go`
- [X] T089 [P] [US7] Escribir prueba de integración del ciclo completo delegar → fallo → reintento → fallo → repliegue inline, en `tests/integration/octopus_failure_test.go`

### Implementación de la Historia 7

- [X] T090 [US7] Implementar la máquina de estados de `data-model.md` §2 con sus topes en `domain/octopus_result.go`
- [X] T091 [US7] Implementar la ampliación única del paquete de contexto a partir de los elementos declarados como faltantes, en `application/usecases/octopus_pack_contract.go`
- [X] T092 [US7] Implementar la decisión de repliegue y la entrega del resultado parcial en `application/usecases/octopus_report.go`

**Punto de control**: un fallo de delegación no atrapa al sistema en un ciclo ni pierde trabajo.

---

## Fase 10: Historia 8 — Aprender de lo ejecutado (P8)

**Objetivo**: la evidencia histórica agregada mejora las estimaciones sin poder saltarse ningún límite duro.

**Prueba independiente**: alimentar evidencia de un patrón, ver que la preferencia por delegar aumenta, y comprobar que una restricción de presupuesto la sigue anulando.

### Pruebas de la Historia 8 ⚠️ (escribir primero, deben fallar)

- [X] T093 [P] [US8] Escribir prueba de AC-014: evidencia favorable mueve el desempate hacia `DELEGATE`, en `domain/octopus_policy_test.go`
- [X] T094 [P] [US8] Escribir prueba de que la evidencia NUNCA salta las reglas 1 a 11 (presupuesto, dependencias, capacidades, seguridad, fan-out y recursión prevalecen), en `domain/octopus_policy_test.go`
- [X] T095 [P] [US8] Escribir prueba de AC-015: con la tabla vacía, toda unidad recibe decisión válida, en `tests/integration/octopus_cold_start_test.go`
- [X] T096 [P] [US8] Escribir prueba de la consulta agregada por clase de tarea (ejecuciones, consumo medio inline y delegado, tasa de éxito) en `adapters/secondary/persistence/octopus_test.go`

### Implementación de la Historia 8

- [X] T097 [US8] Implementar `ClassEvidence` y su uso exclusivo en el desempate de la regla 12, en `domain/octopus_policy.go`
- [X] T098 [US8] Implementar la consulta agregada por `task_class` sobre `octopus_executions` en `adapters/secondary/persistence/octopus.go`
- [X] T099 [US8] Inyectar la evidencia (o `nil` si no hay historial) desde los casos de uso de enrutamiento, en `application/usecases/octopus_route_task.go` y `application/usecases/octopus_route_plan.go`

**Punto de control**: las ocho historias funcionan de forma independiente.

---

## Fase 11: Cierre y asuntos transversales

- [X] T100 [P] Documentar Octopus AAR en `docs/ARQUITECTURA.md`: frontera política contra ejecución, dónde vive cada capa y por qué el dominio no mide texto
- [X] T101 [P] Añadir la sección del módulo y su interruptor al `README.md`
- [X] T102 [P] Registrar la funcionalidad en `CHANGELOG.md`
- [X] T103 Ejecutar la guía completa de `quickstart.md` paso por paso contra el binario recién compilado, no contra la suite: `go build -o mem ./infrastructure` primero
- [X] T104 Verificar cobertura ≥ 80 % con `go test ./... -cover` y cubrir lo que falte
- [X] T105 Ejecutar `gofumpt -l .` y `golangci-lint run` y dejar ambos limpios
- [X] T106 Verificar el rendimiento de SC-004: enrutar un plan de 50 tareas en menos de 1 s, con una prueba de tiempo en `domain/octopus_plan_test.go`
- [X] T107 Repasar la lista completa de invariantes INV-AAR-001 a INV-AAR-019 de `spec.md` y confirmar que cada una tiene al menos una prueba que la respalda; anotar el mapeo en la sección Registro de ejecución

---

## Dependencias y orden de ejecución

### Dependencias entre fases

- **Fase 1 (Preparación)**: sin dependencias
- **Fase 2 (Base)**: depende de la Fase 1. **BLOQUEA todas las historias**
- **Fase 3 (US1)**: depende de la Fase 2
- **Fase 4 (US2)**: depende de la Fase 2. Registra por MCP lo que exista; con solo US1 hecha, registra una tool
- **Fase 5 (US3)**: depende de la Fase 2; comparte archivos con US1 (`octopus_policy.go`, `cmd_octopus.go`), así que **no** conviene ejecutarla en paralelo con la Fase 3
- **Fase 6 (US4)**: depende de la Fase 2; independiente de US3 salvo por la salida de contratos en rutas de plan (T063)
- **Fase 7 (US5)**: depende de la Fase 3 (extiende el orden de evaluación de `octopus_policy.go`)
- **Fase 8 (US6)**: depende de la Fase 2; su parte de persistencia es independiente del resto
- **Fase 9 (US7)**: depende de la Fase 6 (usa el paquete de contexto y el resultado delegado)
- **Fase 10 (US8)**: depende de la Fase 8 (necesita la tabla y la consulta agregada)
- **Fase 11 (Cierre)**: depende de todas las historias que se decidan entregar

### Dentro de cada historia

1. Las pruebas se escriben primero y deben **fallar** antes de implementar
2. Tipos de dominio antes que casos de uso
3. Casos de uso antes que adaptadores (CLI, TUI, MCP)
4. La historia se cierra en su punto de control antes de pasar a la siguiente

### Advertencia sobre paralelismo real

Las marcas `[P]` valen dentro de su fase. Entre fases, tres archivos concentran el conflicto y **no** admiten trabajo simultáneo: `domain/octopus_policy.go` (US1, US5, US8), `adapters/primary/cli/cmd_octopus.go` (US1, US3, US5, US6) y `domain/mcp_tools.go` (US2, US3, US6). Repartir esas historias entre personas a la vez genera conflictos de fusión garantizados.

---

## Ejemplo de ejecución en paralelo: Historia 1

```bash
# Las pruebas de la Historia 1 son archivos y casos independientes:
Tarea: "T015 prueba de tabla del orden de las 13 reglas"
Tarea: "T016 prueba de AC-001 y AC-003"
Tarea: "T017 prueba de AC-002"
Tarea: "T018 prueba de dependencia sin resolver ⇒ WAIT"
Tarea: "T019 prueba de reproducibilidad (100 corridas)"
Tarea: "T020 prueba de arranque en frío"
Tarea: "T021 prueba del caso de uso con TokenCounter falso"

# En la Fase 2, los tipos de dominio son independientes entre sí:
Tarea: "T007 octopus_workunit.go"
Tarea: "T009 octopus_capability.go"
Tarea: "T011 octopus_route.go"
Tarea: "T013 octopus_budget.go"
```

---

## Estrategia de entrega

### Producto mínimo viable

Fases 1, 2, 3 y 4 (Preparación + Base + US1 + US2). Es el corte correcto y **no** solo US1: entregar el enrutador sin su interruptor dejaría encendida por defecto una capacidad grande que el usuario pidió poder apagar, lo que rompe SC-001 e INV-AAR-019. La Historia 2 no es un extra del producto mínimo, es su condición de entrega.

Al terminar: `./mem octopus route` decide y explica, el módulo se enciende y se apaga desde la TUI, y apagado no deja rastro.

### Entrega incremental

1. Fases 1 y 2 → base lista
2. Fases 3 y 4 → **producto mínimo viable**, validar y entregar
3. Fase 5 (US3) → planes completos con dependencias y paralelismo
4. Fase 6 (US4) → contexto mínimo y contratos
5. Fase 7 (US5) → presupuesto y reserva protegida
6. Fase 8 (US6) → simulación y telemetría
7. Fase 9 (US7) → manejo de fallos
8. Fase 10 (US8) → evidencia histórica
9. Fase 11 → cierre

Cada corte añade valor sin romper el anterior.

---

## Notas

- `[P]` significa archivo distinto y sin dependencias pendientes
- Cada tool MCP nueva se añade a `MCPOctopusTools` **en la misma tarea** que la registra, para que el contrato de `tests/contract/mcp_tool_sync_test.go` quede verde en cada punto de control y no al final
- Un test existente se modifica en toda la funcionalidad: T027. Está autorizado y justificado en `plan.md`, nota del principio III. Ningún otro test previo se toca
- El dominio de Octopus no importa `application/ports` ni mide texto: recibe cifras ya calculadas. Romper esto crea un ciclo de imports y destruye la verificabilidad de la política
- Verificar que cada prueba falla antes de implementar; commit por tarea o grupo lógico
- Antes de dar cualquier fase por terminada: recompilar el binario. Validar contra la copia anterior de `mem` es el equivalente a levantar el contenedor sin reconstruir la imagen

---

## Registro de ejecución

| Momento | Comando | Resultado |
|---|---|---|
| Línea base (T001) | `go build -o mem ./infrastructure && go test ./... -count=1` | ✅ verde, 13 paquetes |
| Formato de partida (T002) | `gofmt -l .` | 13 archivos ya divergentes ANTES de esta funcionalidad |
| Cierre del producto mínimo | `quickstart.md` §1–§5 contra el binario | ✅ las 5 secciones |
| Cierre de la funcionalidad | Suite completa + cobertura + formato | ✅ 13 paquetes, dominio 86,6 %, `gofmt` en la línea base exacta |

`gofumpt` y `golangci-lint` no están instalados en este entorno. Se verificó con
`gofmt -l .` y `go vet ./...` (ambos limpios sobre los archivos de esta
funcionalidad), y se dejó constancia para que la verificación con las
herramientas de la constitución se repita donde sí estén disponibles.

### Mapeo de invariantes a pruebas (T107)

| Invariante | Prueba que la respalda |
|---|---|
| INV-AAR-001 no se delega solo porque haya capacidad | `TestRouteTask_OrdenDeEvaluacion` (fila 13), `TestRouteTask_ContextoMinusculoNoAmortizaElArranque` |
| INV-AAR-002 la delegación tiene beneficio que justifica el sobrecosto | `TestRouteTask_OrdenDeEvaluacion` (fila 8), `TestRouteTask_AC001_TareaTrivial` |
| INV-AAR-003 objetivo acotado | `TestWorkUnit_Validate`, `TestExecutionContract_Validate` |
| INV-AAR-004 contexto acotado a la unidad | `TestOctopusContextPack_AislaElContexto`, `TestPackContract_ContratoCompleto` |
| INV-AAR-005 el presupuesto global no se desborda en silencio | `TestRouteTask_AC007_PresupuestoInsuficiente`, `TestRoutePlan_ConsumeElFondoSinDesbordarlo` |
| INV-AAR-006 la reserva de validación se protege | `TestRouteTask_AC008_ReservaProtegida`, `TestBudget_CabeNuncaTocaLaReserva` |
| INV-AAR-007 sin dependencias entre tareas paralelas | `TestRoutePlan_AC005_DependenciaNoComparteGrupo`, `TestRoutePlan_DependenciaTransitivaNoComparteGrupo` |
| INV-AAR-008 se respeta el límite de concurrencia | `TestRoutePlan_TopeDeConcurrencia` |
| INV-AAR-009 recursión acotada | `TestRoutePlan_AC010_ProfundidadMaxima`, `TestNewExecutionContract_ProfundidadRestante` |
| INV-AAR-010 fan-out acotado | `TestRoutePlan_AC009_TopeDeAgentes` |
| INV-AAR-011 sin reintentos indefinidos | `TestNextAfterFailure_ReintentosAcotados`, `TestOctopusCicloDeFalloCompleto` |
| INV-AAR-012 sin transcripciones en el contexto del padre | `TestDelegatedResult_Compactar` |
| INV-AAR-013 sin razonamiento privado persistido | `TestOctopusExecutions_EsquemaSinTextoLibreDeContenido` |
| INV-AAR-014 sin elevación de privilegios | `TestPermissions_NoElevacion`, `TestPackContract_NoElevaPrivilegios` |
| INV-AAR-015 utilizable sin aprendizaje | `TestOctopusArranqueEnFrio`, `TestReportUseCase_SinRepositorioNoRevienta` |
| INV-AAR-016 utilizable sin conteo exacto | `TestRouteTaskUseCase_SinContadorSigueDecidiendo`, `TestBudget_SinDeclararEsIlimitado` |
| INV-AAR-017 sin proveedor de modelo específico | `TestRouteTask_ArranqueEnFrio` (la política no invoca ningún modelo) |
| INV-AAR-018 Octopus no ejecuta | `TestOctopusPlan_NoIniciaNingunProceso` |
| INV-AAR-019 apagado = huella cero | `TestOctopusApagado_NoRegistraNingunaTool`, `TestProtocolo_NoMencionaOctopusConElModuloApagado`, `TestOctopusModuloApagado` |

Las 19 invariantes tienen al menos una prueba. Ninguna quedó solo declarada.
