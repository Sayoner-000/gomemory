# Data Model: gomemory como brazo extensor de contexto histórico para /speckit

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

No hay entidades de dominio nuevas ni tablas SQLite nuevas. Esta feature
extiende una entidad de configuración ya existente y reutiliza estructuras
ya modeladas (feature 008/010). Se documentan aquí para trazabilidad.

## Settings (extensión de entidad existente)

`Settings` (`adapters/secondary/persistence/settings.go`) /
`SettingsData` (`application/ports/settings_repository.go`) — preferencias
por proyecto persistidas en `.memory/settings.json`.

**Campo nuevo**:

| Campo | Tipo | JSON key | Default | Descripción |
|-------|------|----------|---------|-------------|
| `SpeckitContextDisabled` | `bool` | `speckit_context_disabled,omitempty` | `false` (activado) | Apaga el brazo extensor hacia spec-kit para este proyecto. Mismo patrón `*Disabled` opt-out que `CodeGraphDisabled`/`SynapseDisabled`: ausente en un `settings.json` viejo ⇒ activado, sin migración necesaria. |

**Reglas de validación**: ninguna más allá de ser un booleano — mismo
criterio que los campos `*Disabled` existentes (sin rango, sin
dependencias cruzadas obligatorias).

**Transiciones de estado**: dos estados (`false`=activado / `true`=
desactivado), alternables únicamente desde:
1. TUI → pantalla de configuración → fila nueva (toggle inmediato,
   persistido en el acto, igual que `CodeGraphDisabled`).
2. CLI → `mem settings --speckit-context=true|false`.

No hay estado intermedio ni expiración — se lee fresco en cada invocación
del script del hook (sin cachear en proceso, coherente con "Prohibiciones"
de la constitución: *"Cachear en proceso valores que cambian en caliente"*
está prohibido).

## Resumen de historial de especificación (concepto, no entidad persistida)

No es una entidad nueva de datos: es el **mismo** documento Markdown que ya
produce `ContextBuilder.Build()` (memorias agrupadas por tipo + sección
aparte del grafo de código externo cuando aplica), consumido tal cual por
el script del hook. No se persiste una copia separada para esta feature.

## Interruptor del brazo extensor (mapeo a Key Entity de spec.md)

La "Key Entity" *Interruptor del brazo extensor* de `spec.md` se realiza
como el campo `SpeckitContextDisabled` descrito arriba — no requiere una
tabla ni un objeto de dominio propio, es una preferencia booleana más en la
misma estructura `Settings` ya existente.
