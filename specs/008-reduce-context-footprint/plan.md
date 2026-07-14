# Implementation Plan: Reducir la huella de contexto de gomemory en la sesión

**Branch**: `008-reduce-context-footprint` | **Date**: 2026-07-13 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/008-reduce-context-footprint/spec.md`

## Summary

Reducir el porcentaje de la ventana de contexto del agente que consume gomemory
(hoy ~22% del `/usage`, causado por `get_context` ≈82KB que persiste toda la
sesión). El enfoque tiene cuatro palancas, todas operando sobre el **texto
emitido** (agnósticas al agente) y sin borrar datos persistidos:

1. **Presupuesto de contexto** en el caso de uso `Builder.Build()`: truncar el
   contenido largo por entrada y adjuntar puntero `get_memory <id>`, respetando
   un techo de tamaño configurable; conflictos y protocolo nunca se recortan.
2. **Revelación progresiva** en los adaptadores MCP `search_memories` (hoy
   vuelca `m.Content` íntegro) y `list_memories`: extracto acotado + id; el
   detalle íntegro queda en `get_memory`.
3. **Cierre de turno/sesión**: `end_session` guía un resumen estructurado; el
   `hookTurnEnd` emite un recordatorio **neutral** de compactación cuando la
   huella emitida por gomemory en la sesión supera un umbral.
4. **Dedup/upsert en la fuente** en el choke point `InsertMemory`: evitar filas
   casi idénticas (identidad proyecto+tipo+título o `topic_key`), consolidando.

## Technical Context

**Language/Version**: Go >= 1.22

**Primary Dependencies**: `modernc.org/sqlite` (sin CGO), `modelcontextprotocol/go-sdk`, `charmbracelet/bubbletea` (pantalla de settings TUI)

**Storage**: SQLite (store global del usuario) + `settings.json` por proyecto para tunables de usuario

**Testing**: `testing` stdlib + `testify`; cobertura ≥ 80%; mocks por puerto; tests existentes intocables

**Target Platform**: binario CLI autocontenido, portable Linux/macOS/Windows

**Project Type**: proyecto único con arquitectura hexagonal (dominio / aplicación / adaptadores / infraestructura)

**Performance Goals**: `get_context` sigue en hot path instantáneo (truncado O(n) sobre ≤100 memorias ya en memoria); el dedup añade a lo sumo una consulta indexada por `save_memory`

**Constraints**: agnóstico al agente (opera sobre texto emitido, sin comandos de cliente); **opt-in** (presupuesto ≤ 0 = sin límite, comportamiento actual); no romper tests existentes; no borrar ni mutar datos persistidos al acotar; el servidor NO puede medir la ventana del cliente (solo su propia huella emitida)

**Scale/Scope**: cientos de memorias por proyecto; `get_context` lista hasta 100; objetivo de reducir la salida de arranque de ~82KB a ≤ ~20KB por defecto

## Constitution Check

*GATE: Debe pasar antes de Phase 0. Re-evaluado tras Phase 1.*

| Principio | Cumplimiento en este plan |
|-----------|---------------------------|
| **I. Hexagonal** | La lógica de acotado (truncar/medir) vive en el caso de uso `Builder` (aplicación, sin I/O) y en helpers de dominio puro; el render MCP queda en el adaptador `cmd_mcp.go`. El dedup vive en la capa de persistencia bajo el puerto de repositorio. Sin imports de adaptadores desde dominio/aplicación. ✅ |
| **II. SQLite SQL directo** | Nueva columna `topic_key` con el mecanismo existente `addColumnIfMissing` en `migrate(db)` (db.go) + índice `IF NOT EXISTS` en el schema embebido — el proyecto NO usa archivos de migración numerados pese al texto del Principio II; se sigue el patrón real ya usado para `origin_prompt`/`last_prompt`. `INSERT`/`UPDATE`/`SELECT` de dedup con parámetros bind; sin ORM. ✅ |
| **III. Testing First** | Cada palanca arranca con test rojo: presupuesto (truncado + techo + secciones intactas), extracto de `search_memories`, dedup/upsert idempotente, umbral del nudge de compactación. Mocks de puertos existentes reutilizados. Tests actuales no se tocan. ✅ |
| **IV. Configuración** | Los tunables de usuario (presupuesto, umbral) se añaden a `Settings`/`SettingsData` (JSON), consistente con `CodeGraphDisabled`/`CodeGraphCommand` y la pantalla de settings de la TUI. Ver justificación en Complexity Tracking. ✅ (con nota) |
| **V.1 Simplicidad / V.2 Sin parches** | Se reutilizan choke points existentes (`InsertMemory`, `Builder.Build`, `hookTurnEnd`, `computeSaveNudge`) en vez de añadir infraestructura nueva. ✅ |
| **V.7 Idempotencia** | El upsert de dedup es idempotente por diseño (guardar N veces ⇒ 1 fila consolidada). ✅ |
| **V.6 Fire-and-forget** | El nudge de compactación y el dedup best-effort no bloquean ni hacen fallar el guardado (patrón ya usado por `formSynapse`). ✅ |
| **Docs en español** | Todos los artefactos de `specs/` en español latino. ✅ |

**Resultado del gate**: PASA. Una desviación menor documentada (config por `settings.json` en vez de env) — ver Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/008-reduce-context-footprint/
├── plan.md              # Este archivo
├── research.md          # Phase 0: decisiones (presupuesto, proxy de huella, estrategia dedup)
├── data-model.md        # Phase 1: campos/entidades (topic_key, tunables de Settings)
├── contracts/           # Phase 1: contratos de comportamiento de las tools MCP afectadas
│   └── mcp-tools.md
├── quickstart.md        # Phase 1: guía de validación end-to-end
└── tasks.md             # Phase 2 (/speckit-tasks — NO lo crea /speckit-plan)
```

### Source Code (repository root)

Estructura hexagonal existente; esta feature toca estos puntos concretos:

```text
domain/
├── memory.go                     # + campo TopicKey; helper puro de truncado/extracto
application/
├── ports/
│   └── settings_repository.go    # + Budget, CompactThreshold en SettingsData
├── usecases/
│   └── build_context.go          # Builder.Build(): presupuesto + truncado por entrada
adapters/
├── primary/cli/
│   ├── cmd_mcp.go                # search_memories/list_memories: extracto + id
│   └── cmd_hook.go               # hookTurnEnd: recordatorio neutral de compactación
├── secondary/persistence/
│   ├── memory.go                 # InsertMemory: dedup/upsert en el choke point
│   ├── settings.go               # + Budget, CompactThreshold (JSON) + defaults
│   └── db.go                     # topic_key vía addColumnIfMissing + índice en el schema embebido
infrastructure/
└── container.go                  # wiring: pasar Budget/Threshold al Builder desde Settings
tests/
├── unit/                         # truncado, extracto, dedup, umbral (con mocks)
└── integration/                  # BD real: dedup idempotente, get_context bajo presupuesto
```

**Structure Decision**: Proyecto único hexagonal (ya vigente). No se crean paquetes nuevos; se extienden choke points existentes para respetar V.1 (impacto mínimo).

## Complexity Tracking

> Solo desviaciones que requieren justificación.

| Desviación | Por qué se necesita | Alternativa más simple, y por qué se rechaza |
|------------|---------------------|----------------------------------------------|
| Tunables (presupuesto, umbral) en `settings.json` en vez de variables de entorno (Principio IV) | Son **preferencias de usuario que cambian en caliente y por proyecto**, no diferencias de entorno de despliegue. El proyecto ya estableció este patrón para `CodeGraphDisabled`/`CodeGraphCommand`, con pantalla de settings en la TUI. Meterlos en env rompería la coherencia y no serían ajustables desde la TUI. | Variables de entorno: rechazada porque `USE_MOCK_ADAPTERS`/env están reservados a diferencias de entorno, no a preferencias por proyecto; obligaría a reiniciar el proceso y no encajaría en la UI existente. |
| El nudge de compactación mide la **huella emitida por gomemory**, no la ventana real del cliente | Un servidor MCP no tiene acceso al conteo de tokens de la ventana del agente; medir su propia salida acumulada es el único proxy honesto y además es exactamente la métrica que el usuario quiere bajar (el 22%). | Estimar la ventana del cliente: imposible/mentiroso desde el servidor. Pedir al agente que reporte su tamaño: rompería el agnosticismo al agente. |
