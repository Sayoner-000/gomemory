---

description: "Task list for distribuir el brazo extensor gomemory-context vía mem install, transversal a agentes"

---

# Tasks: Distribuir el brazo extensor gomemory-context vía `mem install`, transversal a agentes

**Input**: Design documents from `/specs/012-gomemory-context-distribution/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (todos presentes)

**Tests**: la constitución del proyecto exige TDD para código Go
(Principio III, no negociable) — se incluyen tareas de test para
`InstallSpeckitExtension` y su wiring. Los fixtures de agente
(`claude-artifact/`, `opencode-artifact/`) no se generan en Go: son
copias literales de artefactos ya verificados contra la CLI real
`specify` (ver `contracts/README.md`), no requieren test propio más allá
de confirmar que se copiaron sin alterar el contenido.

**Organization**: tareas agrupadas por historia de usuario de `spec.md`
(US1–US4).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: puede ejecutarse en paralelo (archivos distintos, sin
  dependencia de tareas incompletas)
- **[Story]**: historia de usuario a la que pertenece (US1–US4)
- Cada tarea incluye la ruta de archivo exacta

## Path Conventions

Proyecto único (Go, hexagonal). Rutas reales del repositorio — ver
`plan.md` → Project Structure.

---

## Phase 1: Setup

**Purpose**: crear el esqueleto de directorios de las plantillas embebidas nuevas

- [X] T001 [P] Crear la estructura de directorios
  `infrastructure/templates/gomemory-context/extension/{commands,scripts/bash,scripts/powershell}`,
  `infrastructure/templates/gomemory-context/claude/speckit-gomemory-context-update/`,
  `infrastructure/templates/gomemory-context/opencode/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: la función `InstallSpeckitExtension` con su gate (`.specify/`
presente) y la copia del árbol fuente de la extensión — la base que US1 y
US2 extienden cada una con su artefacto de agente.

**⚠️ CRITICAL**: ninguna historia de usuario puede empezar hasta que esta fase esté completa

- [X] T002 [P] Copiar el árbol fuente de la extensión ya existente en
  `.specify/extensions/gomemory-context/` (`extension.yml`, `README.md`,
  `commands/speckit.gomemory-context.update.md`,
  `scripts/bash/update-gomemory-context.sh`,
  `scripts/powershell/update-gomemory-context.ps1`) a
  `infrastructure/templates/gomemory-context/extension/`, sin modificar
  contenido (copia literal, es la misma extensión de la spec 011)
- [X] T003 Implementar `InstallSpeckitExtension(root string, templatesFS
  fs.FS) error` en `adapters/primary/setup/speckit_extension.go`, por el
  contrato en `contracts/install-speckit-extension.md`: retorna `nil` de
  inmediato si `templatesFS` es `nil` o si `filepath.Join(root,
  ".specify")` no es un directorio (FR-004); si no, llama
  `InstallPlugin(templatesFS, "templates/gomemory-context/extension",
  filepath.Join(root, ".specify", "extensions", "gomemory-context"), nil)`
  y aplica `os.Chmod(..., 0755)` al script bash copiado (depende de T002)
- [X] T004 Tests unitarios (TDD) en
  `adapters/primary/setup/speckit_extension_test.go`: sin `.specify/` →
  ningún archivo creado; con `.specify/` → el árbol de
  `.specify/extensions/gomemory-context/` queda copiado; el script bash
  queda con permiso `0755`; `templatesFS=nil` → no-op sin panic; segunda
  llamada sin cambios → ningún archivo se reescribe (verificar `mtime`)
  (depende de T003)
- [X] T005 [P] Wiring en `adapters/primary/cli/cmd_install.go`: agregar la
  llamada a `setup.InstallSpeckitExtension(target, TemplatesFS)` justo
  después del paso 4b (copia de la constitución), con manejo de error no
  bloqueante (`⚠️` + continuar, mismo patrón que el resto de pasos de
  instalación) (depende de T003)

**Checkpoint**: el árbol fuente de la extensión ya se distribuye correctamente — las historias de agente pueden empezar

---

## Phase 3: User Story 1 - Claude Code recibe el brazo extensor solo con `mem install` (Priority: P1) 🎯 MVP

**Goal**: tras `mem install` en un proyecto con spec-kit, el artefacto que
Claude Code reconoce para el hook `before_specify` queda listo, sin pasos
manuales ni depender de la CLI `specify`.

**Independent Test**: Escenario 1 de `quickstart.md`.

### Implementation for User Story 1

- [X] T006 [P] [US1] Copiar el fixture verificado
  `specs/012-gomemory-context-distribution/contracts/claude-artifact/SKILL.md`
  a
  `infrastructure/templates/gomemory-context/claude/speckit-gomemory-context-update/SKILL.md`
  (copia literal, ya verificada contra la CLI real — ver
  `contracts/README.md`)
- [X] T007 [US1] Extender `InstallSpeckitExtension`
  (`adapters/primary/setup/speckit_extension.go`) agregando la llamada
  `InstallPlugin(templatesFS, "templates/gomemory-context/claude",
  filepath.Join(root, ".claude", "skills"), nil)` (depende de T003, T006)
- [X] T008 [US1] Test unitario: con `.specify/` presente,
  `.claude/skills/speckit-gomemory-context-update/SKILL.md` queda creado y
  su contenido coincide con el fixture; segunda llamada sin cambios no lo
  reescribe (depende de T007)
- [X] T009 [US1] Ejecutar Escenario 1 de `quickstart.md` contra un binario
  real y un proyecto de prueba con `.specify/` (Claude Code): confirmar
  los 3 destinos y el permiso del script, sin haber corrido `specify
  extension add` en ningún momento (depende de T005, T007)

**Checkpoint**: US1 funcional de forma independiente — Claude Code queda cubierto

---

## Phase 4: User Story 2 - Paridad real con OpenCode (Priority: P1)

**Goal**: el mismo `mem install` deja el brazo extensor igual de funcional
para OpenCode.

**Independent Test**: Escenario 2 de `quickstart.md`.

### Implementation for User Story 2

- [X] T010 [P] [US2] Copiar el fixture verificado
  `specs/012-gomemory-context-distribution/contracts/opencode-artifact/speckit.gomemory-context.update.md`
  a
  `infrastructure/templates/gomemory-context/opencode/speckit.gomemory-context.update.md`
  (copia literal, ya verificada contra la CLI real)
- [X] T011 [US2] Extender `InstallSpeckitExtension`
  (`adapters/primary/setup/speckit_extension.go`) agregando la llamada
  `InstallPlugin(templatesFS, "templates/gomemory-context/opencode",
  filepath.Join(root, ".opencode", "commands"), nil)` — mismo archivo que
  T007 (US1), por lo tanto secuencial respecto a esa tarea, no paralelo
  (depende de T003, T010)
- [X] T012 [US2] Test unitario: con `.specify/` presente,
  `.opencode/commands/speckit.gomemory-context.update.md` queda creado y
  su contenido coincide con el fixture; segunda llamada sin cambios no lo
  reescribe (depende de T011)
- [X] T013 [US2] Ejecutar Escenario 2 de `quickstart.md` contra un proyecto
  de prueba inicializado con `--integration opencode`: confirmar que
  ambos artefactos (Claude y OpenCode) existen sin importar cuál
  integración esté activa en `.specify/integration.json` (depende de T009,
  T011)

**Checkpoint**: US1 y US2 funcionan de forma independiente — Claude Code y OpenCode tienen paridad

---

## Phase 5: User Story 3 - Proyectos sin spec-kit no se ven afectados (Priority: P2)

**Goal**: confirmar que el gate de `.specify/` (ya construido en
Foundational, T003) deja los proyectos sin spec-kit completamente
intactos — esta historia no agrega código nuevo, valida el
comportamiento ya implementado.

**Independent Test**: Escenario 3 de `quickstart.md`.

### Implementation for User Story 3

- [X] T014 [P] [US3] Ejecutar Escenario 3 de `quickstart.md`: `mem install`
  en un proyecto de prueba sin `.specify/`, confirmar que no aparece
  ningún archivo bajo `.specify/extensions/gomemory-context/`,
  `.claude/skills/speckit-gomemory-context-update/` ni
  `.opencode/commands/speckit.gomemory-context.update.md` (depende de T005,
  T007, T011 — necesita el wiring completo para ser una prueba real)
- [X] T015 [US3] Confirmar que el caso "sin `.specify/`" del test T004
  (`adapters/primary/setup/speckit_extension_test.go`) sigue cubriendo
  este escenario tras agregar las llamadas de T007/T011 — si no, agregar
  el caso faltante (depende de T004, T007, T011)

**Checkpoint**: US3 verificada — proyectos sin spec-kit quedan exactamente igual que hoy

---

## Phase 6: User Story 4 - Las correcciones futuras llegan solas (Priority: P3)

**Goal**: confirmar que el criterio de escritura diff-aware (heredado de
`InstallPlugin`, ya construido en Foundational) propaga correcciones sin
reescribir archivos sin cambios.

**Independent Test**: Escenario 4 y 5 de `quickstart.md`.

### Implementation for User Story 4

- [X] T016 [US4] Test unitario explícito en
  `adapters/primary/setup/speckit_extension_test.go`: instalar, modificar
  a mano un archivo destino, volver a llamar
  `InstallSpeckitExtension` con el mismo `templatesFS` → el archivo
  modificado vuelve a coincidir con la plantilla embebida; un archivo no
  tocado no cambia su `mtime` (depende de T007, T011)
- [X] T017 [US4] Ejecutar Escenario 4 (corrección se propaga) y Escenario 5
  (dos corridas seguidas sin cambios, idempotente) de `quickstart.md`
  contra un binario real (depende de T009, T013)

**Checkpoint**: las 4 historias de usuario son funcionales de forma independiente

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: documentación, higiene de código, validación final

- [X] T018 [P] Ampliar la subsección "Brazo extensor hacia spec-kit
  (feature 011)" de `docs/architecture.md` (agregada en la spec 011) con
  cómo se distribuye ahora vía `mem install` (feature 012): plantillas
  embebidas, gate `.specify/`, alcance Claude Code + OpenCode
- [X] T019 [P] Revisar el bullet de spec-kit agregado a `README.md` en la
  spec 011 y ajustarlo si da a entender que hace falta un paso manual —
  debe reflejar que `mem install` ya lo deja listo
- [X] T020 `golangci-lint run` (si está disponible) + `gofmt`/`go vet`
  sobre `adapters/primary/setup/speckit_extension.go`,
  `adapters/primary/setup/speckit_extension_test.go`,
  `adapters/primary/cli/cmd_install.go` (depende de T005, T007, T011)
- [X] T021 `go build ./...` + `go test ./...` completo, sin regresiones
  (depende de T020)
- [X] T022 Ejecutar los 5 escenarios completos de `quickstart.md` de punta
  a punta una vez más, de forma consolidada, contra un binario final
  (depende de T021)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — puede empezar de inmediato
- **Foundational (Phase 2)**: depende de Setup (T002 necesita los
  directorios de T001) — BLOQUEA todas las historias de usuario
- **User Stories (Phase 3-6)**: todas dependen de Foundational
  - US1 y US2 comparten archivo (`speckit_extension.go`), así que T007 y
    T011 son secuenciales entre sí aunque pertenezcan a historias
    distintas — pero cada historia sigue siendo independientemente
    valiosa y probable (se puede entregar US1 sola como MVP)
  - US3 y US4 no agregan código nuevo: validan comportamiento ya
    construido en Foundational/US1/US2, por eso dependen de que esas
    fases estén completas para tener algo real que validar
- **Polish (Phase 7)**: depende de que todas las historias deseadas estén completas

### User Story Dependencies

- **US1 (P1)**: depende de Foundational — es el MVP, entregable sola
- **US2 (P1)**: depende de Foundational; comparte archivo con US1
  (`speckit_extension.go`) pero no depende de que US1 esté "terminada" en
  el sentido de negocio — solo en el sentido de "no chocar edits en el
  mismo archivo al mismo tiempo"
- **US3 (P2)**: valida el gate ya construido en Foundational; más
  significativa una vez US1/US2 existen (para probar que, con todo
  implementado, un proyecto sin spec-kit sigue sin verse afectado)
- **US4 (P3)**: valida el criterio de sobrescritura ya heredado de
  `InstallPlugin`; igual que US3, más significativa con US1/US2 ya
  presentes

### Within Each User Story

- Fixture del agente (T006/T010) antes de extender la función
  (T007/T011)
- Extender la función antes de su test (T008/T012)
- Todo lo anterior antes de la validación end-to-end con quickstart
  (T009/T013)

### Parallel Opportunities

- T001 (Setup) sin dependencias
- T002 (Foundational) en paralelo con T001 completándose (distintos
  archivos, aunque T002 necesita los directorios de T001 creados primero
  en la práctica)
- T005 (wiring en `cmd_install.go`) en paralelo con T004 (tests) una vez
  T003 existe — archivos distintos
- T006 (fixture Claude) y T010 (fixture OpenCode) en paralelo entre sí —
  archivos distintos, ninguna historia depende de la otra a nivel de
  fixture
- T007 (US1) y T011 (US2) NO en paralelo — mismo archivo
  (`speckit_extension.go`)
- T018, T019 (Polish, documentación) en paralelo entre sí

---

## Parallel Example: Foundational + fixtures de agente

```bash
# En paralelo, antes de tocar speckit_extension.go:
Task: "Copiar el árbol fuente de la extensión a infrastructure/templates/gomemory-context/extension/"
Task: "Copiar el fixture de Claude a infrastructure/templates/gomemory-context/claude/.../SKILL.md"
Task: "Copiar el fixture de OpenCode a infrastructure/templates/gomemory-context/opencode/....md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (CRÍTICO — bloquea todas las historias)
3. Completar Phase 3: User Story 1
4. **PARAR Y VALIDAR**: correr Escenario 1 de `quickstart.md`
   independientemente
5. Es un MVP entregable: Claude Code ya recibe el brazo extensor solo con
   `mem install`, aunque OpenCode todavía no (US2 pendiente)

### Incremental Delivery

1. Setup + Foundational → base lista (el árbol fuente ya se distribuye,
   aunque ningún agente lo reconoce todavía sin US1/US2)
2. US1 → validar con quickstart → **MVP**: Claude Code cubierto
3. US2 → validar con quickstart → paridad con OpenCode
4. US3 → validar (comportamiento ya construido, solo se confirma)
5. US4 → validar (comportamiento ya construido, solo se confirma)
6. Polish → documentación + limpieza + validación final consolidada

### Parallel Team Strategy

Con más de una persona: Setup+Foundational juntos; luego una persona toma
US1 (fixture + llamada Claude) y otra toma US2 (fixture + llamada
OpenCode) — coordinando el único punto de conflicto real (ambas tocan
`speckit_extension.go`, conviene que una termine su llamada antes de que
la otra la agregue, aunque el conflicto de merge sería trivial). US3/US4
son tareas de validación que cualquiera puede tomar una vez el resto está
listo.

---

## Notes

- [P] = archivos distintos, sin dependencias entre sí
- [Story] mapea cada tarea a su historia de usuario para trazabilidad
- Los fixtures de agente (T006, T010) son copias literales de artefactos
  ya verificados contra la CLI real `specify` en la fase de planificación
  — no se generan ni se adivinan en esta fase
- US3 y US4 no introducen código nuevo: documentan y verifican garantías
  que ya entrega la reutilización de `InstallPlugin` desde Foundational
- Commit tras cada tarea o grupo lógico
- Parar en cada checkpoint para validar la historia de forma independiente
