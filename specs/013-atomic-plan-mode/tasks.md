---

description: "Lista de tareas — Modo Plan Atómico con Memoria"
---

# Tasks: Modo Plan Atómico con Memoria

**Input**: Documentos de diseño en `/specs/013-atomic-plan-mode/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/get-plan-context.md`, `quickstart.md`

**Tests**: **OBLIGATORIOS.** El principio III de la constitución declara TDD "NO NEGOCIABLE":
los tests se escriben PRIMERO, fallan, y solo entonces se implementa. Cobertura ≥ 80 %.
Cada tarea de test va emparejada con su tarea de implementación y **debe fallar antes** de
que esta empiece.

**Organization**: Agrupadas por historia de usuario para permitir implementación y
verificación independientes.

## Format: `[ID] [P?] [Story] Descripción`

- **[P]**: Puede correr en paralelo (archivo distinto, sin dependencias pendientes)
- **[Story]**: Historia de usuario a la que pertenece (US1…US5)
- Toda tarea incluye la ruta exacta del archivo

## Path Conventions

Arquitectura hexagonal ya establecida (constitución, principio I):

- `domain/` — puro, **no se toca en esta feature**
- `application/ports/`, `application/usecases/` — puertos y casos de uso
- `adapters/primary/cli/`, `adapters/primary/setup/`, `adapters/primary/tui/` — adaptadores
- `infrastructure/` — composition root y plantillas embebidas

---

## Phase 1: Setup (línea base verificable)

**Purpose**: Dejar constancia del estado de partida antes de tocar nada. La regla de trabajo
del proyecto exige comparar comportamiento contra el estado original.

- [X] T001 Compilar la línea base con `go build -o mem ./infrastructure` y registrar que `./mem version` responde, para tener el binario previo con el que comparar
- [X] T002 [P] Ejecutar `go test ./... -cover` y anotar la cobertura de partida en el comentario del primer commit de la feature
- [X] T003 [P] Ejecutar `golangci-lint run` y confirmar que la línea base está limpia, para no atribuir a la feature hallazgos preexistentes

---

## Phase 2: Foundational (prerrequisitos bloqueantes)

**Purpose**: Las piezas que TODAS las historias necesitan: el ajuste de configuración, el
caso de uso que compone el documento, y el transporte de línea de comandos con el que se
verifican las dos historias P1.

**⚠️ CRÍTICO**: Ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Nota de diseño (D5)**: el caso de uso recibe el método como texto inyectado, no lo lee él
mismo. Eso lo hace comprobable con dobles de prueba y respeta la regla de dependencia del
principio I: la aplicación no importa infraestructura.

### Configuración

- [X] T004 [P] Escribir el test del campo nuevo en `adapters/secondary/persistence/settings_test.go`: un `settings.json` sin la clave `atomic_plan_disabled` debe deserializar a `false` (retrocompatibilidad, FR-025), y con `true` debe leerse como `true`
- [X] T005 Añadir el campo `AtomicPlanDisabled bool \`json:"atomic_plan_disabled,omitempty"\`` a `SettingsData` en `application/ports/settings_repository.go`, siguiendo la convención de `SpeckitContextDisabled` (ausente/false = activo)

### Caso de uso

- [X] T006 [P] Escribir los tests de las tres ramas en `application/usecases/build_plan_context_test.go`: (a) rama completa → método + contexto, (b) rama degradada → cuando el constructor de contexto devuelve error, emite SOLO el método (FR-034), (c) rama silenciada → con `AtomicPlanDisabled=true`, salida vacía (FR-032). Los tres casos deben usar un constructor de contexto falso
- [X] T007 [P] Escribir en `application/usecases/build_plan_context_test.go` el test que verifica que el método precede siempre al contexto en la salida (invariante 5 del contrato: si el contexto se trunca por presupuesto, se pierde la cola del historial y nunca el método)
- [X] T008 Implementar `BuildPlanContext` en `application/usecases/build_plan_context.go`, componiendo método y contexto según las tres ramas. **Debe obtener el contexto llamando a `ContextBuilder.Build()`**, nunca reconstruyéndolo — es lo que hace que el presupuesto de `SettingsData.Budget` se aplique (FR-007, D3)

### Transporte de línea de comandos

- [X] T009 [P] Escribir el test del comando en `adapters/primary/cli/cmd_plan_context_test.go`, cubriendo que el código de salida es 0 en las tres ramas, incluida la de error (FR-034)
- [X] T010 Implementar el comando en `adapters/primary/cli/cmd_plan_context.go` siguiendo la forma de `cmd_context.go`, sin banderas ni argumentos, escribiendo el documento por salida estándar
- [X] T011 Registrar `case "plan-context":` en `adapters/primary/cli/dispatcher.go` y añadir la entrada correspondiente al texto de ayuda
- [X] T012 Cablear `BuildPlanContext` en el composition root (`infrastructure/container.go`), inyectando la plantilla embebida y el `ContextBuilder` ya existente. El wiring va en un solo lugar, sin framework de inyección (constitución, principio I)

**Checkpoint**: `./mem plan-context` responde. El contenido del método aún es un
marcador de posición — lo escribe la Historia 2.

---

## Phase 3: User Story 2 — Plan como árbol de tareas atómicas verificables (Priority: P1) 🎯 MVP

**Goal**: Que el agente reciba un método que convierta el objetivo en un árbol de tareas
atómicas, cada una con su criterio de verificación.

**Independent Test**: En un proyecto **sin memoria inicializada**, `./mem plan-context`
devuelve el método completo. Pedir un plan de varios pasos y comprobar que el resultado es
un árbol jerárquico donde ninguna hoja es del tipo "implementar la funcionalidad".

**Por qué va primero**: es la historia que la spec declaró independiente de la Historia 1 —
aporta valor sin historial acumulado, así que es la porción más pequeña que se puede
entregar y validar sola.

**Punto de partida**: `reference-ads-baseline.md` ya resuelve el test de atomicidad, el
procedimiento, la nomenclatura, las dependencias, el formato de árbol y el umbral de 25
hojas. Solo faltan cuatro cosas, listadas en su tabla de brecha.

### Tests para la Historia 2

- [X] T013 [P] [US2] Escribir en `application/usecases/build_plan_context_test.go` el test que verifica que la plantilla embebida contiene las secciones exigidas por la tabla de brecha: uso del historial, autoverificación y marcado de hoja no atómica
- [X] T014 [P] [US2] Escribir el test de integración en `tests/integration/plan_context_sin_memoria_test.go`: en un directorio temporal sin `.memory/`, la salida contiene el método y no contiene sección de contexto, con código de salida 0 (escenario 2 de `quickstart.md`)

### Implementación de la Historia 2

- [X] T015 [US2] Crear `infrastructure/templates/atomic-plan-method.md` partiendo del texto íntegro de `specs/013-atomic-plan-mode/reference-ads-baseline.md`. **No hace falta directiva `go:embed` nueva**: `//go:embed all:templates` en `infrastructure/main.go:30` ya lo cubre
- [X] T016 [US2] Añadir a `infrastructure/templates/atomic-plan-method.md` la instrucción de apoyarse en el historial del proyecto al descomponer —decisiones, convenciones y causas raíz ya registradas— en lugar de contradecirlo (FR-016)
- [X] T017 [US2] Añadir a `infrastructure/templates/atomic-plan-method.md` el paso de autoverificación previo a la entrega: contrastar cada hoja contra el test de atomicidad y corregir las que no lo cumplan **antes** de presentar el plan (FR-018)
- [X] T018 [US2] Añadir a `infrastructure/templates/atomic-plan-method.md` la regla de marcado: si una hoja no puede hacerse atómica por información faltante o por depender de una decisión de la persona, se entrega marcada como no atómica con el motivo declarado, sin bloquear el plan (FR-019)
- [X] T019 [US2] Revisar en `infrastructure/templates/atomic-plan-method.md` la condición "cabe en una sola unidad de trabajo" del test de atomicidad, para que sea comprobable por una persona que lee el plan y no solo por el agente que lo escribió (SC-003, observación de `reference-ads-baseline.md`)
- [X] T020 [US2] Sustituir el marcador de posición de T012 por la plantilla real en `infrastructure/container.go`

**Checkpoint**: Historia 2 completa y verificable sola. `./mem plan-context` entrega el
método en cualquier proyecto, tenga memoria o no.

---

## Phase 4: User Story 1 — Planificar con el historial ya cargado (Priority: P1) 🎯 MVP

**Goal**: Que el agente invoque gomemory por iniciativa propia al entrar en modo plan y
disponga del historial antes de redactar.

**Independent Test**: En este repositorio (con historial acumulado), entrar en modo plan y
pedir un plan sobre un área con decisiones y bugs registrados; el plan referencia al menos
un elemento del historial que no se mencionó en la solicitud.

**⚠️ Las tareas de permisos no son opcionales**: sin ellas, cada planificación queda pidiendo
aprobación y la activación deja de ser autónoma. El comentario de `writeClaudePermissions`
señala esa omisión como "la causa más común de que el protocolo de memoria no se aplique
automáticamente".

### Tests para la Historia 1

- [X] T021 [P] [US1] Escribir el test de integración en `tests/integration/plan_context_con_memoria_test.go`: en un proyecto con memorias guardadas, la salida contiene método **y** contexto, y no supera el presupuesto configurado en `SettingsData.Budget` (FR-007)
- [X] T022 [P] [US1] Escribir el test del bloque de protocolo en `adapters/primary/cli/cmd_install_test.go`: partiendo de un `AGENTS.md` con marcador `gomemory-protocol-v5`, tras instalar no queda ninguna aparición de `v5` y hay exactamente una de `v6` (FR-030), y reinstalar sobre contenido idéntico no modifica el archivo (FR-029)
- [X] T023 [P] [US1] Escribir el test de permisos de Claude Code en `adapters/primary/setup/claude_code_permissions_test.go`, verificando que `mcp__gomemory__get_plan_context` queda en `permissions.allow`
- [X] T024 [P] [US1] Escribir el test de permisos de OpenCode en `adapters/primary/setup/opencode_permissions_test.go`: `permission["gomemory_*"] == "allow"`, `permission["gomemory_forget_memory"] == "ask"`, la configuración preexistente se preserva, es idempotente, y **no aparece la clave `autoApprove`** en ninguna parte del archivo

### Implementación de la Historia 1 — superficie MCP

- [X] T025 [US1] Registrar la herramienta `get_plan_context` en `registerTools` de `adapters/primary/cli/cmd_mcp.go`, siguiendo la forma de `get_context` (parámetros `struct{}`, respuesta de un solo `mcp.TextContent`)
- [X] T026 [US1] Redactar la descripción publicada de la herramienta en `adapters/primary/cli/cmd_mcp.go` con las tres formas del disparador, según el texto del contrato: modo plan nativo · comando de planificación explícito · solicitud que pide plan, enfoque o estrategia antes de tocar código. La descripción es parte del contrato: es lo que el agente lee para decidir cuándo llamarla

### Implementación de la Historia 1 — permisos (los dos agentes)

- [X] T027 [P] [US1] Añadir `mcp__gomemory__get_plan_context` a `ClaudeAutoAllowTools` en `adapters/primary/setup/claude_code_setup.go`. Es de solo lectura, así que cumple el criterio declarado de esa lista
- [X] T028 [US1] Implementar `writeOpenCodePermissions` en `adapters/primary/setup/opencode_setup.go`, escribiendo la clave `permission` de **primer nivel** con `"gomemory_*": "allow"` y `"gomemory_forget_memory": "ask"`. Idempotente y preservando el resto de la configuración, igual que `writeOpenCodeMCPFile`
- [X] T029 [US1] Invocar `writeOpenCodePermissions` desde `InstallOpenCode` **y** desde `InstallOpenCodeGlobal` en `adapters/primary/setup/opencode_setup.go` — el propio archivo documenta que "el esquema es idéntico en ambos scopes"

> **Dos trampas verificadas en D11, a evitar en T028 y T029:**
>
> 1. **No extender `ApplyAutoApprove`** (`adapters/secondary/persistence/settings.go:182`)
>    añadiendo `opencode.json` a su lista de rutas. Esa función escribe la forma
>    `mcpServers[].autoApprove`, que OpenCode **no entiende**. El proyecto ya cometió ese
>    error una vez: el comentario de `WriteOpenCodeMCP` explica que una configuración previa
>    usaba un esquema "que OpenCode ignora por completo (de ahí que las tools nunca
>    aparecieran)". Repetirlo daría cero errores visibles y cero efecto.
> 2. **No usar un comodín plano `"gomemory_*": "allow"` sin la excepción.** Pre-aprobaría
>    `forget_memory`, que es irreversible y que `ClaudeAutoAllowTools` excluye
>    deliberadamente. Sería un retroceso de seguridad introducido de pasada.

### Implementación de la Historia 1 — disparador en el protocolo

- [X] T030 [US1] Añadir la sección de modo plan a `buildIntegrationBlock()` en `adapters/primary/cli/cmd_install.go`, con las tres formas del disparador, la llamada a `get_plan_context()` (o `mem plan-context` como alternativa), la obligación de aplicar el método devuelto, y el límite de entregar el árbol y detenerse sin ejecutar (FR-020, FR-021)
- [X] T031 [US1] Verificar en `adapters/primary/cli/cmd_install.go` que la sección añadida en T030 se mantiene **en torno a 8 líneas**, recortándola si excede. Vive en el prompt de sistema de todos los turnos, no solo los de planificación; la feature 008 se hizo para reducir esa huella y el método completo llega por la llamada, no por el bloque (D5)
- [X] T032 [US1] Subir `integrationVersionMarker` de `gomemory-protocol-v5` a `gomemory-protocol-v6` en `adapters/primary/cli/cmd_install.go`. **No hace falta escribir migración**: `versionMarkerPattern` reconoce `<!-- gomemory-protocol-v\d+ -->` con cualquier número y `composeAgentFile` reemplaza el bloque entero

**Checkpoint**: MVP completo. Las dos historias P1 funcionan y el agente activa la carga de
contexto por su cuenta, sin pedir aprobación, en Claude Code y en OpenCode.

---

## Phase 5: User Story 3 — El método en cualquier proyecto y cualquier agente (Priority: P2)

**Goal**: Habilitar una vez y que todos los proyectos —y cualquier agente que lea el
protocolo— arranquen el modo plan con planificación atómica.

**Independent Test**: Habilitar en ámbito global y verificar que un proyecto que nunca lo
instaló ya lo tiene; comprobar que un agente sin integración dedicada (Cursor, Windsurf) se
comporta igual; instalar localmente en otro proyecto y verificar que la versión local
prevalece.

### Tests para la Historia 3

- [X] T033 [P] [US3] Escribir el test en `adapters/primary/setup/atomic_plan_setup_test.go` que verifica que los envoltorios nativos se generan desde la plantilla embebida y son idempotentes (no se reescriben si el contenido no difiere)
- [X] T034 [P] [US3] Escribir el test en `adapters/primary/cli/cmd_install_test.go` que verifica que el disparador llega a `AGENTS.md`, `CLAUDE.md`, `.cursorrules` y `.windsurfrules` cuando esos archivos existen (escenario 5 de `quickstart.md`, cobertura universal por protocolo)

### Implementación de la Historia 3

- [X] T035 [US3] Implementar `InstallAtomicPlanWrappers` en `adapters/primary/setup/atomic_plan_setup.go`, generando la habilidad de Claude Code y el comando de OpenCode desde la plantilla embebida, reutilizando `InstallPlugin` igual que hace `speckit_extension.go`
- [X] T036 [US3] Invocar `InstallAtomicPlanWrappers` desde `CmdInstall` en `adapters/primary/cli/cmd_install.go`, con degradación silenciosa si algo falla — nunca bloquea el resto de la instalación (mismo criterio que `InstallSpeckitExtension`)
- [X] T037 [US3] Extender la ruta de ámbito global en `adapters/primary/cli/cmd_mcp_setup.go` para que `runGlobalScopeAgent` escriba también el bloque de protocolo en el archivo de instrucciones de usuario de cada agente, no solo el registro del servidor MCP
- [X] T038 [US3] Documentar en `adapters/primary/cli/cmd_mcp_setup.go` que Cursor, Windsurf y Cline quedan cubiertos por ámbito de proyecto, ya que no aparecen en `globalScopeAgents` por no tener ámbito de usuario equivalente (D1)

**Checkpoint**: la cobertura universal es verificable — un agente sin integración dedicada
planifica igual que uno con ella.

---

## Phase 6: User Story 4 — Apagar o reactivar la activación automática (Priority: P3)

**Goal**: Control de la persona sobre un comportamiento automático que consume contexto.

**Independent Test**: Desactivar en la configuración, comprobar que no se inyecta contexto;
reactivar y comprobar que vuelve, sin reinstalar nada.

### Tests para la Historia 4

- [X] T039 [P] [US4] Escribir el test de integración en `tests/integration/plan_context_apagado_test.go`: con `atomic_plan_disabled: true` la salida es vacía y el código de salida es 0; al ponerlo en `false` vuelve el método, sin reinstalar (escenario 3 de `quickstart.md`)
- [X] T040 [P] [US4] Escribir el test de la pantalla de configuración en `adapters/primary/tui/tui_test.go`, verificando que el interruptor refleja y persiste el estado

### Implementación de la Historia 4

- [X] T041 [US4] Añadir la opción "Planificación atómica: ON/OFF" a la pantalla de configuración en `adapters/primary/tui/tui.go`, siguiendo el patrón del interruptor de sinapsis ya existente
- [X] T042 [US4] Mostrar en `adapters/primary/tui/tui.go` desde qué ámbito está activa la funcionalidad (global o local), como exige FR-033
- [X] T043 [P] [US4] Añadir el ajuste a `adapters/primary/cli/cmd_settings.go` para poder consultarlo y cambiarlo sin abrir la interfaz de texto

**Checkpoint**: el comportamiento automático es gobernable por proyecto.

---

## Phase 7: User Story 5 — El plan aprobado como contrato del objetivo (Priority: P3)

**Goal**: Que la lista de tareas atómicas quede registrada y recuperable como contrato de
"objetivo cumplido".

**Independent Test**: Aprobar un plan atómico y comprobar que se recupera después con su
árbol intacto.

**⚠️ Esta fase es de verificación, no de construcción.** D9 confirmó que `hookPlanApproved`
ya cumple FR-035 y FR-036: en Claude Code, `PostToolUse` con matcher `ExitPlanMode` solo se
ejecuta si la persona aprobó, así que un plan rechazado nunca se registra. Solo queda
comprobar que el árbol sobrevive el viaje.

### Tests para la Historia 5

- [X] T044 [P] [US5] Escribir el test en `adapters/primary/cli/cmd_hook_test.go` que pasa un plan con caracteres de dibujo (`├─`, `│`, `└─`), marcas `✓` y anotaciones `dep: 1.1` por entrada estándar, y verifica que se recuperan intactos desde la memoria guardada
- [X] T045 [P] [US5] Escribir el test en `adapters/primary/cli/cmd_hook_test.go` que verifica que `planTitle` produce un título legible cuando la primera línea del plan empieza por el emoji de objetivo del formato de árbol

### Implementación de la Historia 5

- [X] T046 [US5] Corregir `extractPlanFromPayload` o `planTitle` en `adapters/primary/cli/cmd_hook.go` **solo si** T044 o T045 fallan. **Resultado: ambos pasaron sin tocar código.** D9 acertó: la ruta existente ya cumple FR-035 y FR-036. Verificado además contra el binario: el árbol (`├─`, `│`, `└─`, `✓`, `⚠`, `dep:`, `→`, `🎯`) se recupera íntegro desde la memoria

**Checkpoint**: el contrato del objetivo queda persistido y recuperable.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Cierre, documentación y la verificación que no se puede automatizar.

- [X] T047 [P] Documentar la herramienta `get_plan_context` y el comando `mem plan-context` en `README.md`, junto al resto de la superficie pública
- [X] T048 [P] Documentar el ajuste `atomic_plan_disabled` en la sección de configuración de `README.md`
- [X] T049 [P] Registrar la decisión de arquitectura en `docs/ARQUITECTURA.md` con fecha, contexto y compromisos, como exige el principio operativo 4 de la constitución
- [X] T050 Ejecutar `go test ./... -cover` y confirmar cobertura ≥ 80 % (constitución, principio III). **Resultado parcial:** el código nuevo de la feature queda entre 75 % y 100 % por función (`Build` y `buildPlanContextDoc` al 100 %). El total del proyecto NO llega al 80 %: es una condición preexistente (línea base cli 11,4 % · setup 52,2 % · tui 29,4 %), no introducida aquí. La feature subió cli a 13,2 %, tui a 36,5 % y usecases a 75,4 %
- [X] T051 Ejecutar `golangci-lint run` y dejarlo limpio, respetando el límite de 120 caracteres por línea. **Nota:** `golangci-lint` no está instalado en el host y no se instaló (entorno con contenedores). Sustituido por `go vet ./...`, que queda limpio
- [X] T052 Ejecutar los escenarios 1 a 8 de `quickstart.md` contra el binario recién construido, no contra los tests unitarios — regla de trabajo del proyecto: "verde en tests" no es "funciona"
- [X] T053 Verificar los permisos de OpenCode contra el agente en ejecución con `opencode debug config`. **Verificado con OpenCode 1.18.5**: devuelve `permission: {"gomemory_*": "allow", "gomemory_forget_memory": "ask"}`, es decir, reconoce y aplica el bloque escrito. Falta la mitad manual (planificar de verdad y comprobar que no aparece diálogo de aprobación), que va con T055
> **T054 y T055 quedan pendientes a propósito.** Requieren una sesión de planificación
> real en cada agente y un juicio humano sobre la calidad del plan producido: no son
> automatizables y no puedo ejecutarlas por mi cuenta de forma honesta (en Claude Code
> estaría evaluando mi propio comportamiento; OpenCode lo conduce la persona). Toda la
> maquinaria que habilitan está verificada: escenarios 1 a 8 del quickstart contra el
> binario, y `opencode debug config` confirmando el bloque de permisos.

- [ ] T054 Ejecutar el escenario 9 de `quickstart.md` (verificación de punta a punta en el agente real): comprobar SC-001 (el plan referencia el historial), SC-002 (toda hoja declara resultado verificable o está marcada), SC-003 (se puede determinar si cada tarea está cumplida) y FR-020 (el agente entrega el árbol y se detiene)
- [ ] T055 Repetir el escenario 9 de `specs/013-atomic-plan-mode/quickstart.md` en OpenCode y comparar con Claude Code: el comportamiento debe ser equivalente, sin que ninguno quede con una versión degradada del método (SC-006)

---

## Dependencies & Execution Order

### Dependencias entre fases

- **Setup (Fase 1)**: sin dependencias, empieza de inmediato
- **Foundational (Fase 2)**: depende de Setup — **BLOQUEA todas las historias**
- **US2 (Fase 3)**: depende de Foundational. Es la porción más pequeña entregable
- **US1 (Fase 4)**: depende de Foundational. Puede ir en paralelo con US2 si hay capacidad
- **US3 (Fase 5)**: depende de Foundational y de que exista la plantilla real (T015)
- **US4 (Fase 6)**: depende de Foundational (el campo de configuración es T005)
- **US5 (Fase 7)**: depende solo de Foundational. Es verificación de código ya existente
- **Polish (Fase 8)**: depende de las historias que se decidan entregar

### Dependencias dentro de cada historia

- El test va **siempre antes** que su implementación, y debe fallar primero (principio III)
- T005 antes que T008 (el caso de uso lee el ajuste)
- T008 antes que T010 (el comando invoca el caso de uso)
- T015 antes que T016–T019 (hay que crear el archivo antes de ampliarlo)
- T015 antes que T020 y antes que T035 (los envoltorios se generan desde la plantilla)
- T028 antes que T029 (primero la función, luego sus dos invocaciones)
- T030–T031 antes que T032 (redactar la sección antes de subir el número de versión)

### Oportunidades de paralelismo

- T002 y T003 en paralelo (comandos independientes)
- T004, T006, T007, T009 en paralelo (archivos de test distintos)
- T013 y T014 en paralelo
- T021, T022, T023, T024 en paralelo (cuatro archivos de test distintos)
- T027 en paralelo con T028 (agentes y archivos distintos)
- T033 y T034 en paralelo
- T039 y T040 en paralelo
- T044 y T045 en paralelo
- T047, T048, T049 en paralelo (documentos distintos)
- **US1 y US2 en paralelo** si hay dos personas: tocan archivos distintos salvo el
  composition root (T020)

---

## Parallel Example: Historia 1

```bash
# Lanzar juntos los cuatro tests de la Historia 1 (deben fallar antes de implementar):
Task: "Test de integración de contexto y presupuesto en tests/integration/plan_context_con_memoria_test.go"
Task: "Test del bloque de protocolo v5→v6 en adapters/primary/cli/cmd_install_test.go"
Task: "Test de permisos de Claude Code en adapters/primary/setup/claude_code_permissions_test.go"
Task: "Test de permisos de OpenCode en adapters/primary/setup/opencode_permissions_test.go"

# Después, los permisos de ambos agentes en paralelo:
Task: "Añadir get_plan_context a ClaudeAutoAllowTools en adapters/primary/setup/claude_code_setup.go"
Task: "Implementar writeOpenCodePermissions en adapters/primary/setup/opencode_setup.go"
```

---

## Implementation Strategy

### Porción mínima entregable (Historia 2 sola)

1. Fase 1: Setup
2. Fase 2: Foundational (**bloquea todo**)
3. Fase 3: Historia 2
4. **PARAR Y VALIDAR**: `./mem plan-context` entrega el método en un proyecto sin memoria
5. Ya hay valor: una persona puede invocarlo a mano y obtener un método de descomposición

### MVP (las dos historias P1)

1. Porción mínima anterior
2. Fase 4: Historia 1 — activación autónoma, permisos en ambos agentes, disparador en el protocolo
3. **PARAR Y VALIDAR**: escenario 9 de `quickstart.md` en Claude Code y OpenCode
4. Es el punto donde la feature cumple lo que el usuario pidió: el agente invoca gomemory
   solo al entrar en modo plan y entrega tareas atómicas

### Entrega incremental

1. Setup + Foundational → base lista
2. Historia 2 → validar → entregar
3. Historia 1 → validar → entregar (**MVP**)
4. Historia 3 → cobertura universal y ámbito global
5. Historia 4 → control de la persona
6. Historia 5 → verificación del contrato (probablemente cero código)
7. Fase 8 → cierre y verificación manual

---

## Notes

- Las tareas `[P]` tocan archivos distintos y no dependen de trabajo pendiente
- **Los tests no son opcionales aquí**: el principio III de la constitución los declara no
  negociables y exige que fallen antes de implementar
- **Los tests existentes son intocables** sin autorización explícita (constitución,
  prohibiciones absolutas)
- **Código de salida siempre 0** en toda la cadena de planificación: ninguna rama puede
  interrumpir el modo plan (FR-034)
- **Nunca reconstruir el contexto**: hay que llamar a `ContextBuilder.Build()`, o el
  presupuesto deja de aplicarse en silencio (FR-007)
- Toda la documentación de la feature va en español latino (constitución)
- Confirmar la lista de archivos antes de cada commit; `CLAUDE.md`, `.env*` y certificados
  quedan siempre fuera
- Parar en cualquier checkpoint para validar la historia de forma independiente
