# Quickstart: validar la base optimizada

Guía para comprobar la feature de principio a fin. El contrato está en [contracts/apertura-de-base.md](contracts/apertura-de-base.md) y los valores en [research.md](research.md) R9-R11.

## 0. Antes de cambiar el código: golden del esquema

Con el código de `fix/windows-ci-cierre`, sin la feature aplicada, se genera el golden del catálogo de una base nueva (R7):

```sh
go test ./adapters/secondary/persistence/ -run '^TestSchemaGolden$' -count=1 -args -update-schema-golden
```

Queda en `adapters/secondary/persistence/testdata/schema.golden`, con la línea `user_version=0`, las filas de `sqlite_master` y el `pragma_table_info` de cada tabla. Es la referencia de FR-008: al terminar la feature, el diff solo puede ser `user_version=2` y la línea de `idx_sessions_project_created`. Ese diff se guarda en `evidence/schema-diff.txt`.

## 1. Pruebas locales

```sh
go test ./adapters/secondary/persistence/ -run 'Schema|Migrate|Plan|Pragma|Concurrent|Atomic' -count=1 -v
go test ./... -count=1
go vet ./... && GOOS=windows go vet ./...
```

Resultado esperado:

- El esquema es idéntico al golden salvo el índice nuevo (FR-008).
- Tres aperturas seguidas dejan el mismo catálogo (FR-003, SC-009).
- `TestSchemaGolden_VersionBumpsWithCatalog` falla si el esquema cambia sin subir `schemaVersion` (FR-019).
- Con otra conexión manteniendo una escritura abierta, abrir una base al día tarda menos de 50 ms (FR-018, SC-010).
- Un subproceso que confirma y sale sin cerrar no pierde la escritura, y `integrity_check` da `ok` (US4-AC4).
- Ningún `sql.Open("sqlite", …)` de producción esquiva `sqliteDSN` (FR-012).
- Un fallo provocado a mitad de la preparación (una vista `memories` creada antes) deja la base sin las tablas de instincts ni de socratica (FR-001).
- Cada conexión del pool reporta los pragmas de R9 (SC-007).
- Ninguna consulta frecuente hace `SCAN <tabla>` (FR-016, SC-008).
- `TestOpenDBFile_AperturaConcurrenteDeUnaBaseNueva` y las pruebas de migración aditiva siguen pasando (FR-007, SC-005).

## 2. Comprobación manual de una base real

```sh
mem init   # en un proyecto de prueba
# mem.db vive en <DataHome>/projects/<ProjectKey>/ (GOMEMORY_DATA_HOME o ~/.local/share/gomemory)
DB=$(ls -t "${GOMEMORY_DATA_HOME:-$HOME/.local/share/gomemory}"/projects/*/mem.db | head -1)
sqlite3 "$DB" 'PRAGMA journal_mode; EXPLAIN QUERY PLAN SELECT id FROM sessions WHERE project = "p" AND ended_at IS NULL ORDER BY created_at DESC, rowid DESC LIMIT 1;'
```

Resultado esperado: `wal` y `SEARCH sessions USING INDEX idx_sessions_project_created`. Los pragmas por conexión (`synchronous`, `temp_store`…) no se ven desde el cliente `sqlite3`: los comprueba la prueba de SC-007.

## 3. Medición en Windows (FR-009)

1. Crear `diag/windows-timing-041` a partir de `041-migracion-transaccional` y reaplicar `3e81733` (instrumentación y workflow `diag-windows-timing`). Añadir la marca `close`. Todo commit y push espera la aprobación de la persona.
2. Ajustar el `on.push.branches` del workflow de diagnóstico a esa rama y hacer push.
3. Agregar el registro con el mismo script de la línea base y anotar en `evidence/windows-timing.md`:

| Métrica | Línea base | Objetivo |
|---|---|---|
| `db.open` en una base nueva, máx | 1.760 ms | < 400 ms (SC-002) |
| `fold` en `hook expand stop`, máx | 1.809 ms | < 500 ms (SC-003) |
| `db.open` en una base preparada, p50 | 6 ms | ≤ 10 ms (SC-004) |
| `flush` en `hook expand pre-tool`, p50 | 42 ms | referencia, sin empeorar |
| `close` (servidor MCP y CLI), máx | — | ≤ 100 ms (FR-013) |

4. Ejecutar 20 veces seguidas en Windows CI `TestHookExpandInstinctsTwoProjects` y `TestHookExpand_SuggestionOncePerEpisodeAndRecorded`: deben salir 20 de 20 (SC-001).

## 4. Integración

La rama `diag/windows-timing-041` no se integra. La feature se integra con el CI completo (`test.yml`) en verde en los tres jobs (SC-006).
