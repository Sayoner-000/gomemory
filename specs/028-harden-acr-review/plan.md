# Plan de Implementación: Fortalecimiento de la revisión ACR

**Rama**: `028-harden-acr-review` | **Fecha**: 2026-08-29 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de la funcionalidad en `/specs/028-harden-acr-review/spec.md`

## Resumen

La revisión adversarial (ACR, feature 027) puede aprobar una revisión que todavía
contiene un defecto grave, no puede cerrar una revisión de solo lectura y publica
métricas que no coinciden con su contrato. Los tres defectos fueron confirmados por
dos revisores independientes en la revisión `acr_96710834-8273-49f3-bd11-42764b2f11d4`.

El enfoque técnico es mover al **dominio** las invariantes que hoy no existen o viven
dispersas —cobertura total del consenso, derivación conservadora de severidad,
inmutabilidad de rondas y estados terminales, autorización de corrección— y hacer que
la capa de aplicación no pueda persistir ninguna transición sin haberlas validado antes.
La forma de conseguirlo es aditiva: se añaden columnas y una tabla de re-juicios por
revisor, no se reescribe el ledger existente. Ningún hallazgo histórico se borra:
"quitar los hallazgos" significa registrar su corrección con `review_fix_record`,
obtener `RESOLVED` de dos re-juicios independientes y validar el target corregido con
una revisión adversarial nueva.

## Contexto Técnico

**Lenguaje/Versión**: Go 1.25.0 (toolchain go1.25.11); la constitución exige >= 1.22

**Dependencias primarias**: `modernc.org/sqlite` v1.52.0 (sin CGO),
`github.com/modelcontextprotocol/go-sdk` v1.6.1, `github.com/google/uuid` v1.6.0,
`charm.land/bubbletea/v2` (TUI, no afectada)

**Almacenamiento**: SQLite por proyecto (`mem.db`), SQL directo sin ORM, migraciones
idempotentes en `adapters/secondary/persistence/db.go` (`CREATE TABLE IF NOT EXISTS`
+ `addColumnIfMissing`)

**Testing**: `testing` stdlib + `testify`; `tests/unit/`, `tests/integration/`,
`tests/contract/`, más los tests colocados junto al paquete (`domain/*_test.go`,
`application/usecases/*_test.go`)

**Plataforma objetivo**: binario autocontenido `mem` para macOS, Linux y Windows

**Tipo de proyecto**: proyecto único con arquitectura hexagonal
(dominio → aplicación → adaptadores → infraestructura)

**Objetivos de rendimiento**: SC-008 — `review_status` y `review_finalize` responden en
menos de 2 s para revisiones de hasta 1.000 hallazgos en entorno local

**Restricciones**:
- Migración aditiva obligatoria: las revisiones ya persistidas deben seguir leyéndose
- El ledger es append-only para la evidencia; ninguna corrección borra hallazgos
- Los revisores y el actor de corrección son externos: gomemory valida y persiste, no ejecuta modelos
- Redacción de secretos obligatoria en todo campo de texto nuevo

**Escala/Alcance**: revisiones locales por proyecto; 7 casos de uso, 5 tablas del ledger,
7 tools MCP y 4 subcomandos de CLI afectados

## Verificación Constitucional

*PUERTA: debe pasar antes de la Fase 0 y volver a revisarse tras la Fase 1.*

| Principio | Estado | Cómo lo cumple este plan |
|---|---|---|
| I. Arquitectura hexagonal | ✅ | Las invariantes nuevas (cobertura de consenso, severidad derivada, política de corrección, transición de estado) viven en `domain/`, puro y sin I/O. Los casos de uso solo orquestan; el ledger sigue detrás de `ports.ReviewRepository` / `ports.ConsensusRepository`. Ningún import de adaptadores desde dominio o aplicación. |
| II. SQLite con SQL directo | ✅ | La tabla `rejudgments` y las columnas nuevas se añaden con `CREATE TABLE IF NOT EXISTS` y `addColumnIfMissing`, con parámetros bind. La transacción de `RecordFix` usa `BEGIN IMMEDIATE` + commit explícito. |
| III. Testing First | ✅ | Cada FR entra con su test en rojo primero. **Excepción declarada y autorizada por la spec** (sección Assumptions): los tests existentes que hoy exigen el comportamiento defectuoso —consenso parcial aceptado, severidad tomada del orquestador, métricas sin `duration`— se actualizan; el resto queda intacto. Cada test modificado se lista en `tasks.md` con su justificación. |
| IV. Configuración y entorno | ✅ | La política de revisión ya existe en `Settings` (`review_max_fix_rounds`, `review_auto_fix_severities`); este plan la **lee** en `StartReview` en vez de reimplantar defaults, y añade `review_fix_authorized` al mismo lugar. Sin valores nuevos hardcodeados. |
| V. Principios operativos | ✅ | Idempotencia (FR-005) es requisito explícito; fallar rápido en el borde (validación completa antes de escribir, patrón ya usado en `RecordFix` y `RejudgeReview`); causa raíz y no parche; documentación en español. |

**Resultado de la puerta (pre-Fase 0)**: PASA con una excepción declarada en el
principio III, ya autorizada por la especificación.

**Resultado de la puerta (post-Fase 1)**: PASA. El diseño no introduce ninguna capa
nueva, ningún framework, ninguna dependencia adicional y ninguna tabla que no derive
directamente de un requisito funcional. La sección de Seguimiento de Complejidad queda
vacía.

## Estructura del Proyecto

### Documentación (esta funcionalidad)

```text
specs/028-harden-acr-review/
├── plan.md              # Este archivo
├── spec.md              # Especificación (ya validada por checklists/requirements.md)
├── research.md          # Fase 0 — decisiones técnicas y alternativas
├── data-model.md        # Fase 1 — entidades, campos, invariantes, transiciones
├── quickstart.md        # Fase 1 — guía de validación ejecutable
├── contracts/
│   ├── mcp-tools.md     # Contrato de las tools MCP de revisión (delta sobre 027)
│   └── cli-contracts.md # Contrato de `mem review` (delta sobre 027)
├── checklists/
│   └── requirements.md  # Calidad de la spec (completado)
└── tasks.md             # Fase 2 — lo genera /speckit.tasks, NO este comando
```

### Código fuente (raíz del repositorio)

```text
domain/                                  # Puro, sin I/O — aquí viven las invariantes
├── review.go                            # Review, Target, ReviewStatus, transiciones
├── verdict.go                           # DeriveVerdict, ReJudgmentState
├── finding.go                           # Finding, Severity, EvidenceClass
├── fix.go                               # AuthorizeFix, NextFixRound
├── consensus.go                         # ConsensusFinding, ConsensusStatus
├── review_learning.go                   # ReviewLearning → Memory
├── review_policy.go                     # NUEVO: política (fix autorizado, rondas, severidades)
├── consensus_coverage.go                # NUEVO: cobertura/unicidad y severidad derivada
└── rejudgment.go                        # NUEVO: ReJudgment por revisor y su agregación

application/
├── ports/
│   ├── review_repository.go             # +identidad esperada de revisores
│   └── consensus_repository.go          # +re-juicios por revisor, +fix transaccional
└── usecases/
    ├── start_review.go                  # Lee la política del proyecto; persiste revisores esperados
    ├── submit_reviewer_result.go        # Guarda de estado terminal + identidad esperada
    ├── build_consensus.go               # Cobertura total, severidad derivada, idempotencia
    ├── record_fix.go                    # Cadena de targets + transición indivisible
    ├── rejudge_review.go                # Re-juicio por revisor y agregación
    ├── finalize_review.go               # Métricas del contrato + duración
    └── promote_review_memory.go         # Exige veredicto APPROVED

adapters/
├── secondary/persistence/
│   ├── db.go                            # Migración aditiva: tabla rejudgments + columnas
│   └── review.go                        # Persistencia del ledger + redacción
└── primary/cli/
    ├── cmd_review.go                    # Congelado de cambios pendientes (--pending)
    └── cmd_mcp_review_tools.go          # DTOs con nombres del contrato publicado

tests/
├── contract/                            # Contratos MCP y CLI de revisión
├── integration/                         # Ciclo completo sobre BD real
└── unit/                                # Invariantes de dominio
```

**Decisión de estructura**: proyecto único con arquitectura hexagonal, la que ya
usa el repositorio. No se crean directorios nuevos: los tres archivos nuevos de dominio
entran en `domain/`, junto a los que ya modelan la revisión.

## Enfoque por Historia de Usuario

Cada historia es entregable de forma independiente y deja el sistema en verde.

| Historia | Prioridad | Núcleo técnico | FR cubiertos |
|---|---|---|---|
| US1 — Impedir aprobaciones falsas | P1 | `domain/consensus_coverage.go` + `BuildConsensus` reescrito | FR-001..FR-007 |
| US2 — Corregir y revalidar con trazabilidad | P1 | `domain/rejudgment.go` + tabla `rejudgments` + fix transaccional | FR-008..FR-014 |
| US3 — Ciclo de vida y política | P2 | `domain/review_policy.go` + guardas de estado terminal | FR-015..FR-021 |
| US4 — Auditoría y target completo | P2 | DTOs del contrato + `--pending` en la CLI | FR-022..FR-027 |
| US5 — Cerrar sin borrar historial | P3 | Ejecución del protocolo sobre la revisión original y una nueva | FR-028..FR-030 |

US1 es la que cierra el defecto HIGH y debe ir primero. US2 depende de US1 solo para
compartir el patrón de validación-antes-de-escribir, no para funcionar. US5 es
verificación en vivo: no aporta código de producto y depende de que US1..US4 estén
implementadas.

## Riesgos y Mitigaciones

| Riesgo | Impacto | Mitigación |
|---|---|---|
| La derivación conservadora de severidad rompe revisiones históricas al releerlas | Un ledger antiguo deja de cargar | La derivación se aplica solo en escritura (`BuildConsensus`); la lectura devuelve lo persistido tal cual |
| `rejudgment_state` agregado en `consensus_findings` queda desincronizado de la tabla nueva | Dos fuentes de verdad para lo mismo | La columna pasa a ser **derivada**: se recalcula desde `rejudgments` en cada escritura, nunca se acepta del llamador |
| Actualizar tests existentes oculta una regresión real | Falso verde | Cada test modificado se lista en `tasks.md` con el comportamiento viejo y el nuevo; la validación final es la revisión adversarial de US5, no la suite |
| Congelar cambios pendientes con `git status --porcelain` incluye archivos ignorados o binarios enormes | Target no reproducible | Se usa la lista de rutas de git (respeta `.gitignore`) y se hashea contenido con separadores nulos, como ya hace `resolveFileTarget` |

## Seguimiento de Complejidad

> Sin violaciones constitucionales que justificar. Esta sección queda vacía
> deliberadamente: el plan no añade proyectos, capas, frameworks ni dependencias.
