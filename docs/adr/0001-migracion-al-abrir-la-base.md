# ADR 0001: la base de memoria se migra al abrirla

- **Estado**: aceptado
- **Fecha**: 2026-10-23
- **Responsable**: josegomezJ3810
- **Excepción de la constitución**: §6 Migraciones (registrada en `.specify/memory/constitution.md`, §25.1)
- **Referencias**: spec 009 (HU4, esquema solo aditivo); spec 041 (`specs/041-migracion-transaccional/`)

## Contexto

La constitución (§6) pide un runner de migraciones versionado (golang-migrate en Go) y solo permite migrar al arrancar en desarrollo y pruebas. Producción debe migrar como un job separado, con backup y plan de reversión.

gomemory no tiene ese entorno. Es una CLI y un servidor MCP local, y cada persona tiene una base SQLite por proyecto en su almacén global, a menudo decenas. Esas bases las crean y abren binarios de versiones distintas, y también hooks globales que actúan en cualquier repositorio. No hay despliegue ni un momento central en el que ejecutar un job de migración.

## Decisión

Cada base se migra al abrirla (`persistence.Open`):

1. **Solo aditiva**: `CREATE … IF NOT EXISTS` y `ALTER TABLE … ADD COLUMN` tolerado. Nunca `DROP`, `RENAME` ni cambios de tipo (spec 009, HU4).
2. **Atómica**: toda la preparación en una única transacción `BEGIN IMMEDIATE … COMMIT` (spec 041, FR-001).
3. **Versionada**: `PRAGMA user_version` guarda la versión de esquema. Una base al día no ejecuta DDL ni pide el bloqueo de escritura (spec 041, FR-018).
4. **Vigilada**: un golden del catálogo hace que cualquier cambio de esquema obligue a subir `schemaVersion` (spec 041, FR-019).

## Alternativas descartadas

- **golang-migrate con migraciones numeradas**: exige un paso de despliegue que no existe y numeración inmutable. Una base abierta por un binario anterior quedaría «sucia» para el runner, y el retroceso de versión es habitual en una CLI local.
- **Migrar en `mem install` o `mem update`**: no cubre las bases que crean los hooks globales en proyectos donde nunca se ejecutó `install`, ni las que abre un binario de otra versión.
- **Job separado con backup**: no hay infraestructura. El almacén es local y el backup ya existe por sesión (`hook session-end`).

## Consecuencias

- **Riesgo técnico**: un cambio de esquema que no sea aditivo rompería bases existentes. Lo mitiga la regla aditiva, revisada en cada feature que toca `migrateSchema`.
- **Riesgo técnico**: olvidar subir `schemaVersion` dejaría bases sin migrar. Lo impide `TestSchemaGolden_VersionBumpsWithCatalog`.
- **Riesgo de seguridad**: ninguno nuevo. La migración no lee datos de la persona ni ejecuta SQL externo.
- **Controles compensatorios**:
  - transacción única (todo o nada);
  - `user_version` (idempotencia sin bloqueo);
  - golden del catálogo (FR-008 y FR-019);
  - pruebas desde una base vacía y desde esquemas anteriores (`TestMigrate_*`, `TestMigracion028EsAditivaSobreEsquemaPrevio`, `TestMigrateCompression_BaseVaciaYAnterior`).
- **Revisión**: el 2027-01-23, con la renovación trimestral del catálogo (§0), o antes si gomemory pasa a tener un despliegue central.
