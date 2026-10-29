# Implementation Plan: HCD — protocolo propio de especificación y desarrollo continuo

**Branch**: `045-hcd-protocol` (se crea en T001, antes de implementar) | **Date**: 2026-10-27 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/045-hcd-protocol/spec.md`

## Summary

HCD es un protocolo propio de GoMemory que conduce una unidad de trabajo desde la intención hasta la entrega verificada. La persona lo usa con `/hcd`, `$hcd` y una secuencia de skills `hcd-*` (al estilo `speckit-*`). Esas skills delegan cada mutación en un motor interno `mem hcd … --json`, que aplica las reglas duras: revisiones, idempotencia, atomicidad, «sin evidencia no hay verificado» y aprobación citada.

Reparto de las fuentes de verdad:

- El contenido redactado vive en artefactos Markdown versionados (`hcd/features/{NNN}-{slug}/definition.md` y `delivery.md`).
- El estado de control vive en tablas `hcd_*` de la base SQLite del proyecto, enlazadas con el grafo de GoMemory (aristas `hcd_links`, decisiones como memorias `decision` con relaciones `supersedes`).
- `verification.md` se genera desde ese estado.

Spec Kit sigue intacto. La §18 de la Constitución se corrige en D1 para exigir artefactos por enfoque.

**MVP** = recorrido mínimo de US2, US3 y US6, más lo imprescindible de US4 (retomar) y US8 (agente autorizado), con `/hcd` en Claude Code. Después siguen US1 (ceremonia en la TUI), US5 (Codex y OpenCode verificados), gobierno (excepciones y Playbook) y US7 (migración).

## Technical Context

**Language/Version**: Go (versión fijada en `go.mod` del repositorio).

**Primary Dependencies**: solo las existentes: biblioteca estándar, el driver SQLite que ya usa `persistence` y Bubble Tea para la pantalla TUI. **Sin dependencias nuevas** (§20, R5).

**Storage**: base SQLite por proyecto que ya abre GoMemory; tablas `hcd_*` nuevas, aditivas, con `schemaVersion` de 3 a 4 (§25.1, R2). Artefactos Markdown bajo `hcd/` en el repositorio del proyecto.

**Testing**: `go test`. Pruebas unitarias junto al código; contrato e integración en `tests/contract` y `tests/integration`; SQLite real en archivo temporal, como en `persistence/socratica_test.go`; guarda de capas en `tests/contract/layer_rules_test.go`.

**Target Platform**: CLI local y servidor MCP en macOS, Linux y Windows (el CI ya cubre Windows; ojo con las rutas y con los `t.TempDir`).

**Project Type**: CLI, más skills de agente y TUI.

**Performance Goals**: las consultas (`resume`, `status`, `next`) responden en menos de 300 ms con unidades de hasta 50 tareas, para que `/hcd` no añada latencia perceptible. Los timeouts de consulta siguen `domain.DefaultQueryTimeout`.

**Límites operativos configurables** (§20: ningún límite que dependa del entorno queda fijo en el código): siguen el patrón de `domain.SocraticaSettings` (punteros para distinguir ausente de inválido, valor por defecto y rango validados en `domain`). Las claves nuevas en `settings.json` son `hcd_heartbeat_timeout_ms` (por defecto 200, mínimo 50), `hcd_grant_timeout_hours` (por defecto 24, mínimo 1) e `hcd_inactive_days` (por defecto 7, mínimo 1).

**Constraints**: sin nuevas herramientas MCP (spec); no tocar `specs/` (salvo `specs/045-hcd-protocol/`, donde se guardan las evidencias de esta feature), `adapters/secondary/speckit` ni las skills `speckit-*` (FR-006); SQL parametrizado; `UnitOfWork` en las mutaciones; secretos nunca persistidos (`domain.ProtectText`).

**Scale/Scope**: decenas de unidades por proyecto y decenas de tareas por unidad. 9 skills en el MVP y 3 posteriores.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| § | Regla | Cumplimiento | Estado |
|---|---|---|---|
| 3 | Hexagonal; ports en `application/ports` | Dominio `domain/hcd*.go` puro; casos de uso `application/usecases/hcd_*.go`; ports con la convención `{Name}Repository` para persistencia (`HCDRepository`, `HCDArtifactRepository`, `HCDContextRepository`) más `HCDUnitOfWork`; adaptadores `persistence/hcd*.go`, `hcdfiles/` y `cli/hcd/`. Lo vigila `layer_rules_test.go`. Los ports previos que no siguen la convención (`original_store.go`, `speckit_reader.go`) quedan fuera del alcance y no se imitan. | ✅ |
| 4 | Composition root único | El wiring se hace en `infrastructure/container.go`, como `NewSocraticaStore`. El nombre del archivo se cubre con la excepción §25.2, aprobada en T004. | ⚠ Gate condicionado a T004 (aprobación) y T005 (aplicación) |
| 5 | SQL parametrizado, `UnitOfWork`, timeouts, sin `SELECT *` | Patrón de `SocraticaStore` (`WithTransaction` y `writeContext` con reintento ante BUSY). | ✅ |
| 6 / 25.1 | Migración al abrir, solo aditiva | Solo `CREATE TABLE/INDEX IF NOT EXISTS hcd_*`, `schemaVersion` a 4 y golden actualizado; pruebas desde base vacía y desde v3. | ✅ |
| 10 | Pruebas deterministas, reloj y IDs inyectados, cobertura ≥ 80 % sin bajar la línea base | Reloj mediante el port `ports.ClockPort` existente. IDs de elementos deterministas (`HCD-NNN`, `D-NN`, …) derivados del estado. `execution_id` y `operation_id` generados mediante una función inyectable en el caso de uso (no existe un port de IDs). Pruebas de transiciones sin red. | ✅ |
| 15/20 | Sin dependencias nuevas; sin secretos | Parser con la biblioteca estándar; citas y evidencias pasan por `ProtectText`. | ✅ |
| 16 | Commits convencionales y español | Aplica a la implementación. | ✅ |
| 18 | Documentación obligatoria del proyecto | Se actualizan `docs/DATABASE.md` (tablas `hcd_*`, esquema v4, retención de heartbeats) y `docs/ARQUITECTURA.md` (slice HCD, fuentes de verdad, flujo skill → motor → estado de control), y se crea `docs/adr/0002-hcd-estado-de-control-en-sqlite.md` (tareas T003, T034 y T116). | ✅ |
| 18 | Artefactos Spec Kit obligatorios por feature | **Conflicto**: las unidades HCD no los tendrán. Se resuelve con FR-010 y la propuesta [constitution-18-amendment.md](./contracts/constitution-18-amendment.md), tarea bloqueante de D1 con aprobación del Implementador. Esta feature 045 sí cumple la §18 vigente (es Spec Kit). | ⚠ Gate condicionado: ninguna unidad HCD real antes de aprobar la corrección |
| 21.6 | Idempotencia | `hcd_operations` (R9). | ✅ |

**Resultado**: PASS con una condición explícita (§18), gestionada como tarea y no como excepción. No hay violaciones que justificar en Complexity Tracking.

**Re-check post-diseño**: el diseño de la Fase 1 mantiene las mismas capas y no añade dependencias ni herramientas MCP. El gate sigue en PASS con la condición de la §18.

## Project Structure

### Documentation (this feature)

```text
specs/045-hcd-protocol/
├── spec.md
├── plan.md                         # este archivo
├── research.md                     # R1–R18
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── engine-cli.md               # mem hcd: comandos, JSON y errores
│   ├── skills.md                   # /hcd, $hcd, secuencia hcd-*
│   ├── artifact-format.md          # definition/delivery/evidencia/verification
│   └── constitution-18-amendment.md
├── checklists/requirements.md
└── tasks.md                        # /speckit-tasks
```

### Source Code (repository root)

```text
domain/
├── hcd.go                      # Unit, Kind, State, transiciones, invariantes
├── hcd_definition.go           # Story, Criterion, validación de la definición mínima
├── hcd_delivery.go             # Slice, Task, AtomicityViolations, grafo sin ciclos
├── hcd_evidence.go             # vigencia, veredicto PASS/FAIL/BLOCKED, cierre
├── hcd_revision.go             # ContentRevision (huella de 12 hex)
├── hcd_errors.go               # errores tipados del motor
├── hcd_quote.go                # validación de la cita de aprobación
├── hcd_control.go              # Execution, Evidence, Decision, Clarification, Block, Approval, Link, Focus, Heartbeat
├── hcd_governance.go           # precedencia normativa (FR-011)
└── hcd_*_test.go

application/
├── ports/
│   ├── hcd_repository.go       # HCDRepository (estado de control) y HCDContextRepository (lectura para la sección «HCD activo»)
│   ├── hcd_unit_of_work.go     # HCDUnitOfWork
│   └── hcd_artifact_repository.go # HCDArtifactRepository (leer/escribir/hash de artefactos)
└── usecases/
    ├── hcd_flow.go             # Start, Define, Validate, Approve, núcleo mutate (revisión, idempotencia, edición externa)
    ├── hcd_clarify.go          # aclaraciones
    ├── hcd_query.go            # Resume, Next, Show (solo lectura)
    ├── hcd_delivery.go         # Slice, Tasks, Check, Run, Report, vencimiento de concesiones
    ├── hcd_block.go            # bloqueos
    ├── hcd_verify.go           # Verify, Status, Close
    ├── hcd_verify_render.go    # render determinista de verification.md
    ├── hcd_memory.go           # RecordDecision, memorias, supersedes, conflictos, memory sync
    ├── hcd_approach.go         # List, detección de enfoque, classify
    ├── hcd_governance.go       # Exception, PlaybookShow/Validate/Adopt
    ├── hcd_migrate.go          # MigratePreview/Activate
    ├── hcd_focus.go            # unidad activa, heartbeat y desvíos (FR-041a–c)
    ├── build_context.go        # (mod) sección «## HCD activo» (FR-041d)
    ├── session_delta.go        # (mod) "## HCD activo" en deltaSections
    └── hcd_*_test.go

adapters/
├── secondary/
│   ├── persistence/
│   │   ├── hcd_schema.go       # tablas hcd_* (aditivas)
│   │   ├── hcd.go              # HCDStore: HCDRepository + HCDUnitOfWork
│   │   ├── db.go               # (mod) incluir hcdSchema, schemaVersion 4
│   │   └── hcd_test.go
│   └── hcdfiles/
│       ├── store.go            # HCDArtifactRepository sobre hcd/ del proyecto
│       ├── parse.go            # formato de contracts/artifact-format.md
│       └── *_test.go
└── primary/
    ├── cli/
    │   ├── hcd/cmd_hcd.go      # slice CLI: Run(deps, args) int
    │   ├── hcd/output.go       # envoltorio JSON y exit codes
    │   ├── dispatcher.go       # (mod) case "hcd"
    │   ├── cmd_hook.go         # (mod) heartbeat en todo turn-end (incluidos los fines de turno de subagentes), antes de la salida temprana del checkpoint
    │   └── presentation.go     # (mod) commandHelp de mem hcd
    ├── setup/
    │   └── hcd_skills_setup.go # InstallHCDSkills (InstallAgentSkill + envoltorios OpenCode)
    └── tui/
        └── specification.go    # screenSpecification (US1, incremento P2)

infrastructure/
├── container.go                # (mod) wiring HCDStore, HCDArtifactRepository, casos de uso
└── templates/hcd/
    ├── hcd/SKILL.md, hcd-define/SKILL.md, … hcd-close/SKILL.md
    ├── commands/hcd*.md        # envoltorios OpenCode
    ├── hcd-playbook.md         # valor por defecto del documento fijado `playbook` (R18)
    └── playbook/templates/     # definition.md, delivery.md (plantillas de unidad)

domain/seed.go                  # (mod) entrada `playbook` en PinnedDocs + TopicHCDPlaybook
adapters/primary/cli/cmd_constitution.go  # (mod) atajo `mem playbook`; --sync con destino por alias

tests/
├── contract/hcd_engine_contract_test.go   # JSON, códigos de salida, consultas sin efecto
└── integration/hcd_flow_test.go           # recorrido mínimo extremo a extremo con el binario
```

**Structure Decision**: un slice vertical HCD que atraviesa las capas existentes con el mismo patrón que Socrática (features 039, 042 y 044): dominio puro, casos de uso con ports, adaptador de persistencia con su `*_schema.go`, slice CLI propio y plantillas embebidas instaladas por `setup`. No se reorganizan paquetes (fuera de alcance).

## Fases de entrega (alineadas con `tasks.md`)

| Fase | Contenido | Historias / criterios | Salida verificable |
|---|---|---|---|
| **1. Gobierno (D1)** | Crear la rama y la línea base; aprobar y aplicar la corrección de la §18 y la excepción §25.2; Playbook como documento fijado `playbook` (valor por defecto que remite a la Constitución sin copiarla); `mem playbook`; plantillas de unidad; ADR | FR-007, FR-007a a FR-010 | §18 aplicada con `mem constitution`; `mem playbook` funcional |
| **2. Fundamentos** | Dominio, ports, `hcd_schema` + `HCDStore`, `hcdfiles`, esqueleto `mem hcd --json`, `Focus`, `DATABASE.md` | Base de todo, FR-041a | Esquema v4 desde una base vacía y desde v3; contrato JSON |
| **3. US2** 🎯 | `start`, `define`, `validate`, `clarify`, `approve` (con `--request`), consultas; skills `hcd`, `hcd-define`, `hcd-clarify`, `hcd-approve` | C-03, C-04, C-11, C-17, C-18, FR-016a, FR-017a/b | Quickstart, escenarios 1 (pasos 1–3) y 4 |
| **4. US3** 🎯 | `slice`, `tasks`, `check`, `run`, `report`, `block`, `list`; skills `hcd-slice`, `hcd-tasks`, `hcd-implement` | C-05, C-06, C-07 | Escenario 1 (pasos 4–5) |
| **5. US6** 🎯 | `verify`, `verification.md`, invalidación, `close`, `close --reject` y `cancel`; skills `hcd-verify`, `hcd-close`; evidencia real con `/hcd` en Claude Code | C-08, C-09, C-22, C-23, SC-001 | Escenarios 1 y 3 |
| **6. US4** | Decisiones → memorias, `decision`, conflictos, heartbeat, sección «HCD activo» | C-10, C-12, C-13, FR-041b–d, FR-042, SC-011 | Escenarios 5 y 8 |
| **7. US8** | Ejecución incierta, sin estados de cumplimiento externos, detención localizada | C-19, FR-028, FR-036 | Escenario 2 |
| **8. US1** | Detección de enfoques, `classify`, pantalla «Especificación» en la TUI (inicia y consulta, no aprueba) | C-01, C-02, FR-004, FR-017b | Escenario 6 |
| **9. US5** | Instalación de las 12 skills y sus envoltorios; equivalencia de `Start` entre la TUI y la CLI; evidencias reales | C-16, SC-008 | Escenario 7 |
| **10. Gobierno ampliado (US6)** | Precedencia normativa, excepciones, `playbook show/validate/adopt` | FR-011, FR-012, FR-037, C-14, C-15 | Pruebas de precedencia, vencimiento y adopción |
| **11. US7** | `migrate --preview/--activate`, skill `hcd-migrate` | C-20, C-21, SC-009 | Migración representativa |
| **12. Cierre** | Documentación, suite completa, quickstart completo, matriz de criterios, Spec Kit sin cambios | SC-002, SC-003, C-24 | Evidencias en `specs/045-hcd-protocol/evidence/` |

Las fases 1 a 5 forman el MVP; la 6 completa la memoria; de la 7 a la 11 son incrementos sobre el recorrido comprobado.

## Riesgos y cierre

| Riesgo | Mitigación en el diseño |
|---|---|
| Una cita de aprobación fabricada por el agente (FR-017a) | Heurística de cita (R7), origen visible en `verification.md` y riesgo declarado en el Playbook. Se aceptó en Clarifications. |
| Divergencia entre los artefactos y el estado de control | Comparación de hashes antes de cada mutación (FR-040c); los estados nunca se escriben en Markdown. |
| Cambio de esquema que rompa bases existentes | Solo tablas nuevas; golden de `schemaVersion`; prueba desde v3. |
| Tamaño (72 FR) | Fases 1–5 como MVP con salida verificable cada una; el resto, incremental. |
| Marca de feature activa en archivos de contexto del agente (estilo `SPECKIT`) | Marca interna `hcd_focus` única por proyecto y leída por todos los agentes (R15); prueba que verifica que HCD no modifica nada fuera de `hcd/`. |
| Heartbeat que ralentice o rompa el turno | Corre en proceso dentro de `turn-end`, con plazo ≤ 200 ms y best-effort; no crea memorias; retención compactada (R16). |
| Skills que avanzan sin el motor | Regla 2 del contrato de skills y una prueba que verifica que cada `SKILL.md` contiene las reglas comunes (como `tui_docs_test`). |

## Complexity Tracking

§4 nombra `infrastructure/composition.go`, pero el proyecto usa `infrastructure/container.go` como único composition root. Como esta feature añade cableado ahí, la desviación se registra como excepción §25.2 (texto en `contracts/constitution-18-amendment.md`), que se aprueba junto con la §18 en T004.

Sin violaciones sin justificar: la §4 queda cubierta por la excepción §25.2 registrada (ver arriba) y el conflicto de la §18 se resuelve con una corrección aprobada (FR-010), no con una excepción.
