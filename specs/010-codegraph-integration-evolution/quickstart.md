# Quickstart: Evolución de la Integración con Grafo de Código Externo

**Feature**: `010-codegraph-integration-evolution`

Guía de validación manual end-to-end para las tres historias, una vez
implementadas. Requiere `codebase-memory-mcp` (o cualquier proveedor
compatible con el CLI descrito en `research.md`) instalado y en el PATH, y
un proyecto de prueba ya indexado por él (`index_repository` corrido a
mano — gomemory nunca lo dispara, ver FR-008).

## Prerrequisitos

```bash
go build -o mem ./infrastructure/
codebase-memory-mcp cli index_repository '{"root_path":"'"$(pwd)"'"}'   # indexado manual, una vez
./mem settings --show   # confirma que code_graph_providers/adr_sync_enabled aparecen
```

## Historia 1 — Anotación de impacto al guardar

```bash
./mem settings --code-impact-annotation=true
# elegir un archivo/símbolo que el proveedor reporte como hotspot
./mem save -t "cambio riesgoso" -y bugfix -f "adapters/secondary/persistence/memory.go" "toqué InsertMemory"
./mem search "cambio riesgoso"
```

**Resultado esperado**: el contenido de la memoria guardada incluye la nota
de impacto ("archivo de alto impacto: N llamadores directos") si
`memory.go` (o el símbolo correspondiente) está en los hotspots del
snapshot vigente. Con `--code-impact-annotation=false`, o guardando un
archivo que no es hotspot, la memoria se guarda igual sin anotación —
confirmar que en ningún caso el comando tarda perceptiblemente más.

## Historia 2 — Sincronización de ADR (bidireccional)

```bash
./mem settings --adr-sync=true
./mem save -t "Usar SQLite WAL" -y architecture "decisión de concurrencia de escritura"
./mem adr-sync status   # debe listar la memoria recién guardada con → (gomemory→proveedor)

# En el otro sentido: crear un ADR directo en el proveedor externo
codebase-memory-mcp cli manage_adr '{"action":"create","title":"Adoptar tree-sitter","project":"..."}'
./mem code-refresh      # mismo comando ya usado para refrescar el snapshot
./mem search "tree-sitter"
```

**Resultado esperado**: el ADR creado directamente en el proveedor aparece
como memoria `architecture` en gomemory tras el refresco (←), y no se
duplica en refrescos posteriores. Apagar `--adr-sync=false` y repetir un
`mem save` de tipo `architecture` no debe generar ninguna llamada al
proveedor (verificable por ausencia de nuevas filas en `adr_sync_records`
vía `mem adr-sync status`).

## Historia 3 — Múltiples proveedores con fallback automático

```bash
./mem settings --code-graph-providers=proveedor-inexistente,codebase-memory-mcp
./mem context | head -5   # debe enriquecerse igual, usando el segundo proveedor (el primero no existe en el PATH)
```

**Resultado esperado**: `get_context` se enriquece con el resumen de
arquitectura sin que la persona haya tenido que quitar
`proveedor-inexistente` de la lista — el loop ya existente en
`build_context.go` lo salta en silencio (snapshot `Available=false`) y
muestra la sección del proveedor que sí respondió.
Simular la caída del proveedor activo (renombrar temporalmente el binario)
y confirmar que, tras el siguiente refresco (TTL 60s), el sistema sigue
funcionando (sin enriquecimiento si no queda ningún candidato disponible, o
con el siguiente candidato si lo hay) sin ninguna intervención manual.

## Regresión: comportamiento sin ninguna de las tres capacidades activas

```bash
./mem settings --adr-sync=false --code-impact-annotation=false --code-graph-providers=
./mem save -t "memoria normal" -y learning "contenido cualquiera" -f "README.md"
```

**Resultado esperado**: idéntico al comportamiento actual (pre-feature) en
todo — mismo tiempo de guardado, mismo formato de `get_context`, sin
llamadas al proveedor externo salvo el `get_architecture` agregado que ya
existe hoy.
