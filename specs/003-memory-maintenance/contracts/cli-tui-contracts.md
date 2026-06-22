# Contratos: CLI y TUI — Mantenimiento de Memoria

## Comandos CLI nuevos

### `mem purge`

Vacía memorias del almacén. Alcance por defecto: proyecto actual (FR-003).

```text
mem purge [flags]

Flags:
  --project <nombre>      Proyecto objetivo (default: proyecto actual detectado por FindRoot)
  --all                   Purgar TODOS los proyectos del archivo .memory/mem.db (requiere confirmación reforzada)
  --type <tipo>           Filtrar por tipo: learning|decision|architecture|bugfix|pattern|discovery
  --older-than-days <N>   Solo memorias más viejas que N días (0 = sin filtro de antigüedad)
  --yes                   Omitir el prompt interactivo (uso no interactivo/scripts) — la confirmación
                           sigue existiendo en la forma de pasar esta bandera explícitamente (FR-002)

Comportamiento:
  - Sin --yes: imprime un resumen ("se van a borrar N memorias del proyecto X") y pide escribir
    "si" para confirmar. Cualquier otra respuesta cancela sin borrar nada (Acceptance Scenario US1.2).
  - Con --all sin --project: alcance = todos los proyectos del archivo.
  - Sin --all y sin --project: alcance = proyecto actual.
  - Al completar: imprime cantidad eliminada y limpieza de relaciones huérfanas (FR-004, FR-011).

Exit codes:
  0   Purga ejecutada (incluyendo el caso "0 memorias eliminadas")
  1   Cancelado por el usuario, o error (alcance inválido, BD no encontrada)
```

### `mem compact`

Recupera espacio en disco. Nunca borra memorias (FR-006).

```text
mem compact

Comportamiento:
  - Mide tamaño de .memory/mem.db antes de VACUUM.
  - Ejecuta VACUUM.
  - Mide tamaño después.
  - Imprime "antes: X MB → después: Y MB (liberado: Z MB)" usando dustin/go-humanize (FR-007).
  - Si no había espacio que reclamar, informa explícitamente sin tratarlo como error
    (Acceptance Scenario US2.2).

Exit codes:
  0   Compactación ejecutada (con o sin espacio liberado)
  1   Error (BD no encontrada, BD bloqueada tras agotar el busy timeout)
```

### `mem gc`

Garbage collection a demanda — purga memorias más viejas que un umbral (FR-009). Internamente
reutiliza `MaintenanceRepository.Purge` con `PurgeFilter.OlderThanDays` forzado.

```text
mem gc [flags]

Flags:
  --project <nombre>      Proyecto objetivo (default: proyecto actual)
  --all                   Aplicar a todos los proyectos
  --older-than-days <N>   Umbral de retención (default: 90)
  --yes                   Omitir el prompt interactivo

Comportamiento: idéntico a `mem purge`, salvo que --older-than-days tiene default 90 (no 0) y
el texto de confirmación dice "garbage collection" en vez de "purga total", para que el usuario
entienda que es una limpieza por antigüedad, no un vaciado completo.
Nunca se dispara automáticamente (FR-009) — solo existe como este comando explícito o la acción
equivalente en la TUI.

Exit codes: iguales a `mem purge`.
```

### `mem uninstall [dir]`

Reverso completo de `mem install [dir]` (FR-012b). Acción separada de `mem purge` — además de
los datos, remueve la instalación de la herramienta.

```text
mem uninstall [dir] [flags]

Flags:
  --yes   Omitir el prompt interactivo

Comportamiento (simétrico a CmdInstall en cmd_install.go):
  1. Resuelve `target` (default ".") igual que mem install.
  2. Pide confirmación (a menos que --yes): "esto eliminará el binario, hooks, configuración
     MCP, entradas en AGENTS.md/CLAUDE.md y TODA la memoria guardada en <target>. ¿Continuar?"
  3. Si se confirma:
     a. Remueve el bloque delimitado por integrationMarker/integrationVersionMarker de
        AGENTS.md/CLAUDE.md/CLAUDE.txt/.cursorrules/.windsurfrules (si el archivo queda vacío
        tras quitar el bloque y fue creado enteramente por mem install, se elimina el archivo;
        si tiene contenido adicional del usuario, se conserva el resto).
     b. Remueve la entrada "gomemory" de .mcp.json, .cursor/mcp.json, .windsurf/mcp_config.json,
        .cline/mcp_settings.json (inverso de setupOpenCode/setupClaude/setupCursor/etc.).
     c. Remueve .claude/plugins/gomemory/ y las entradas de hooks que apuntan ahí en
        .claude/settings.json (SessionStart/PreCompact/UserPromptSubmit/SessionEnd).
     d. Remueve el directorio .memory/ completo (datos incluidos).
     e. Remueve el binario `mem` en <target> — usando os.SameFile para detectar si es el binario
        en ejecución; en Linux/macOS se borra igual (unlink válido sobre ejecutable en uso); en
        Windows, si falla por bloqueo, se reporta para borrado manual posterior (research.md #5).
  4. Imprime un resumen de qué se eliminó y qué no se encontró (Acceptance Scenario US4.3).

Exit codes:
  0   Desinstalación completada (incluso si algunos componentes no existían)
  1   Cancelado por el usuario, o error irrecuperable (ej. target no es un directorio)
```

## Acciones nuevas en la TUI (`adapters/primary/tui/tui.go`)

| Tecla | Pantalla origen | Acción |
|-------|------------------|--------|
| `m` | `screenList` | Abre `screenMaintenance`: muestra `StorageStats` (conteo proyecto/total, tamaño de archivo) y 3 opciones: Purgar, Compactar, GC |
| `enter` sobre "Compactar" | `screenMaintenance` | Ejecuta `Compact()` directo (no destructivo, sin confirmación adicional) y muestra resultado vía `statusMsg` (mismo patrón ya usado por el toggle `a` de autoApprove) |
| `enter` sobre "Purgar" / "GC" | `screenMaintenance` | Abre una sub-pantalla de confirmación que exige escribir el nombre del proyecto (o "TODOS" si se eligió alcance global) antes de habilitar la ejecución — análogo a un formulario de `screenSave` pero de solo confirmación |
| `esc` | sub-pantallas | Cancela sin ejecutar nada, vuelve a `screenList` |

El header de `listView()` se extiende para mostrar el tamaño del archivo junto al conteo ya
existente (`"%s · %d memorias"` → agrega `· %s en disco` con `humanize.Bytes`), cumpliendo
FR-008/FR-010/SC-003 sin requerir una pantalla adicional solo para consultar tamaño.

`mem uninstall` **no** tiene equivalente en TUI — simétrico a que `mem install` tampoco lo tiene;
ambas son operaciones que se ejecutan típicamente fuera de una sesión interactiva de trabajo.
