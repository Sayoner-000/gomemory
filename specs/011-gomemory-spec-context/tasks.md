---

description: "Task list for gomemory como brazo extensor de contexto histórico para /speckit"

---

# Tasks: gomemory como brazo extensor de contexto histórico para /speckit

**Input**: Design documents from `/specs/011-gomemory-spec-context/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (todos presentes)

**Tests**: la constitución del proyecto exige TDD para código Go
(Principio III, no negociable) — se incluyen tareas de test para los
cambios en Go (settings). El script bash/PowerShell de la extensión
spec-kit no tiene framework de test en este repositorio (mismo criterio ya
aceptado para `agent-context/scripts/`); su verificación es `quickstart.md`.

**Organization**: tareas agrupadas por historia de usuario de `spec.md`
(US1–US4), para poder implementar y probar cada una de forma independiente.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: puede ejecutarse en paralelo (archivos distintos, sin
  dependencia de tareas incompletas)
- **[Story]**: historia de usuario a la que pertenece (US1–US4)
- Cada tarea incluye la ruta de archivo exacta

## Path Conventions

Proyecto único (Go, hexagonal) + extensión spec-kit bundleada. Rutas reales
del repositorio, sin opciones alternativas (ver `plan.md` → Project
Structure).

---

## Phase 1: Setup

**Purpose**: crear el esqueleto de la nueva extensión spec-kit

- [X] T001 [P] Crear la estructura de directorios de la extensión:
  `.specify/extensions/gomemory-context/`,
  `.specify/extensions/gomemory-context/commands/`,
  `.specify/extensions/gomemory-context/scripts/bash/`,
  `.specify/extensions/gomemory-context/scripts/powershell/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: el campo de settings que tanto US1 (el script lo lee) como US4
(TUI/CLI lo exponen) necesitan. Sin esto, ninguna historia puede probarse.

**⚠️ CRITICAL**: ninguna historia de usuario puede empezar hasta que esta fase esté completa

- [X] T002 [P] Agregar campo `SpeckitContextDisabled bool` (json:
  `speckit_context_disabled,omitempty`, default `false`=activado) al struct
  `Settings` en `adapters/secondary/persistence/settings.go`, con
  comentario siguiendo el mismo patrón `*Disabled` que `CodeGraphDisabled`
  (ver `data-model.md`)
- [X] T003 [P] Agregar el mismo campo `SpeckitContextDisabled bool` al
  struct `SettingsData` en `application/ports/settings_repository.go`
  (espejo del puerto)
- [X] T004 Mapear `SpeckitContextDisabled` entre `Settings` (adaptador) y
  `SettingsData` (puerto) en `adapters/secondary/persistence/repositories.go`
  (depende de T002, T003)
- [X] T005 Test unitario `TestReadSettings_PreservesSpeckitContextDisabled`
  en `adapters/secondary/persistence/settings_test.go`, siguiendo el patrón
  de `TestReadSettings_PreservesExplicitBooleans` (línea 105): roundtrip
  `WriteSettings`/`ReadSettings` con el campo en `true`, y default `false`
  cuando la clave está ausente del JSON (depende de T002)

**Checkpoint**: el campo de settings existe y persiste correctamente — las historias de usuario pueden empezar

---

## Phase 3: User Story 1 - Contexto histórico automático al crear una especificación (Priority: P1) 🎯 MVP

**Goal**: el flujo de `/speckit-specify` incorpora automáticamente (hook
mandatorio) el resumen de `mem context`, sin que la persona lo pida.

**Independent Test**: Escenario 1 (y 4, degradación transparente) de
`quickstart.md` — invocar `/speckit-specify` y confirmar que el resumen
aparece sin haber abierto archivos de `specs/` a mano.

### Implementation for User Story 1

- [X] T006 [P] [US1] Crear `.specify/extensions/gomemory-context/extension.yml`
  con `hooks.before_specify` (`optional: false`, mandatorio — ver
  `research.md` #2) apuntando al comando `speckit.gomemory-context.update`,
  siguiendo la forma fijada en
  `contracts/gomemory-context-extension.yml`
- [X] T007 [P] [US1] Crear
  `.specify/extensions/gomemory-context/commands/speckit.gomemory-context.update.md`
  documentando el comportamiento del comando (llama al script bash/
  PowerShell, nunca falla el flujo) por
  `contracts/update-gomemory-context-script.md`
- [X] T008 [P] [US1] Implementar
  `.specify/extensions/gomemory-context/scripts/bash/update-gomemory-context.sh`:
  1) localizar `./mem` (raíz del proyecto) o `mem` en `PATH`, si no existe
  terminar en 0 sin salida; 2) leer
  `.memory/settings.json` con `grep` para la clave
  `"speckit_context_disabled"\s*:\s*true` — si está presente, terminar en 0
  sin salida (sin depender de `mem settings --show`, para no acoplar US1 a
  US4); 3) ejecutar `mem context` y volcar su stdout; 4) si el comando
  falla, terminar en 0 sin salida — exactamente el contrato de
  `contracts/update-gomemory-context-script.md`
- [X] T009 [P] [US1] Implementar
  `.specify/extensions/gomemory-context/scripts/powershell/update-gomemory-context.ps1`
  con la misma lógica que T008 (paridad Windows, mismo patrón que
  `agent-context/scripts/powershell/update-agent-context.ps1`)
- [X] T010 [US1] Escribir
  `.specify/extensions/gomemory-context/README.md` (mismo formato que
  `agent-context/README.md`): qué hace, por qué existe, cómo desactivarla
  (menciona ambas vías: `specify extension disable gomemory-context` y el
  interruptor de gomemory — remite a US4 aunque aún no exista la UI)
  (depende de T006-T009)
- [X] T011 [US1] Ejecutar Escenario 1 y Escenario 4 de `quickstart.md`
  contra un proyecto de prueba con la extensión instalada, confirmando que
  el resumen aparece automáticamente y que la ausencia de `mem`/memorias no
  rompe el flujo (depende de T006-T010)

**Checkpoint**: US1 funcional de forma independiente — el resumen aparece automáticamente al crear una especificación

---

## Phase 4: User Story 2 - Distinguir historia de decisiones vs. estructura de código (Priority: P1)

**Goal**: confirmar y documentar que el resumen mantiene las secciones de
gomemory (historia/decisiones) y del grafo externo (estructura de código)
separadas y rotuladas — esta separación **ya existe** en
`application/usecases/build_context.go` (`writeCodeProviderSection`,
feature 010) y ya está cubierta por `build_context_test.go`; esta historia
no agrega lógica nueva, solo verifica que el camino nuevo (script → `mem
context`) no la rompe y la documenta para quien mantenga la extensión.

**Independent Test**: Escenario 2 de `quickstart.md` — puede probarse
directamente sobre `mem context` sin necesitar la extensión instalada.

### Implementation for User Story 2

- [X] T012 [US2] Agregar una sección al README de la extensión
  (`.specify/extensions/gomemory-context/README.md`, de T010) explicando el
  contrato de separación de fuentes: la salida de `mem context` siempre
  distingue "Decisiones/Bugfixes/Patrones" (gomemory) de
  "## Grafo de código externo (\<provider\>)" (grafo externo) cuando este
  último está disponible — cita `writeCodeProviderSection` en
  `application/usecases/build_context.go` como la garantía verificada
- [X] T013 [P] [US2] Correr `go test ./application/usecases/...` (incluye
  `build_context_test.go`, regresión de la separación por origen) y el
  Escenario 2 de `quickstart.md` como evidencia de que el script de US1 no
  altera el rotulado — sin cambios de código, solo verificación

**Checkpoint**: US2 verificada — el resumen distingue las dos fuentes sin ambigüedad, con y sin la extensión instalada

---

## Phase 5: User Story 3 - Mismo contexto disponible en planificación y aclaración (Priority: P3)

**Goal**: extender el mismo mecanismo (script de US1) a `/speckit-plan` y
`/speckit-clarify` vía hooks opcionales.

**Independent Test**: invocar `/speckit-plan` o `/speckit-clarify` sobre
una especificación existente en una sesión nueva y confirmar que el hook
opcional ofrece el mismo resumen.

### Implementation for User Story 3

- [X] T014 [US3] Extender `.specify/extensions/gomemory-context/extension.yml`
  (de T006) agregando `hooks.before_plan` y `hooks.before_clarify`
  (`optional: true` — a diferencia de `before_specify`, ver `research.md`
  #2 — mismo comando `speckit.gomemory-context.update`), por
  `contracts/gomemory-context-extension.yml`
- [X] T015 [US3] Actualizar el README de la extensión (de T010) explicando
  por qué `before_plan`/`before_clarify` son opcionales (requieren
  confirmación) mientras `before_specify` es mandatorio (depende de T014)
- [X] T016 [US3] Validar manualmente: abrir una sesión nueva sobre una
  especificación ya creada, invocar `/speckit-plan` o `/speckit-clarify`, y
  confirmar que el bloque `Optional Pre-Hook` aparece y, al ejecutarlo,
  entrega el mismo resumen que en `/speckit-specify` (depende de T014)

**Checkpoint**: US3 funcional — el resumen histórico también está disponible al planificar/aclarar

---

## Phase 6: User Story 4 - Encendido/apagado del brazo extensor sin depender de spec-kit (Priority: P2)

**Goal**: interruptor visible y editable desde la TUI/CLI de gomemory,
independiente de la configuración de spec-kit.

**Independent Test**: Escenario 3 (y 5) de `quickstart.md`.

### Implementation for User Story 4

- [X] T017 [US4] Agregar la fila `"Brazo extensor spec-kit: " +
  onOff(!s.SpeckitContextDisabled)` a la lista `rows` de la pantalla de
  configuración en `adapters/primary/tui/tui.go` (junto a las filas de
  "Grafo de código externo"/"Sinapsis automática", ~línea 1609-1615), y
  subir `configOptions` de `5` a `6` (~línea 700)
- [X] T018 [US4] Agregar `case 5` en `updateConfig()` de
  `adapters/primary/tui/tui.go` (~línea 761, junto al `case 4` de
  sinapsis) que alterna `s.SpeckitContextDisabled`, persiste con
  `m.settingsRepo.Write`, y setea `m.statusMsg`/`m.statusTimer` — mismo
  patrón exacto que `case 0` (grafo de código externo) (depende de T017)
- [X] T019 [P] [US4] Agregar el flag `--speckit-context` (bool, default
  `true`) a `CmdSettings` en `adapters/primary/cli/cmd_settings.go`, wireado
  vía `fs.Visit` a `settings.SpeckitContextDisabled = !*speckitContext`,
  siguiendo el patrón exacto de `--code-graph` (línea 12/47-48), por
  `contracts/mem-settings-cli.md`
- [X] T020 [US4] Agregar la línea `Brazo extensor spec-kit: %v\n` a
  `printSettings()` en `adapters/primary/cli/cmd_settings.go` (depende de
  T019, mismo archivo)
- [X] T021 [US4] Ejecutar Escenario 3 (toggle en TUI persiste y apaga el
  script sin error) y Escenario 5 (sin efecto en un proyecto sin
  `.specify/`) de `quickstart.md` (depende de T017-T020, T008-T009)

**Checkpoint**: las 4 historias de usuario son funcionales de forma independiente

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: cierre — registro formal de la extensión, validación completa, higiene de código

- [X] T022 [P] Completar `.specify/extensions/gomemory-context/README.md`
  con instrucciones de instalación (`specify extension install
  .specify/extensions/gomemory-context`, ver `research.md` #6)
- [X] T023 Registrar la extensión con la CLI `specify` (NO editar
  `.specify/extensions/.registry` ni `.specify/extensions.yml` a mano —
  ver `research.md` #6) y confirmar que queda listada junto a
  `agent-context`
- [X] T024 Ejecutar los 5 escenarios completos de `quickstart.md` de punta
  a punta en un proyecto de prueba con spec-kit instalado (depende de
  todas las historias)
- [X] T025 [P] `golangci-lint run` + `gofmt`/`gofumpt` sobre los archivos
  Go tocados: `settings.go`, `settings_repository.go`, `repositories.go`,
  `settings_test.go`, `tui.go`, `cmd_settings.go`
- [X] T026 [P] Mencionar el brazo extensor spec-kit como capacidad opcional
  en `docs/architecture.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — puede empezar de inmediato
- **Foundational (Phase 2)**: sin dependencia de Setup (directorios
  distintos) — BLOQUEA todas las historias de usuario
- **User Stories (Phase 3-6)**: todas dependen de Foundational (Phase 2)
  - US1 y US2 pueden avanzar en paralelo (P1 ambas, sin dependencia mutua)
  - US3 depende del script de US1 (reutiliza el mismo comando) — no puede
    empezar antes de que T006-T009 existan
  - US4 no depende de US1/US2/US3 en el código Go, pero su validación
    (T021) sí necesita el script de US1 (T008-T009) para verificar que el
    toggle lo gatea de verdad
- **Polish (Phase 7)**: depende de que todas las historias deseadas estén completas

### User Story Dependencies

- **US1 (P1)**: depende de Foundational — sin dependencia de otras historias
- **US2 (P1)**: depende de Foundational — sin dependencia de código de otras
  historias (su "implementación" ya existe desde feature 010); su
  validación end-to-end (T013) es más rica una vez US1 existe, pero no la
  requiere
- **US3 (P3)**: depende de Foundational **y** de los archivos creados en
  US1 (T006 extension.yml, T008/T009 scripts) — no es independiente a nivel
  de archivo, sí a nivel de valor entregado (se puede desactivar sin tocar
  US1)
- **US4 (P2)**: depende de Foundational — independiente de US1/US2/US3 en
  el código Go (TUI/CLI tocan archivos distintos); su prueba de extremo a
  extremo usa el script de US1

### Within Each User Story

- Foundational antes que cualquier historia
- Dentro de US1: `extension.yml`/comando/scripts (T006-T009) antes que el
  README (T010) antes que la validación (T011)
- Dentro de US4: fila de TUI (T017) antes que su handler (T018); flag CLI
  (T019) antes que la línea de `printSettings` (T020); ambos antes de la
  validación (T021)

### Parallel Opportunities

- Todas las tareas de Foundational marcadas [P] (T002, T003) en paralelo
- Una vez completo Foundational: T006, T007, T008, T009 (US1) en paralelo
  entre sí (archivos distintos)
- T013 (US2) en paralelo con el resto de US1 una vez exista `mem context`
  (ya existe desde feature 010 — no depende de esta feature)
- T019 (US4, `cmd_settings.go`) en paralelo con T017 (US4, `tui.go`) —
  archivos distintos
- T022, T025, T026 (Polish) en paralelo entre sí

---

## Parallel Example: User Story 1

```bash
# Una vez completo Foundational (T002-T005), lanzar junto:
Task: "Crear extension.yml con hook before_specify mandatorio"
Task: "Crear el comando speckit.gomemory-context.update.md"
Task: "Implementar update-gomemory-context.sh"
Task: "Implementar update-gomemory-context.ps1"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (CRÍTICO — bloquea todas las historias)
3. Completar Phase 3: User Story 1
4. **PARAR Y VALIDAR**: correr Escenario 1 y 4 de `quickstart.md`
   independientemente
5. Es un MVP entregable: el resumen automático ya funciona, aunque todavía
   no haya interruptor de apagado (US4) ni cobertura en plan/clarify (US3)

### Incremental Delivery

1. Setup + Foundational → base lista
2. US1 → validar con quickstart → **MVP**: resumen automático en
   `/speckit-specify`
3. US2 → validar (ya viene gratis de feature 010, solo se documenta y
   confirma)
4. US4 → validar con quickstart → interruptor visible en la TUI/CLI
5. US3 → validar con quickstart → mismo resumen en `/speckit-plan`/
   `/speckit-clarify`
6. Polish → registro formal de la extensión + limpieza

### Parallel Team Strategy

Con más de una persona: completar Setup+Foundational juntos; luego una
persona en US1 (extensión spec-kit), otra en US4 (TUI/CLI de gomemory) —
son archivos completamente distintos y no se pisan. US2 es solo
documentación/validación (rápida, cualquiera la toma). US3 espera a que
US1 tenga al menos T006-T009 listos.

---

## Notes

- [P] = archivos distintos, sin dependencias entre sí
- [Story] mapea cada tarea a su historia de usuario para trazabilidad
- US2 no introduce lógica nueva: su valor ya lo entregó feature 010
  (`writeCodeProviderSection`); aquí solo se documenta y se verifica que el
  camino nuevo no lo rompe
- El gate del interruptor (T008) lee `.memory/settings.json` directamente
  con `grep`, no invoca `mem settings --show` — así US1 no depende de que
  US4 ya esté implementada
- Commit tras cada tarea o grupo lógico
- Parar en cada checkpoint para validar la historia de forma independiente
