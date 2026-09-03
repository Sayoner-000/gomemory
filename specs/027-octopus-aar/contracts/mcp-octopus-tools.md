# Contrato: superficie MCP de Octopus

Cuatro tools, registradas **solo** con el módulo encendido. Los nombres viven en `domain/mcp_tools.go` como única fuente de verdad, junto a `MCPOctopusTools` y `MCPToolsFor(octopusEnabled bool)`.

```go
ToolOctopusRoutePlan = "octopus_route_plan"
ToolOctopusRouteTask = "octopus_route_task"
ToolOctopusReport    = "octopus_report"
ToolOctopusStatus    = "octopus_status"
```

Ninguna es destructiva: tres son de solo lectura y cómputo, y `octopus_report` solo inserta telemetría propia. Todas son auto-aprobables.

---

## `octopus_route_task`

Evalúa una unidad de trabajo.

**Entrada**

| Campo | Tipo | Obligatorio |
|---|---|---|
| `objective` | cadena | Sí |
| `task_id` | cadena | No (se genera si falta) |
| `task_class` | cadena | No |
| `dependencies` | lista de cadenas | No |
| `files` | lista de cadenas | No |
| `read_only` | booleano | No |
| `complexity` | `trivial\|low\|medium\|high` | No |
| `risk` | `trivial\|low\|medium\|high` | No |
| `capabilities` | objeto de capacidades | No (ausente ⇒ conservador ⇒ `INLINE`) |
| `remaining_budget` | objeto de presupuesto | No |
| `overrides` | objeto de política | No |

**Salida**: ruta, razón (código y texto), presupuesto de contexto, presupuesto de salida, costo estimado desglosado, marca de estimación y, con ruta `WAIT`, las dependencias que bloquean.

---

## `octopus_route_plan`

Enruta un grafo de tareas. Acepta la lista de unidades con la misma forma que `octopus_route_task`, más `plan_id`, el presupuesto total y las capacidades. **`octopus_route_plan` nunca inicia nada**: toda llamada es una simulación (INV-AAR-018), sin parámetro `dry_run`. Las decisiones se registran como telemetría para que el runtime pueda informar después sus resultados reales.

**Salida**: el plan completo con las decisiones ordenadas por identificador de tarea, los grupos paralelos, el estado del presupuesto resultante y el número de agentes delegados, incluido el desglose legible que exige FR-044.

**Error** (y solo estos): grafo con ciclos, dependencia a una tarea inexistente, identificador duplicado o vacío. Falta de presupuesto o de capacidades **no** es error.

---

## `octopus_report`

Recibe el resultado real de una ejecución. Entrada: `task_id`, `route`, `status`, `context_tokens`, `output_tokens`, `duration_ms`, `quality`.

Fire-and-forget: un reporte para una tarea sin decisión previa se ignora sin error. Nunca devuelve un fallo que pueda romper el flujo del runtime.

---

## `octopus_status`

Sin entrada. Devuelve el estado del módulo, los topes efectivos, el presupuesto de la sesión y los agregados de telemetría: conteos por ruta, consumo estimado y real, reducción de contexto, éxitos, fallos, reintentos, repliegues y ancho de paralelismo.

---

## Invariantes de la superficie

- Los cuatro nombres están en `domain/mcp_tools.go` y en `MCPOctopusTools`, y el test de contrato los compara contra el `tools/list` real con el módulo encendido.
- Ninguna salida contiene contenido de contexto, transcripciones ni razonamiento privado; las razones provienen del catálogo cerrado.
- Ninguna operación crea, lanza ni cancela procesos.
