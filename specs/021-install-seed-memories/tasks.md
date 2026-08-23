---
description: "Lista de tareas — Feature 021"
---

# Tasks: Instalación sin artefactos — reglas y constitución como memorias semilla

**Input**: Documentos de diseño en `/specs/021-install-seed-memories/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: **OBLIGATORIOS**. No es una opción en este proyecto: la constitución declara
TDD *NO NEGOCIABLE* (principio III) — el test se escribe primero, falla, y solo
entonces se implementa. Cada tarea de implementación va precedida de la suya.

**Organization**: agrupadas por historia de usuario para que cada una sea implementable
y verificable por separado.

## Format: `[ID] [P?] [Story] Descripción`

- **[P]**: paralelizable (archivo distinto, sin dependencias pendientes)
- **[Story]**: `[US1]`..`[US5]` — solo en las fases de historia

## Path Conventions

Proyecto Go único con arquitectura hexagonal, rutas desde la raíz del repositorio:
`domain/`, `application/{ports,usecases}/`, `adapters/{primary,secondary}/`,
`infrastructure/`, `tests/{unit,integration,contract}/`.

---

## Phase 1: Setup

**Purpose**: línea base verificada y catálogo de dominio, del que dependen tres historias.

- [X] T001 Registrar la línea base verde ejecutando `go build ./... && go vet ./... && go test ./...` y anotar el conteo de paquetes en verde, para poder distinguir después una regresión propia de una preexistente
- [X] T002 [P] Escribir el test del catálogo en `domain/seed_test.go`: alias únicos y no vacíos, claves de tópico únicas con prefijo `gomemory:`, y un nombre de plantilla por entrada
- [X] T003 Crear `domain/seed.go` con las constantes `TopicWorkRules` / `TopicConstitution`, el tipo `PinnedDoc` y el catálogo `PinnedDocs` según [contracts/pinned-docs.md](./contracts/pinned-docs.md) §1

**Checkpoint**: el catálogo compila y sus invariantes están cubiertos.

---

## Phase 2: Foundational (Prerequisitos bloqueantes)

**Purpose**: dos correcciones de defecto y el seam compartido. Bloquean TODAS las historias.

**⚠️ CRÍTICO**: sembrar antes de T013 publicaría la constitución en el ADR externo de
quien tenga `adr_sync_enabled=true`; emitir la sección antes de T005 duplicaría las
reglas en el contexto. Ninguna historia puede empezar hasta cerrar esta fase.

### A0 — Defecto latente de la clave de tópico (FR-030)

- [X] T004 [P] Escribir el test de regresión en `adapters/secondary/persistence/memory_test.go`: tras guardar una memoria con `topic_key`, `ListMemories` debe devolverla con `TopicKey` poblado. **Debe fallar hoy** — es la prueba de que el defecto existe
- [X] T005 Añadir `COALESCE(topic_key,'')` al `SELECT` y su destino al `Scan` de `ListMemories` en `adapters/secondary/persistence/memory.go` (~línea 488), alineándola con `ListAllMemories`

### Consulta por clave de tópico (FR-006, FR-031)

- [X] T006 [P] Escribir el test de `GetMemoryByTopicKey` en `adapters/secondary/persistence/memory_test.go`: devuelve la memoria cuando existe, `(nil, nil)` cuando no —nunca un error de "no encontrado"—, y no cruza proyectos
- [X] T007 Implementar `GetMemoryByTopicKey(db, project, topicKey)` en `adapters/secondary/persistence/memory.go`, con la misma consulta parametrizada que usa `findDuplicate` (~línea 317)
- [X] T008 [P] Declarar el puerto `MemoryTopicQuerier` en `application/ports/memory_topic.go` según [contracts/seed-defaults.md](./contracts/seed-defaults.md) §Puerto requerido
- [X] T009 Exponer `ByTopicKey` en `MemoryRepository` en `adapters/secondary/persistence/repositories.go`

### A1 — Vía de inserción inerte (FR-032, FR-033, FR-034)

- [X] T010 [P] Escribir el test G2 en `adapters/secondary/persistence/memory_test.go`: tras `InsertSeedMemory`, `memory_relations` no tiene filas nuevas, incluso con `SetSynapseEnabled(true)` y una sesión activa
- [X] T011 [P] Escribir el test G3 en `adapters/secondary/persistence/memory_test.go`: con `SetAdrSyncEnabled(true)` y un proveedor ADR simulado, `InsertSeedMemory` de una memoria `architecture` no registra ningún intento de exportación
- [X] T012 Extraer el núcleo `insertMemory(db, m, opts)` desde `InsertMemory` en `adapters/secondary/persistence/memory.go`, dejando `InsertMemory` como `insertMemory(db, m, insertOpts{})` sin ningún cambio de comportamiento
- [X] T013 Añadir `InsertSeedMemory(db, m)` sobre el núcleo en `adapters/secondary/persistence/memory.go`: omite `formSynapse` y `exportToADR`, y **conserva** `RedactPrivate`/`RedactSecrets` (data-model §3bis)
- [X] T014 [P] Declarar el puerto `MemorySeeder` en `application/ports/memory_seeder.go`
- [X] T015 Exponer `InsertSeed` en `MemoryRepository` en `adapters/secondary/persistence/repositories.go`

### Resolución de documentos fijados (compartida por US4 y US5)

- [X] T016 [P] Escribir el test de `ResolvePinnedDoc` en `application/usecases/pinned_docs_test.go`: devuelve el contenido de la memoria cuando existe, el de reserva cuando no —marcando el origen—, y error cuando no hay ninguno
- [X] T017 Implementar `ResolvePinnedDoc(topics, project, topicKey, fallback)` en `application/usecases/pinned_docs.go`. Recibe el texto de reserva por parámetro: el caso de uso **no** importa el sistema de archivos embebido (constitución, principio I)

**Checkpoint**: los dos defectos corregidos y el seam disponible. Las historias pueden empezar.

---

## Phase 3: User Story 1 — Reglas y constitución en la memoria (Priority: P1) 🎯 MVP

**Goal**: las semillas existen, sobreviven a la edición de la persona y las reglas llegan íntegras al agente en cada sesión.

**Independent Test**: en un proyecto sin memoria previa, provocar el primer uso y comprobar con `mem context` que `## Reglas de trabajo (memoria fijada)` aparece con el texto completo — quickstart §3 y §8.

### Tests for User Story 1 ⚠️ (escribir primero, deben fallar)

- [X] T018 [P] [US1] Test de `SeedDefaults` en `application/usecases/seed_defaults_test.go` cubriendo C1..C6 de [contracts/seed-defaults.md](./contracts/seed-defaults.md): contenido vacío omitido, existente intacto, inserción cuando falta, `topics` nil sin pánico, error de consulta propagado sin abortar el resto, y `created` vacío en la segunda llamada
- [X] T019 [P] [US1] Test G1 en `application/usecases/seed_defaults_test.go`: el contenido persistido es idéntico carácter por carácter al texto de origen. Es el guardián contra una edición futura de plantilla que active un patrón de secreto
- [X] T020 [P] [US1] Test en `application/usecases/build_context_test.go`: con `Budget=24000`, la sección fijada emite el contenido **íntegro**, sin `→ get_memory` ni recorte a 200 caracteres
- [X] T021 [P] [US1] Test en `application/usecases/build_context_test.go`: el título de la semilla de reglas no aparece en `## Preferencias del Usuario` (depende de T005 para que la comparación por clave funcione)
- [X] T022 [P] [US1] Test en `application/usecases/build_context_test.go`: con `IndexMode=true` la sección colapsa a `→ get_memory <id>`, sin excepción
- [X] T023 [P] [US1] Test en `application/usecases/build_context_test.go`: con 200 memorias creadas después de la semilla, la sección fijada sigue presente (FR-031)
- [X] T024 [P] [US1] Test en `application/usecases/build_context_test.go`: con `Topics` nil la sección se omite sin error, igual que `Graph`/`Recorder`

### Implementation for User Story 1

- [X] T025 [US1] Implementar `SeedDefaults` en `application/usecases/seed_defaults.go` con semántica comprobar-y-omitir, insertando por `MemorySeeder` con `SessionID` y `Filepath` vacíos
- [X] T026 [US1] Añadir el campo opcional `Topics ports.MemoryTopicQuerier` al `Builder` en `application/usecases/build_context.go`
- [X] T027 [US1] Emitir `## Reglas de trabajo (memoria fijada)` en `Build()` en `application/usecases/build_context.go`, justo tras `# Memoria del Proyecto` (~línea 195), sin pasar por `acota()` ni `fits()`, salvo en `IndexMode`
- [X] T028 [US1] Excluir la memoria con `TopicKey == domain.TopicWorkRules` del bucle de `## Preferencias del Usuario` en `application/usecases/build_context.go` (~línea 251)
- [X] T029 [US1] Cablear `contextBuilder.Topics = memRepo` en `infrastructure/container.go` (~línea 70), junto a `Graph`/`Counter`/`Recorder`
- [X] T030 [US1] Invocar `SeedDefaults` en `CmdInstall` (`adapters/primary/cli/cmd_install.go`) como paso 2b, tras verificar la memoria, informando `✅ sembradas` o `ℹ️ ya presentes` y sin abortar ante error
- [X] T031 [US1] Invocar `SeedDefaults` en `CmdMCP` (`adapters/primary/cli/cmd_mcp.go`) tras el auto-arranque de sesión y antes de `server.Run`, con `log.Printf` ante error y nunca fatal
- [X] T032 [P] [US1] Test de integración en `tests/integration/seed_bootstrap_test.go`: arrancar el servidor MCP en un directorio limpio siembra ambas memorias sin haber ejecutado `install`

**Checkpoint**: US1 completa. Las reglas llegan íntegras y sobreviven a reinstalaciones.

---

## Phase 4: User Story 2 — Instalar sin ensuciar el repositorio (Priority: P2)

**Goal**: `mem install` y `mem update` no crean archivos de instrucciones, copia de constitución ni carpetas de agentes no solicitados.

**Independent Test**: instalar en un directorio vacío y comprobar que la raíz no contiene ninguno de los cinco artefactos — quickstart §2.

### Tests for User Story 2 ⚠️

- [X] T033 [P] [US2] Reescribir `tests/integration/protocol_block_test.go`: sustituir `TestInstallPreservesContentAroundLegacyProtocolBlock` por `TestInstallEliminaArchivosDeAgenteYRespalda`. El test viejo verificaba un comportamiento retirado a propósito; se documenta el motivo en el propio archivo
- [X] T034 [P] [US2] Ampliar `TestNoRepoFilesCreated` en `tests/integration/lazy_init_test.go` para exigir ausencia de `AGENTS.md`, `CLAUDE.md`, `speckit-constitution-gen.md`, `.windsurf/` y `.cline/`
- [X] T035 [P] [US2] Test en `adapters/primary/setup/activation_inspect_test.go`: el canal de instrucciones de ámbito proyecto reporta `not_applicable` con motivo, y el de ámbito usuario sigue evaluándose igual
- [X] T036 [P] [US2] Test del mensaje final — implementado como `TestInstallMensajeFinal` en `tests/integration/protocol_block_test.go` en vez de en `cmd_install_test.go`: verificarlo contra el binario real, y no contra la función, es lo que exige la regla de campo 2 del proyecto

### Implementation for User Story 2

- [X] T037 [US2] Eliminar el paso 4 (bloque de `AGENTS.md`/`CLAUDE.md`, ~líneas 117-159) y la función `defaultAgentFile` de `adapters/primary/cli/cmd_install.go`. **Conservar** `composeAgentFile`, `protocolStart` y `protocolEnd`: los usa `setup-mcp --scope global`
- [X] T038 [US2] Eliminar el paso 4b (copia de `speckit-constitution-gen.md`, ~líneas 161-173) de `adapters/primary/cli/cmd_install.go`
- [X] T039 [US2] Quitar `setupWindsurf(target)` y `setupCline(target)` del paso 5 de `adapters/primary/cli/cmd_install.go` (~líneas 209-210), conservando ambas funciones para `setup-mcp --agents`
- [X] T040 [US2] Reescribir el mensaje final de `adapters/primary/cli/cmd_install.go` (~líneas 220-232) según [contracts/install-artifacts.md](./contracts/install-artifacts.md) §Contrato del mensaje final
- [X] T041 [US2] Cambiar el canal de instrucciones de ámbito proyecto a `StateNotApplicable` en `adapters/primary/setup/activation_inspect.go` (~línea 196), con el motivo declarado

**Checkpoint**: instalación limpia verificada. `mem update` lo hereda por delegación.

---

## Phase 5: User Story 5 — Reemplazar reglas y constitución con las mías (Priority: P2)

**Goal**: exportar, editar e importar los documentos fijados desde consola y TUI, con paridad de capacidades.

**Independent Test**: exportar las reglas, añadirles una línea, reimportarlas y ver la línea nueva en `mem context` — quickstart §14.

### Tests for User Story 5 ⚠️

- [X] T042 [P] [US5] Test en `adapters/primary/cli/cmd_docs_test.go`: `list` deriva el estado comparando contra la plantilla (`por defecto` / `personalizado` / `sin sembrar`) y muestra la fecha de modificación
- [X] T043 [P] [US5] Test en `adapters/primary/cli/cmd_docs_test.go`: `show`/`export` sin `-o` escriben **solo** el documento en stdout; todo aviso va a stderr
- [X] T044 [P] [US5] Test en `adapters/primary/cli/cmd_docs_test.go`: `import` crea el documento si falta y lo reemplaza si existe, conservando siempre la clave de tópico
- [X] T045 [P] [US5] Test en `adapters/primary/cli/cmd_docs_test.go`: `import` de archivo vacío o ilegible falla con motivo y **deja intacto** el documento anterior. Es el peor modo de fallo de esta capacidad
- [X] T046 [P] [US5] Test en `adapters/primary/cli/cmd_docs_test.go`: `reset` restaura la plantilla, e `import` con contenido idéntico informa "sin cambios" sin escribir
- [X] T047 [P] [US5] Test en `adapters/primary/cli/cmd_docs_test.go`: `import --topic` acepta una clave fuera del catálogo, y un alias desconocido falla listando los válidos
- [X] T048 [P] [US5] Test en `application/usecases/pinned_docs_test.go`: importar y restaurar usan la vía inerte — 0 relaciones nuevas y 0 exportaciones con la sincronización externa activada
- [X] T049 [P] [US5] Test en `adapters/primary/tui/tui_docs_test.go`: las filas del menú de configuración se generan recorriendo `domain.PinnedDocs` y quedan al final, sin desplazar `configRowAtomicPlan` ni `configRowPlanGuard`
- [X] T050 [P] [US5] Test de paridad en `tests/contract/pinned_docs_parity_test.go`: para cada entrada del catálogo, CLI y TUI ofrecen el mismo conjunto de operaciones (ver, exportar, importar, restaurar)

### Implementation for User Story 5

- [X] T051 [US5] Implementar `ImportPinnedDoc` y `ResetPinnedDoc` en `application/usecases/pinned_docs.go`, escribiendo por la vía inerte y rechazando contenido vacío tras depurar
- [X] T052 [US5] Implementar `CmdDocs` en `adapters/primary/cli/cmd_docs.go` con los subcomandos `list|show|export|import|reset`, `--all`, `-o` y `--topic`, según [contracts/pinned-docs.md](./contracts/pinned-docs.md) §2
- [X] T053 [US5] Registrar `case "docs"` en `adapters/primary/cli/dispatcher.go` y añadir la línea de ayuda en `adapters/primary/cli/cli.go`
- [X] T054 [US5] Añadir `configRowDocsBase` y las filas generadas del catálogo al final del menú de configuración en `adapters/primary/tui/tui.go`, respetando la convención declarada en `configRowReindexGraph`
- [X] T055 [US5] Implementar `screenDocs` en `adapters/primary/tui/tui_docs.go` con las acciones ver / exportar / importar / restaurar, reutilizando el patrón de entrada de ruta de `screenImport`
- [X] T056 [US5] Exigir confirmación antes de restaurar en `adapters/primary/tui/tui_docs.go`, reutilizando el patrón de `screenMaintenanceConfirm`

**Checkpoint**: US5 completa. Las plantillas quedan como *default* y el contenido pasa a ser del equipo.

---

## Phase 6: User Story 3 — Limpiar lo que dejaron las instalaciones anteriores (Priority: P3)

**Goal**: retirar los artefactos legados, con respaldo previo de lo que puede contener texto propio.

**Independent Test**: preparar un directorio que simule una instalación antigua, instalar y comprobar que desapareció con su respaldo — quickstart §5 y §6.

### Tests for User Story 3 ⚠️

- [X] T057 [P] [US3] Test en `adapters/primary/cli/cmd_install_cleanup_test.go`: `AGENTS.md`, `CLAUDE.md` y `CLAUDE.txt` se copian a `.memory/backups/agent-files/` con su contenido íntegro y luego se eliminan
- [X] T058 [P] [US3] Test en `adapters/primary/cli/cmd_install_cleanup_test.go`: si el respaldo no se puede escribir, el original **no** se borra y se informa el motivo
- [X] T059 [P] [US3] Test en `adapters/primary/cli/cmd_install_cleanup_test.go`: `.windsurf` con solo `gomemory` se elimina entera; `.cline` con otro servidor conserva ese servidor y pierde solo la entrada de gomemory
- [X] T060 [P] [US3] Test en `adapters/primary/cli/cmd_install_cleanup_test.go`: un JSON inválido queda byte por byte igual; una segunda pasada no produce salida ni cambios; `.cursorrules`/`.windsurfrules` no se tocan

### Implementation for User Story 3

- [X] T061 [US3] Implementar `cleanupLegacyArtifacts(target)` en `adapters/primary/cli/cmd_install_cleanup.go` según [data-model.md](./data-model.md) §5, reutilizando la lógica de desregistro de `removeMCPEntries` (`cmd_uninstall.go` ~línea 134)
- [X] T062 [US3] Invocar `cleanupLegacyArtifacts` en `CmdInstall` (`adapters/primary/cli/cmd_install.go`) como paso 3b, después de la siembra, para que un fallo de siembra deje los archivos legados en su sitio

**Checkpoint**: US3 completa. `mem update` la hereda por delegación en `install`.

---

## Phase 7: User Story 4 — Aplicar la constitución bajo demanda (Priority: P4)

**Goal**: `/constitution` y `mem constitution` sirven la constitución vigente desde la memoria.

**Independent Test**: pedir la constitución y comprobar que devuelve el documento completo, y la versión editada tras personalizarla — quickstart §7.

### Tests for User Story 4 ⚠️

- [X] T063 [P] [US4] Test en `adapters/primary/cli/cmd_constitution_test.go`: devuelve la memoria cuando existe, la plantilla con aviso a stderr cuando no, y la versión editada tras personalizarla
- [X] T064 [P] [US4] Test en `adapters/primary/cli/cmd_constitution_test.go`: `--sync` escribe `.specify/memory/constitution.md` cuando hay spec-kit e informa que no aplica cuando no lo hay, **sin crear** `.specify/`
- [X] T065 [P] [US4] Test en `adapters/primary/setup/constitution_setup_test.go`: los envoltorios se escriben en las dos rutas, **no** contienen el texto de la constitución, y no se reescriben si el contenido no difiere

### Implementation for User Story 4

- [X] T066 [US4] Implementar `CmdConstitution` en `adapters/primary/cli/cmd_constitution.go` como atajo sobre `ResolvePinnedDoc`, con el flag `--sync`
- [X] T067 [US4] Registrar `case "constitution"` y `case "rules"` en `adapters/primary/cli/dispatcher.go` y sus líneas de ayuda en `adapters/primary/cli/cli.go`
- [X] T068 [US4] Implementar `InstallConstitutionWrappers` en `adapters/primary/setup/constitution_setup.go`, clonando el patrón de `atomic_plan_setup.go`, con un cuerpo que instruye resolver desde la memoria en el momento de la invocación
- [X] T069 [US4] Invocar `InstallConstitutionWrappers` en `CmdInstall` (`adapters/primary/cli/cmd_install.go`) como paso 4e, junto a `InstallAtomicPlanWrappers` (~línea 185), avisando sin bloquear ante error

**Checkpoint**: las cinco historias operativas.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T070 [P] Actualizar `docs/MANUAL.md`: modelo de semillas, `mem docs`, `mem constitution`, `mem rules`, y retirar la descripción de los archivos que ya no se generan
- [X] T071 [P] Actualizar `docs/architecture.md`: documentos fijados, vía de inserción inerte y la corrección de `ListMemories`
- [X] T072 [P] Actualizar `README.md` e `INSTALLATION.md`: qué crea y qué ya no crea la instalación, y cómo reemplazar reglas y constitución
- [X] T073 [P] Añadir `mem docs`, `mem constitution` y `mem rules` a la tabla de comandos en `docs/MANUAL.md` y `README.md`
- [X] T074 Ejecutar `go build ./... && go vet ./... && go test ./...` y comparar con la línea base de T001: cero regresiones
- [X] T075 Ejecutar los 16 escenarios de [quickstart.md](./quickstart.md) contra el binario real, no solo la suite — regla de campo 2 del proyecto
- [X] T076 SC-005 verificado directamente en vez de con `mem usage` (que necesita sesión activa para registrar): 0 archivos del repo contienen el bloque de protocolo y `mem context` no lo duplica — lo entrega solo el MCP en `initialize`
- [X] T077 Guardar en gomemory las decisiones tomadas durante la implementación y cerrar con `end_session`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Fase 1)**: sin dependencias.
- **Foundational (Fase 2)**: depende de Setup. **BLOQUEA todas las historias.**
- **US1 (Fase 3)**: depende de Fase 2. Es el MVP.
- **US2 (Fase 4)**: depende de Fase 2 y de US1 — retirar los archivos antes de que las semillas funcionen sería una pérdida neta de funcionalidad.
- **US5 (Fase 5)**: depende de Fase 2 (catálogo, vía inerte, resolutor). **No** depende de US2 ni de US3.
- **US3 (Fase 6)**: depende de US2 (comparte el punto de invocación en `CmdInstall`).
- **US4 (Fase 7)**: depende de Fase 2 (resolutor). **No** depende de US5: ambos usan `ResolvePinnedDoc`, que vive en Foundational precisamente para que no se encadenen.
- **Polish (Fase 8)**: depende de las historias que se decidan entregar.

### Orden dentro de cada historia

1. Los tests se escriben y **fallan** antes de implementar (constitución III).
2. Dominio → puertos → adaptadores → wiring.
3. La historia se cierra antes de pasar a la siguiente prioridad.

### Dependencias finas que importan

- **T005 antes de T021**: sin la columna, la comparación por clave da siempre falso y el test de no-duplicación pasaría por el motivo equivocado.
- **T013 antes de T025**: sembrar por la vía normal publicaría la constitución en el ADR externo con `adr_sync_enabled=true`.
- **T017 antes de T052 y T066**: ambos comandos resuelven con la misma función.
- **T030 antes de T062**: la siembra va antes que la limpieza, para que un fallo de siembra no deje a la persona sin las reglas en ninguna de las dos formas.
- **T003 antes de T054**: las filas de la TUI se generan del catálogo.

### Parallel Opportunities

- **Fase 2**: T004, T006, T008, T010, T011, T014 y T016 son de archivos distintos y corren en paralelo. T005/T007/T012/T013 tocan `memory.go` y van en serie.
- **Fase 3**: los siete tests T018-T024 en paralelo; después T026-T028 en serie sobre `build_context.go`.
- **Fase 4**: T033-T036 en paralelo; T037-T040 en serie sobre `cmd_install.go`.
- **Fase 5**: los nueve tests T042-T050 en paralelo; T052 y T055 en paralelo entre sí (CLI y TUI son archivos distintos).
- **Fase 6**: T057-T060 en paralelo, todos sobre un archivo de test nuevo con directorios temporales independientes.
- **Fase 8**: T070-T073 en paralelo.
- **Entre historias**: con la Fase 2 cerrada, **US5 y US4 pueden avanzar en paralelo con US1/US2** si hay más de una persona; US3 espera a US2.

---

## Parallel Example: Fase 2

```
Simultáneamente (archivos distintos, sin dependencias entre sí):
  T004  memory_test.go        — regresión de topic_key en ListMemories
  T006  memory_test.go        — GetMemoryByTopicKey            [mismo archivo que T004: coordinar]
  T008  ports/memory_topic.go — puerto MemoryTopicQuerier
  T010  memory_test.go        — G2, sin sinapsis               [mismo archivo: coordinar]
  T011  memory_test.go        — G3, sin export ADR             [mismo archivo: coordinar]
  T014  ports/memory_seeder.go— puerto MemorySeeder
  T016  usecases/pinned_docs_test.go — ResolvePinnedDoc

En la práctica: T008, T014 y T016 son verdaderamente paralelos entre sí y con el
bloque de tests de persistencia; los cuatro tests de `memory_test.go` se escriben en
una sola pasada sobre ese archivo.
```

---

## Implementation Strategy

### MVP: Fases 1 + 2 + 3 (US1)

Deja el producto en un estado coherente y entregable: las reglas y la constitución
viven en la memoria, llegan íntegras al agente y sobreviven a la edición de la
persona. La instalación sigue generando archivos —redundantes, pero inofensivos—
hasta la US2.

### Entrega incremental sugerida

| Incremento | Fases | Valor entregado |
|---|---|---|
| 1 | 1 + 2 | **Dos defectos del producto actual corregidos**, entregable por sí solo: `ListMemories` deja de omitir la clave de tópico y la publicación accidental al ADR externo queda cerrada |
| 2 | + 3 (US1) | MVP: las reglas llegan íntegras desde la memoria |
| 3 | + 4 (US2) | El repositorio deja de ensuciarse en instalaciones nuevas |
| 4 | + 5 (US5) | El contenido pasa a ser del equipo, por consola y TUI |
| 5 | + 6 (US3) | Los proyectos existentes quedan limpios |
| 6 | + 7 (US4) + 8 | `/constitution` bajo demanda y documentación al día |

El incremento 1 merece atención: **corrige defectos que existen hoy** y tiene valor
aunque el resto de la feature se posponga.

### Criterio de "hecho" por historia

Ninguna historia se marca completa sin ejecutar sus escenarios de
[quickstart.md](./quickstart.md) **contra el binario real**. Verde en la suite no es
"funciona" — regla de campo 2 de este proyecto.

| Historia | Escenarios de quickstart |
|---|---|
| US1 | §3, §4, §8, §12, §13 |
| US2 | §2, §9, §10 |
| US5 | §14, §15, §16 |
| US3 | §5, §6 |
| US4 | §7 |
