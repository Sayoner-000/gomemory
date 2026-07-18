# Implementation Plan: Mitigación de riesgos operativos de gomemory

**Branch**: `009-mitigacion-riesgos` | **Date**: 2026-07-18 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/009-mitigacion-riesgos/spec.md`

## Summary

Cuatro mejoras independientes y de bajo acoplamiento entre sí, cada una mitigando un riesgo operativo distinto identificado en un proyecto de un mes/un autor: (1) backup automático local al cierre de sesión reutilizando el export/import existente; (2) segunda capa de redacción de secretos por patrones conocidos + hardening de permisos de archivo; (3) tabla FTS5 para `memories` con ranking `bm25()` y fallback a `LIKE`, replicando el patrón ya usado en `code_search`; (4) documentación en código de la convención de migraciones aditivas y de versionado del bundle de export. Ninguna requiere una dependencia nueva ni cambia contratos públicos existentes (MCP tools, comandos CLI).

## Technical Context

**Language/Version**: Go >=1.22 (stack congelado del proyecto)

**Primary Dependencies**: `modernc.org/sqlite` (sin CGO, ya en uso); sin dependencias nuevas

**Storage**: SQLite (`mem.db` por proyecto, bajo el data-home global resuelto por `DataHome()` en `adapters/secondary/persistence/globalstore.go:89`)

**Testing**: `testing` stdlib + `testify`, siguiendo la organización existente (`tests/unit`, `tests/integration`, `tests/contract`)

**Target Platform**: CLI/MCP autocontenido — Linux, macOS, Windows

**Project Type**: CLI + servidor MCP (single project, arquitectura hexagonal ya establecida)

**Performance Goals**: sin objetivo cuantitativo nuevo; la mejora de búsqueda (User Story 3) debe mantener el mismo orden de magnitud de latencia que hoy (sub-100ms) para los volúmenes actuales (0–135 filas por proyecto)

**Constraints**: sin dependencias runtime nuevas; el binario debe seguir siendo autocontenido (~16MB); ninguna de las cuatro mejoras puede romper compatibilidad con datos/bundles ya persistidos

**Scale/Scope**: volumen actual 0–135 memorias por proyecto; diseño debe tolerar crecimiento sin degradar UX, sin sobredimensionar para escalas que el proyecto no tiene hoy

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Cumplimiento |
|---|---|
| I. Arquitectura Hexagonal | ✅ Cambios respetan capas: `RedactSecrets` va en `domain/` (puro, sin I/O); el snapshot de backup se orquesta como caso de uso en `application/usecases/`, reutilizando los puertos `MemoryRepository`/`RelationRepository` ya existentes — no se agregan imports de adaptadores en dominio/aplicación. |
| II. SQLite con SQL Directo | ⚠️ **Desviación documentada**: la constitución describe migraciones como archivos SQL numerados (`001_nombre.sql`), pero el código real usa un único `migrate()` en `db.go` con `CREATE TABLE IF NOT EXISTS`/`addColumnIfMissing` inline (sin archivos `.sql` separados). Esta feature sigue el patrón **real** ya establecido (coherente con la regla de campo "la constitución es referencia de cómo escribir código, no un mandato de ritual por tarea" del CLAUDE.md del proyecto) — la tabla FTS5 nueva se agrega en el mismo `migrate()`, de forma idempotente, igual que `code_search`. No se introduce un sistema de migraciones nuevo. |
| III. Testing First | ✅ Se seguirá TDD: tests de `RedactSecrets`, del dispatcher de búsqueda FTS/LIKE, y del snapshot de backup se escriben antes de la implementación, en `tests/unit/` e `tests/integration/` según corresponda. |
| IV. Configuración y Entorno | ✅ El límite de snapshots retenidos (N) se lee de una variable de entorno con default documentado (no hardcodeado), consistente con "TODO valor que cambie entre entornos DEBE venir de variables de entorno". |
| V. Principios Operativos (Simplicidad, sin parches, fire-and-forget) | ✅ El backup en `hookSessionEnd` sigue el patrón fire-and-forget ya usado en ese archivo (nunca aborta el cierre de sesión); la redacción de secretos no agrega configuración externa (lista de patrones fija en código, como ya es `RedactPrivate`). |

**Resultado**: Gate aprobado. La única desviación (migraciones vs. constitución) ya existe en el código base hoy, no la introduce esta feature, y se documenta explícitamente en vez de forzar un ritual (archivos `.sql` numerados) que rompería el patrón ya establecido — ver `Complexity Tracking`.

## Project Structure

### Documentation (this feature)

```text
specs/009-mitigacion-riesgos/
├── plan.md              # Este archivo
├── research.md          # Fase 0
├── data-model.md         # Fase 1
├── quickstart.md         # Fase 1
├── contracts/            # Fase 1 (contratos internos: nuevas funciones de dominio/aplicación)
└── tasks.md              # Fase 2 (/speckit-tasks, no generado por este comando)
```

### Source Code (repository root)

```text
domain/
├── redact.go                    # + RedactSecrets (patrones de secretos conocidos)
└── portability.go                # + comentario de convención junto a ExportVersion

application/usecases/
├── portability.go                # sin cambios funcionales (reutilizado)
└── backup.go                     # NUEVO: CreateSnapshot(memRepo, relRepo, project, dir, keep) — orquesta export + escritura + poda

adapters/secondary/persistence/
├── db.go                         # + tabla FTS5 `memory_search` en migrate(); + comentario convención migraciones; + chmod dir/archivo
├── memory.go                     # SearchMemories → dispatcher + searchMemoriesFTS + searchMemoriesLike; dual-write a memory_search en Insert/Update/Delete; + llamada a RedactSecrets
├── session.go                    # + llamada a RedactSecrets junto a RedactPrivate
└── globalstore.go                # + resolución de <DataHome>/backups/<project-key>/; + hardening de permisos (0700/0600)

adapters/primary/cli/
└── cmd_hook.go                   # hookSessionEnd: + llamada best-effort a CreateSnapshot

tests/unit/
├── domain/redact_test.go         # casos de RedactSecrets (existente redact_test.go se extiende)
└── application/backup_test.go    # CreateSnapshot: poda, contenido del bundle

tests/integration/
├── persistence/search_test.go    # FTS5 vs LIKE fallback, ranking, sincronización del índice
└── persistence/backup_test.go    # snapshot real contra BD real + restauración
```

**Structure Decision**: Se mantiene el proyecto único existente (arquitectura hexagonal ya establecida). No se crean módulos ni paquetes nuevos de alto nivel — todo el trabajo cae dentro de `domain/`, `application/usecases/` (un archivo nuevo, `backup.go`, para no sobrecargar `portability.go` con orquestación de filesystem) y los adaptadores de persistencia/CLI ya existentes.

## Complexity Tracking

> Fill ONLY if Constitution Check has violations that must be justified

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|---------------------------------------|
| Migraciones inline en `db.go` en vez de archivos `.sql` numerados (Principio II) | Es el patrón **ya vigente** en todo el código base (`migrate()` maneja las 6 tablas actuales, incluida la FTS5 de código); introducir archivos `.sql` numerados solo para esta feature crearía dos sistemas de migración conviviendo en el mismo proyecto | Migrar TODO el esquema existente a archivos `.sql` numerados es un cambio no solicitado, de alcance mucho mayor, y arriesga romper el arranque de un proyecto de un mes en producción activa — desproporcionado para agregar una tabla FTS5 más |
