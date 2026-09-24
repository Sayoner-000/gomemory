# Research: Compatibilidad del plugin de OpenCode con v2 sin romper v1.18

**Feature**: [spec.md](spec.md) · **Fecha**: 2026-09-24

Todo lo que sigue se verificó **contra binarios reales de OpenCode** instalados
de forma aislada (`HOME` y prefijo npm propios en el scratchpad de la sesión),
no solo contra la documentación. Versiones ejercitadas:

| Versión | Paquete npm | Cómo se ejecutó |
|---|---|---|
| 2.0.16 | `@opencode/cli` (el paquete `opencode-ai` sigue en 1.x; v2 se publica con otro nombre) | `opencode reload`, `plugin list`, `debug config`, `mcp list` |
| 1.18.32 | instalación local de la persona usuaria | `opencode debug config` con `HOME` aislado |
| 1.18.20 | `opencode-ai@1.18.20` | ídem |
| 1.17.0 | `opencode-ai@1.17.0` | ídem |

Tipos de referencia: `@opencode/plugin@2.0.16`, `@opencode/schema@2.0.16`,
`@opencode/ai@2.0.16`, `@opencode/client@2.0.16` y `https://opencode.ai/v2/openapi.json`.

---

## R-1 · Reproducción del fallo

- **Hallazgo**: con el `gomemory.ts` actual en `~/.config/opencode/plugins/`,
  OpenCode 2.0.16 registra
  `failed to load plugin … PluginModule.LoadError: Plugin must export a default definition with an id and an effect or setup function. (cause: SchemaError(Missing key at ["default"]))`.
  Es el mismo error que reportan los usuarios.
- **Causa raíz**: el plugin solo tiene un export con nombre
  (`export const GomemoryPlugin: Plugin = async (...) => {...}`). v2 valida
  `default` con un esquema y exige `{ id, setup }` (o `effect`).

## R-2 · ¿v2 descubre `~/.config/opencode/plugins/`?

- **Decisión**: se mantiene la ubicación actual de instalación.
- **Evidencia**: el log de v2 muestra `loading plugin id=…/home2/.config/opencode/plugins/gomemory.ts`
  y activa un watcher sobre el archivo. La documentación solo menciona
  `.opencode/plugins/`, pero el binario también carga el directorio de
  usuario. Esto resuelve el supuesto abierto en el spec.
- **Alternativas descartadas**: registrar el plugin en la clave `plugins` de
  `opencode.json`. Es innecesario, porque el autodescubrimiento funciona, y
  además duplicaría la carga.

## R-3 · Forma del export: dual en un solo archivo

- **Decisión**: un único `gomemory.ts` que exporta:
  1. `export default { id: "gomemory", setup(ctx) {…}, server(input) {…} }`: `setup` es la rama v2 y `server` la rama v1.
  2. Se **conserva** `export const GomemoryPlugin` (fábrica v1) como red de seguridad para cargadores 1.x antiguos y para `gomemory.test.mjs`.
- **Evidencia** (spike con marcas en un log):

| Versión | Llama a | ¿Carga doble por el export con nombre? |
|---|---|---|
| 2.0.16 | `setup(ctx)` → `ctx.location.directory` = directorio del proyecto | No (lo ignora) |
| 1.18.32 | `server(input)`; `input` trae `client, project, worktree, directory, serverUrl, $` | No |
| 1.18.20 | `server(input)` | No |
| 1.17.0 | `server(input)` | No |

- **Consecuencia sobre el spec**: el piso "1.18.29" de la guía oficial es
  conservador. **No hace falta una variante solo v1**: FR-003 se satisface con
  el archivo dual. Por debajo de 1.17.0 no se verificó; el export con nombre
  se conserva para cubrirlo si esos cargadores recorren los exports.
- **Sin dependencias en runtime**: el objeto literal `{ id, setup }` basta.
  `Plugin.define` es la identidad y no hay que importar `@opencode/plugin`.
  Solo se permiten `import type` (se borran al ejecutar) y módulos `node:*`.
- **Alternativas descartadas**: instalar dos archivos (v1/v2) según la versión
  detectada. Fue descartada porque se rompe cuando la persona actualiza
  OpenCode después de instalar gomemory (caso borde del spec), y porque el
  instalador tendría que ejecutar `opencode --version`.

## R-4 · Ejecutar `mem` desde v2

- **Decisión**: la rama v2 ejecuta `mem` con `node:child_process.execFile`
  (`cwd` = `ctx.location.directory`, `stdin` opcional y timeout). La rama v1
  conserva el `$` de Bun para no cambiar el comportamiento ni los tests v1.
- **Evidencia**: `ctx` de v2 no trae `$` ni una API de subprocesos (solo
  `ctx.shell.hook("create.before")`, que intercepta los comandos del agente).
  En el spike, `execFile("echo", …)` respondió `child-ok` dentro de `setup`.
- **Alternativas descartadas**: `Bun.$`, porque el paquete de plugin v2 trae
  `source.node.js` y `source.bun.js`, así que el host puede ser Node.

## R-5 · Correspondencia de capacidades v1 → v2

| Capacidad | v1 | v2 (verificado en tipos) | Paridad |
|---|---|---|---|
| Inicio de sesión | `event` `session.created` | `ctx.event.subscribe()` → `session.created` | ✅ |
| Checkpoint al quedar idle | `event` `session.idle` + `client.session.messages` | `session.idle` (`data.sessionID`) + acumulación desde `ctx.tool.hook("execute.after")` (`tool`, `input`, `sessionID`, `status`) | ✅ (ver R-6) |
| Cierre de sesión | `event` (fin) | `session.deleted` / cleanup de `setup` | ✅ |
| Procedencia del prompt | `chat.message` → `output.parts[].text` | `ctx.session.hook("prompt")` → `event.prompt.text` | ✅ |
| Inyección de protocolo, contexto, Octopus, nudge y aviso | `experimental.chat.system.transform` → `output.system.push(string)` | `ctx.session.hook("context")` → `event.system.push({ type: "text", text })` | ✅ |
| Recuperación post-compactación | `event` `session.compacted` → `pendingRecovery` → próximo transform | `session.compaction.ended` → `pendingRecovery` → próximo hook `context` | ✅ |
| Resumen de compactación (C6) | `client.session.messages` → mensaje `info.summary === true` | `ctx.session.context({ sessionID })`, forma distinta (ver R-7) | ⚠ best-effort |
| Preservar memoria al compactar (C1) | `experimental.session.compacting` → `output.context.push` | `ctx.session.hook("compaction")` → `event.system.push(...)` (el hook recibe la petición de resumen: `system`, `messages`) | ✅ |
| Salida de subagentes | `tool.execute.after` (`tool === "task"`, `output.output`) | `ctx.tool.hook("execute.after")` (`tool === "task"`, `status === "completed"`, `result`) | ✅ (extraer el texto de `Tool.Result`) |
| Texto del modo plan → `plan-approved` | `msg.info.mode === "plan"` al escanear mensajes | `ctx.session.context()` al quedar idle, filtrando por agente plan | ⚠ best-effort (ver R-7) |

- **Filtro por directorio (nuevo en v2)**: la suscripción de eventos entrega
  eventos de **todas** las ubicaciones del servidor (se observó
  `event.location.directory` en cada evento). La rama v2 debe descartar los
  que no correspondan a `ctx.location.directory`. v1 no lo necesitaba porque
  cada instancia del plugin estaba atada a un directorio.
- **Forma del evento**: los datos van en `event.data`, no en `event.properties` (v1).

## R-6 · Checkpoint de fin de turno en v2

- **Decisión**: la rama v2 acumula por `sessionID` los archivos editados o
  escritos (`edit`/`write` → `input.filePath|path|file`) y los comandos
  (`bash` → `input.command`) desde `tool.hook("execute.after")` con
  `status === "completed"`. En `session.idle` vacía el acumulador en
  `mem hook turn-end` con **el mismo JSON** que v1 (`{ files, commands }`).
- **Motivo**: no depende de la forma interna de los mensajes v2 y evita
  releer la conversación completa en cada turno. Además hace innecesario el
  marcador `lastCheckpointedMessage`.
- **Riesgo**: los nombres de tools de v2 (`bash`, `edit`, `write`, `task`) no
  se verificaron con una sesión con modelo. Se validan en el quickstart Q3;
  si difieren, se ajusta una tabla de nombres única y compartida.

## R-7 · Lecturas de mensajes en v2 (C6 y modo plan)

- **Decisión**: ambas lecturas usan `ctx.session.context({ sessionID })` a
  través de un normalizador pequeño. Si la forma no se reconoce, la rama v2
  **omite** la captura y registra `mem hook channel-error opencode …` (FR-005).
  No falla en silencio y no rompe el turno.
- **Motivo**: C6 ya es best-effort en v1 (el respaldo `RecoverySteps` cubre la
  ausencia del resumen). La forma exacta de `SessionMessageInfo` es profunda
  y no se pudo ejercitar sin credenciales de modelo en el entorno aislado.

## R-8 · Configuración `opencode.json` (MCP y permisos)

- **Decisión**: **no se cambia** lo que escribe el instalador.
- **Evidencia**: con el `opencode.json` que genera gomemory
  (`mcp.gomemory {type, command, enabled}` y `permission {"gomemory_*": "allow", …}`),
  `opencode debug config` en v2 lo normaliza por su cuenta a
  `mcp.servers.gomemory {…, disabled: false}` y
  `permissions: [{action: "gomemory_*", resource: "*", effect: "allow"}, …]`.
  `opencode mcp list` muestra `gomemory pending` (el binario `mem` no estaba
  en el `PATH` aislado).
- **Riesgo residual**: que `gomemory_*` siga siendo el nombre de *acción* con
  el que v2 evalúa las tools MCP. Se verifica en el quickstart Q4. Si no
  coincide, se añade la regla con el nombre v2 sin quitar la v1.

## R-9 · Archivo de pruebas copiado a la carpeta de plugins (defecto previo)

- **Hallazgo**: `InstallPlugin` copia todo el directorio embebido
  `plugin/opencode`, así que `gomemory.test.mjs` termina en
  `~/.config/opencode/plugins/`. Ya está así en la máquina de la persona
  usuaria. v2.0.16 no lo carga (solo carga `.ts`/`.js`), pero es un artefacto
  de desarrollo instalado en producción, y OpenCode podría cargarlo en el
  futuro (importa `node:test`).
- **Decisión** (regla de trabajo 7: el hallazgo se cierra, no solo se declara):
  el instalador excluye `*.test.*` y borra el `gomemory.test.mjs` heredado
  del destino, igual que ya limpia la instalación anidada `plugins/gomemory/`.

## R-10 · `cbm-augment.ts`

- **Hallazgo**: ningún archivo del repositorio lo genera ni lo instala. Es de
  `codebase-memory-mcp`.
- **Decisión**: fuera de alcance para corregirlo. `mem doctor` avisa cuando OpenCode es 2.x
  y la carpeta de plugins tiene archivos `.ts`/`.js` sin `export default`
  (heurística textual y sin ejecutarlos), aclarando que no son de gomemory
  (FR-010). Se recomienda abrir un issue en ese proyecto.

## R-11 · Detección de versión para el diagnóstico

- **Decisión**: `mem doctor` ejecuta `opencode --version` con timeout. La
  salida v2 es `opencode v2.0.16` y la v1 es `1.18.32`: se acepta un prefijo
  `v` opcional. Si falla, informa "versión no detectada" y sigue.
  El instalador **no** necesita la versión (R-3).

## R-12 · Hallazgos contra el binario real durante la implementación

- **Fin de turno**: OpenCode 2.0.16 no emite `session.idle`; emite `session.execution.succeeded|failed|interrupted` sin `location`. El adaptador los traduce a `session.idle`.
- **Tools renombradas**: `shell` reemplaza a `bash` y `subagent` a `task` (lista real: `edit, glob, grep, question, read, shell, skill, subagent, websearch, write, execute`). Las tools MCP se invocan vía `execute` (codemode), y la regla de permiso `gomemory_forget_memory: ask` se sigue aplicando.
- **`location` de los eventos**: `session.created` trae la ubicación de la sesión; `opencode api session.create` sin `location` crea la sesión en el HOME del servicio y el plugin del proyecto la ignora, que es lo correcto.
- **Diseño final**: la rama v2 es un adaptador sobre la fábrica v1 intacta (ver tasks.md, "Desvío de diseño"), no un `GomemoryCore` extraído.
