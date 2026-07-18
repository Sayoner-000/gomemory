# Tasks: Mitigación de riesgos operativos de gomemory

**Input**: Design documents from `/specs/009-mitigacion-riesgos/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (todos presentes)

**Tests**: Incluidos y OBLIGATORIOS — la constitución del proyecto (`Principio III: Testing First`) exige TDD estricto (Red-Green-Refactor), no es opcional aquí.

**Organización**: Tareas agrupadas por historia de usuario (spec.md), en orden de prioridad P1→P4. Cada historia es independientemente implementable, testeable y entregable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Puede ejecutarse en paralelo (archivos distintos, sin dependencias pendientes)
- **[Story]**: US1 (backup), US2 (secretos+permisos), US3 (búsqueda FTS5), US4 (convención documentada)

---

## Phase 1: Setup

**Purpose**: Preparación mínima compartida. No hay inicialización de proyecto ni dependencias nuevas que instalar (stack ya congelado, sin librerías nuevas).

- [X] T001 [P] Documentar la variable de entorno `GOMEMORY_BACKUP_KEEP` (default `10`) — **ajuste de alcance real**: el proyecto no tiene `.env.example` ni documenta `GOMEMORY_DATA_HOME` (su único precedente) en ningún archivo separado; se documenta igual que ese precedente, como comentario Go en la constante de su punto de definición, no en un archivo nuevo que rompería la convención real

---

## Phase 2: Foundational

**Purpose**: Prerrequisitos bloqueantes compartidos por las 4 historias.

**No hay fase foundational bloqueante**: las 4 historias son independientes por diseño (ver spec.md, sección de prioridades) y no comparten un componente base nuevo que construir primero. La única coordinación necesaria es a nivel de archivo, no de dependencia funcional: **T020 (US3)** y **T023 (US4)** ambas editan `migrate()` en `db.go` — deben aplicarse de forma secuencial (no simultánea) para evitar conflictos de merge, aunque ninguna depende funcionalmente de la otra. Ver nota en Dependencies.

**Checkpoint**: no aplica — se puede pasar directo a cualquier historia tras Phase 1.

---

## Phase 3: User Story 1 - Recuperar memorias tras pérdida de datos (Priority: P1) 🎯 MVP

**Goal**: Generar automáticamente, al cerrar sesión, un snapshot exportable de memorias+relaciones, con retención acotada y restaurable vía el import ya existente.

**Independent Test**: quickstart.md sección 1 — cerrar sesión, verificar snapshot nuevo, simular pérdida del `mem.db`, restaurar con `mem import` y confirmar que las memorias reaparecen sin duplicados.

### Tests for User Story 1 ⚠️

> Escribir estos tests PRIMERO, confirmar que fallan antes de implementar

- [X] T002 [P] [US1] Test unitario de `CreateSnapshot` (bundle válido, poda de snapshots antiguos al superar `keep`, error si `backupDir` es inutilizable) en `application/usecases/backup_test.go` (repos reales sobre SQLite temporal, como `portability_test.go` — este repo no usa mocks, usa `persistence.Init(t.TempDir())`)
- [X] T003 [P] [US1] Test de `BackupDir` (resuelve bajo `<DataHome>/backups/<key>/`, distinto del directorio de datos activo) en `adapters/secondary/persistence/backup_test.go`, + test de integración end-to-end del hook (`TestHookSessionEndCreatesBackupSnapshot`, `TestHookSessionEndWithoutActiveSessionSkipsBackup`) en `tests/integration/backup_hook_integration_test.go`

### Implementation for User Story 1

- [X] T004 [US1] Implementado `CreateSnapshot(memRepo, relRepo, project, backupDir string, keep int) (path string, err error)` en `application/usecases/backup.go`, reutilizando `ExportProject`+`EncodeBundle` sin modificarlos; poda por `ModTime` (no por nombre) vía `pruneSnapshots`
- [X] T005 [US1] Agregado `BackupDir(key string) (string, error)` en `adapters/secondary/persistence/globalstore.go`, bajo `<DataHome>/backups/<key>/`
- [X] T006 [US1] Integrada llamada best-effort a `CreateSnapshot` en `hookSessionEnd` vía `backupSessionSnapshot`, `adapters/primary/cli/cmd_hook.go` — error descartado en silencio, nunca aborta el cierre de sesión
- [X] T007 [US1] `GOMEMORY_BACKUP_KEEP` (default `10`) leída con `os.Getenv` en `backupSessionSnapshot`, mismo patrón que `dataHomeEnvOverride`; documentada como comentario Go en la constante (no hay `.env.example` en el repo real — ver nota en T001)
- [X] T008 [P] [US1] Agregada sección en `README.md` (junto a "Memoria portable") sobre el backup automático, retención por `GOMEMORY_BACKUP_KEEP`, y la advertencia de no sincronizar `mem.db` crudo

**Verificado**: `go build ./...`, `go vet ./...` y `go test ./...` pasan completos tras esta historia.

**Checkpoint**: User Story 1 completamente funcional y testeable de forma independiente (quickstart.md §1).

---

## Phase 4: User Story 2 - Evitar que un secreto pegado quede en texto plano (Priority: P2)

**Goal**: Redacción por patrones de secretos conocidos como segunda capa (además de `<private>`), más hardening de permisos de archivo/directorio.

**Independent Test**: quickstart.md sección 2 — guardar una memoria con un secreto de prueba fuera de `<private>`, confirmar que queda redactada en la BD; confirmar permisos `0600`/`0700` en un proyecto nuevo.

### Tests for User Story 2 ⚠️

> Escribir estos tests PRIMERO, confirmar que fallan antes de implementar

- [X] T009 [P] [US2] Test de `RedactSecrets` (6 patrones + sin falsos positivos + composición con `RedactPrivate` en cualquier orden) en `domain/redact_test.go` (archivo nuevo)
- [X] T010 [P] [US2] Tests de integración: `InsertMemory`/`ImportMemory`/`SetSessionLastPrompt` redactan secretos de prueba fuera de `<private>` en la BD real, en `adapters/secondary/persistence/redact_persistence_test.go`
- [X] T011 [P] [US2] Test de permisos `0700`/`0600` tras `Open` en proyecto nuevo, en `adapters/secondary/persistence/permissions_test.go` (skip explícito en Windows vía `runtime.GOOS`)

### Implementation for User Story 2

- [X] T012 [US2] Implementado `RedactSecrets(s string) string` en `domain/redact.go` con los 6 patrones fijos y placeholder `[REDACTED:<label>]`
- [X] T013 [P] [US2] `RedactSecrets` encadenada con `RedactPrivate` en `memory.go` (`InsertMemory`, `ImportMemory`)
- [X] T014 [P] [US2] `RedactSecrets` encadenada con `RedactPrivate` en `session.go` (`SetSessionLastPrompt`)
- [X] T015 [US2] `os.MkdirAll` cambiado de `0755` a `0700` en `globalstore.go` (`doMigrateLegacy`) y `db.go` (`EnsureDir`)
- [X] T016 [US2] `os.Chmod(path, 0600)` agregado tras `sql.Open` en `db.go` (`Open`) y `globalstore.go` (`doMigrateLegacy`)

**Verificado**: `go build ./...`, `go vet ./...` y `go test ./...` pasan completos (incluye `legacy_migration_test.go`, no roto por el hardening de permisos).

**Checkpoint**: User Stories 1 y 2 funcionan de forma independiente (quickstart.md §2).

---

## Phase 5: User Story 3 - Encontrar la memoria correcta cuando el volumen crece (Priority: P3)

**Goal**: Tabla FTS5 `memory_search` con ranking `bm25()`, dual-write manual desde las escrituras de `memories`, y fallback a `LIKE` si FTS5 no está disponible en el build.

**Independent Test**: quickstart.md sección 3 — buscar un término presente en varias memorias con distinta densidad, confirmar orden por relevancia real; confirmar que borrar/editar una memoria mantiene `memory_search` sincronizada.

### Tests for User Story 3 ⚠️

> Escribir estos tests PRIMERO, confirmar que fallan antes de implementar

- [X] T017 [P] [US3] Test de ranking real (memoria más antigua pero más relevante gana a una más reciente pero menos densa — diseñado para ser rojo bajo el balde título/contenido+recencia actual) en `adapters/secondary/persistence/memory_search_test.go`
- [X] T018 [P] [US3] Test de fallback: `DROP TABLE memory_search` simula build sin FTS5, la búsqueda sigue devolviendo resultados correctos, en el mismo archivo
- [X] T019 [P] [US3] Test de sincronización: insertar, consolidar por dedup (`UPDATE`) y borrar mantiene `memory_search` 1:1 con `memories`, en el mismo archivo

### Implementation for User Story 3

- [X] T020 [US3] Agregada `CREATE VIRTUAL TABLE IF NOT EXISTS memory_search USING fts5(title, content, memory_id UNINDEXED)` en `migrate()`, `db.go`, replicando `code_search`
- [X] T021 [US3] Dual-write vía `upsertMemorySearch`/`deleteMemorySearch` (helpers nuevos) en `InsertMemory` (rama dedup-UPDATE y rama INSERT), `ImportMemory` y `DeleteMemory`, `memory.go` — no hay `UpdateMemory` separado en este repo, la actualización ocurre dentro de `InsertMemory` vía dedup
- [X] T022 [US3] `SearchMemories` dividida en dispatcher + `searchMemoriesFTS` (MATCH + `ORDER BY rank`) + `searchMemoriesLike` (código original, como fallback), con `scanMemories` compartido — replica `SearchCodeNodes`/`searchCodeNodesFTS`/`searchCodeNodesLike`

**Verificado**: `go build ./...`, `go vet ./...` y `go test ./...` pasan completos.

**Checkpoint**: User Stories 1, 2 y 3 funcionan de forma independiente (quickstart.md §3).

---

## Phase 6: User Story 4 - No perder compatibilidad entre versiones futuras (Priority: P4)

**Goal**: Documentar en el propio código, sin construir infraestructura nueva, la convención de migraciones aditivas y de versionado del bundle de export.

**Independent Test**: quickstart.md sección 4 — `grep` confirma que ambos comentarios existen en los puntos correctos del código.

### Implementation for User Story 4

*(Sin tests — esta historia no cambia comportamiento, solo agrega documentación en el código; no aplica TDD)*

- [X] T023 [US4] Agregado comentario de convención de migraciones solo-aditivas en `migrate()`, `db.go` (aplicado después de T020, sin conflicto)
- [X] T024 [P] [US4] Agregado comentario junto a `domain.ExportVersion` en `portability.go` con la regla de bump + migración explícita (`migrateBundleVxToVy`)

**Verificado**: `grep -n "aditiv" db.go` y `grep -n "ExportVersion" -A3 portability.go` confirman ambos comentarios en el lugar correcto (quickstart.md §4). `go build`/`vet`/`test ./...` completos siguen en verde.

**Checkpoint**: Las 4 historias de usuario funcionan de forma independiente (quickstart.md completo).

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Cierre transversal tras completar las historias deseadas.

- [X] T025 [P] Agregada sección "Mitigación de Riesgos Operativos" en `README.md` (resumen de las 4 historias + link a `specs/009-mitigacion-riesgos/`), más la mención de la segunda capa de redacción y permisos en el bullet de privacidad existente
- [X] T026 Ejecutado `quickstart.md` completo contra el binario real (`go build -o /tmp/mem-verify ./infrastructure`) en un proyecto temporal real: `mem save` con secreto fuera de `<private>` → redactado; `hook session-start`/`session-end` → snapshot de backup generado con el contenido ya redactado; `mem search` con densidad de término distinta → orden por relevancia real confirmado; `memory_search` presente en `sqlite_master`. **Hallazgo no bloqueante**: `mem.db-wal`/`mem.db-shm` (creados de forma perezosa por el driver SQLite) heredan el umask del proceso (`0644`) en vez de `0600` explícito — el directorio contenedor ya es `0700`, así que ningún otro usuario del sistema puede alcanzarlos igualmente (necesitan permiso de tránsito sobre el directorio), por lo que se documenta como limitación conocida y no se agrega lógica extra para perseguir archivos creados de forma asíncrona por el driver (desproporcionado para el mismo resultado de seguridad ya logrado)
- [X] T027 [P] Cobertura verificada por función nueva (no por paquete completo — los paquetes tocados son grandes y preexistentes, subir su agregado a 80% requeriría testear código no relacionado con esta feature): `RedactSecrets` 100%, `CreateSnapshot` 81.2%, `pruneSnapshots` 83.3%, `BackupDir` 75%, `upsertMemorySearch` 83.3%, `deleteMemorySearch` 100%, `SearchMemories`/`searchMemoriesFTS`/`searchMemoriesLike`/`scanMemories` 80–87.5%
- [X] T028 `golangci-lint` no está instalado en este entorno (`which golangci-lint` → not found); sustituido por `gofmt -l` (ningún archivo tocado por esta feature aparece con formato pendiente — los 3 archivos reportados son preexistentes y fuera de alcance) + `go vet ./...` limpio

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias, puede iniciar de inmediato
- **Foundational (Phase 2)**: no bloquea nada (ver nota arriba); solo hay una coordinación de archivo entre T020 (US3) y T023 (US4)
- **Historias de usuario (Phase 3-6)**: cada una puede iniciar apenas termine Phase 1; no dependen funcionalmente entre sí
- **Polish (Phase 7)**: depende de que las historias que se quieran entregar estén completas

### User Story Dependencies

- **US1 (P1)**: sin dependencia de otras historias
- **US2 (P2)**: sin dependencia de otras historias
- **US3 (P3)**: sin dependencia funcional de otras historias; **coordinación de archivo** con US4 sobre `db.go` `migrate()` (aplicar T020 antes que T023)
- **US4 (P4)**: sin dependencia funcional; ver coordinación arriba

### Dentro de cada historia

- Tests (donde aplica, todas menos US4) escritos y en rojo ANTES de implementar
- US1: `CreateSnapshot` (T004) antes de integrarlo en el hook (T006)
- US2: `RedactSecrets` (T012) antes de conectarlo en los call sites (T013, T014); permisos de directorio (T015) antes que chmod de archivo (T016)
- US3: tabla FTS5 (T020) antes del dual-write (T021) antes del dispatcher (T022)
- US4: sin orden interno relevante entre T023 y T024, salvo la coordinación externa con T020

### Oportunidades de paralelismo

- T002, T003 (tests US1) en paralelo
- T009, T010, T011 (tests US2) en paralelo
- T017, T018, T019 (tests US3) en paralelo
- T013, T014 (call sites de `RedactSecrets`) en paralelo entre sí
- T001, T008, T024, T025, T027 en paralelo con cualquier otra tarea activa (archivos distintos, sin dependencias)
- Las 4 historias pueden trabajarse en paralelo por personas distintas, salvo la coordinación puntual T020↔T023 sobre `db.go`

---

## Parallel Example: User Story 1

```bash
# Lanzar juntos los tests de la Historia 1:
Task: "Test unitario de CreateSnapshot en tests/unit/usecases/backup_test.go"
Task: "Test de integración de snapshot+restauración en tests/integration/backup_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 solamente)

1. Completar Phase 1: Setup
2. Completar Phase 3: User Story 1 (backup automático)
3. **DETENER Y VALIDAR**: correr quickstart.md §1 de forma independiente
4. Este es ya el MVP — mitiga el riesgo de mayor impacto (pérdida irreversible de datos) con el menor esfuerzo

### Entrega incremental

1. Setup → Foundation lista (trivial en este caso)
2. Agregar US1 → validar independientemente → MVP entregado
3. Agregar US2 (secretos+permisos) → validar independientemente
4. Agregar US3 (búsqueda FTS5) → validar independientemente
5. Agregar US4 (convención documentada) → validar independientemente (coordinar con US3 en `db.go`)
6. Cada historia suma valor sin romper las anteriores

---

## Notes

- [P] = archivos distintos, sin dependencias pendientes
- [Story] mapea cada tarea a su historia de usuario para trazabilidad
- Verificar que los tests fallan antes de implementar (Principio III, no negociable)
- Hacer commit tras cada tarea o grupo lógico
- Detenerse en cada checkpoint para validar la historia de forma independiente
- Única excepción de paralelismo real a vigilar: T020 (US3) y T023 (US4) comparten el bloque `migrate()` en `db.go` — no ejecutarlas simultáneamente aunque no exista dependencia funcional entre ellas
