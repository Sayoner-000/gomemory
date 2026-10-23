# Implementation Plan: Base de memoria optimizada, idempotente y con índices

**Branch**: `041-migracion-transaccional` | **Date**: 2026-10-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/041-migracion-transaccional/spec.md`

## Summary

La base original de gomemory (`mem.db`, que incluye el grafo de código propio) se optimiza en tres frentes:

1. **Preparación idempotente y atómica.** `migrate` corre dentro de una única transacción `BEGIN IMMEDIATE … COMMIT`: una sola confirmación en lugar de unas 66, y todo o nada. Una base ya al día se reconoce por `PRAGMA user_version` y no pide el bloqueo de escritura (R13).
2. **Configuración de conexión.** Cada conexión se abre con las mejores prácticas de SQLite para WAL, fijadas en el DSN: `synchronous=NORMAL`, `cache_size`, `temp_store=MEMORY`, `mmap_size` y `journal_size_limit`. Al cerrar se ejecuta `PRAGMA optimize` acotado.
3. **Índices.** Se añade `idx_sessions_project_created` y una prueba que vigila los planes de las consultas frecuentes.

Medición base: preparar una base nueva cuesta hasta 1.760 ms en Windows CI. El objetivo es menos de 400 ms y que no se pierda nada en el primer turno.

## Technical Context

**Language/Version**: Go 1.27 (go.mod `go 1.27.0`; toolchain local go1.27.3)

**Primary Dependencies**: `modernc.org/sqlite v1.58.0` (driver puro Go, pragmas por DSN con `_pragma=`), `database/sql`

**Storage**: SQLite en modo WAL. `mem.db` por proyecto en `<DataHome>/projects/<ProjectKey>/`. `instincts.db` global en `<DataHome>/`.

**Testing**: `go test` (pruebas unitarias junto al código en `adapters/secondary/persistence`; integración y contrato en `tests/`). Windows CI con `test.yml` y el workflow de diagnóstico `diag-windows-timing`.

**Target Platform**: Linux, macOS y Windows (amd64 y arm64), como CLI, servidor MCP y hooks de vida corta.

**Project Type**: CLI y servidor MCP local (arquitectura hexagonal: `domain`, `application`, `adapters`, `infrastructure`).

**Performance Goals**: preparar una base nueva en menos de 400 ms en el peor caso en Windows CI (SC-002). Abrir una base preparada en 10 ms o menos de mediana (SC-004). Ninguna consulta frecuente con `SCAN` (SC-008).

**Constraints**:
- Plazos de los hooks sin cambios: 150 ms para comprimir y 2 s para consolidar y registrar (FR-010).
- Esquema solo aditivo (spec 009).
- Sin integridad referencial (FR-014).
- Sin dependencias nuevas.

**Scale/Scope**: unos 66 pasos de DDL en `migrate`, 1 índice nuevo, 1 versión de esquema (`schemaVersion`), 3 aperturas de bases (`openDBFile`, `MigrateLegacy`, `OpenInstinctGlobal`) y `Container.Close`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio (constitución 2.0.0) | Cumplimiento |
|---|---|
| §1.2 Go: SQLite con `database/sql` y un driver mantenido | ✅ Se mantiene `modernc.org/sqlite` y no hay dependencias nuevas. |
| §5 Persistencia: transacciones controladas; una conexión del pool por operación | ✅ La preparación usa una `*sql.Tx` del pool. Los pragmas por DSN llegan a cada conexión. |
| §6 «`IF NOT EXISTS` no convierte automáticamente una migración en segura. Se valida el estado resultante» | ✅ Golden del catálogo antes y después (FR-008) y prueba de idempotencia (SC-009). |
| §6 «CI prueba desde una base vacía y desde la versión anterior soportada» | ✅ Golden desde una base vacía y pruebas aditivas existentes desde esquemas anteriores (028, compresión), más SC-005. |
| §6 Runner de migraciones (golang-migrate) y «el startup puede migrar solo en desarrollo» | ✅ **Excepción registrada según §23** el 2026-10-23: constitución §25.1 y ADR [docs/adr/0001-migracion-al-abrir-la-base.md](../../docs/adr/0001-migracion-al-abrir-la-base.md). Responsable josegomezJ3810; revisión el 2027-01-23. Esta feature no amplía la excepción: la hace atómica, versionada y vigilada. |
| §16 «Código, identificadores, mensajes técnicos internos y commits en inglés» | ✅ Esta feature usa nombres de pruebas, identificadores, mensajes de fallo de las pruebas y mensajes de commit en inglés (análisis D3). Los comentarios siguen el idioma del archivo que los rodea, y la documentación, la spec y las tareas van en español (§16). El resto del repo no se migra en esta feature. |
| §10 Pruebas deterministas, sin red; repositorios contra el motor real | ✅ Todas las pruebas usan SQLite real en `t.TempDir()`. La medición de tiempos va aparte, en el workflow de diagnóstico, no en aserciones de la suite. |
| §10 «No se modifican pruebas existentes solo para hacer pasar una implementación» | ✅ No se tocan las pruebas existentes. Se añaden las de FR-001, FR-003, FR-008, FR-011 y FR-016. |

**Resultado**: pasa. La excepción de §6 está registrada (§25.1 y ADR 0001). Aviso: `.specify/` está en `.gitignore`, así que la copia en la constitución es local; el registro versionado es el ADR, que vive en `docs/adr/` y sí se integra.

**Re-check post-diseño**: data-model, contrato y quickstart no introducen violaciones nuevas. Sigue pasando.

## Project Structure

### Documentation (this feature)

```text
specs/041-migracion-transaccional/
├── plan.md
├── research.md            # R1-R12: causa, transacción DDL, pragmas, índices, optimize, FK
├── data-model.md          # estados de la preparación, configuración por base, consultas vigiladas
├── quickstart.md          # golden previo, pruebas, comprobación manual, medición en Windows
├── contracts/
│   └── apertura-de-base.md
├── checklists/
│   └── requirements.md
└── tasks.md               # /speckit-tasks
```

### Source Code (repository root)

```text
adapters/secondary/persistence/
├── db.go                      # openDBFile (DSN de R9), migrate(db) = atajo por user_version + BeginTx/migrateSchema/user_version/Commit (R6, R13), migrateSchema(ex), addColumnIfMissing(ex), schemaVersion, índice de sesiones
├── sqlite_dsn.go              # NUEVO: construcción única del DSN con los pragmas de R9 y busy_timeout por base
├── globalstore.go             # MigrateLegacy usa el DSN común
├── instincts.go               # OpenInstinctGlobal usa el DSN común con busy_timeout=100
├── testdata/schema.golden     # NUEVO: user_version + catálogo (R7)
├── db_schema_golden_test.go   # NUEVO: FR-008/FR-019 (golden y -update-schema-golden), FR-003/SC-009
├── db_migrate_atomic_test.go  # NUEVO: FR-001, FR-005, FR-006, FR-007, FR-004/SC-005
├── db_migrate_contention_test.go # NUEVO: FR-018/SC-010 (base al día con un escritor abierto)
├── db_durability_test.go      # NUEVO: US4-AC4 (salida abrupta tras commit, en subproceso)
├── db_pragmas_test.go         # NUEVO: FR-011/SC-007, FR-014
├── sqlite_open_guard_test.go  # NUEVO: FR-012 (todo sql.Open pasa por sqliteDSN, R14)
└── db_query_plans_test.go     # NUEVO: FR-016/SC-008 sobre la lista vigilada

infrastructure/
├── container.go               # campo channel; Close ejecuta analysis_limit=400 + optimize solo si channel != "hook" (FR-013, análisis G1)
└── container_close_test.go    # NUEVO: Close sin error; con el canal hook no ejecuta optimize

docs/adr/
└── 0001-migracion-al-abrir-la-base.md   # NUEVO: ADR de la excepción §6 (§25.1)

specs/041-migracion-transaccional/evidence/
├── windows-timing.md          # línea base y resultados por fase
└── schema-diff.txt            # diff del catálogo antes/después (FR-008)
```

**Structure Decision**: todo el cambio vive en el adaptador de persistencia, más un punto en `infrastructure` (el cierre). El dominio y los casos de uso no cambian. Las pruebas son unitarias en el mismo paquete, porque necesitan funciones no exportadas (`migrate`, `openDBFile`). Solo las de comportamiento de punta a punta van en `tests/`.

## Phases

- **Fase 0 (research.md)**: completa. Todas las incógnitas están resueltas con evidencia, sin `NEEDS CLARIFICATION`.
- **Fase 1 (diseño)**: data-model.md, contracts/apertura-de-base.md y quickstart.md están completos. El paso de contexto del agente no aplica: el repo no tiene `CLAUDE.md` ni `AGENTS.md` con marcadores SPECKIT.
- **Fase 2**: `/speckit-tasks`. Orden, alineado con tasks.md:
  1. Golden previo.
  2. Migración transaccional con `user_version` (Foundational).
  3. Medición en Windows (US1).
  4. Idempotencia y contención (US2).
  5. Compatibilidad (US3).
  6. DSN común, índice, planes y `optimize` (US4).
  7. Polish: suite completa, segunda medición y CI.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| `synchronous=NORMAL` puede perder el último commit ante un corte de energía | Es la contrapartida estándar de WAL, documentada en Assumptions. Si solo se cae el proceso, no hay pérdida. La base nunca queda corrupta. |
| `mmap_size` en sistemas de archivos que no lo admiten | SQLite vuelve a la lectura normal. Se cubre con la suite en los tres sistemas operativos. |
| Una migración larga dentro de una transacción bloquea a otro proceso que abre a la vez | `busy_timeout=5000` y `migrateWithRetry`. La transacción dura milisegundos tras el cambio, frente a los segundos de antes. |
| Abrir una base ya al día esperaría detrás de otro escritor (análisis U1) | Atajo por `user_version` antes de `BEGIN IMMEDIATE`. Medido: 0,17 ms frente a 337 ms (R13). Prueba de contención con SC-010 < 50 ms. |
| Cambiar el esquema sin subir `schemaVersion` deja bases sin migrar | `TestSchemaGolden` falla si el catálogo cambia sin que cambie la versión (FR-019, R7). |
| `PRAGMA optimize` alarga el cierre del servidor MCP | `analysis_limit=400` acota el trabajo. Se mide en la instrumentación (R11). |
| Los hooks sí pasan por `Container.Close` (`infrastructure/main.go:169`, `defer closeContainer`), así que `optimize` correría dentro del plazo de 2 s de `turn-end` (análisis G1) | `Close` solo ejecuta `optimize` cuando el canal del contenedor no es `"hook"`, y una prueba lo comprueba. |
| El golden se rompe con cualquier cambio de esquema futuro | Es lo que se busca: obliga a regenerarlo a sabiendas en el mismo cambio que toca el esquema. |

## Excepciones registradas

- **§6, migración al abrir**: análisis C1 y D2, registrada el 2026-10-23 a petición de la persona. Está en la constitución §25.1 (versión 2.1.0) y en el ADR `docs/adr/0001-migracion-al-abrir-la-base.md`, con regla afectada, motivo, alternativas, riesgos, controles compensatorios, responsable, aprobación (2026-10-23), revisión (2027-01-23), ADR y seguimiento. No hay ticket externo; el seguimiento es esta spec.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Migración al abrir, sin runner (golang-migrate) (§6), registrada en §25.1 y en el ADR 0001 | Excepción preexistente de spec 009: CLI local sin despliegue, con miles de bases por usuario creadas por versiones distintas. | Un runner versionado exigiría un paso de despliegue que no existe. Esta feature no amplía la excepción: la hace atómica y versionada. |
