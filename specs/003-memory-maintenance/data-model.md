# Data Model: Mantenimiento de Memoria (Purga, Compactación y Garbage Collector)

No se agregan entidades de **dominio** (`domain/`). Las estructuras nuevas son DTOs de la capa de aplicación (`application/ports/`), siguiendo el mismo patrón ya usado por `ports.SettingsData` (puerto que define su propio tipo de transporte sin pasar por `domain/`).

## Entidades existentes reutilizadas (sin cambios)

### Memory (`domain/memory.go`)

Objeto principal sobre el que actúan purga y GC.

| Campo | Tipo | Relevancia para este feature |
|-------|------|-------------------------------|
| `ID` | `int64` | Identifica filas a borrar |
| `Project` | `string` | Acota el alcance de purga/GC (FR-003, FR-005) |
| `Type` | `domain.MemoryType` | Filtro opcional de purga (FR-003) |
| `CreatedAt` | `string` | Base del filtro de antigüedad (FR-009) |

### Memory Relation (`domain/relation.go`, tabla `memory_relations`)

| Campo | Tipo | Relevancia para este feature |
|-------|------|-------------------------------|
| `MemoryIDA`, `MemoryIDB` | `int64` | Deben limpiarse cuando `Memory` con ese `ID` se borra (FR-004) — ver `research.md` punto 4 |

## Entidades nuevas (capa de aplicación — `application/ports/maintenance_repository.go`)

### StorageStats

Snapshot de tamaño/cantidad usado para decidir y verificar el efecto de purga/compactación (Key Entity "Storage Report" del spec).

```go
type StorageStats struct {
    ProjectMemoryCount int64 // memorias del proyecto actual
    TotalMemoryCount   int64 // memorias de todos los proyectos en este archivo .memory/mem.db
    FileSizeBytes      int64 // tamaño en disco de .memory/mem.db
}
```

- **Validación**: ninguna (solo lectura, `Stats` es de solo consulta).
- **Relaciones**: `ProjectMemoryCount` ≤ `TotalMemoryCount` siempre (un archivo puede alojar más de un proyecto, ver esquema en `db.go`).

### PurgeFilter

Criterio de borrado, usado tanto por `mem purge` como por `mem gc` (Key Entity "Retention Policy" del spec).

```go
type PurgeFilter struct {
    Project       string // proyecto objetivo; ignorado si All=true
    All           bool   // true = todos los proyectos del archivo (requiere --all explícito, FR-003)
    Type          string // "" = cualquier tipo; valor de domain.MemoryType en string
    OlderThanDays int    // 0 = sin filtro de antigüedad; >0 = solo memorias más viejas que N días
}
```

- **Validación** (en el adaptador, antes de ejecutar el `DELETE`):
  - Si `All == false` y `Project == ""` → error (alcance ambiguo; CLI/TUI deben resolver el proyecto actual antes de llegar aquí)
  - `OlderThanDays` negativo → tratado como `0` (sin filtro), nunca como error duro — principio de "fallar rápido" se cumple en el borde (parseo de flags), no aquí
- **Uso por `mem purge`**: `Project` = proyecto actual (default) o el indicado, `All` solo con `--all`, `Type`/`OlderThanDays` opcionales
- **Uso por `mem gc`**: igual que purge, pero `OlderThanDays` con default 90 si no se especifica (ver `research.md` punto 2 y assumption del spec)

## Puerto: MaintenanceRepository

```go
type MaintenanceRepository interface {
    Stats(project string) (StorageStats, error)
    Purge(filter PurgeFilter) (deleted int64, err error)
    Compact() (beforeBytes int64, afterBytes int64, err error)
}
```

- **`Stats`**: `SELECT COUNT(*) ... WHERE project = ?` + `SELECT COUNT(*)` total + `os.Stat` del archivo `.memory/mem.db`.
- **`Purge`**: `DELETE FROM memories WHERE ...` (condiciones según `PurgeFilter`) + limpieza de `memory_relations` huérfanas en la misma transacción; retorna cantidad de memorias eliminadas (FR-011).
- **`Compact`**: mide tamaño antes, ejecuta `VACUUM`, mide tamaño después; nunca toca filas (FR-006).

## Flujo de "desinstalación completa" (FR-012b) — sin entidad de datos propia

No introduce entidades nuevas: opera sobre artefactos de filesystem ya producidos por `mem install` (binario `mem`, `.claude/plugins/gomemory/`, entradas en `.mcp.json`/`.cursor/mcp.json`/etc., bloque delimitado por `integrationMarker`/`integrationVersionMarker` en `AGENTS.md`/`CLAUDE.md`, y `.memory/`). Se documenta como flujo procedural en `quickstart.md` y `contracts/cli-tui-contracts.md`, no como modelo de datos.

## State / Transiciones

No hay máquina de estados: cada operación (`Purge`, `Compact`, desinstalación) es una transacción atómica de una sola pasada — o se aplica por completo, o no se aplica (rollback en error), consistente con Principio II ("`commit()` explícito en escritura, rollback automático en error").
