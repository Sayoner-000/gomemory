# Changelog

All notable changes to gomemory are documented in this file.

The format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/).

## [2.11.0] - 2026-08-24

### Novedades

#### Copiar, pegar y desplazar contenido en la TUI

La TUI permite copiar la vista actual con `Ctrl+Y`. En el detalle de una memoria, la copia incluye el contenido completo aunque solo una parte esté visible. Los campos activos también reciben correctamente el contenido pegado, incluido el pegado entre corchetes del terminal.

Los detalles extensos se pueden recorrer con las flechas, `j`/`k`, `PgUp`/`PgDn`, `Home`/`End` y `g`/`G`.

### Correcciones

#### Codex usa un único registro MCP global

Las instalaciones anteriores creaban una tabla `gomemory_*` por proyecto y fijaban su directorio con `cwd`. Cuando ese directorio dejaba de existir, Codex intentaba iniciar el servidor desde una ruta inválida y mostraba `No such file or directory (os error 2)`.

La instalación ahora migra esas tablas a un único registro `[mcp_servers.gomemory]`, ejecuta `mem mcp` sin depender del directorio de un proyecto y conserva intacta la configuración de otros servidores. Antes de reemplazar el archivo crea un respaldo, mantiene sus permisos y realiza una escritura atómica.

#### Constitución predeterminada sin referencias específicas

La constitución incluida por defecto ya no contiene el título ni la autoría asociados a una organización o persona. El contenido queda disponible como base agnóstica para cualquier proyecto compatible.

## [2.10.2] - 2026-08-23

### Cambios en `mem install`

#### OpenCode registra su memoria en el scope usuario, ya no por proyecto

El registro del servidor MCP y los permisos de las tools se escribían en el `opencode.json` de cada proyecto. Ese registro duplicaba lo que la configuración global del agente ya resuelve: OpenCode combina el nivel usuario con el de proyecto, y el plugin siempre se instala en el directorio global.

Ahora `mem install` registra el servidor y sus permisos una sola vez en `~/.config/opencode/opencode.json`, y retira el registro que instalaciones anteriores dejaron en el proyecto. Si tras retirarlo el archivo solo conserva el `$schema`, se elimina. Si conserva configuración ajena a gomemory, se reescribe sin esas claves y permanece.

`mem doctor` verifica la migración con un canal nuevo (`opencode · user · server_config`): se muestra en verde cuando el registro global existe y como problema con su remedio cuando falta. Antes, perder esa entrada no producía ningún síntoma.

## [2.10.1] - 2026-08-23

### Correcciones

#### La vitalidad de los canales era ciega para Claude Code

El informe de vitalidad solo vigilaba el canal de entrada al plan de OpenCode: los hooks de Claude Code no dejaban rastro de actividad, así que un cambio silencioso del agente dejaba la inyección muerta y el informe seguía en verde.

Ahora la entrada al plan registra el ejercicio del canal (`claude · user · plan_entry`), en paridad con lo que hace el complemento de OpenCode, y el informe vigila ambos agentes. Un canal que demostró funcionar y se apagó aparece como problema con su efecto y su remedio; uno que nunca se ejerció sigue sin alarmar, porque las sesiones no se atribuyen por agente.

#### Salida doble y ruta inválida en la entrada al plan

Ante un fallo al localizar el proyecto, o con el gate del plan atómico deshabilitado, el hook `plan-entered` continuaba ejecutándose después de emitir su salida de silencio: podía trabajar sobre una ruta inválida y, con el gate activo, emitía el documento vacío dos veces.

Ahora cada salida temprana termina la ejecución del hook.

#### Un conteo de sesiones fallido se confundía con cero

Si la consulta de sesiones fallaba, `SessionsSince` devolvía cero: el informe trataba un dato que no pudo leerse como evidencia de que nadie trabajó. En la misma línea, si la lectura del contexto o del recordatorio de guardado fallaba dentro del complemento de OpenCode, la inyección se reducía sin dejar rastro — justo lo que la vitalidad quería evitar.

Ahora un conteo fallido devuelve «desconocido» y el informe calla donde no hay evidencia en ninguna dirección; en el complemento, esos fallos quedan anotados como `channel-error`, con el mismo rastro que ya tenían los fallos del checkpoint de turno.

## [2.10.0] - 2026-08-23

### La matriz de canales como fuente única

Qué artefacto recibe cada agente estaba declarado en catorce tablas repartidas por el código, y solo tres se consultaban realmente: al cambiar un artefacto había que acordarse de editar todas, y el olvido de la mayoría producía los bugs recurrentes entre agentes.

Ahora esa relación vive en una sola matriz (`domain/channel_matrix.go`) y los consumidores derivan de ella lo que necesitan. Un test de contrato verifica que cada agente con soporte tenga sus canales declarados y coherentes con lo que la instalación escribe.

### Economía del contexto en plan-context

La operación de contexto para planificar reenviaba íntegro el historial que la sesión ya había recibido por otro canal. Medido en este repositorio: el contexto de planificación pasó de 12.214 a 1.057 tokens (91 % menos), y el historial duplicado era de unos 11.156 tokens.

Ahora cada entrega queda registrada por sesión y por canal. Una entrega posterior suprime el material ya enviado; una sesión nueva arranca con el historial completo porque no hereda ese registro.

Si la sesión perdió el material —por una compactación, por ejemplo— y el registro sigue creyendo que el agente lo tiene, `mem plan-context --full` entrega el historial completo ignorando el registro.

### Diagnóstico accionable en `mem doctor`

El informe nombraba el archivo ausente y nada más: saber si el problema importaba y encontrar el comando correcto exigía conocer el sistema por dentro.

Ahora, para cada canal roto, el informe describe qué deja de funcionar y propone el comando que lo restablece. Los remedios se agrupan cuando varios canales se corrigen con el mismo comando, para no sugerir ejecuciones repetidas. La salida `--json` entrega los mismos datos como campos (`efecto`, `remedio`, `remedio_advierte`).

### Vitalidad de los canales

El informe comprobaba que el artefacto existiera, pero un complemento presente cuyo agente renombró la operación que usa queda muerto con el archivo intacto: presencia y salud no son lo mismo.

Ahora el sistema registra cuándo se ejerció cada canal por última vez y qué falló si algo falló:

- El complemento de OpenCode anota cada ejercicio correcto (`mem hook channel-fired`) y cada fallo (`mem hook channel-error`) que antes absorbía en silencio.
- Un test de contrato verifica que los hooks que registra el complemento siguen declarados en la interfaz publicada por OpenCode: si el agente renombra una operación, el fallo aparece en la batería de tests y no en la máquina de quien usa la herramienta.
- El informe distingue un canal sin uso porque no hubo trabajo de uno que no responde habiéndolo: con sesiones recientes y sin actividad del canal, lo reporta como problema; sin sesiones, lo declara como degradación que no requiere acción.

### Correcciones

#### La desinstalación dejaba la configuración MCP de algunos agentes

La lista de archivos de configuración MCP estaba escrita a mano y solo conocía el esquema de algunos agentes: la entrada de un agente con esquema propio sobrevivía a toda desinstalación, sin errores ni avisos.

Ahora la lista se deriva de la matriz de canales, así que ya no pueden separarse. Los registros que pertenecen a la persona y no al proyecto —como `~/.codex/config.toml`— quedan fuera por diseño y el informe lo declara.

#### `mem uninstall` no retiraba los artefactos de OpenCode

La desinstalación retiraba la configuración de Claude pero dejaba intactos los permisos pre-aprobados que la instalación escribe en `opencode.json`.

Ahora retira esos permisos con la misma simetría. El plugin de OpenCode no se elimina: reside en el HOME del usuario y lo comparten todos los proyectos.

## [2.9.0] - 2026-08-23

### Cambios en `mem install`, reglas y constitución

A partir de esta versión, `mem install` **ya no escribe archivos de instrucciones en el repositorio destino**.

El bloque que anteriormente se inyectaba en `AGENTS.md`/`CLAUDE.md` duplicaba información que el servidor MCP ya entrega en la respuesta `initialize`. Además, la constitución que se copiaba al repositorio era una versión congelada de 635 líneas. Mantener ambas copias introducía un riesgo evidente: bastaba modificar una de ellas para que terminaran divergiendo.

En su lugar, esta versión centraliza la información en memoria y mantiene dos documentos:

- **Reglas de trabajo:** `get_context()` las entrega completas en cada sesión.
- **Constitución:** permanece disponible en memoria y se consulta cuando es necesaria.

#### Las reglas son un punto de partida

Las reglas incluidas por defecto proporcionan un punto de partida para el equipo. Sin embargo, ofrecerlas sin una forma sencilla de reemplazarlas convertiría a `gomemory` en la autora de las normas de cada equipo.

Por eso esta versión incorpora `mem docs`, que permite consultar, exportar, importar y restaurar los documentos administrados por memoria:

```bash
mem docs list
mem docs export rules -o reglas.md
mem docs import rules reglas.md
mem docs reset rules
```

Las cuatro operaciones también están disponibles desde la TUI. Un **test de contrato** garantiza que ambas superficies mantengan el mismo comportamiento.

El catálogo interno utiliza un diseño **table-driven**: incorporar un nuevo documento requiere agregar una entrada al catálogo, sin necesidad de crear un nuevo comando ni una nueva pantalla.

### Novedades

- **Inicialización automática de reglas y constitución:** `mem install` y el arranque del servidor MCP crean estos documentos únicamente cuando todavía no existen.
- **Los documentos existentes nunca se sobrescriben:** una vez creado, el documento pasa a estar bajo control del equipo y las operaciones posteriores respetan su contenido.
- **Nuevas operaciones de documentación:** se incorporan `mem docs`, `mem constitution [--sync]`, `mem rules` y `mem seed`.
- **Envoltorio `/constitution`:** disponible para Claude Code y OpenCode. No mantiene una copia del texto; resuelve la constitución directamente desde la memoria cuando se invoca.
- **Instalación automática de Windsurf y Cline eliminada:** ambos agentes dejaban una carpeta en la raíz de cada proyecto para almacenar un único JSON. Siguen disponibles de forma explícita mediante:
  ```bash
  mem setup-mcp --agents windsurf,cline
  ```

### Correcciones

#### `ListMemories` no devolvía `topic_key`

`ListMemories` no incluía `topic_key`, a diferencia de `ListAllMemories`. Como consecuencia, la columna llegaba vacía a todos los consumidores que utilizaban esta vía, sin generar errores ni advertencias.

Ahora ambas operaciones mantienen la información necesaria.

#### La inicialización podía publicar la constitución en un ADR externo

Con `adr_sync_enabled=true`, la instalación podía publicar accidentalmente la constitución completa en el ADR externo configurado por el usuario. Esto podía ocurrir de forma síncrona y sin una acción explícita del usuario.

La inicialización, importación y restauración utilizan ahora una **inserción sin efectos secundarios**, evitando cualquier sincronización externa no solicitada.

La depuración de secretos continúa activa. Esta protección forma parte del procesamiento de seguridad y no se utiliza como canal lateral para la sincronización.

#### Las memorias fijadas podían desaparecer del contexto

Una memoria fijada podía dejar de aparecer silenciosamente porque su inclusión dependía de la ventana de recencia.

Ahora las memorias fijadas se resuelven mediante su **clave de tópico**, garantizando su presencia independientemente de la antigüedad.

#### `mem install` podía utilizar el store incorrecto

Cuando `mem install` se ejecutaba desde un directorio diferente al repositorio destino, podía crear las memorias en el store equivocado.

Además, el comando podía informar que los documentos iniciales ya estaban presentes aunque en realidad no se hubiera escrito nada.

Ahora la instalación resuelve correctamente el destino antes de realizar la inicialización y reporta el estado real de la operación.

#### `mem docs export` no escribía correctamente el archivo

El comando:

```bash
mem docs export <alias> -o <archivo>
```

terminaba escribiendo el contenido en `stdout` y dejando vacío el archivo indicado.

La causa estaba en el parser de flags de Go, que deja de procesar flags después del primer argumento posicional. Se corrigió el procesamiento de los argumentos para que `-o` funcione correctamente.

#### Flake en los tests de integración

Se corrigió un flake preexistente en los tests de integración.

Un proceso desacoplado continuaba escribiendo en `.memory/` después de que el test hubiera terminado, provocando una condición de carrera con la limpieza del directorio temporal.

Ahora el proceso se gestiona correctamente antes de finalizar el test, evitando la escritura posterior y la competencia con la limpieza.

## [2.8.0] - 2026-08-20

### Added

- **`mem usage`** — a measured token benchmark per session. Reports baseline
  tokens (what a response would have cost unoptimized), emitted tokens, the
  saved delta and reduction ratio, broken down by operation and by channel
  (`mcp`, `cli`, `tui`). Available as `mem usage [--session ID|--all] [--json]`.
  The header always declares that the counting method is a neutral
  approximation (~4 chars/token), not any provider's tokenizer — figures are
  comparable against themselves, never against anyone's billing. An optional
  reference-window setting (`usage_window_tokens`, off by default) adds an
  estimated "footprint avoided" percentage, clearly labeled `(estimated)`.
  Machine-readable contract: [`docs/USAGE-REPORT-CONTRACT.md`](docs/USAGE-REPORT-CONTRACT.md).
- **Usage screen in the interactive UI** (`u` key) — shows the same session
  report as `mem usage`, plus an on-demand context-optimization snapshot for
  a specific task (same engine as `mem pack build`), which never persists
  between visits.
- **`mem consolidate [--apply]`** — merges redundant memories within a
  project by two criteria: shared topic key, and automatic activity
  checkpoints with byte-identical content. Previews by default (the
  operation is irreversible); no content is lost, distinct text within a
  merged group is concatenated into the row that's kept. Also available from
  the interactive UI's Maintenance screen.
- **`mem get <id>`** — retrieves a memory's full detail by ID from the
  command line, mirroring the `get_memory` MCP tool's capability on the CLI
  channel.
- **Index mode for `get_context`** (`context_index_mode` setting, off by
  default) — emits the working protocol in full plus a one-line index per
  memory (id, type, title), with detail fetched on demand via the existing
  `get_memory` capability. Reversible: toggling it off returns emission to
  byte-identical output.

### Changed

- `charmbracelet/bubbletea` v0.26.1 → v1.3.10, `bubbles` v0.18.0 → v1.0.0,
  `lipgloss` v1.0.0 → v1.1.0. No application code changes were needed; every
  existing screen and test kept its exact behavior.

### Fixed

- **MCP usage-recording middleware never fired.** The fallback middleware
  that records non-self-reporting tool calls (`save_memory`, `get_memory`,
  and others) type-asserted the request params to `*mcp.CallToolParams`, but
  the SDK hands middlewares the *unparsed* params (`*mcp.CallToolParamsRaw`)
  — the assertion silently failed for every call. Found by an end-to-end
  test that drives a real MCP client against the real server, not a
  schema-listing check. Fixed and covered by two new integration tests.
- `search_memories`/`list_memories` usage accounting could report emitted
  tokens higher than baseline tokens for short memories, because the raw
  baseline only summed memory content and ignored the rendered wrapper
  (id/type/title/formatting). Baseline is now derived as emitted + what was
  actually truncated, guaranteeing baseline ≥ emitted.
- `ListAllMemories` never selected the `topic_key` column, so every memory
  loaded through it appeared to have no topic key regardless of what was
  stored — this silently broke topic-key grouping before it was ever used.
- `mem usage`'s "no session" report collapsed with the "all sessions"
  report internally (both used an empty session ID), so an idle project
  could show accumulated historical totals instead of zeros.

## Earlier releases

See [GitHub Releases](https://github.com/Sayoner-000/gomemory/releases) for
v2.7.0 and earlier.
