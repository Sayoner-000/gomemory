# Quickstart — Señal de grafo de código en Retrieval de ContextPack

**Feature**: [spec.md](./spec.md) · Contratos: [contracts/cli.md](./contracts/cli.md),
[contracts/go-api.md](./contracts/go-api.md), [contracts/mcp-tools.md](./contracts/mcp-tools.md)

Guía de validación manual, no una suite de tests. Cada paso demuestra un criterio de
aceptación de una historia de usuario del spec. Requiere `codebase-memory-mcp` (u otro
`CodeGraphProvider`) configurado e indexado sobre el mismo repo — ver
`docs/MANUAL.md` sección de grafo de código externo si no está configurado todavía.

## Prerrequisitos

- Binario compilado con esta feature: `go build -o mem ./infrastructure`.
- Un proveedor de grafo de código externo ya indexado sobre este proyecto (`mem index`
  dispara el reindexado externo, feature 016) — confirmar con:

```bash
mem context | grep -A 3 "Grafo de código externo"
```

Si no aparece nada, no hay snapshot disponible todavía — los pasos de este quickstart que
dependen de él van a comportarse como "sin proveedor" (no van a fallar, simplemente no
mostrarán la señal nueva).

- Al menos una memoria guardada cuyo `--file` apunte a un símbolo que hoy sea un hotspot
  vigente (ver la lista de "Hotspots" en la salida de `mem context` de arriba, o correr
  `search_graph` con `min_degree` alto). Ejemplo, sustituyendo `<archivo-hotspot>` por uno
  real de esa lista:

```bash
mem save -t "Nota de prueba sobre símbolo caliente" -f "<archivo-hotspot>" -y learning \
  "Memoria de prueba para el quickstart de la feature 018."
```

## Historia 1 — Boost de prioridad para memorias ligadas a un hotspot

```bash
mem pack build --task "Nota de prueba sobre símbolo caliente" --max-tokens 300 --json | \
  jq '.Items[] | select(.ID | startswith("memory:")) | {Source, Priority}'
```

**Resultado esperado**: la memoria guardada arriba aparece con `Priority: 1` (Relevant, ya
que `PriorityRelevant = 1` en el `iota` de `domain.Priority`), no `Priority: 2` (Optional)
— pese a que su `MemoryType` (`learning`) normalmente clasificaría como Optional/Relevant
según relevancia de búsqueda, no por estar ligada a un hotspot.

```bash
mem pack build --task "Nota de prueba sobre símbolo caliente" --max-tokens 300 --no-code-graph --json | \
  jq '.Items[] | select(.ID | startswith("memory:")) | {Source, Priority}'
```

**Resultado esperado**: la misma memoria, con la prioridad que le tocaría solo por tipo/
relevancia — sin el boost, para confirmar que `--no-code-graph` realmente lo desactiva.

## Historia 2 — Ítem de orientación arquitectónica dentro del presupuesto

```bash
mem pack build --task "cualquier tarea" --max-tokens 4000 --json | \
  jq '.Items[] | select(.ID == "codegraph:architecture")'
```

**Resultado esperado**: con presupuesto amplio, aparece un ítem cuyo `Content` coincide
con el mismo resumen que ya muestra `mem context` bajo "Grafo de código externo"
(totales, lenguajes, clusters, hotspots).

```bash
mem pack build --task "cualquier tarea" --max-tokens 50 --json | \
  jq '.Stats.ItemsDiscarded'
```

**Resultado esperado**: con presupuesto muy ajustado, el ítem de arquitectura queda
descartado (contabilizado en `ItemsDiscarded`), sin que la construcción falle.

## Historia 3 — Desactivar la señal por invocación

```bash
diff \
  <(mem pack build --task "cualquier tarea" --max-tokens 4000 --json | jq -S 'del(.RawTokenCount, .TokenCount, .CompressionRate, .Stats)') \
  <(mem pack build --task "cualquier tarea" --max-tokens 4000 --json --no-code-graph 2>/dev/null; \
    : ) # comparación manual: confirmar visualmente que --no-code-graph no trae ítems codegraph:*
```

**Resultado esperado** (verificación simple, sin diff automatizado por el ruido de
tokens/orden): `mem pack build ... --no-code-graph` nunca incluye un ítem con `ID`
`codegraph:architecture`, y ninguna memoria sube de prioridad por el grafo.

## No-regresión (FR-009)

```bash
mem context            # comportamiento existente, get_context() — no se toca en esta feature
mem pack build --task "cualquier tarea" --max-tokens 4000 --no-code-graph --json
```

**Resultado esperado**: ambos se comportan exactamente igual que antes de esta feature.
Sin proveedor de grafo de código configurado en absoluto, `mem pack build` (con o sin
`--no-code-graph`) produce salida idéntica a la que producía antes de esta feature — cero
error, cero mención al grafo de código.
