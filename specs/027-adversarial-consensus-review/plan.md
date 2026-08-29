# Plan de implementación: Revisión Adversarial por Consenso

**Rama**: `main` | **Fecha**: 2026-08-29 | **Especificación**: [spec.md](spec.md)

**Entrada**: Especificación de `specs/027-adversarial-consensus-review/spec.md`

## Resumen

Incorporar a GoMemory un protocolo de revisión adversarial de dos revisores independientes sobre un target congelado, con un motor de consenso, una política de corrección acotada y re-revisión, un presupuesto de rondas, tres estados terminales y promoción de conocimiento reutilizable a memoria. La decisión arquitectónica central: **GoMemory no orquesta modelos ni ejecuta revisores** — sigue el mismo patrón que ya usa `judge_memories`/`RecordVerdict` (el agente que llama razona y decide, GoMemory persiste y valida). El agente/skill orquestador ejecuta los revisores A/B y el agente de corrección (como subagentes o llamadas aisladas) y reporta sus resultados a GoMemory vía tools MCP y CLI; GoMemory es la única autoridad que congela el target, valida cada envío contra las reglas del protocolo, calcula el estado de consenso admisible, cuenta las rondas, deriva el veredicto terminal y promueve memoria — sin que ninguna de esas decisiones dependa solo de que el prompt del agente las respete.

## Contexto técnico

**Lenguaje/versión**: Go 1.25 (módulo `mem`)

**Dependencias principales**: `modelcontextprotocol/go-sdk` (tools MCP nuevas), `modernc.org/sqlite` (persistencia, sin CGO), `github.com/google/uuid` (identificadores de revisión); ninguna dependencia nueva de terceros

**Persistencia**: SQLite existente (`mem.db` en el store global por proyecto), esquema ampliado de forma aditiva en `adapters/secondary/persistence/db.go`; reutiliza la tabla `memories` (con `topic_key`) para la promoción de conocimiento, sin tabla de memoria paralela

**Pruebas**: `testing` + `testify`; unitarias colocadas junto a cada paquete (`domain/*_test.go`, `application/usecases/*_test.go`, `adapters/secondary/persistence/*_test.go`), contrato en `tests/contract/`, integración de los dos flujos end-to-end (aprobado / escalado) en `tests/integration/`

**Plataforma objetivo**: binario CLI + servidor MCP multiplataforma existente (Linux/macOS/Windows), sin infraestructura nueva

**Tipo de proyecto**: extensión del monolito hexagonal existente de GoMemory (nuevo agregado de dominio + casos de uso + adaptadores), no un proyecto ni servicio nuevo

**Objetivos de rendimiento**: sin metas de latencia propias — el protocolo no está en una ruta caliente; el costo dominante es el tiempo de los revisores/agente de corrección, fuera del control de GoMemory

**Restricciones**: GoMemory no ejecuta ni orquesta modelos; el nivel de independencia declarado nunca se presenta como mayor que el efectivamente alcanzado; ninguna operación de entrega (commit/push/merge/PR/deploy/release) se dispara desde este protocolo; el esquema se amplía solo de forma aditiva (sin `DROP`/`RENAME`, conforme a la política ya vigente en `migrate()`)

**Escala/alcance**: uso interactivo por proyecto (una revisión activa típica a la vez, historial acumulativo sin límite duro); mismo orden de magnitud que el resto de tablas de `mem.db`

## Comprobación de la constitución

*Puerta evaluada antes de la investigación y nuevamente después del diseño.*

- **Arquitectura hexagonal**: Cumple. Dominio nuevo puro (`domain/review.go`, `finding.go`, `evidence.go`, `consensus.go`, `verdict.go`) sin I/O; puertos en `application/ports`; casos de uso en `application/usecases`; adaptadores concretos en `adapters/primary/cli`, `adapters/primary/cli` (tools MCP) y `adapters/secondary/persistence`. Ningún import de adaptadores desde dominio/aplicación.
- **SQLite con SQL directo**: Cumple. Tablas nuevas vía `CREATE TABLE IF NOT EXISTS` dentro del mismo `migrate()` aditivo; una columna aditiva (`memories.source_review_id`) vía `addColumnIfMissing`; parámetros bind en todo acceso nuevo; sin ORM.
- **Pruebas primero**: Cumple. Los invariantes del protocolo (derivación de veredicto, elegibilidad de corrección, presupuesto de rondas) son funciones puras de dominio, testeables sin base de datos ni agentes reales; se escriben antes de la implementación.
- **Configuración y entorno**: Cumple. `max_fix_rounds` y las severidades auto-corregibles son configurables por proyecto (reutilizando `adapters/secondary/persistence/settings.go`), con los defaults del spec (2 rondas; CRITICAL/HIGH) si no se configuran.
- **Principios operativos**: Cumple. Reutiliza la tabla `memories` y su dedup por `topic_key` (feature 008) para la promoción de conocimiento en vez de introducir un almacén paralelo; cambio mínimo sobre el esquema existente; idempotencia vía índices únicos, igual que `memory_relations`.
- **Documentación en español**: Cumple en todos los artefactos de esta funcionalidad.

**Resultado previo a Fase 0**: APROBADO, sin excepciones.

**Resultado posterior a Fase 1**: APROBADO. El diseño no introduce una arquitectura nueva ni dependencias externas adicionales; extiende los mismos puertos/casos de uso/tablas que ya existen para memorias y relaciones.

## Estructura del proyecto

### Documentación de esta funcionalidad

```text
specs/027-adversarial-consensus-review/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── mcp-tools.md
│   └── cli-contracts.md
├── checklists/
│   └── requirements.md
└── tasks.md                  # Se generará con /speckit-tasks
```

### Código fuente (raíz del repositorio)

```text
domain/
├── review.go                 # Target, Review, ReviewStage (tipos puros)
├── finding.go                 # Finding, Severity, EvidenceClass
├── consensus.go               # ConsensusStatus, matching estructural (INV-001..004)
├── verdict.go                 # Verdict, DeriveVerdict() puro (INV-009..011)
└── fix.go                     # FixDelta, ReJudgmentState (INV-006..008)

application/ports/
├── review_repository.go       # Review, Target, ReviewerResult, Finding
├── consensus_repository.go    # ConsensusFinding, FixRound
└── (reutiliza) memory_repository.go   # promoción de memoria vía topic_key

application/usecases/
├── start_review.go            # congela el target (FR-001..004)
├── submit_reviewer_result.go  # envío idempotente de un revisor (FR-005..013, FR-039)
├── build_consensus.go         # valida y persiste el veredicto de consenso propuesto (FR-014..018)
├── record_fix.go              # autoriza y registra un fix delta (FR-019..022)
├── rejudge_review.go          # ronda de re-revisión acotada (FR-023..025)
├── finalize_review.go         # deriva el estado terminal (FR-026..030)
└── promote_review_memory.go   # extrae conocimiento reutilizable (FR-031..035)

adapters/secondary/persistence/
└── review.go                  # implementa los puertos anteriores sobre mem.db

adapters/primary/cli/
├── cmd_review.go              # `mem review --diff|--commit|--file`, status, history, show
└── cmd_mcp_review_tools.go    # review_start/submit/status/consensus/fix_record/finalize

tests/contract/
└── review_protocol_test.go    # esquemas de tools MCP y comandos CLI

tests/integration/
├── review_approved_flow_test.go   # revisión → consenso → fix → re-revisión → aprobado
└── review_escalated_flow_test.go  # dos rondas fallidas → escalado
```

**Decisión de estructura**: se extiende el monolito hexagonal existente con un agregado nuevo (`review`), sin crear un módulo, servicio o binario separado, y reutilizando `mem.db`, `memories`/`topic_key` y el patrón `judge_memories`/`RecordVerdict` ya vigente para separar "quién razona" de "quién impone las reglas".

## Seguimiento de complejidad

No aplica: la Comprobación de la constitución no registra violaciones.
