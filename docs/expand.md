# Supertools expand

`mem expand` reúne reglas de protección, inspección de configuración, sugerencias de flujo y aprendizaje de patrones. Se activa desde la instalación en Claude Code, Codex y OpenCode mediante cinco eventos: inicio de sesión, prompt, antes y después de una herramienta y fin de turno.

## Estado e interruptores

```sh
mem expand status --json
mem expand off
mem expand on
mem expand off rules
mem expand on rules
```

El estado informa el cableado leído de los archivos reales, las reglas y sus errores, las activaciones `expand.*` de siete días y el último informe de shield. `off` apaga todo; `on` reactiva también las capacidades apagadas individualmente. Los ajustes se guardan en `.memory/settings.json`. Los nombres válidos son `rules`, `shield`, `flow` e `instincts`.

## Reglas

Las reglas propias viven en `.gomemory/rules/*.md`, un directorio versionable. Las cachés y el estado local permanecen en `.memory/`.

```markdown
---
name: protect-env
event: file
match: regex
pattern: '\.env$'
action: warn
---
Revisa que el cambio no incluya credenciales.
```

Eventos: `bash`, `file`, `prompt`, `stop`, `any`. Coincidencias: `contains` y `regex` de RE2. Acciones: `warn` o `block`; prevalece la más restrictiva. Límites: 64 archivos, 16 KiB por archivo y 512 caracteres por patrón. Una regla inválida se informa y se omite.

La regla de serie `destructive-shell` detecta por tokens el borrado recursivo forzado, `git reset --hard`, pushes forzados (`--force`, `-f`, `--mirror` y refspecs `+rama`), la restauración del árbol de trabajo completo (`git checkout .`, `git checkout -- ./`, `git restore :/`…), `git clean -f` (salvo `-n`/`--dry-run`) y `find … -delete`. `--force-with-lease` y el borrado de una rama remota (`git push origin :rama`, `--delete`) generan un aviso; `git restore --staged .`, que solo deshace el indexado, se permite. Un comando citado por `echo` no se ejecuta ni se interpreta como destructivo.

La regla reconoce los envoltorios habituales: rutas absolutas (`/bin/rm`), `sudo`, `doas` y `pkexec` con sus opciones, `su -c`, `runuser`, `env` (también la línea de `env -S` o `--split-string`), `command`, `exec`, `nohup`, `nice`, `time`, `timeout`, `stdbuf`, `setsid`, `ionice`, `chrt`, `taskset`, `busybox`, `xargs` y `find -exec`; las palabras de control (`if … then`, `{ … }`, `!`) y los subshells `( … )`; los comandos anidados en `sh|bash|zsh|dash|ksh|mksh|csh|tcsh|fish -c "…"` aunque lleven opciones del shell (`--norc`, `-o pipefail`), `eval`, la acción de `trap`, el contenido de `$(…)`, `` `…` `` (también anidadas con `` \` ``), `<(…)` y `>(…)`, y el script que un shell lee por la entrada estándar: un heredoc (`bash <<EOF`), un herestring (`bash <<< '…'`) o lo que le llega por tubería desde `echo`, `printf` o `cat <<EOF`. También analiza lo que sigue a un `&` de segundo plano (sin confundirlo con redirecciones como `2>&1`). Las redirecciones no son argumentos, estén donde estén: `>/dev/null rm -rf x` o `sudo >/dev/null rm -rf x` siguen siendo un `rm`, y las sustituciones de su destino (`ls >$(…)`) se analizan. Un herestring es entrada estándar del comando. Las comillas `$'…'` (con sus escapes decodificados) y `$"…"` se tratan como cualquier literal, las continuaciones con barra invertida se unen y el cuerpo de un heredoc dirigido a un comando que no es un shell es dato, no comandos. Si el delimitador no va citado (`<<EOF`), el shell ejecuta sus `$(…)` y `` `…` `` antes de entregar el texto, así que se analizan también en `cat <<EOF`; con el delimitador citado (`<<'EOF'`, `<<"EOF"`, `<<\EOF`) o la sustitución escapada (`\$(…)`), el cuerpo es literal. Más de cuatro niveles de anidamiento se bloquean sin seguir analizando.

Resuelve lo que se conoce sin ejecutar nada: las variables asignadas con un valor literal en la propia línea (`R=rm; $R -rf x`, también hacia `sh -c`) y las sustituciones que solo hacen `echo` o `printf`, cuya salida se calcula como la del shell: los formatos de `printf` (`%s`, `%b`, `%c`, `%d`, `%x`… con ancho y precisión, reutilizando el formato) y los escapes (`\x6d`, `\155`, `\u…`), y en `echo` los escapes salvo con `-E`, como hacen dash y zsh. Así `$(printf '%s%s' r m) -rf x` se reconoce como `rm`, también cuando la sustitución va pegada a otro texto (`r$(printf m) -rf x`). Las sustituciones se evalúan en el orden en que el shell las ejecuta, antes de su comando: en `X=rm; echo $($X -rf x); X=echo` cuenta el valor vigente en ese momento. Un `#` al principio de una palabra abre un comentario que no se analiza; dentro de una palabra o entre comillas es texto. Una sustitución desconocida tras un shell (`sh $(true)`) puede quedar vacía, así que no cuenta como archivo de script y el shell sigue leyendo su entrada. Un `printf` cuya salida no se puede calcular (`%q`, un número inválido) se bloquea cuando acabaría como comando —en posición de ejecutable o por tubería a un shell— y se permite como argumento.

Es un guardarraíl ante comandos que propone el agente, no un sandbox: no interpreta variables del entorno ni alias, no conoce la salida de otros comandos (`$(cat archivo) -rf x`), no lee scripts ejecutados desde un archivo (`bash script.sh`, `sh < script`, `cat script | sh`, `. script`, `source script`) ni analiza código de otros intérpretes (`python -c`, `node -e`), y no cubre `flock` ni `watch`. Si el hook agota su plazo, la regla de serie sigue decidiendo sobre los comandos de shell (sin cargar reglas propias ni anulaciones), y un comando de shell que supera el tamaño analizable se bloquea.

Para anular la regla de serie, añade una regla válida con `overrides: destructive-shell` y `enabled: false`, o ejecuta `mem expand off rules`.

```sh
mem expand rules --json
git diff | mem expand rules --check
printf 'git push --force' | mem expand rules --check --event bash
```

`--check` devuelve 0 al permitir, 1 al avisar, 2 al bloquear y 3 ante entrada ilegible. Sin evento explícito, evalúa las rutas y líneas añadidas de un diff como una escritura de archivo.

## Skills de flujo

`mem install` (con alcance proyecto o global) y `mem setup-mcp --scope global` distribuyen `verify-loop`, `search-first`, `iterative-retrieval`, `strategic-compact` e `instincts` en las configuraciones de agentes presentes. No crea carpetas de configuración para agentes ausentes. Fuentes canónicas: `infrastructure/templates/expand`; copias publicadas: `skills/`, verificadas byte a byte.

El router sugiere verificar antes de un commit o PR, buscar antes de añadir una dependencia, recuperar contexto antes de delegar y compactar tras 50 herramientas interceptadas. El ajuste `expand_compact_threshold` admite de 10 a 1000. Cada sugerencia se emite una vez por episodio; compactar restablece el episodio. Los avisos de pre-tool nunca aprueban herramientas en nombre de la persona.

Los destinos de skills son `~/.claude/skills`, `~/.codex/skills` y `~/.config/opencode/skills`. OpenCode 2.x lee `<dir>/skills` de cada directorio de configuración que muestra `opencode debug config`: `~/.config/opencode` siempre y `~/.opencode` solo cuando el proyecto está bajo el home. Al reinstalar, una copia en `~/.opencode/skills/<skill>` se retira solo después de escribir el destino nuevo, y solo si es idéntica a la instalada y no hay otros archivos en su directorio. Una copia modificada o con archivos auxiliares se conserva; bórrala a mano si ya no la usas. La desinstalación limpia ambas rutas; véase `specs/040-expand-supertools/evidence/us3-skills-opencode.md`.

## Shield

```sh
mem doctor --shield
mem doctor --shield --json
```

Shield lee configuraciones conocidas e instrucciones de agentes. Detecta secretos en claro, hooks que descargan y ejecutan código remoto (una descarga con `curl`, `wget`, `irm` o `iwr` canalizada a un shell, `python`, `node`, `perl`, `php`, `ruby` o PowerShell, también tras `sudo`, `env` o con ruta, como `| /bin/bash`; y sin tubería: `iex (irm …)`, `DownloadString(…)`, `bash <(curl …)` y `bash -c "$(curl …)"`). Descargar a un archivo y ejecutarlo después (`curl -o /tmp/i.sh && bash /tmp/i.sh`) no se marca como ejecución remota: se informa como ruta de hook insegura cuando el archivo está en un directorio temporal o fuera del home y del PATH. Shield también detecta rutas de hook inseguras, credenciales MCP (incluidas cabeceras `Authorization` con `Bearer`, `Basic` o `Token` literales) y permisos amplios. El informe incluye archivo, categoría, severidad y descripción sin el valor del secreto, además de archivos `skipped`.

Límites: 32 archivos y 1 MiB por archivo. El lector no modifica contenido ni fechas. El escaneo automático se repite cuando cambia la huella de rutas, tamaños o fechas. El comando explícito fuerza un nuevo escaneo. Con critical devuelve 2; con high, 1; en los demás casos conserva el comportamiento habitual de doctor. Sin `--shield`, doctor no escanea.

Las referencias a otra fuente (`$VAR`, `${VAR}`, `env:VAR`, `{env:VAR}` y `{file:ruta}` de OpenCode, `%VAR%`), los bloques privados cerrados y los permisos específicos de gomemory no generan hallazgos por sí mismos; una apertura `<private` mal formada o sin cierre no oculta al escáner el resto del archivo. Sin `HOME` absoluto (systemd, cron, contenedores) solo se escanea el proyecto y el informe lo indica en `skipped`. Los números de línea corresponden al archivo original; en las configuraciones que el lector transforma antes de analizar (TOML de Codex y el extracto `mcpServers` de `~/.claude.json`) el hallazgo se informa sin línea. Rutas y descripciones se depuran de controles de terminal, también al leer el informe guardado. `mem expand status` muestra solo un resumen por severidad. Un hallazgo verdadero requiere revisar la configuración; shield no la reescribe.

## Instincts

Los comandos y las correcciones explícitas se observan silenciosamente en una cola acotada. Al iniciar o cerrar un turno se pliegan hasta 500 observaciones. El plegado ocurre después de que el hook escriba su decisión y tiene un plazo propio de 2 s, así que nunca consume los 150 ms de la decisión; si vence o falla, la cola se conserva para el siguiente evento y queda registrado `expand.timeout` o `expand.error`. Cada UUID cuenta una vez, incluso si el proceso termina entre el commit y la retirada de la cola.

La clave de un comando sale del primer comando significativo de la línea: se saltan las asignaciones `VAR=valor`, `sudo` y los comandos triviales de lectura o navegación (`cat`, `sed`, `grep`, `rg`, `ls`, `head`, `tail`, `find`, `echo`, `cd`, `wc`, `pwd`, `sqlite3` y `python3 -c`). Una línea formada solo por triviales no se observa. El contexto inyecta hasta 8 instincts con confianza ≥ 0.7 en la forma `` `go test` · 40 usos · último 2026-10-24 (0.8) ``, sin la muestra de la ejecución. Además, deja fuera los que no se observan desde hace más de 30 días y las filas antiguas cuya muestra ya no produce su slug con este criterio. Esas filas no se borran.

El store global (`instincts.db` en el directorio de datos del usuario) es auxiliar: si no abre, guardar y leer memorias sigue funcionando, `mem doctor` informa la degradación y solo `mem expand instincts` y `mem sanitize` fallan de forma explícita hasta repararlo.

Las claves y los slugs se derivan del comando ya protegido: una credencial reconocida nunca forma parte del nombre de un patrón. `mem sanitize` reduce los slugs antiguos que la contengan y, si chocan con uno existente, los fusiona (suma observaciones en el proyecto y une proyectos en el registro global). Si otro proceso mantiene abierto `instincts.db`, el saneamiento queda pendiente (`SANITIZE_BUSY`); repítelo con los clientes cerrados para vaciar su WAL.

Privacidad: la cola y `instinct_global` conservan hasta 200 caracteres de las correcciones observadas, protegidos con la redacción de secretos y `<private>`, y un patrón presente en dos o más proyectos se comparte entre ellos. Un dato personal que no encaje en los patrones de credencial puede viajar así fuera del proyecto: envuélvelo en `<private>…</private>` o apaga la supertool con `mem expand off instincts`.

La confianza crece de 0,3 a 0,9. Un patrón con confianza mínima de 0,5 en dos proyectos aparece como global. El contexto incorpora hasta ocho patrones con confianza mínima de 0,7. Un rechazo local también impide inyectar la variante global.

```sh
mem expand instincts list --json
mem expand instincts list --all
mem expand instincts confirm cmd:go-test
mem expand instincts reject cmd:git-status
mem expand instincts evolve cmd:go-test
```

Confirmar guarda una preferencia con `topic_key=instinct:<slug>`. Confirmación y rechazo son finales. Una propuesta global también puede confirmarse o rechazarse en un proyecto que nunca la observó: queda una fila local con ese estado, sin observaciones, y el rechazo impide inyectarla allí. `evolve` requiere TTY, usa el selector de harness de Socratica, muestra una propuesta validada y solo instala después de confirmarla. Sin TTY devuelve 2; cancelar devuelve 3. No hay herramientas disponibles para el modelo durante la generación.

`mem sanitize` cubre las descripciones del proyecto, el registro global y las muestras históricas de la cola. Las observaciones nuevas ya se redactan antes de persistirse.

## Compatibilidad y validación

| Capacidad | Claude Code 2.1.306 | Codex 0.163.0 | OpenCode 2.0.32 |
|---|---|---|---|
| Bloqueo destructivo de shell | Verificado | Verificado en TUI | Verificado con servidor propio |
| Escrituras interceptadas | Write/Edit y variantes | apply_patch | write/edit/patch |
| Bloqueo de fin de turno | Un bloqueo por cierre | Un bloqueo por cierre | Aviso en el siguiente turno |
| Aviso antes de herramienta | Contexto del evento | Contexto del evento | Contexto en el siguiente turno |

En OpenCode, `tool.execute.before` invoca `mem hook expand pre-tool` para todas las herramientas, también `read` o `grep`: la clasificación por tipo vive solo en Go para no duplicarla en el plugin, y las herramientas que no son de shell ni de escritura ni cuentan ni se evalúan. Cada invocación cuesta del orden de 20–50 ms; Claude Code filtra antes con el matcher del hook.

`codex exec` no ejecuta los hooks interactivos y no sirve para verificar esa ruta. Después de instalar o actualizar OpenCode 2.x, reinicia su servicio para cargar el plugin nuevo; `opencode run --standalone` crea un servidor propio para la prueba.

Los resultados, límites y comprobaciones reales están en [la evidencia de 040](../specs/040-expand-supertools/evidence/README.md). No se añaden dependencias ni procesos Node/Python en tiempo de ejecución. Las ideas de ECC se reinterpretan para gomemory; [NOTICE](../infrastructure/templates/expand/NOTICE) contiene la atribución y licencia MIT.
