# Implementation Plan: Benchmark de tokens por sesión (`mem usage`) y tres optimizaciones validadas con esa medición

**Branch**: `main` (sin rama dedicada) | **Date**: 2026-08-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/020-token-usage-benchmark/spec.md`

## Summary

gomemory calcula el ahorro de contexto en cada llamada y lo descarta. Esta feature lo persiste como
una entidad nueva e independiente —el registro de uso— y lo expone en `mem usage` y en una pantalla
propia de la interfaz interactiva, para que el ahorro deje de ser una intuición y pase a ser un dato
que se puede citar y comparar contra sí mismo.

El enfoque técnico se apoya en tres decisiones que salieron de la investigación de la fase 0:

1. **El registro nace en el caso de uso, y el canal es un dato de construcción del grabador.**
   `ports.UsageRecorder` recibe `(operación, línea base, emitido)`; la etiqueta de canal viaja
   dentro del adaptador que lo construye. Añadir un canal emisor nuevo es construir el grabador con
   otra etiqueta: cero líneas en dominio, puerto, caso de uso y formateador (FR-017, SC-005).
2. **El costo de los descriptores publicados se mide preguntándole al servidor, no replicando su
   registro.** El SDK de MCP registra cada operación con genéricos y una función anónima por
   llamada, así que no existe una lista homogénea que extraer. En su lugar se conecta un cliente
   por transporte en memoria contra el servidor real y se serializa lo que devuelve `ListTools`.
   Eso mide exactamente lo que gomemory publica y sigue siendo correcto cuando se añada una
   operación número veinte. Ver [research.md §3](./research.md).
3. **La fase B cambia de blanco, y la razón es un dato, no una opinión.** La premisa del brief —que
   las memorias con la misma clave de tópico se acumulan— es falsa en la base real: cero grupos con
   más de una fila. Lo que sí se acumula son los registros automáticos de actividad. Ver
   [research.md §5](./research.md) y la sección Complexity Tracking de este documento.

## Technical Context

**Language/Version**: Go 1.25.0 (toolchain go1.25.11); el módulo declara `go 1.25.0`

**Primary Dependencies**: `modelcontextprotocol/go-sdk` v1.6.1 · `modernc.org/sqlite` (sin CGO) ·
`charmbracelet/bubbletea` v0.26.1 → v1.3.10 (US5) · `charmbracelet/bubbles` v0.18.0 → v1.0.0 ·
`charmbracelet/lipgloss` v1.0.0 → v1.1.0 · `testify`

**Storage**: SQLite con SQL directo, sin ORM. Tabla nueva `usage_records`, aditiva. Migración en
`migrate()` de `adapters/secondary/persistence/db.go`, con `CREATE TABLE IF NOT EXISTS` y
timestamps con la constante `Now` (`datetime('now', '-5 hours')`, UTC-5).

**Testing**: `testing` stdlib + `testify`. TDD estricto (Principio III): el test se escribe primero
y falla. Dobles ya existentes que se reutilizan: `openTestDB`, `captureStdout`, `fakeProjectRepo`,
`spyMemoryRepo`, `newConfigTestModel`, `tuiSettingsStub`.

**Target Platform**: binario autocontenido para Linux, macOS y Windows; sin dependencias en tiempo
de ejecución.

**Project Type**: herramienta de línea de comandos con interfaz interactiva y servidor MCP sobre
stdio, en arquitectura hexagonal.

**Performance Goals**: registrar una emisión debe ser despreciable frente a la emisión misma —una
sola inserción con parámetros bind, sin lecturas previas. El reporte agrega por sesión con un
índice `(project, session_id)`, no recorre la tabla completa.

**Constraints**: registrar NUNCA puede impedir emitir (FR-006, `fire-and-forget` del Principio V.6).
El grabador es opcional en todas las dependencias: con el campo en nulo, todo emisor sigue
funcionando igual. La migración debe ser idempotente (Principio V.7).

**Scale/Scope**: una emisión registrada por llamada que emite contexto; en la base real del propio
proyecto, 427 memorias y sesiones de decenas de llamadas. El histórico de uso sigue el ciclo de
vida de las sesiones.

## Constitution Check

*GATE: debe pasar antes de la fase 0. Reevaluado tras la fase 1.*

| Principio | Cómo lo cumple este diseño | Estado |
|---|---|---|
| **I. Arquitectura hexagonal** | `domain/usage.go` es puro (sin I/O ni imports de infraestructura). `ports.UsageRepository` y `ports.UsageRecorder` se declaran en la capa de aplicación. `persistence/usage.go` y el grabador con etiqueta de canal son adaptadores. El wiring ocurre solo en `infrastructure/container.go`. | ✅ Pasa (antes y después del diseño) |
| **II. SQLite con SQL directo** | `CREATE TABLE IF NOT EXISTS usage_records` + `CREATE INDEX IF NOT EXISTS`, dentro del `migrate()` existente. Parámetros bind en las cuatro operaciones. Timestamps con la constante `Now` (UTC-5). Sin ORM. | ✅ Pasa |
| **III. Testing first** | Cada grupo de tareas arranca por su test en rojo. Repositorio nuevo ⇒ test de repositorio obligatorio. Cobertura ≥ 80 % en los paquetes nuevos. Ningún test existente se modifica. | ✅ Pasa |
| **IV. Configuración y entorno** | `UsageWindowTokens` y el modo de emisión entran en la struct única `ports.SettingsData`, como campos declarativos con valor por defecto, sin lógica. | ✅ Pasa |
| **V.6 Fire-and-forget** | FR-006: un fallo al registrar no bloquea ni altera la emisión en curso. Mismo trato que la huella en caracteres existente. | ✅ Pasa |
| **V.7 Idempotencia** | La migración se puede repetir sin efecto (SC-011). La consolidación previsualiza por defecto y solo aplica cuando se lo piden. | ✅ Pasa |
| **Dependencias: «más de 2 versiones menores detrás debe actualizarse»** | `bubbletea` v0.26.1 está muy por debajo de la línea v1 vigente. US5 lo sube a v1.3.10. | ✅ Pasa (US5 existe precisamente por esto) |
| **Stack: «TUI charmbracelet/bubbletea — última»** | Se sube a la última de la línea **v1**, no a v2. Desviación declarada abajo. | ⚠ Justificada en Complexity Tracking |
| **Documentación en español latino** | Todos los artefactos de `specs/020-…` y los comentarios nuevos van en español, con nombres técnicos en inglés. | ✅ Pasa |

Ninguna puerta falla sin justificación. Las dos desviaciones se registran abajo.

## Project Structure

### Documentation (this feature)

```text
specs/020-token-usage-benchmark/
├── spec.md                      # Especificación (ya existe)
├── plan.md                      # Este archivo
├── research.md                  # Fase 0 — decisiones y hallazgos medidos
├── data-model.md                # Fase 1 — entidades, esquema, invariantes
├── contracts/
│   ├── usage-report.md          # Contrato de la salida legible por máquina
│   └── usage-recorder.md        # Contrato del puerto de registro y del canal
├── quickstart.md                # Fase 1 — guía de validación ejecutable
├── checklists/requirements.md   # Ya existe
└── tasks.md                     # Fase 2 — lo genera /speckit-tasks
```

### Source Code (repository root)

```text
domain/
└── usage.go                     # NUEVO · UsageRecord, UsageReport, agregados. Puro.

application/
├── ports/
│   ├── usage_repository.go      # NUEVO · Record / BySession / Sessions / Totals
│   └── usage_recorder.go        # NUEVO · seam neutral Record(op, raw, final)
└── usecases/
    ├── build_usage_report.go    # NUEVO · agrega registros → domain.UsageReport
    ├── consolidate_memories.go  # NUEVO · fase B, agrupa y funde
    └── build_context.go         # MODIFICADO · contadores raw/final, modo índice

adapters/
├── primary/
│   ├── cli/
│   │   ├── cmd_usage.go         # NUEVO · CmdUsage + FormatUsageReport
│   │   ├── cmd_mcp.go           # MODIFICADO · etiqueta de canal + coste de esquemas
│   │   ├── cmd_mcp_schemas.go   # NUEVO · medición vía transporte en memoria
│   │   ├── dispatcher.go        # MODIFICADO · case "usage"
│   │   └── cli.go               # MODIFICADO · entrada de ayuda (NO tocar Usage())
│   └── tui/
│       └── tui.go               # MODIFICADO · screenUsage + tecla `u`
└── secondary/
    ├── persistence/
    │   ├── usage.go             # NUEVO · repositorio de registros de uso
    │   ├── db.go                # MODIFICADO · tabla e índice, solo aditivo
    │   └── settings.go          # MODIFICADO · UsageWindowTokens, modo de emisión
    └── usage/
        └── recorder.go          # NUEVO · grabador con etiqueta de canal

infrastructure/
└── container.go                 # MODIFICADO · wiring, único lugar
```

**Structure Decision**: se conserva la estructura hexagonal ya establecida del repositorio
(`domain/`, `application/{ports,usecases}`, `adapters/{primary,secondary}`, `infrastructure/`). No
se introduce ninguna carpeta de nivel superior. El único directorio nuevo es
`adapters/secondary/usage/`, que aloja el grabador —un adaptador secundario más, hermano de
`adapters/secondary/tokens/`—. Los tests viven junto a su paquete, con el sufijo `_test.go`, como
en todo el repositorio.

## Puntos de anclaje verificados en el código

Todo lo de esta tabla se comprobó leyendo el código en el momento de planificar, no se asumió.

| Anclaje | Ruta | Qué se hará ahí |
|---|---|---|
| `Builder` (struct) | `application/usecases/build_context.go:87` | Añadir contadores de caracteres y el grabador opcional |
| `Builder.acota()` | `application/usecases/build_context.go:122` | Sumar el contenido íntegro a la línea base antes de truncar |
| `Builder.fits()` | `application/usecases/build_context.go:135` | Sumar a la línea base también lo que no cabe |
| Tope de checkpoints | `application/usecases/build_context.go:302` (`i >= 5`) | Punto donde se descartan 75 de 80 registros de actividad |
| `ports.ContextBuilder` | `application/ports/context_builder.go` | Solo expone `Build()`/`WriteFile()`: el registro va dentro de `Build()`, no en un método nuevo del puerto |
| `migrate()` | `adapters/secondary/persistence/db.go:105` | Tabla e índice nuevos, entre el esquema y los `addColumnIfMissing` |
| `const Now` | `adapters/secondary/persistence/db.go:15` | Timestamp UTC-5 del registro |
| `findDuplicate()` | `adapters/secondary/persistence/memory.go:317` | Excluye checkpoints del dedup por identidad — origen del hallazgo de la fase B |
| Middleware MCP | `adapters/primary/cli/cmd_mcp.go:46-56` | Aporta la etiqueta de canal y el emitido de respaldo |
| `FormatContextStats` | `adapters/primary/cli/cmd_pack.go:219` | Se reutiliza tal cual para la sección de snapshot |
| `enum screen` | `adapters/primary/tui/tui.go:24-38` | `screenUsage` se añade al final, tras `screenEditSetting` |
| Teclas de `updateList` | `adapters/primary/tui/tui.go:544-603` | Ocupadas: `q / j k enter s a m c o`. **`u` está libre** |
| `Usage()` (ayuda) | `adapters/primary/cli/cli.go` | **No renombrar**: el comando nuevo es `CmdUsage` |
| Dispatcher | `adapters/primary/cli/dispatcher.go` | `usage` no existe hoy; también falta `doctor` en la ayuda |

## Fases de entrega

| Fase | Historias | Entregable | Criterio de corte |
|---|---|---|---|
| **A** | US1, US2, US5 | `mem usage`, pantalla de uso, dependencia al día | Autónoma. Entrega los dos artefactos pedidos y produce los datos que justifican B y C. **Corte natural si el trabajo debe pausarse.** |
| **B** | US3 | Consolidación de grupos redundantes | Requiere A para poder demostrar su Δ (FR-030) |
| **C** | US4 | Emisión en modo índice | Requiere A por la misma razón (FR-035) |

US5 se agrupa con la fase A a propósito: toca la misma superficie que US2, y verificar la interfaz
una sola vez cuesta menos que verificarla dos.

## Complexity Tracking

> Dos desviaciones respecto de lo escrito en la constitución o en el brief de entrada. Ambas se
> declaran aquí en vez de resolverse en silencio.

| Desviación | Por qué es necesaria | Alternativa más simple, y por qué se rechaza |
|---|---|---|
| **La constitución fija «bubbletea — última», y este plan sube a v1.3.10, no a v2.0.9.** | v2 cambia la ruta del módulo (`/v2`) y rompe la interfaz de programación. FR-039 exige que ninguna pantalla existente cambie de comportamiento y que ningún test existente se modifique; una migración a v2 no puede garantizar eso dentro de esta feature. | Subir a v2 ahora. Se rechaza porque convertiría una feature de medición en una migración de interfaz, con riesgo sobre las nueve pantallas ya existentes y sobre pruebas que el Principio III declara intocables. Queda como feature aparte. |
| **La fase B amplía el criterio de agrupación más allá de la clave de tópico.** | La spec exige en FR-030 y SC-008 un Δ **medido** y verificable. Medido contra la base real: cero grupos de clave de tópico con más de una fila, y el 55 % de los registros automáticos de actividad recientes con contenido byte a byte idéntico. Con solo la clave de tópico, el Δ es demostrablemente cero y la fase falla su propio criterio de aceptación. | Implementar solo la clave de tópico, como decía el brief. Se rechaza porque produciría una función correcta que no ahorra nada y una historia que no puede cumplir su criterio de éxito. FR-026 se conserva íntegro; lo que se añade es un segundo criterio de agrupación bajo la misma operación. Ver [research.md §5](./research.md). |

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Colisión de nombres: `Usage()` ya existe en `cli.go` como texto de ayuda | El comando es `CmdUsage` en `cmd_usage.go`. Verificado: hoy no hay ningún `case "usage"` en el dispatcher |
| Sesgo del porcentaje si solo se registran las operaciones que optimizan | FR-005: las que no optimizan se registran con línea base igual a lo emitido, vía el middleware de canal |
| Contar dos veces una misma emisión (caso de uso + middleware) | El middleware solo registra las operaciones que el caso de uso no reportó; se resuelve con una marca por llamada. Contrato en [contracts/usage-recorder.md](./contracts/usage-recorder.md) |
| El modo índice degrada la utilidad del contexto | FR-032 blinda el protocolo, FR-034 exige valor por defecto conservador y reversibilidad, y la medición de la fase A permite cuantificar la pérdida, no solo el ahorro |
| Que el conteo se lea como tokens de un proveedor | FR-013: la cabecera lo declara. Se reutiliza el único contador del proyecto, sin introducir otro |

## Progreso

- [x] **Fase 0** — investigación: [research.md](./research.md)
- [x] **Fase 1** — diseño: [data-model.md](./data-model.md), [contracts/](./contracts/),
      [quickstart.md](./quickstart.md)
- [x] Constitution Check reevaluado tras el diseño: sin violaciones nuevas
- [ ] **Fase 2** — desglose de tareas: lo genera `/speckit-tasks`
