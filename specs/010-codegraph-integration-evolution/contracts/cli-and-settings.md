# Contrato: CLI y Settings

**Feature**: `010-codegraph-integration-evolution`

gomemory es un CLI + servidor MCP, no un servicio HTTP: su "contrato
externo" es la superficie de comandos/flags (`mem settings`, `mem mcp`) y las
tools/recursos MCP ya documentados en `docs/architecture.md`. Esta feature
no agrega tools MCP nuevas — las tres capacidades son transparentes dentro
de flujos ya existentes (`save_memory`, `get_context`, refresco de
snapshot). Lo único que se agrega a la superficie pública es configuración y
un comando de solo-lectura para inspeccionar el estado de sincronización.

## `mem settings` — flags nuevos

```
mem settings --code-graph-providers=cmd1,cmd2,...   # reemplaza/extiende --code-graph-command con una lista ordenada
mem settings --adr-sync=true|false                  # default: false
mem settings --code-impact-annotation=true|false    # default: true
mem settings --show                                 # (ya existe) debe listar también los 3 campos nuevos
```

**Compatibilidad**: `--code-graph-command` (ya existente) sigue funcionando
sin cambios — internamente se trata como `code_graph_providers` de un solo
elemento si `code_graph_providers` está vacío. No es necesario migrar
`settings.json` existentes a mano.

## `mem adr-sync status` (nuevo, solo lectura)

Cubre FR-010 (estado consultable de la sincronización, no necesariamente
visible por defecto en `get_context`).

```
$ mem adr-sync status
ADR sincronizados: 12 ok · 1 pendiente · 0 fallidos
  [42] "Usar Fiber para routing"         → ADR-0007 (gomemory→proveedor, ok, hace 2h)
  [-]  "Migrar a SQLite WAL"             ← ADR-0003 (proveedor→gomemory, ok, hace 1d)
  [51] "Redis para cache de sesión"      → ADR-0011 (gomemory→proveedor, pendiente, hace 10m)
```

No expuesto vía MCP (mismo criterio que `mem purge`/`mem gc`: operación de
mantenimiento/diagnóstico, no una tool que un agente deba invocar en medio
de una tarea).

## Efecto observable en tools MCP ya existentes (sin cambio de firma)

| Tool | Cambio de comportamiento (no de firma) |
|---|---|
| `save_memory` | Si `code_impact_annotation_enabled=true`, `filepath` no vacío, y el snapshot cacheado marca ese archivo como hotspot → el `content` persistido incluye la anotación de impacto (Historia 1). Si `adr_sync_enabled=true` y `type` es `architecture`/`decision` → se dispara el export best-effort a ADR después de confirmar el guardado (Historia 2) |
| `get_context` | Sin cambios de formato — las memorias importadas desde ADR externos (Historia 2) aparecen igual que cualquier memoria `architecture` nativa, ya agrupadas por tipo |

## Settings JSON — forma final de `.memory/settings.json`

```jsonc
{
  "auto_approve": false,
  "code_graph_disabled": false,
  "code_graph_command": "",            // legado, se sigue leyendo
  "code_graph_providers": [],          // nuevo, prioridad explícita
  "adr_sync_enabled": false,           // nuevo
  "code_impact_annotation_disabled": false, // nuevo — ausente/false = activada
  "budget": 24000,
  "compact_threshold": 48000,
  "dedup_window_days": 7
}
```

**Nota de implementación**: el campo interno se llama
`code_impact_annotation_disabled` (no `_enabled`) — mismo motivo que
`code_graph_disabled` ya existente: un `bool` en JSON no distingue "el campo
no está" de "está en `false`", y esta capacidad debe quedar **activada por
defecto**. El flag de `mem settings` sigue expresándose en positivo
(`--code-impact-annotation=true|false`), igual que `--code-graph` ya invierte
internamente hoy.
