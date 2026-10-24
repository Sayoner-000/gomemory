# Contrato: apertura y preparación de las bases de gomemory

Es un contrato interno del paquete `adapters/secondary/persistence`. No hay API pública nueva: la CLI, el MCP y los hooks siguen llamando a las mismas funciones.

## `persistence.Open(root) (*sql.DB, error)` y `OpenByKey(key)`

**Precondición**: `root` es la raíz de un proyecto. Para `OpenByKey`, la base puede no existir, y entonces se devuelve `nil, nil`, como hoy.

**Postcondiciones**:

1. Cada conexión del pool devuelto tiene la configuración de R9. `PRAGMA synchronous` devuelve 1 (NORMAL), `PRAGMA temp_store` 2 (MEMORY), y así con el resto, consultando desde cualquier conexión (FR-011, SC-007).
2. La base contiene todas las estructuras de la versión en curso, `idx_sessions_project_created` incluido (FR-004, FR-015).
3. Si `Open` devuelve error, la base no ganó ninguna estructura de esa preparación (FR-001).
4. Llamar a `Open` N veces sobre la misma raíz deja el catálogo (`sqlite_master`) idéntico al de la primera (FR-003, SC-009).
5. Dos `Open` concurrentes sobre una base nueva devuelven una base completa sin error, en un tiempo acotado por `busy_timeout` y `migrateWithRetry` (FR-007).
6. Tras `Open`, `PRAGMA user_version` vale `max(valor previo, schemaVersion)`: nunca baja (FR-018).
7. Si la base ya tenía `user_version >= schemaVersion`, `Open` no ejecuta `BEGIN` ni DDL y no espera a otros escritores: termina aunque otra conexión mantenga una escritura abierta. La cifra de SC-010 (< 50 ms) se mide en Windows y no se comprueba en la suite (§10).

**Invariantes**:

- El esquema solo se amplía: no hay `DROP`, `RENAME` ni cambios de tipo (spec 009, HU4).
- `foreign_keys` queda desactivado (FR-014).
- Todo cambio del catálogo que produce `migrateSchema` va con un aumento de `schemaVersion`, y `TestSchemaGolden` lo hace cumplir (FR-019).

## `persistence.OpenInstinctGlobal(dataHome)`

**Postcondiciones**: la configuración de R9 con `busy_timeout=100`. Conserva `secure_delete=ON` y su esquema propio (fuera del alcance de la migración transaccional, R6).

## `infrastructure.Container.Close()`

**Postcondición**: si el canal del contenedor no es `"hook"`, antes de cerrar `mem.db` ejecuta `PRAGMA analysis_limit=400; PRAGMA optimize;` y no propaga su error (FR-013). Con el canal `"hook"` no lo ejecuta: los hooks sí llegan a `Close` a través del `defer closeContainer` de `main` y tienen un plazo de 2 s (análisis G1). El resultado de `Close` sigue siendo el de cerrar las bases.

## Guarda de aperturas (FR-012, R14)

Ningún `.go` de producción de `adapters/secondary/persistence/` llama a `sql.Open("sqlite", …)` con un DSN que no venga de `sqliteDSN(…)`.

## Prueba de planes de consulta (FR-016)

**Entrada**: una base nueva preparada por `Open` y la lista vigilada de tasks.md T019, con los mismos textos SQL que usa el código y su `archivo:línea`.

**Salida**: falla si algún plan contiene `SCAN <tabla>` sobre una tabla del esquema. Los recorridos de tablas virtuales FTS5 se permiten, porque no usan índices B-tree.
