# Contrato: masa de memorias (`mem mass` y `pack_build`)

## `mem mass` (CLI nueva, US3)

```
mem mass [--task "<texto>"] [--top N]
```

| Flag | Default | Regla |
|------|---------|-------|
| `--task` | vacío | Con texto, las semillas son los resultados de la búsqueda con peso `1/(rango+1)`. Vacío: las semillas son la sesión activa y las anclas a hotspot, o uniforme si no hay ninguna. |
| `--top` | 15 | Debe ser > 0; si no, falla con un mensaje de uso y código ≠ 0. |

Salida (stdout):

```
Masa de memorias — semillas: <semillas>

  1. [<id>] (<tipo>) <título> — masa 0.1234
  2. …

masa = centralidad en el grafo de memorias sembrado en <semillas>; no mide importancia ni corrección
```

- Nunca lista checkpoints.
- Formato de masa: 4 decimales. El orden es por masa↓ y después por id↑.
- Con `--task`, `<semillas>` es `tarea "<texto>" (N)`, donde N es el número de resultados no checkpoint. La búsqueda trae como máximo 20 resultados, el mismo tope por omisión que `pack_build` (`defaultCandidateLimit`); `--top` solo limita la salida.
- Con `--task` sin resultados de búsqueda: `Sin memorias que coincidan con "<texto>": masa no calculada` y código 0.
- Proyecto sin relaciones válidas: se lista igual (la masa se concentra en las semillas) con la misma línea final.
- Dos ejecuciones sobre el mismo almacén producen salidas idénticas byte a byte.

## `pack_build` (MCP) y `mem pack build` (cambia, US4)

Entrada: sin parámetros nuevos. Ambos adaptadores pasan las relaciones del
proyecto a `ContextRequest.Relations`.

Efecto en el `ContextPack`:

- Hasta 5 ítems adicionales con id `memory:<N>`, `Priority = optional`, que no estaban entre los candidatos de la búsqueda y tienen masa > 0 con semillas en esos candidatos.
- Van después de todos los demás ítems, así que ante el presupuesto son los primeros en descartarse.
- `Stats.ItemsRetrieved` los incluye.

Sin relaciones (`Relations == nil`, como en la TUI), el `ContextPack` es
idéntico al actual.
