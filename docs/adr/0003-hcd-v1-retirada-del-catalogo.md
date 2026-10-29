# ADR 0003: el schema v5 retira el catálogo HCD v1

- **Estado**: aceptado
- **Fecha**: 2026-10-29
- **Responsable**: josegomezJ3810
- **Excepción**: ADR 0001, regla 1 («solo aditiva, nunca `DROP`»)
- **Referencias**: feature 045 v2 (`specs/045-hcd-protocol/`, FR-024); ADR 0002

## Contexto

La primera versión de HCD (045 v1) añadió 16 tablas `hcd_*` con el schema v4. Esa versión nunca se publicó (`v2.40.3` usa v3), pero el binario compilado desde la rama se instaló localmente y migró a v4 85 bases globales. Entre ellas está la del propio gomemory, con 796 memorias y 0 unidades HCD; las demás son proyectos de prueba, con 61 unidades de ensayo.

HCD v2 reduce el catálogo a 3 tablas (`hcd_units`, `hcd_evidence`, `hcd_approvals`). Tres de esos nombres coinciden con tablas v1 de columnas incompatibles, así que `CREATE TABLE IF NOT EXISTS` no basta: la base quedaría con las tablas viejas y las inserciones nuevas fallarían.

## Decisión

El schema v5 elimina las 16 tablas `hcd_*` v1 **solo cuando la base viene de `user_version = 4`**, y crea el catálogo v2 en la misma transacción de la migración (ADR 0001, regla 2). Ninguna otra tabla se toca.

- Una base nueva o v3 nunca ejecuta el `DROP`: solo una v4 tuvo el catálogo v1.
- La lista de tablas es constante (`hcdV1Tables`) y la cubren `TestMigrate_V4WithHCDV1CatalogDropsItAndKeepsMemories` y `TestDropHCDV1OnlyFromVersion4`.

## Verificación

Una copia de la base real de gomemory (v4, 808 memorias, 525 sesiones, 16 tablas `hcd_*`) migró a v5 con las 808 memorias intactas (mismo total de contenido), las 525 sesiones, 3 tablas `hcd_*` y `PRAGMA integrity_check` = `ok` (2026-10-29).

## Consecuencias

- Se pierden las unidades HCD v1 de ensayo; ninguna versión publicada las creó.
- La regla «nunca `DROP`» sigue vigente para todo lo demás. Esta excepción no se extiende a ninguna otra tabla ni a versiones futuras.

## Alternativas descartadas

- **Recrear las bases**: perdería memorias reales.
- **Nombres nuevos sin `DROP`**: respeta la regla al pie de la letra, pero deja 16 tablas huérfanas para siempre en las bases de desarrollo y nombres peores en el producto publicado.
