# Implementation Plan: Gate de pre-escritura, evidencia de anclas y masa de memorias

**Branch**: `031-memory-gate-evidence-mass` (sin rama creada: se trabaja sobre `main`) | **Date**: 2026-09-13 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/031-memory-gate-evidence-mass/spec.md`

## Summary

gomemory incorpora tres capacidades que se entregan por separado, en orden de
riesgo creciente:

1. **Gate de pre-escritura** (US1): avisa al guardar si la memoria se parece a
   otra del mismo tipo. Siempre guarda. Usa umbrales propios medidos sobre el
   almacén real y declara cuando no pudo comprobar.
2. **Evidencia de anclas** (US2): sección de contexto con las memorias ancladas
   a archivos que faltan en disco (movida, movida ambigua, huérfana candidata).
   Es una hipótesis, no una orden de borrado.
3. **Masa** (US3/US4): PageRank personalizado sobre el grafo de memorias, sin
   checkpoints ni aristas de veredicto. Ordena la sección Sinapsis, alimenta
   `mem mass` y añade hasta 5 vecinos opcionales a `pack_build`.

La investigación sobre el almacén real (research.md) destapó dos defectos que
el plan cierra:

- `ListRelations` recorta a 20 cualquier `limit > 50`, así que el contexto hoy
  solo ve 20 relaciones. Se cierra en US3, porque el cambio toca esa lectura.
- La anomalía 207/209 prueba que el dedup por título no actuó al insertar. Se
  cierra en el prerrequisito P0, como bugfix directo fuera del SDD.

## Technical Context

**Language/Version**: Go 1.27 (módulo `mem`)

**Primary Dependencies**: stdlib; `modelcontextprotocol/go-sdk` (tools MCP); `modernc.org/sqlite` (solo lo usa el calibrador en modo `ro`). Sin dependencias nuevas.

**Storage**: SQLite existente (`memories`, `memory_relations`, `code_files`). Sin migraciones: todo se deriva en lectura.

**Testing**: `testing` stdlib, con tests junto al código. Integración y calibración en `tests/integration/` (el calibrador lleva `//go:build calibration`).

**Target Platform**: binario `mem` autocontenido (macOS/Linux/Windows); servidor MCP sobre stdio.

**Project Type**: CLI + servidor MCP con arquitectura hexagonal (`domain/`, `application/`, `adapters/`, `infrastructure/`).

**Performance Goals**: el gate añade ≤ ~5 ms por guardado con cientos de memorias (una consulta `ListAll` y ≤ 24 Jaccard con poda). La masa converge en ≤ 100 iteraciones sobre ~80 nodos.

**Constraints**: `get_context` respeta `Budget` (24 000 caracteres en este proyecto); salida determinista; ningún cambio de firma en `ports.MemoryRepository` ni en `ports.RelationLister` (memoria 167).

**Scale/Scope**: 152 memorias (78 no checkpoint), 110 relaciones (65 huérfanas) y 420 archivos indexados hoy.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación | Estado |
|-----------|------------|--------|
| I. Hexagonal | `GradeAnchor` y `ComputeMass` son puras en `domain/`. Gate, grafo de masa y secciones van en `application/usecases/`. Tres puertos estrechos nuevos en `application/ports/` (sin ampliar interfaces). El cableado ocurre solo en `infrastructure/container.go` y en los adaptadores primarios. El uso de `os.Stat` en `build_context.go` reutiliza la dependencia que ya tiene para `WriteFile`. | ✅ |
| II. SQLite con SQL directo | Sin SQL nuevo en producción: se reutilizan `ListAll`, `ListAllRelations` y `FileHashesQuery`. El calibrador hace un `SELECT` sin parámetros sobre una conexión `?mode=ro`. | ✅ |
| III. Testing first | Cada hoja de tareas empieza por su test en rojo. **Ningún test existente se modifica**: se comprobó que ninguno fija el orden de Sinapsis ni el recorte de relaciones, y la invariante `ItemsRetrieved = Σ categorías` del pack se conserva. | ✅ |
| IV. Configuración | Los umbrales del gate y `d = 0.85` son constantes de algoritmo documentadas, no valores de entorno. No hay ajustes nuevos. | ✅ |
| V. Operativos | Simplicidad: un solo caso de uso de masa compartido por tres llamadores. Sin parches: el recorte de relaciones se corrige en su causa (lectura completa). Fallar rápido: aplica a P0. El gate es consultivo, así que no bloquea (fire-and-forget del aviso). Idempotencia: sin escrituras nuevas. | ✅ |
| Documentación en español | Artefactos de `specs/` y `docs/` en español; identificadores en inglés (memoria 132). | ✅ |
| Prohibiciones | No se cachea en proceso nada que cambie en caliente; no se importan adaptadores desde aplicación; no hay valores de configuración hardcodeados. | ✅ |

**Re-check post-diseño (Fase 1)**: sin cambios. El diseño no introduce
violaciones, así que Complexity Tracking queda vacío.

**Desviación ya establecida en el repo, no introducida aquí**: la
constitución pide `tests/unit/`, pero la práctica vigente coloca `*_test.go`
junto al código. Esta feature sigue la práctica vigente.

## Project Structure

### Documentation (this feature)

```text
specs/031-memory-gate-evidence-mass/
├── spec.md
├── plan.md              # este archivo
├── research.md          # R0–R8: datos del almacén, anomalía 207/209, decisiones
├── data-model.md        # entidades derivadas y puertos estrechos
├── quickstart.md        # validación V1–V7 contra el sistema real
├── contracts/
│   ├── save-gate.md
│   ├── context-sections.md
│   └── mass.md
├── checklists/requirements.md
└── tasks.md             # lo genera /speckit-tasks
```

### Source Code (repository root)

```text
domain/
├── anchor_evidence.go            # NUEVO  AnchorGrade, AnchorEvidence, GradeAnchor (US2)
├── anchor_evidence_test.go       # NUEVO
├── memory_mass.go                # NUEVO  MassGraph, MassEdge, ComputeMass (US3)
└── memory_mass_test.go           # NUEVO

application/ports/
└── context_builder.go            # +MemoryFullLister, +RelationFullLister, +IndexedFilesQuerier

application/usecases/
├── save_gate.go                  # NUEVO  CheckNearDuplicates, LoadGateCandidates, umbrales (US1)
├── save_gate_test.go             # NUEVO
├── memory_mass.go                # NUEVO  BuildMassGraph, RankMass, seeds (US3/US4)
├── memory_mass_test.go           # NUEVO
├── build_context.go              # anclas sin evidencia; Sinapsis por masa; ListAll por aserción
├── build_context_test.go         # +tests nuevos (no se modifican los existentes)
├── build_context_pack.go         # ContextRequest.Relations + vecinos opcionales
└── build_context_pack_test.go    # +tests nuevos

adapters/primary/cli/
├── cmd_mcp.go                    # save_memory: gate; pack_build: Relations
├── cmd_save.go                   # gate → stderr
├── cmd_pack.go                   # mem pack build: Relations
├── cmd_mass.go                   # NUEVO  mem mass [--task] [--top]
├── cmd_mass_test.go              # NUEVO
├── cmd_mcp_gate_test.go          # NUEVO  test MCP en memoria (patrón cmd_mcp_usage_test.go)
└── dispatcher.go                 # case "mass"

adapters/secondary/persistence/
└── memory_dedup_test.go          # P0: reproducción de la anomalía 207/209 (+ fix en memory.go si procede)

infrastructure/
└── container.go                  # contextBuilder.Files = codeGraphRepo

tests/integration/
└── save_gate_calibration_test.go # NUEVO  //go:build calibration, lectura ro del almacén real

README.md, docs/MANUAL.md, docs/architecture.md   # documentación con bloques "qué NO afirma"
```

**Structure Decision**: la del proyecto existente. Hexagonal con un paquete
por capa y tests junto al código. No se crean paquetes nuevos.

## Fases de implementación

Las hojas atómicas las genera `/speckit-tasks`. Aquí va el orden, las
dependencias y el criterio de hecho de cada fase.

### P0 — Prerrequisito: anomalía 207/209 (bugfix directo, fuera del SDD)

- Test de reproducción en `memory_dedup_test.go`. Primero, dos `Insert` con mismo título, tipo y sesión devuelven el mismo id. Después, un error distinto de `sql.ErrNoRows` en la consulta de dedup no produce un duplicado silencioso.
- Si falla: `findDuplicateTx` propaga los errores que no son "sin filas" y el `Insert` falla rápido. Commit `fix(persistence)` propio.
- Fusionar 207/209: ⚠ requiere aprobación explícita (irreversible).
- **Hecho cuando**: la causa queda escrita en una memoria de bugfix, con el test en verde.

### Fase 1 — US1 Gate (dep: P0) · commit `feat(memory): gate de pre-escritura`

1. Calibrador `tests/integration/save_gate_calibration_test.go`, que produce la tabla y los valores de `T_title`/`T_body` (la persona los confirma si hay compromiso).
2. `save_gate.go` y sus tests: dispara con un par tipo 207/209; no dispara en upsert, con checkpoint ni entre tipos; la poda por tamaño no pierde recall; un error produce `Checked=false`; es determinista.
3. Integración en `save_memory` (MCP) y `mem save` (stderr), con el test MCP en memoria.
- **Hecho cuando**: se cumple V4 contra el MCP real reiniciado.

### Fase 2 — US2 Evidencia de anclas (∥ Fase 1) · commit `feat(context): evidencia de anclas`

1. `GradeAnchor` con su tabla de casos, incluidas la absoluta dentro y fuera de `root`, un directorio y un índice vacío.
2. Puerto `IndexedFilesQuerier`, `Builder.Files` y cableado en `container.go`.
3. Sección en `Build()` con tests: la huérfana aparece; la vigente, la absoluta fuera, la de `Files=nil` y un presupuesto agotado no generan avisos.
- **Hecho cuando**: V5 da 0 falsos positivos sobre el almacén real.

### Fase 3 — US3 Masa y Sinapsis (∥ Fases 1–2) · commit `feat(context): masa de memorias`

1. `ComputeMass` y sus tests de invariantes: suma 1; la semilla domina; la masa colgante vuelve a `p`; `supersedes` es unidireccional; permutar la entrada no cambia la salida; semillas vacías dan uniforme.
2. `BuildMassGraph`/`RankMass` con tests de las reglas de FR-016, incluidas las aristas huérfanas y las de checkpoint.
3. `Build()`: `ListAll` por aserción (cierra el recorte a 20), Sinapsis por masa con leyenda y `fits()`. Tests: un conflicto antiguo con más de 20 relaciones posteriores aparece; 0 checkpoints; orden por masa; determinismo.
4. `mem mass` con sus tests y registro en `dispatcher.go`.
- **Hecho cuando**: se cumple V6 (ranking y sección Sinapsis).

### Fase 4 — US4 Vecinos en el pack (dep: Fase 3.2) · mismo commit que la Fase 3 o uno propio

1. `ContextRequest.Relations` y sus tests: nil da un pack idéntico (comparación profunda con la salida previa); con relaciones añade ≤ 5 opcionales al final; el presupuesto las descarta primero.
2. Cableado en `pack_build` (MCP) y `mem pack build`.
- **Hecho cuando**: se cumple V6 (pack).

### Fase 5 — Verificación y documentación (dep: 1–4)

- V1 y V2 del quickstart.
- README (Key Features, CLI `mem mass`), `docs/MANUAL.md` y `docs/architecture.md`, cada uno con su bloque "qué NO afirma".
- Guardar las decisiones (umbrales medidos, reglas de aristas) y cerrar la sesión.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| El gate avisa demasiado y el agente lo ignora | Umbrales medidos (SC-002), máximo 3 candidatos, silencio en los upserts. |
| El umbral calibrado sobre 78 memorias no generaliza | El calibrador queda versionado detrás de una etiqueta: se recalibra con un comando. |
| Reordenar Sinapsis cambia el contexto que ven todos los agentes | Solo cambia el orden y el universo de una sección ya existente. Leyenda explícita. Tope de 12 intacto. |
| Leer todas las relaciones y memorias en cada `get_context` | Cientos de filas, dos consultas indexadas por `project`. Sin caché (prohibición constitucional). |
| El índice de código está desactualizado y produce una "movida" falsa | Leyenda de hipótesis. "Movida" solo con un candidato único; con varios, "ambigua". |
| El binario servido es el viejo durante la validación | Paso V2 obligatorio antes de V3–V6. |

## Agent context update

No se ejecuta: el repositorio no tiene `CLAUDE.md` ni `AGENTS.md` con los
marcadores `<!-- SPECKIT START -->`/`<!-- SPECKIT END -->`, y crear
`CLAUDE.md` va contra la regla de la persona de no versionar ese archivo. La
referencia al plan vive en `.specify/feature.json`.

## Complexity Tracking

No aplica: sin violaciones de la constitución.
