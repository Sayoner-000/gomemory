# Contrato: reporte de uso

**Feature**: 020-token-usage-benchmark · **Fecha**: 2026-08-20 · **Versión del contrato**: 1

Este documento es la referencia para cualquier consumidor del reporte de uso de gomemory. Está
escrito para poder integrarse **sin leer el código**.

## Principio rector

**La salida legible por máquina es la forma que manda.** La salida legible por personas es una
traducción de ella, nunca al revés (FR-012). Si las dos discrepan, la legible por máquina tiene
razón y la otra es un defecto.

---

## 1. Invocación

```
mem usage                      # sesión activa (por defecto)
mem usage --session <id>       # una sesión concreta
mem usage --all                # acumulado de todas las sesiones del proyecto
mem usage --json               # salida legible por máquina (combinable con las anteriores)
```

Sin sesión activa y sin `--session` ni `--all`, se reporta la última sesión con registros; si no
hay ninguna, se emite un reporte en ceros con `scope: "empty"`. **Nunca un error.**

---

## 2. Esquema de la salida legible por máquina

```jsonc
{
  "contract_version": 1,
  "project": "go_memory",
  "scope": "session",          // "session" | "all" | "empty"
  "session_id": "a1b2c3d4…",   // ausente cuando scope != "session"
  "counting_method": "approximate",
  "counting_note": "Aproximación neutral (~4 caracteres por token). Comparable contra sí misma.",

  "calls": 4,
  "baseline_tokens": 8120,
  "emitted_tokens": 5310,
  "saved_tokens": 2810,
  "reduction_ratio": 0.3460,

  "schema_tokens": 1842,       // costo de los descriptores publicados; 0 si no se midió
  "schema_operations": 19,

  "window_tokens": 0,          // ventana de referencia; 0 = sin ventana
  "window_ratio": null,        // null cuando window_tokens == 0 — ESTIMADO cuando no es null

  "by_operation": [
    { "key": "build_context",   "calls": 1, "baseline_tokens": 6000, "emitted_tokens": 3500 },
    { "key": "search_memories", "calls": 2, "baseline_tokens": 1800, "emitted_tokens": 1500 },
    { "key": "save_memory",     "calls": 1, "baseline_tokens":  320, "emitted_tokens":  310 }
  ],

  "by_channel": [
    { "key": "mcp", "calls": 3, "baseline_tokens": 7800, "emitted_tokens": 5000 },
    { "key": "cli", "calls": 1, "baseline_tokens":  320, "emitted_tokens":  310 }
  ]
}
```

### 2.1 Garantías que un consumidor puede dar por ciertas

| # | Garantía |
|---|---|
| G1 | `baseline_tokens - saved_tokens == emitted_tokens`, exacto (SC-002) |
| G2 | `reduction_ratio == 0` cuando `baseline_tokens == 0`. Nunca `NaN`, nunca división por cero |
| G3 | La suma de `baseline_tokens` de `by_operation` iguala el total. Lo mismo para `by_channel` |
| G4 | La suma de `calls` de `by_operation` y la de `by_channel` iguala `calls` |
| G5 | `window_ratio` es `null` exactamente cuando `window_tokens == 0` |
| G6 | `by_operation` y `by_channel` vienen ordenados de mayor a menor `baseline_tokens` |
| G7 | Los campos existen siempre; un valor ausente se expresa como `0` o `null`, nunca omitiendo la clave (salvo `session_id`, que sí se omite fuera del ámbito de sesión) |

### 2.2 Medido frente a estimado

**Un solo campo del contrato es estimado**: `window_ratio`. Todo lo demás es medido.

`window_tokens` es un valor que **provee el usuario**, no una lectura del entorno. gomemory no puede
leer la ventana de contexto de ningún cliente y no la presume: el valor por defecto es `0`, y con
`0` el porcentaje sencillamente no existe (`null`). Ningún valor por defecto corresponde a la
ventana de un agente concreto.

`counting_method` vale `"approximate"`: el conteo es una aproximación neutral de unos cuatro
caracteres por token, no el tokenizador de ningún proveedor. **Las cifras son comparables contra sí
mismas** —diferencias entre un antes y un después, porcentajes— y no contra la facturación de
nadie.

### 2.3 Estabilidad y evolución

- `contract_version` sube solo ante un cambio incompatible.
- Añadir claves nuevas **no** sube la versión: un consumidor debe ignorar las que no conozca.
- `by_operation[].key` y `by_channel[].key` son **conjuntos abiertos**. Un canal desconocido aparece
  con su etiqueta; un consumidor no debe validar contra una lista cerrada ni descartar lo que no
  reconozca.

---

## 3. Salida legible por personas

Traducción de lo anterior. Su cabecera **debe** declarar el método de conteo (FR-013).

```
Uso de contexto — proyecto go_memory · sesión a1b2c3d4
Conteo aproximado neutral (~4 caracteres por token). Las cifras son comparables
contra sí mismas, no contra la facturación de ningún proveedor.

Llamadas:              4
Línea base:        8 120 tokens
Emitido:           5 310 tokens
Ahorro:            2 810 tokens  (34,60 %)

Descriptores publicados: 1 842 tokens en 19 operaciones

Por operación
  build_context      1 llamada    6 000 →  3 500   (-41,67 %)
  search_memories    2 llamadas   1 800 →  1 500   (-16,67 %)
  save_memory        1 llamada      320 →    310   ( -3,13 %)

Por canal
  mcp                3 llamadas   7 800 →  5 000
  cli                1 llamada      320 →    310
```

Con `usage_window_tokens` en `0` —su valor por defecto— **no aparece ninguna línea de porcentaje
sobre ventana**: todo lo impreso es medido (SC-003). Al configurar una ventana mayor que cero se
añade una única línea, rotulada:

```
Huella evitada:    1,41 % de una ventana de 200 000 tokens  (estimado)
```

---

## 4. Cómo se añade un canal emisor nuevo

Este procedimiento es el criterio de agnosticismo verificable de la feature (FR-017, SC-005).

1. Construir el grabador de uso con la etiqueta del canal nuevo, en el composition root.
2. Eso es todo.

No se toca `domain/usage.go`, ni `ports.UsageRepository`, ni `ports.UsageRecorder`, ni
`usecases.BuildUsageReport`, ni el formateador. Si alguno de esos cinco hubiera que modificarlo, el
cableado está mal y hay que rehacerlo: significa que la capacidad volvió a definirse en el formato
de un canal.
