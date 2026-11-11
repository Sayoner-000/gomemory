# Changelog

All notable changes to gomemory are documented in this file.

The format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/).

## [v2.54.1] - 2026-11-11

### Fixed

- Las descripciones de las skills HCD se escriben entrecomilladas en el frontmatter YAML, para que los harness con parser estricto no las rechacen.

### Security

- Toolchain de Go fijado en `go1.27.4` (parche de seguridad); los binarios publicados se compilan con él.
- Dependencias actualizadas para reducir superficie de ataque: `modelcontextprotocol/go-sdk` v1.8.0, `modernc.org/sqlite` v1.62.0 (`libc` v1.80.1), `golang.org/x/sys` v0.50.0, `x/sync` v0.25.0, `x/time` v0.16.0, `x/oauth2` v0.38.0, `bubbletea` v2.1.1, `lipgloss` v2.1.0 y `x/ansi` v0.11.9. `govulncheck` no reporta vulnerabilidades.

### Changed

- Informe de auditoría ACR + ADM del delta posterior a v2.54.0 en `docs/audits/2026-11-10-acr-adm-delta-v2.54.0.md`.

## [v2.54.0] - 2026-11-10

### Added

- Regla de serie `native-read-over-shell` (`domain/rules_read.go`). Bloquea leer un archivo de código con `cat`, `bat`, `less`, `more`, `head`, `tail`, `nl`, `sed`, `awk` o con un intérprete inline (`python -c`, `node -e`, `ruby -e`, `perl -ne`, `python3 -`) que recibe la ruta como argumento, y pide la lectura nativa del harness o el grafo de código. En Codex, que no tiene lectura nativa, solo avisa.
- `RuleInput.Agent` y `RuleInput.GraphCoverage`: las reglas conocen el runtime que ejecuta el comando y la cobertura del grafo de cada ruta que toca. El adaptador (`newGraphCoverage` en `adapters/primary/cli/hook_expand_graph.go`) busca el `.codegraph/` más cercano a la ruta y, para un directorio, comprueba con un recorrido acotado que contenga código indexado.
- Reglas de trabajo `workrules-v3` (`infrastructure/templates/agent-preamble.md`), organizadas en cuatro bloques: evidencia antes que cambios, proceso, herramientas nativas y grafo, y memoria y contexto. Incluyen un glosario de siglas (ACR, ADM, HCD) para que el agente invoque la skill en lugar de preguntar qué significan. Son genéricas para cualquier harness y declaran gomemory como única memoria y fuente de instrucciones, sin `AGENTS.md` ni `CLAUDE.md`.
- Informe de auditoría ACR + ADM en `docs/audits/2026-11-10-acr-adm-v2.54.0.md` (`acr_8ee6f431-8db6-4a61-81b3-19741ab0674c`, `APPROVED`).

### Changed

- `graph-before-grep` actúa en cualquier proyecto con gomemory y sobre rutas de otros repositorios, porque calcula la cobertura para la ruta buscada y no solo para el proyecto de la sesión. Resuelve `cd` y `git -C`, y bloquea `grep -r`, `rg` y `git grep` sin filtro sobre directorios con código indexado. Una búsqueda con un filtro no indexado (`-g '*.md'`) sigue permitida.
- El recordatorio de arranque (`locales/es|en/get_context.md.tmpl`) y el protocolo del plugin de OpenCode explican la regla de herramientas nativas.
- La descripción de las skills `adversarial-consensus-review` y `defensive-qa-audit` menciona sus siglas (ACR y ADM), que es lo que el harness usa para activarlas.
- Las semillas de reglas v2 sin editar se actualizan solas a la v3 con `mem update`; las personalizadas no se tocan.
- Guías HTML en la edición `v2.54` (`Guia-Rapida-HCD_GOMEMORY-v2.54.html`, `Guia-SDD_HCD_GOMEMORY-v2.54.html` y `Guia-Tecnica_GOMEMORY-v2.54.html`), y actualización de `README.md`, `docs/README.md`, `docs/ARQUITECTURA.md`, `docs/MANUAL.md`, `docs/expand.md` y las guías para desarrolladores.

### Fixed

- Esquemas MCP rechazados por clientes estrictos: `graph_status`, `start_session`, `pack_savings` y `octopus_status` publicaban un `object` sin `properties`, y `socratica_flow.preparation.value` el esquema `true`. Algunos clientes descartaban entonces la lista de tools entera y el agente recurría a la consola. `clikit.AddTool` publica ahora `properties` en todo objeto y los campos `any` como `object`.
- `graph-before-grep` ya no bloquea búsquedas en rutas externas sin grafo por aplicarles las extensiones del proyecto de la sesión (ADM 1).
- `native-read-over-shell` detecta la lectura con un intérprete inline que recibe la ruta como argumento aparte (ADM 2).
- Un heredoc o here-string ya no hace que se omitan las comprobaciones de un comando que también nombra rutas (ADM 3).
- `graph-before-grep` resuelve `git -C` y evalúa la cobertura del repositorio correcto (ADM 4).
- Las reglas reconocen las rutas absolutas de Windows (`C:/…`, `C:\…` y UNC): son externas y no se resuelven contra un `cd` previo.
- `getCatalog` ya no guarda en caché un catálogo de idiomas vacío. En los tests, una primera llamada sin plantillas a la vista dejaba sin idiomas a las siguientes, y la consola de `mem install` dejaba de preguntar el idioma según el orden de ejecución.
- El manual de referencia para desarrolladores nombraba una regla inexistente (`inline-Workaround`); ahora lista las cuatro reglas de serie reales.

## [v2.53.0] - 2026-11-10

### Added

- Examen socrático en `/hcd-define` (HCD-057): en carril `full`, el motor exige registrar el examen de supuestos, alternativas y criterios mediante `mem hcd review --kind socratic --action complete` antes de autorizar la aprobación. Soporta de cero a tres preguntas justificadas y preserva respuestas u omisiones sin cerrar bloqueos no resueltos.
- Control explícito de la revisión splash en `/hcd-approve` (HCD-057): el motor exige registrar la decisión de aceptar (`--action accept`) u omitir (`--action skip`) la revisión splash. Al aceptarla, se registran su inicio (`--action start`) y resultado (`--action complete`); un hallazgo de severidad `Blocker` detiene la aprobación hasta subsanarse o descartarse con cita explícita (`--action dismiss`).
- Subcomando CLI `mem hcd review`: interfaz para registrar eventos de examen socrático y revisión splash con parámetros `--kind`, `--action`, `--revision`, `--operation-id`, `--input` (archivo o stdin `-`), `--quote`, `--attempt`, `--reason`, `--finding`, `--client` y `--json`.
- Esquema de persistencia v13: tabla inmutable `hcd_definition_reviews` con control de concurrencia optimista (CAS) sobre el último evento de la revisión, clave única idempotente por `operation_id` e índice `idx_hcd_definition_reviews_unit`.
- Saneamiento de privacidad (`mem sanitize`): categoría `hcd_definition_reviews` en `domain/privacy_maintenance.go` y lector/aplicador en persistencia para limpiar etiquetas `<private>` y tokens en las revisiones de definición históricas.
- Informe de auditoría ACR + ADM en `docs/audits/2026-11-09-acr-adm-worktree-hcd057.md` (`acr_cfdfdc49-3177-4be1-9bb8-d666cdfeb2f8`, veredicto `APPROVED`).

### Changed

- Actualización de las guías HTML de gomemory a la edición `v2.53` (`Guia-Rapida-HCD_GOMEMORY-v2.53.html`, `Guia-SDD_HCD_GOMEMORY-v2.53.html` y `Guia-Tecnica_GOMEMORY-v2.53.html`), junto con `README.md`, `docs/README.md`, `docs/ARQUITECTURA.md` y `docs/MANUAL.md`.
- Plantillas de skills HCD en `infrastructure/templates/hcd/` (`hcd-define`, `hcd-approve` y `hcd`): incorporan el examen socrático y el flujo de aceptación u omisión de splash delegando todos los registros al motor.

### Fixed

- Máquina de estados en `ValidateTransition` (`domain/hcd_definition_review.go`): las transiciones `complete`, `fail`, `unavailable` y `dismiss` se evalúan contra la ventana activa posterior al último `accept`, permitiendo reintentos válidos tras un `skip` previo en la misma revisión (ACR H-02 / C-002).
- Lectura segura de entrada en `mem hcd review` (`adapters/primary/cli/hcd/execute.go`): decodificación acotada a 1 MiB mediante `io.LimitReader`, `DisallowUnknownFields` y rechazo de tokens sobrantes tras el objeto JSON (ACR H-03 / C-005).
- Ayuda detallada en CLI (`adapters/primary/cli/cli.go`): `helpReference` incorpora el subcomando `review` en la sección de `mem hcd` (ACR H-05 / C-004).

## [v2.52.0] - 2026-11-08

### Added

- Idioma del proyecto configurable entre español (`es`, por defecto) e inglés (`en`), con cambio en caliente mediante `mem lang [es|en]` o `mem settings --lang <código>` y selección durante `mem install` (HCD-056). Los paquetes de idioma viven en `infrastructure/templates/locales/<código>/` (`meta.json`, `messages.json`, `get_context.md.tmpl` y `atomic_plan.md.tmpl`) y el adaptador `adapters/secondary/i18n` los descubre desde el `embed.FS` a través del puerto `ports.LocaleCatalog` (`application/ports/i18n.go`). `get_context`, `get_plan_context`, `mem://context`, `mem context` y los avisos de los hooks leen el idioma activo en cada llamada sin reiniciar el servidor MCP.
- `mem expand status` y `mem expand off` comprueban y retiran también los hooks de `agy` en `~/.gemini/settings.json` (HCD-055). La salida de texto muestra `agy ✓/✗` y `--json` incluye el campo `wiring.agy`.
- Dos guías HTML para equipos de desarrollo en `docs/`: `Guia-Explicacion-Developers-GOMEMORY.html` y `Manual-Referencia-Developers-GOMEMORY.html` (con comparativa de los tres grafos de código y explicación de la regla `graph-before-grep`).
- Informe de auditoría ACR + ADM en `docs/audits/2026-11-08-acr-adm-v2.52.0.md` (`acr_01ed3c29-8b10-4fec-aaa4-90c3bfcc5fde` y `acr_a702b671-bc8e-4d40-a7c4-dae21e64ee02`, `APPROVED`).

### Changed

- Guías HTML actualizadas a la edición `v2.52` (`Guia-Rapida-HCD_GOMEMORY-v2.52.html`, `Guia-SDD_HCD_GOMEMORY-v2.52.html` y `Guia-Tecnica_GOMEMORY-v2.52.html`), junto con `README.md`, `docs/README.md`, `docs/ARQUITECTURA.md`, `docs/CONFIGURATION.md`, `docs/MANUAL.md` y `docs/expand.md`.

### Fixed

- `get_context` y `get_plan_context` ya no modifican en sitio el `Builder` compartido de `deps.ContextBuilder` al resolver el idioma de la llamada (ACR `acr_01ed3c29` H-01 / C-004). `(*usecases.Builder).WithLanguage` y `contextBuilderForLang` devuelven una copia superficial por invocación, evitando condiciones de carrera cuando concurren varias llamadas MCP.
- Las plantillas `get_context.md.tmpl` (`es` y `en`) usan el recordatorio corto `memoryProtocolReminder` de HCD-040 en lugar del bloque antiguo de 43 líneas, manteniendo `JUEZ IMPARCIAL`/`IMPARTIAL JUDGE`, `PRIVACIDAD`/`PRIVACY`, el aviso sobre `MEMORY.md` y `CodeRetrievalPolicyShort` dentro del tope de 2.200 bytes del primer prompt (ACR `acr_01ed3c29` H-02 / C-002).
- `get_plan_context` reconoce las cabeceras de contexto en todos los idiomas registrados (`isContextDocumentHeader`) para no duplicarlas, y `persistence.SettingsRepository` normaliza a `"es"` cualquier código de idioma no soportado o con separadores de ruta (ACR `acr_01ed3c29` M-01 / M-02).
- Las pruebas de `tests/contract` y `tests/integration` que abren sockets en loopback o invocan `ps`/`pgrep` se saltan con `skipIfNoLoopback` y `skipIfNoProcessInspection` cuando corren en un sandbox que restringe esas operaciones (ADM L-04).

## [v2.51.2] - 2026-11-07

### Added

- Documentación explícita del subagente `splash` en el flujo HCD dentro de `docs/MANUAL.md` (índice y paso 4 de la ceremonia, `/hcd-approve`).
- Integración del rol de `splash` (modelo Elephant-Goldfish) en las tres guías HTML: `Guia-Rapida-HCD_GOMEMORY-v2.51.html` (paso approve), `Guia-SDD_HCD_GOMEMORY-v2.51.html` (capítulo 16, auditoría de 4 defectos y función de Blockers) y `Guia-Tecnica_GOMEMORY-v2.51.html` (capítulo 10, tarjetas de arquitectura EGM y gates de aprobación).

### Changed

- Depuración de patrones sintéticos y humanización de la narrativa documental en `docs/README.md`, `docs/ARQUITECTURA.md`, `docs/architecture.md` y `docs/AGENT-INTEGRATION.md`, eliminando adornos innecesarios y uniformando el estilo técnico.

## [v2.51.1] - 2026-11-07

### Added

- Índice maestro de documentación en `/docs` (`docs/README.md`) para navegación completa en GitHub y GitLab con enlaces a todas las guías, arquitectura, referencias y manuales.
- Documentación integral del subagente `splash` (modelo Elephant-Goldfish / revisor sin memoria) y su integración en `/hcd-approve` en `docs/ARQUITECTURA.md`, `docs/architecture.md` y `docs/AGENT-INTEGRATION.md`.
- Pruebas que demuestran que una compresión que vence el plazo de 150 ms del hook `tool-output` omite la concesión automática de referencias sin tocar el almacén de concesiones (HCD-054).

### Fixed

- El hook `tool-output` ya no consume la nota de recuperación si la salida comprimida se descarta por superar el presupuesto (HCD-053). El marcador solo se anota cuando la salida se entrega efectivamente al agente.
- Los campos JSON comprimidos por el hook `tool-output` se ordenan alfabéticamente para garantizar que la nota de recuperación vaya de manera determinista en el primer campo (HCD-053).
- Las pruebas de integración del hook `tool-output` y de tope de originales toleran la carga intermitente de los runners de CI sin relajar el presupuesto en producción (HCD-053).
- `mem hcd analyze` ya no emite avisos `PATH-MISSING` falsos para rutas de configuración de agentes bajo el directorio HOME (`.claude`, `.codex`, `.gemini`, `.config`, `.cursor`, `.windsurf`, `.cline`) citadas en la definición (HCD-052).

## [v2.51.0] - 2026-11-07

### Added

- `/hcd-approve` ofrece una revisión `splash` antes de pedir la aprobación (HCD-051). En la ceremonia completa, la skill pregunta si se lanza el subagente `splash` sobre el documento de la unidad. `splash` arranca sin la memoria del proyecto y busca rutas citadas que no existen, criterios que no se pueden comprobar, supuestos sin evidencia y contradicciones entre secciones. Los hallazgos se muestran antes de la definición. Un Blocker no deja pedir la cita hasta que se corrige o se descarta de forma explícita; Major y Nit son informativos. Si `splash` no está disponible, una línea remite a `mem setup` y la aprobación sigue. Corregir la definición antes de aprobarla no cuesta nada; hacerlo después obliga a aprobarla de nuevo y a repetir todas las verificaciones.

### Changed

- `mem setup` instala `splash` también en Codex (`~/.codex/agents/splash.toml`, con `sandbox_mode = "read-only"`) y en agy (`~/.gemini/config/agents/splash/agent.md`, con `inheritMcp: false`), y `mem uninstall` lo retira (HCD-051). HCD-047 había concluido que esos dos agentes no leían definiciones de agente en archivo; un sondeo en sesiones reales de agy 1.5.3 y Codex 0.167.0 mostró que sí. En Codex, `splash` se lanza ahora con `spawn_agent(agent_type: "splash", fork_turns: "none")`, que carga sus instrucciones de revisor, y el aviso del hook lo recomienda así.

### Fixed

- En sandboxes que no permiten ejecutar `ps`, una conversación ya no pierde su identidad (ADM-14). El enlace con el agente en Unix necesita `ps` para identificar el proceso del host; si el entorno lo deniega, gomemory no reutiliza un PID sin verificarlo. El arnés puede fijar `GOMEMORY_CONVERSATION_ID` con el identificador de la conversación para los hooks, el MCP y la CLI. Esa variable solo se usa cuando no hay `session_id` del hook ni enlace del host, y no debe compartirse entre agentes. Además, `ps` tiene un plazo de 1 segundo, se ejecuta con `LC_ALL=C` para que el formato de la hora de arranque no dependa del idioma del sistema, y rechaza un PID inválido.
- Las pruebas de integración que levantan un servidor HTTP local se saltan en sandboxes que deniegan `bind` o `connect` en loopback, en lugar de fallar (ADM-15). Cualquier otro error de red sigue haciendo fallar la prueba.

### Known issues

- En Codex, el subagente `splash` hereda los servidores MCP globales, gomemory incluido, y el archivo del agente no lo impide (se probaron `enabled = false` y `enabled_tools = []`). Sus instrucciones le prohíben usar herramientas MCP, pero ese aislamiento es por instrucción, no impuesto. En agy, `inheritMcp: false` sí lo deja sin MCP.

## [v2.50.1] - 2026-11-07

### Fixed

- `mem hcd analyze` ya no da un `PATH-MISSING` falso cuando un documento cita varias líneas con barra, como `tests/integration/session_delta_test.go:52/55` (HCD-050). Antes solo se quitaban los sufijos `:12`, `:12:3`, `:152-159` y `:160,190`, y con `:52/55` la ruta se buscaba con el sufijo pegado. Corregir la cita en el documento para quitar el aviso cambiaba la definición, así que obligaba a aprobarla de nuevo y a verificar otra vez todas las tareas. El aviso `TASK-NO-FILE` usa la misma regla y también reconoce esas rutas.

## [v2.50.0] - 2026-11-07

### Added

- Historial de decisiones en la memoria (HCD-046). Cada unidad HCD tiene una ficha propia (`topic_key` `hcd/HCD-NNN`) que el motor actualiza en cada paso, en lugar de dejar una memoria suelta por paso. Cada informe de `docs/audits/` tiene la suya (`audit/<archivo>`) con el veredicto, los hallazgos y su estado. Las fichas se enlazan con los archivos que citan en la tabla nueva `memory_files` (esquema 12), así que aparecen en el contexto cuando el agente va a editar uno de esos archivos. `mem hcd index` muestra qué haría y `mem hcd index --apply` lo escribe; se puede repetir sin duplicar nada. Para consultar: `mem hcd history [texto] [--file <ruta>]`, o la tool MCP `hcd_history` desde el agente.
- Subagente `splash`, un revisor que empieza sin memoria (HCD-047). No recibe el aviso de arranque, ni las tools, ni las instrucciones de gomemory, así que revisa el trabajo sin conocer el razonamiento que lo produjo. Funciona en Claude Code y OpenCode (como agente de solo lectura, instalado por `mem setup`), en agy (`invoke_subagent` con `TypeName: "splash"`) y en Codex (`spawn_agent` con `fork_turns: "none"`; si falta, el hook avisa). `mem uninstall` lo retira.
- `GOMEMORY_TERMINAL` da a la CLI y a la TUI una sesión propia (HCD-048). Con `GOMEMORY_TERMINAL=pane-1`, la conversación es `terminal-pane-1`. El MCP y los hooks ignoran la variable.

### Fixed

- La CLI vuelve a compartir la sesión del proyecto con el agente (HCD-048). Desde la v2.49.0, `mem` ejecutado en una terminal tomaba el TTY como identidad (`terminal-ttys017`) y abría una sesión que el MCP y los hooks no veían. Mientras esa sesión estaba abierta, `get_plan_context` reenviaba el historial completo en cada turno y la compactación no restablecía lo ya entregado. El CI no lo detectaba porque corre sin TTY. Ahora el TTY no cuenta: sin `GOMEMORY_TERMINAL`, la CLI usa la misma sesión que el agente. Las sesiones `terminal-…` que queden abiertas se cierran solas tras 4 horas sin actividad.
- La evidencia de `mem hcd verify` aplica la privacidad a la salida completa antes de recortarla (HCD-043). Antes, si un bloque `<private>` empezaba fuera del extracto, su contenido podía quedar en la evidencia.
- `filepath` y `topic_key` entran en el contrato de privacidad (HCD-044), como ya lo estaban el título y el contenido. Guardar o importar una memoria con texto `<private>` en esos campos da un error de validación. Las filas antiguas se protegen al leerlas, y `mem sanitize` deja `filepath` protegido y `topic_key` vacío. La exportación también los protege, aunque el repositorio no lo haya hecho antes (ADM-10).
- El plazo de `mem hcd verify` también termina los procesos que lanza la comprobación (HCD-045). La comprobación corre en su propio grupo de procesos y, al vencer el plazo, se termina el grupo entero; antes, un hijo podía seguir vivo y bloquear la salida. Además, un PID 0 ya no lleva a matar el grupo de gomemory (ADM-08). En Windows, que `taskkill` no encuentre el proceso porque ya terminó deja de contar como fallo, y cualquier otro error sí se informa (ADM-09).
- Las pruebas unitarias de `mem update` y de la consulta de releases ya no abren un puerto local (HCD-049). En sandboxes sin permiso de `bind`, `httptest` entraba en pánico y tumbaba el paquete entero. `TestPsInfo_ProcesoActual` se salta donde `ps` no está permitido (ADM-11).

## [v2.49.2] - 2026-11-06

### Changed

- El primer prompt de cada sesión es más corto (HCD-040). El recordatorio del protocolo mantiene todas sus reglas, pero cada una en una o dos frases: el guardado, el juez y la privacidad; «no confundir sistemas» ya no repite los nombres de las tools, que el ToolSearch del principio ya carga. La política de recuperación de código usa una versión corta, `domain.CodeRetrievalPolicyShort`. Las instrucciones MCP conservan la larga. Medido con el binario: sin Octopus el primer prompt baja de 4.864 a 3.044 caracteres (−37 %) y con Octopus, de 5.978 a 4.158 (−30 %). El mismo recordatorio cierra `get_context` y `get_plan_context`, así que allí también se ahorran unos 1.800 caracteres por llamada.

### Fixed

- En Windows, la copia local de `mem.exe` se retira a la primera (HCD-039). Para reconocer la copia, gomemory la ejecuta con `version` y la borra justo después. Windows a veces mantiene bloqueado un instante el ejecutable recién usado, y el borrado fallaba con «being used by another process». `mem install` y el arranque de sesión avisaban entonces «no se pudo retirar», y el job de Windows del CI fallaba de vez en cuando. Ahora `atomicfs.Remove` reintenta ese fallo durante 2 segundos como máximo, con la misma espera que ya usaba el reemplazo de archivos.
- El grafo de código propio resuelve las llamadas hacia funciones que se crean después (HCD-041). Si `a.go` llamaba a `F` y `F` aparecía más tarde en `b.go`, el reindexado incremental no volvía a mirar `a.go` porque no había cambiado, y `get_symbol` no mostraba ese llamador hasta un `mem index -force`. Ahora las llamadas sin resolver se guardan en la tabla `code_pending_calls` y se resuelven en cuanto aparece la función. El esquema pasa a la versión 11. Las bases anteriores no tienen guardadas las llamadas pendientes antiguas: ejecuta `mem index -force` una vez en cada proyecto después de actualizar.

## [v2.49.1] - 2026-11-05

### Changed

- El contexto de arranque usa todo su tope y no se envía dos veces (HCD-037). Si una sección de memorias no cabía entera, el recorte de los hooks la quitaba completa, porque solo cortaba entre párrafos. Ahora corta después de la última entrada que cabe. Las entradas que llegan completas quedan anotadas como entregadas, y `get_context` las sustituye por una línea `⟦ya entregado⟧`. Lo que el recorte dejó fuera llega entero, y `get_plan_context` sigue enviando el documento completo. En este proyecto el arranque pasa de 5.552 a 9.932 caracteres (35 entradas más) y el `get_context` siguiente baja de 17.835 a 11.774. Sumando las dos cosas, cada sesión gasta un 7 % menos.

### Fixed

- En Windows, los hooks vuelven a encontrar la conversación de su agente (HCD-038). Para enlazar un hook sin `session_id` con su agente, la v2.49.0 consultaba `ps`, que en Windows no existe. Los hooks `turn-end` y `user-prompt-submit` caían entonces en una conversación vacía, así que perdían el estado por turno y repetían contexto ya entregado. Ahora gomemory lee el proceso padre y su hora de arranque con la API de Windows, y reconoce `cmd`, PowerShell y `pwsh` como shells intermedios. El job de Windows del CI vuelve a verde.

## [v2.49.0] - 2026-11-05

### Added

- Revisión ACR de seguimiento (HCD-032). `review_start` acepta `since_review_id`; en la CLI es `mem review --commit <sha> --since <review-id>`. La revisión nueva hereda lo que la anterior dejó asentado y solo revisa los archivos que cambiaron entre los dos commits. Un hallazgo heredado no se vuelve a adjudicar si su archivo sigue igual; si cambió, se reabre. Los revisores lo citan con `settled_ref`. Esquema 9, aditivo.
- Ahorro estimado de una revisión de seguimiento (HCD-035). Con delta de git, `review_start` devuelve el bloque `savings` y la CLI imprime una línea «ahorro estimado». Mide lo que el revisor deja de leer, no los tokens que gasta el modelo. En la historia de gomemory sale entre un 93 % y un 99 % menos de entrada.
- Tres avisos nuevos en `mem hcd analyze` (HCD-034), todos MEDIUM y sin bloquear el cierre: `TASK-NO-FILE` cuando una tarea no dice qué archivo toca, `PATH-MISSING` cuando una ruta citada no existe (solo desde `verify`) y `CHEAPEST-FILLER` cuando la «Alternativa más barata» es un «ninguna» sin motivo.

### Changed

- La skill `adversarial-consensus-review` explica cómo encadenar una revisión con la anterior: `since_review_id` al iniciar y `settled_ref` para lo heredado (HCD-033). Las dos copias conservan sus finales de línea CRLF.

### Fixed

- Dos agentes en el mismo proyecto ya no se quitan la sesión (HCD-036). Compartían la sesión de gomemory, el registro de entregas y el estado por turno, así que cada vez que uno arrancaba o cerraba, el otro recibía de nuevo el contexto (unos 4.300 tokens) y el bootstrap (unos 1.500). Ahora cada conversación tiene lo suyo: su sesión (`sessions.conversation_id`, esquema 10, aditivo) y su estado en `.memory/conversations/<id>/`. Un proceso sabe a qué conversación pertenece por el `session_id` del hook, por el enlace que SessionStart publica para su proceso host o por su terminal. Si no tiene ninguno de los tres, usa la sesión de proyecto sin conversación, que nunca es la de un agente.
- Las sesiones abandonadas se cierran solas tras 4 horas sin actividad, sean de quien sean. El MCP ya no deja una sesión huérfana cuando arranca a la vez que el hook SessionStart.
- Las unidades locales HCD-030 y HCD-031 pasan a ser HCD-032 y HCD-033 para no chocar con las del remoto.

## [v2.48.0] - 2026-11-05

### Added

- Regla de serie `native-edit-over-scripts` en el motor de `expand` (HCD-030): intercepta en `PreToolUse:Bash` comandos que usen scripts inline (`python`, `python3`, `node`, `bun`, `deno`, `ruby`, `perl`), reemplazos in-place (`sed -i`, `awk`) y comodines de shell (`cat >`, `echo >`, `printf >`, `tee`) sobre extensiones de código de desarrollo estándar (`.ts`, `.tsx`, `.js`, `.jsx`, `.vue`, `.svelte`, `.astro`, `.go`, `.py`, `.rs`, `.java`, `.kt`, `.cs`, `.cpp`, `.php`, `.rb`, `.swift`, `.sh`, `.dart`, `.sql`, etc.), guiando pedagógicamente al agente a usar herramientas nativas de edición (`edit_file`/`Edit`) para diffs auditables y activación de contexto de grafo (HCD-028).

### Fixed

- Numeración correlativa HCD al iniciar unidades (HCD-031): `HCD.Start` ahora calcula `MAX(sqlite_max, disk_max) + 1` considerando los archivos `hcd/HCD-*.md` existentes en el workspace, evitando colisiones de prefijo en clones frescos o entornos con base de datos SQLite recién inicializada.

## [v2.47.0] - 2026-11-05

### Added

- Aviso mínimo a subagentes (`mem hook subagent-start`): en Claude Code, OpenAI Codex, Antigravity y OpenCode, los subagentes reciben un aviso breve de solo lectura en lugar de todo el prompt de proyecto, conservando contexto y tokens.
- Regla del core `graph-before-grep`: intercepta y bloquea comandos `grep` sobre archivos de código indexados en el grafo, guiando al agente hacia las herramientas estructurales (`codegraph_explore`, `search_graph`, `search_code`, `get_symbol`).
- Inyección de contexto de grafo en edición: al modificar archivos de código, los hooks inyectan llamadores, dependencias y radio de impacto del símbolo, acotado por cuota de tokens y deduplicado una vez por archivo y sesión.

### Fixed

- Preservación de aristas entrantes en el grafo interno SQLite al reindexar incrementalmente archivos modificados, descartando aristas sin destino cuando un símbolo desaparece y evitando desconexiones entre llamadores.

## [v2.46.0] - 2026-11-04

### Added

- Comando `mem optimize` con soporte para flags `--dry-run`, `--yes`, `--target` (`all`, `internal`, `external`) y `--reindex-external`.
- Saneamiento transaccional en SQLite de relaciones huérfanas entre memorias, consolidación de duplicados y purga en orden seguro de nodos, aristas y paquetes huérfanos del grafo interno con aislamiento estricto por proyecto.
- Purga de filas huérfanas en el índice virtual FTS5 `code_search` antes de la optimización y ejecución de `PRAGMA optimize`.
- Mantenimiento y saneamiento de residuos de snapshots para proveedores externos de grafo de código (`codebase-memory-mcp` y `codegraph`), preservando snapshots válidos de otros proveedores y tolerando la ausencia del binario en PATH.
- Opción interactiva "Optimizar (grafos, relaciones e índices)" en el menú de mantenimiento de la TUI (tecla `m` → opción 4), con pantalla de confirmación y reporte de elementos saneados en la barra de estado.

## [v2.45.2] - 2026-11-04

### Fixed

- `mem install`, `mem update` y `mem seed` comparan `.specify/memory/constitution.md` con la constitución vigente del proyecto, no con la plantilla del binario. Con una constitución personalizada y ya sincronizada, avisaban en cada ejecución de que la copia de Spec Kit tenía ediciones y recomendaban `mem constitution --sync`, que no cambiaba nada.
- Si la copia de Spec Kit es una plantilla anterior sin tocar, se sincroniza con la constitución del proyecto. Antes se reescribía con la plantilla del binario y perdía las secciones propias, como las excepciones del proyecto.

## [v2.45.1] - 2026-11-04

### Fixed

- `mem setup-mcp --scope global` muestra Antigravity CLI aunque no haya nada que cambiar: `✓ agy: ya registrado en ~/.gemini/config (sin cambios)`. Antes no imprimía ninguna línea de agy, aunque lo contaba entre los registros completados.
- Cuando todas las habilidades ya están instaladas, el panel Habilidades de `setup-mcp` muestra `✓ todas al día (sin cambios)` en lugar de quedar vacío.

## [v2.45.0] - 2026-11-04

### Added

- La consola tiene un solo juego de símbolos de estado: `✓` completado, `✗` error, `⚠` aviso, `○` informativo o pendiente, `●` activo, `−` omitido, `▸` seleccionado y `⚑` marcado. Los define `console` y los pinta lipgloss con los colores del tema. Sin terminal, con `NO_COLOR` o con `TERM=dumb` salen sin color ni secuencias ANSI.
- Dos pruebas de guarda: `TestSinEmojis` falla si vuelve a aparecer un emoji en la salida de `cli`, `setup`, `console` o `tui`, y `TestCicloDeVida_SinFmtPrint` falla si un comando del ciclo de vida escribe directamente en stdout.

### Changed

- `mem setup-mcp`, `mem install`, `mem uninstall` y `mem update` escriben su salida a través de la consola, igual que `mem doctor`. `setup-mcp` agrupa el resultado en tres paneles (Agentes, Integración y Habilidades) y resume cada familia de habilidades en una línea con el número de archivos y sus carpetas, en lugar de listar cada archivo.
- La salida de terminal ya no usa emojis (`✅ ❌ ⚠️ ℹ️ 🔌 📦`…). En la TUI, los errores se marcan con `✗` en vez de `✕`, y las memorias de preferencia con `⚑` en vez de `♥`.
- Las rutas bajo el directorio personal se muestran abreviadas con `~` en la salida de `setup-mcp`.

### Fixed

- `mem setup-mcp --scope global` sin `--agents` registra también Antigravity CLI (`agy`) si está instalado. Si no lo está, lo indica y no crea `~/.gemini/config`.
- La sección Evidencia de los documentos HCD muestra la hora local del equipo con su desfase (`2026-11-04 04:41 -05:00`). Antes mostraba la hora UTC sin zona, y un registro de la tarde podía aparecer con la fecha del día siguiente.
- La política de recuperación de código nombra a Antigravity y aclara que `get_context`, `search_memories` y `search_code` llegan comprimidas desde el servidor, mientras que `get_symbol` devuelve una respuesta compacta pero sin comprimir.
- El hook `expand` reserva tiempo suficiente en equipos lentos y en Windows.
- Las pruebas de contrato leen los archivos compartidos con reintentos, para evitar colisiones de acceso en Windows.

## [v2.44.4] - 2026-11-03

### Fixed

- **ADM-06:** En Antigravity CLI, las llamadas en transcript con comillas dobles externas (`"args":{"Path":"\"...\""}`) ya no impiden la reindexación de archivos `.go` en el cierre de turno (`cleanAgyArg` en `hook_agy.go`).
- **ADM-07:** `InspectAgyEntry` en `agy_setup.go` admite parches semánticos compatibles de la serie `1.4.x`, evitando degradar falsamente `mem doctor` a `StateOutdated` ante actualizaciones menores de `agy`.
- **HCD CLI:** `mem hcd` admite nativamente los clientes `antigravity` y `gemini` en su CLI.
- **CodeGraph en Antigravity:** `ensureCodeGraphMCP` detecta y conecta CodeGraph MCP en `~/.gemini/config/mcp_config.json` usando targets explícitos (`claude,codex,opencode,antigravity`) en vez de `--target auto`, evitando la creación espuria de `~/.gemini/GEMINI.md` y `settings.json`.
- **Diagnóstico Codebase-Memory MCP:** `mem doctor` avisa si falta `codebase-memory-mcp` en el `mcp_config.json` de Antigravity cuando su ejecutable está disponible en el `PATH`.
- **Compresión en Antigravity:** Demostrada la compresión de contexto en sesión real de Antigravity CLI 1.4.12, con generación de referencias `⟦mem⟧` y recuperación íntegra vía `mem pack retrieve`.

## [v2.44.3] - 2026-11-02

### Fixed

- `mem pack retrieve` y `pack_retrieve` devuelven el original aunque la base esté ocupada. Antes, si no podían renovar su caducidad, fallaban con el original ya leído. En ese caso, el original conserva la caducidad que tenía.
- La recuperación de originales por CLI y MCP tiene un plazo propio de 6 segundos, en lugar de 1 segundo.

## [v2.44.2] - 2026-11-02

### Fixed

- Un original guardado como `título: contenido` ya no se retira mientras una memoria viva contenga ese mismo texto.
- Dos planes aprobados con el mismo título ya no se pisan: cada plan tiene una clave derivada de su contenido, y reenviar el mismo plan actualiza su memoria.
- `install.sh --uninstall` avisa y termina con error si no puede borrar un binario. Antes lo daba por no encontrado.
- `mem consolidate` informa las memorias que borró de verdad. Antes, tras un fallo a mitad, informaba el total previsto.

## [v2.44.1] - 2026-11-02

### Fixed

- En agy, las skills de gomemory, incluido `/hcd`, se instalan en `~/.gemini/config/skills`. Ya no dependen de que Claude Code esté instalado.
- `mem install --agents agy` ya no escribe `~/.agents/skills` en `skills.json`, y convierte a rutas absolutas las entradas que empiezan por `~/`. agy 1.4.12 descarta esas entradas y solo lo registra en su log.
- `mem uninstall` retira también las skills de `~/.gemini/config/skills`.

### Known issues

- En agy, el primer prompt sigue nombrando `ToolSearch`, que es una herramienta de Claude Code.
- No se probó `/hcd` en una sesión interactiva de agy.

## [v2.44.0] - 2026-11-02

### Added

- gomemory ya funciona con Antigravity CLI (`agy`). Se activa con `mem install --agents agy` o `mem setup-mcp --scope global --agents agy`, que escriben MCP, hooks y skills en `~/.gemini/config`. Antes de cambiar un archivo guardan una copia, y no tocan la configuración de otras herramientas.
- En agy, los hooks entregan el contexto, aplican las reglas de `expand` y registran lo que se hizo en cada turno. Ya no dependen de `jq` ni de rutas de Claude. Si un hook falla, agy sigue funcionando.
- `mem doctor` muestra agy y avisa si su versión no es la comprobada, 1.4.12. `mem uninstall` quita solo las entradas de gomemory.
- `mem doctor` indica por agente y tipo de contenido qué compresión está instalada, cuál se ejecutó y cuál llegó al modelo.
- `mem pack retrieve --automatic --agent <agente> --session-id <id>` solo devuelve originales a la sesión que los generó. El esquema pasa a la versión 8 con la tabla `automatic_original_grants`. La recuperación manual sigue funcionando por proyecto.
- En Codex 0.166.0, el hook `tool-output codex-shell` puede sustituir resultados de `Bash`. Solo actúa en esa versión y después de confiar en el hook.
- OpenCode envía `session_id` al hook de salidas.

### Changed

- `mem install` reemplaza el grupo manual `gomemory-lifecycle` de agy por el grupo oficial, con copia previa.
- El instalador interactivo acepta `agy` en `--agents`.

### Fixed

- La primera salida reducida tras compactar vuelve a incluir la nota de recuperación.
- `mem pack retrieve` ejecutado por shell devuelve el original completo. Antes, el hook volvía a comprimirlo.

### Known issues

- agy no tiene compresión automática de salidas ni aviso tras compactar, porque `PostToolUse` no recibe la salida y no existe un evento de compactación. Se probó con `agy -p`; faltan la sesión interactiva, los subagentes y las herramientas de edición.
- En agy, el primer prompt nombra `ToolSearch`, que es una herramienta de Claude Code.
- `skills.json` añade `~/.agents/skills`. Las skills que gomemory instala en otras carpetas no se registran con esa entrada.
- HCD-014 sigue abierta. Solo Claude tiene una comparación real: 33200 bytes menos en seis casos sintéticos, que no equivalen a tokens facturados. En Codex, dos ejecuciones no redujeron la salida cuando el shell corría dentro de su herramienta `exec`. OpenCode está sin comparar.

## [v2.43.1] - 2026-11-01

### Fixed

- `mem purge`, `mem gc`, `mem forget` y los borrados de la TUI comparten una sola política: retiran el texto de `memories`, del índice de búsqueda y de páginas libres y WAL de `mem.db`. También retiran de `compression_originals` los originales idénticos a lo borrado, salvo que una memoria viva conserve ese texto; una purga sin filtros vacía su alcance. Un original que solo incluye el texto dentro de otro sobrevive hasta caducar, a los 7 días, y un contenido editado conserva sus copias anteriores hasta entonces, igual que un registro antiguo cuya copia se guardó redactada.
- La garantía no cubre tablas derivadas: sesiones, evidencia HCD, Socrática y `channel_activity` no se modifican.
- Si otra conexión o proceso retiene la base más de 200 ms, el borrado termina con éxito y una advertencia de que el texto sigue en disco; `mem compact` vacía y comprueba también el WAL. Corrupción, E/S o disco lleno siguen siendo un error.
- Una base existente intenta una limpieza única al abrirse con esta versión; si no puede, la repara el siguiente `mem purge`, `mem gc` o `mem compact`. Hasta entonces, `mem forget` no retira restos anteriores.
- Todas las conexiones usan `secure_delete`, y `mem forget` reescribe el índice de búsqueda: su coste crece con el índice, no con el almacén de originales.
- `mem` abre y borra sin error si `memory_search` no es una tabla FTS5.
- `install.sh --uninstall` solo retira binarios con el marcador de gomemory, sin ejecutarlos, y avisa de los que conserva.
- Los envoltorios de Socrática se escriben de forma atómica y rechazan un destino simbólico.
- El ledger ACR incluye el `Claim` en la huella: un reenvío con otro texto en la misma ronda se rechaza.

### Changed

- `mem purge`, `mem gc`, `mem forget`, `mem consolidate` y la tool `forget_memory` salen con éxito y una advertencia cuando el borrado se confirmó y solo queda la limpieza pendiente. Un script que necesite esa garantía debe leer stderr, o `limpieza_pendiente` en el texto de la tool.
- Plugin de OpenCode, fábrica v1: cambio deliberado de comportamiento. El marcador de checkpoint avanza solo tras persistir, un fallo emite `channel-error` y se reintenta, y un idle solapado repasa la sesión al terminar. La referencia de `specs/032-opencode-v2-plugin-compat/baseline/gomemory.v1.ts` se actualizó con este cambio.

## [v2.43.0] - 2026-10-31

### Added

- `mem context --keep-baseline` entrega el baseline universal entero en cada turno; lo usa OpenCode.
- `mem doctor` informa si el plugin de OpenCode instalado está desactualizado.
- `setup-mcp --scope global` nombra los archivos que conservan bloques por no haberse elegido su agente.

### Changed

- gomemory ya no escribe en `CLAUDE.md` ni `AGENTS.md`: `mem setup-mcp --scope global` retira los bloques completos heredados y deja una copia junto a cada archivo.
- El baseline universal llega por contexto como documento fijado, con la plantilla embebida para proyectos ya instalados.
- Protocolo v12: las reglas van antes de la lista de herramientas, porque algunos clientes truncan las instrucciones MCP.
- `mem doctor` solo recomienda un comando cuando puede retirar el bloque sin riesgo; en otro caso pide hacerlo a mano.

### Removed

- El refuerzo de preferencias de los hooks: las preferencias llegan en el contexto de sesión.

### Fixed

- `mem install` y `mem update` ya no retiran archivos `AGENTS.md`, `CLAUDE.md` o `CLAUDE.txt` propios: solo quitan bloques completos, con copia privada en `.memory/backups/agent-files/` sin reemplazar copias anteriores.
- `mem uninstall` ya no pierde el texto posterior a un bloque, no elimina archivos por su título y sale con código 3 si no puede retirar un bloque.
- Un bloque sin marcador de cierre deja el archivo intacto y se informa cómo retirarlo a mano.
- `mem uninstall --all` conserva los binarios fuera de `HOME`, no sigue enlaces fuera de él e inventaría también el baseline sin protocolo. No protege frente a un proceso del mismo usuario que altere esos directorios durante la ejecución.
- `mem hcd start` es atómico bajo concurrencia: sin errores del driver, documentos huérfanos ni intenciones cruzadas.
- `mem import` valida el bundle antes de escribir, no duplica memorias con secretos e informa lo importado si falla a mitad.
- Los recortes por bytes ya no parten caracteres UTF-8; los timestamps HCD corruptos se rechazan, y olvidar una memoria retira también su índice de búsqueda.

### Security

- `mem export` falla si no puede fijar permisos `0600`.
- Los razonamientos de relaciones se limpian de controles de terminal al leerlos.

## [v2.42.1] - 2026-10-29

### Fixed

- Con el nivel `max`, `mem search` podía esconder tras una marca `⟦mem⟧ ×N filas` la memoria cuyo título coincidía exactamente con la consulta, y el agente no recibía su ID. Ahora esa fila sigue siempre visible (sin distinguir mayúsculas ni espacios extremos) y el resto de la tabla se comprime como antes. La opción interna que lo garantiza protege también ese literal en cualquier otro compresor. En el corpus de HCD-002, la calidad pasa de 95/100 a 100/100 observaciones.

## [v2.42.0] - 2026-10-29

### Added

- `pack_build` y `mem pack build` miden el presupuesto sobre la respuesta completa que se entrega (Markdown, JSON o texto MCP). Si el contenido crítico no cabe, devuelven un error explícito en vez de recortarlo, y una consulta sin coincidencias se declara («Sin coincidencias para la consulta») en vez de rellenar el paquete.
- Opción `--receipt` (MCP: `receipt`) en `pack_build` / `mem pack build`: añade el recibo de procedencia (fuentes, huellas, seleccionado u omitido y por qué) y la contabilidad ampliada, que separa selección, compresión, delta y metadatos. Cada item omitido por presupuesto lleva una ref cuyo original se guarda antes de prometerla. Por defecto no viajan.
- `pack_retrieve` acepta `project` y `mem pack retrieve` acepta `--project`, ambos por defecto el proyecto actual.
- `mem doctor` informa el soporte de Codex por mecanismo (salida propia, selección, delta de sesión, salidas externas y compactación) y separa la política local del soporte real del host, que queda sin verificar en vez de extrapolar el rechazo histórico de Codex 0.159.0.

### Changed

- `get_context`, `mem context` y el contexto de arranque entregan un extracto de 80 caracteres por entrada (antes 200), con su puntero `get_memory` al original íntegro. Las reglas fijadas, el HCD activo y los conflictos se siguen entregando completos. En un corpus sintético calibrado con el uso real, reduce un 24,88% los bytes entregados (recuperaciones incluidas), sin pérdida de calidad y con una latencia p95 un 8% menor. El contexto de arranque ahora incluye las decisiones recientes dentro de su tope. Aplica por igual a Claude Code, Codex y OpenCode.

### Fixed

- Una ref de un original comprimido podía recuperarse desde otro proyecto de la misma base. Ahora responde igual que una ref inexistente, sin revelar su contenido, y la deduplicación del almacén de originales distingue el proyecto.

## [v2.41.0] - 2026-10-29

### Added

- HCD (Harness Continual Development): protocolo propio de gomemory para llevar una unidad de trabajo (feature, evolución, corrección o mejora técnica) desde la intención hasta una entrega verificada y cerrada con tu aprobación.
  - **Ceremonia completa**: `define → clarify → approve → slice → tasks → implement → verify → close`, más `analyze` para comprobar la coherencia.
  - **Vía rápida**: el mismo protocolo sin `approve` ni `slice`, para el trabajo ordinario; la única aprobación es la del cierre. Una unidad rápida se promueve a la ceremonia completa sin perder lo hecho.
  - **Diez skills** (`hcd`, `hcd-define`, `hcd-clarify`, `hcd-approve`, `hcd-slice`, `hcd-tasks`, `hcd-implement`, `hcd-verify`, `hcd-analyze`, `hcd-close`) para Claude Code, Codex y OpenCode, que instalan `mem install` y `mem update`.
  - **Motor `mem hcd`** (`start`, `show`, `next`, `clarify`, `approve`, `verify`, `analyze`, `close`, `cancel`, `promote`, con `--json`). El paso se deriva siempre del documento, las aprobaciones y la evidencia.
  - **Un documento por unidad**, `hcd/HCD-NNN-<slug>.md`, que viaja en el PR. SQLite guarda solo el control en tres tablas; las decisiones se reflejan como memorias `type=decision`.
  - **Gates**: una tarea solo está verificada con evidencia vigente del motor, que caduca si cambian el código o las tareas; un `fix` exige observar el rojo (`--expect-fail`) antes del cambio; aprobar liga la cita a la revisión exacta; cerrar se rechaza con hallazgos `CRITICAL`.
  - **Normas transversales**: las skills aplican la Constitución y el Playbook; si cambian después de aprobar la definición, `analyze` lo reporta y el cierre exige reaprobar.
  - **Aviso pre-edición**: en Claude Code, editar código sin unidad activa avisa una vez por sesión, sin bloquear.
  - La TUI («Especificación») crea unidades de cualquiera de los dos carriles, y el contexto de arranque muestra la sección «HCD activo».
- Playbook HCD como documento fijado (`mem playbook`, `mem docs show playbook`), con reglas de la ceremonia, Arquitectura Limpia y SOLID.

### Changed

- Esquema de la base: versión 6. Añade `hcd_units`, `hcd_evidence` y `hcd_approvals`. Las bases v3 migran al abrir, sin tocar las filas existentes.
- Constitución distribuida 2.1.1: la §18 admite los dos enfoques (Spec Kit y HCD) con un documento único por unidad HCD. Las copias sembradas intactas se actualizan solas.

### Medición frente a la primera versión de HCD (no publicada)

| | Primera versión | Esta versión |
|---|---|---|
| Código de producción HCD | 7.010 líneas | 2.415 |
| Tests HCD | 5.527 líneas | 1.751 |
| Tablas `hcd_*` | 16 | 3 |
| Comandos del motor | 28 | 10 |
| Requisitos de la spec | 72 | 29 |

La ceremonia se conserva entera; lo que se redujo es la maquinaria interna.

### Fixed

- La reaprobación de una definición HCD se rechaza si faltan secciones obligatorias o hay aclaraciones abiertas.
- Los marcadores de aclaración en historias, slices o tareas también bloquean la aprobación.

### Documentación

- Se actualizan las guías SDD_HCD y técnica a gomemory 2.41, y se conserva solo la edición vigente.
- `docs/MANUAL.md` suma la sección 22 (HCD). ADR 0002 (estado de control en SQLite) y ADR 0003 (retirada del catálogo HCD de desarrollo).

## [Unreleased]

## [v2.40.3] - 2026-10-27

### Fixed

- OpenCode 2.x: la política de delegación Octopus y el protocolo de memoria nombran las tools como las ve Code Mode, dentro de la tool `execute` (`tools.gomemory.<tool>`, `tools["codebase-memory-mcp"].<tool>`). Antes daban el nombre de 1.x (`gomemory_<tool>`) y el agente recibía `Unknown tool`. El protocolo indica también cómo llamarlas si el servidor tiene `codemode: false`.
- La política Octopus lista las claves exactas del schema de `octopus_route_task` (`objective` obligatoria, `task_class`, `complexity`, `risk`, `files`, `dependencies`, `read_only`, `context_tokens` y `subagents`). Descritas en prosa, el agente inventaba claves como `goal` o `budget_context`, y la tool las rechazaba por faltar `objective`.
- En OpenCode 2.x la política indica que la tool `subagent` exige `agent` (por ejemplo `general`), `description` y `prompt`.
- Octopus ya no marca como trabajo duplicado cualquier objetivo cuando el proyecto tiene una memoria extensa, como una constitución. Los términos del objetivo deben coincidir dentro de un pasaje de 64 tokens significativos; una memoria larga que sí contiene el objetivo sigue contando.
- Octopus expande los globs del alcance (`files: ["app/routes/*.py"]`). Antes se leían como rutas literales, el contexto medido se quedaba en el texto del objetivo y la delegación quedaba vetada por debajo del mínimo delegable.

### Tests

- Pruebas de la política Octopus por runtime y contra el schema real, del protocolo y la política en el camino v2 del plugin, de la cobertura por pasaje de `trabajoCubierto` y de la expansión de globs en `leerAlcance`.

### Documentación

- `docs/MANUAL.md` describe los nombres de tool en Code Mode de OpenCode 2.x y cómo mide Octopus el alcance y el trabajo duplicado.
- Las guías vigentes (`docs/*-v2.40.html`) indican gomemory 2.40.3.

## [v2.40.2] - 2026-10-26

### Changed

- Las entradas de `mem socratica` y de la tool MCP `socratica_flow` viven en su propio paquete (`adapters/primary/cli/socratica`), igual que Review. El paquete `cli` raíz conserva solo el envoltorio `CmdSocratica`, que mantiene los códigos de salida, y registra la tool con `socratica.RegisterTools`. La salida no cambia.
- La base compartida `adapters/primary/cli/clikit` incorpora la lectura acotada de entradas (`ReadBounded`, `ErrPayloadTooLarge`) y el selector de harness de Socratica (`SelectSocraticHarness`), que ahora usa también `mem expand evolve`.

### Tests

- Las pruebas de Socratica (CLI, `specify`, selector de harness, salida segura en terminal y stdin acotado) se mueven al paquete del slice; las de lectura acotada y del selector, a `clikit`.

### Documentación

- `specs/044-cli-socratica-slice` recoge la especificación, el plan, las tareas y la evidencia de la extracción. `docs/architecture.md` describe los slices `review` y `socratica`, sus reglas de dependencia y el árbol de `cli/`.
- Las guías vigentes (`docs/*-v2.40.html`) indican gomemory 2.40.2.

## [v2.40.1] - 2026-10-25

### Fixed

- `mem socratica pending --cancel <id>` con un identificador inexistente dice que el flujo no existe, en vez de mostrar el error interno `sql: no rows in result set`. `--resume` usa el mismo mensaje y ya no presenta un fallo de la base como «no existe».
- Un flujo de Socratica aprobado que no llegó a ejecutarse ya no ofrece «Editar» en la TUI ni en la CLI: el dominio solo admite editar en revisión y el intento terminaba en error.
- La TUI permite cancelar una reserva `specify` ya enviada cuyo contenido privado no está en esta instancia. Antes anunciaba «x Cancelar» pero rechazaba la tecla, aunque el dominio y la CLI sí lo permitían.

### Changed

- Las acciones que admite cada estado de un flujo de Socratica se deciden en un único caso de uso (`LocalActions`). La TUI y los menús de la CLI solo las presentan, cada uno con sus teclas y textos de siempre.
- `review_status` y `mem review show`/`status` obtienen el linaje de una revisión del mismo caso de uso (`BuildReviewStatus`). La salida no cambia.
- Las entradas de `mem review` y de las tools `review_*` viven en su propio paquete (`adapters/primary/cli/review`), con una base compartida en `adapters/primary/cli/clikit`. La salida no cambia.

### Tests

- Nueva prueba de contrato de las reglas de capas de la arquitectura hexagonal: `domain` sin dependencias internas, `application` solo de `domain` y sus ports, y cruces de adaptador primario a secundario limitados a una lista explícita que solo puede encoger. Corre en `go test ./...`.
- Prueba de paridad entre las acciones locales de Socratica y las transiciones que acepta el dominio.

### Documentación

- `specs/043-vertical-slices-assessment` recoge la evaluación de la base Go para desarrollar por vertical slices y su roadmap. `docs/architecture.md` refleja el paquete nuevo de Review.
- Las guías vigentes (`docs/*-v2.40.html`) indican gomemory 2.40.1.

## [v2.40.0] - 2026-10-24

### Added

- Socratica tiene un modo nuevo, `specify`, que prepara una idea, una petición o una mejora técnica como entrada de `speckit-specify`. El agente completa por etapas la intención, el alcance (incluido, excluido, restricciones, supuestos y pendientes) y las historias con sus criterios y entregas. gomemory comprueba cada etapa en orden, genera el texto final y lo ata a la revisión que la persona aprueba con «Aprobar y generar especificación». Solo el primer `ack` confirmado concede la ejecución (`execution_granted=true`). Con esa concesión, el agente invoca `speckit-specify` una vez con el texto aprobado y comunica el resultado con `report`. Socratica no escribe la especificación ni ejecuta la necesidad.
- Se invoca con `/socratica-hu <texto>` en Claude Code y OpenCode, y con `$socratica prepara para speckit-specify: <texto>` en Codex. `mem install`, `mem setup-mcp --scope global` y `mem update` instalan el envoltorio `socratica-hu`.
- Modo privado para `specify`: el contenido vive solo en la memoria del proceso MCP que inició el flujo, y la base guarda una reserva sin texto. Otra instancia solo puede consultar o cancelar esa reserva.
- `mem socratica pending` y la TUI muestran la etapa, la revisión, las preguntas usadas y lo que falta de un flujo `specify`. Solo permiten cancelarlo.

### Changed

- El catálogo de la base pasa a la versión 3. La migración es aditiva: `preparation_json` en flujos e historial y `payload_policy` en flujos. Un `ALTER` que falla ahora aborta la migración, en vez de dejar `user_version` marcado sobre un esquema incompleto. Esto vale para todas las columnas añadidas por migración.
- Un flujo `specify` conserva la política con la que se creó: activar el modo privado después no lo redacta al enviarse, y su resultado se puede seguir comunicando.
- Una actualización concurrente de un flujo se informa como `revision_conflict`, la misma categoría que una revisión vieja.

### Fixed

- Desactivar auto-approve vuelve a retirar la lista `autoApprove` de `.mcp.json` y `.cursor/mcp.json`. Una salida anticipada hacía inalcanzable ese camino.
- `mem settings`, `mem install` y la TUI informan cuando no pueden aplicar auto-approve (JSON ilegible o escritura fallida), en vez de dar un falso éxito.
- Reenviar un flujo ya enviado mientras hay otro pendiente para el mismo agente devuelve un error claro, en vez de `UNIQUE constraint failed` de SQLite.
- Cancelar un flujo con una preparación de una versión más nueva la conserva intacta, campos desconocidos incluidos.
- `GOMEMORY_NO_CODE_REFRESH=1` apaga el refresco en segundo plano del snapshot del grafo de código externo (`mem code-refresh`). Las suites de integración y contrato lo activan: ese proceso seguía escribiendo en `.memory` después del comando y, en Windows, impedía borrar el directorio temporal de la prueba.

### Seguridad

- Auto-approve reescribe la configuración MCP de forma atómica y conserva su modo, en lugar de forzar 0644. Rechaza un `.mcp.json` o un `.cursor/` enlazados dentro del proyecto, para no sustituir el enlace ni escribir fuera del repositorio. La regla de enlaces vive ahora en `adapters/secondary/configpath`, compartida con `setup`.

### Documentación

- `docs/socratica.md` documenta el modo `specify`: recorrido, preguntas, revisión, entrega única, recuperación y privacidad. `docs/DATABASE.md` documenta el catálogo 3, y `MANUAL.md` y `architecture.md` recogen los cambios de este release.
- Nueva edición de las guías: SDD, edición 7, y técnica, edición 5 (`docs/*-v2.40.html`); reemplaza las guías v2.39.

## [v2.39.0] - 2026-10-24

### Changed

- Los instincts llegan al contexto como patrones, no como la muestra de una ejecución: `` `go test` · 40 usos · último 2026-10-24 (0.8) ``. La clave sale del primer comando significativo, así que salta asignaciones `VAR=valor`, `sudo` con sus opciones, `cd` y los comandos triviales de lectura (`cat`, `sed`, `grep`, `rg`, `ls`, `head`, `tail`, `find`, `echo`, `wc`, `pwd`, `sqlite3` y `python3 -c`). Deja de inyectar lo que lleva 30 días sin observarse y las filas guardadas con el criterio anterior, sin borrar datos. Medido sobre una base real: de 8 líneas y unos 1.400 caracteres a 5 líneas y 253.

### Fixed

- Un proyecto con la raíz bajo el directorio temporal del sistema (`mktemp -d`) ya no cuenta como segundo proyecto para promover un instinct a global. Sigue leyendo los instincts globales.
- `sudo -u app git commit` y las demás formas de `sudo` con opciones se reconocen como commit, así que el recordatorio de `verify-loop` vuelve a salir.

### Seguridad

- `mem update` limita lo que acepta de la red, con los mismos topes que el instalador: 128 MiB el archivo, 256 MiB lo expandido y 1 MiB los metadatos y `checksums.txt`. En un tar.gz cuentan también las entradas que no son el binario, porque se descomprimen al saltarlas. Superar un tope aborta sin tocar el binario instalado.
- Los generadores de Socratica (claude, codex y opencode) acotan su salida a 8 MiB en stdout y 64 KiB en stderr. Al superarla, el proceso se cancela al momento en lugar de esperar al timeout, y la generación falla como respuesta inválida.

### Documentación

- El diagrama «How it works» y la tabla de agentes del README dejan de mostrar Cursor, que ya no se usa en el proyecto. El código lo sigue soportando y `INSTALLATION.md` lo documenta.
- Nueva edición de las guías: SDD, edición 6, y técnica, edición 4 (`docs/*-v2.39.html`); reemplaza las guías v2.38. El manual, la arquitectura, `expand.md`, `socratica.md` y `DATABASE.md` recogen los cambios de este release.

## [v2.38.0] - 2026-10-23

### Removed

- Se retira el soporte de Windsurf y Cline. `mem install` deja de ofrecerlos, y `mem setup-mcp` ya no los acepta en `--agents` ni los configura con `all`, así que nunca crea `.windsurf/` ni `.cline/`. Una selección de agentes guardada por una versión anterior los descarta al reinstalar. `mem uninstall` sigue retirando la entrada de gomemory que una versión anterior les escribió. Los agentes soportados son Claude Code, Codex, OpenCode y Cursor.

### Documentación

- El README se reorganiza en torno al problema, el inicio rápido y el funcionamiento, con un diagrama Mermaid de la arquitectura actual y un logo transparente para los temas claro y oscuro. Corrige además una afirmación: el proyecto sí recibe una carpeta `.memory/`, que `mem install` añade a `.gitignore`.
- Nueva edición de las guías: SDD, edición 5, y técnica, edición 3 (`docs/*-v2.38.html`); reemplaza las guías v2.37. `INSTALLATION.md`, el manual, la arquitectura y el protocolo de memoria ya no mencionan Windsurf ni Cline como soportados.

## [v2.37.0] - 2026-10-23

### Changed

- Prepara la base de memoria en una sola transacción y registra su versión: un fallo revierte la preparación y una base al día omite el DDL sin esperar a otro escritor.
- Unifica la configuración de las conexiones SQLite: WAL con `synchronous=NORMAL`, caché bajo demanda, temporales en memoria, lectura mapeada y un límite para el WAL retenido. Añade el índice de sesiones y actualiza las estadísticas al cerrar, fuera de los hooks. Ante un corte de energía, NORMAL puede perder la última escritura confirmada; si solo cae el proceso, se conserva.
- Medido en Windows CI, con 20 repeticiones por prueba: abrir una base nueva pasa de 554 ms p50 / 1.760 ms máx a 40 / 219 ms; abrir una base preparada, de 6 a 3 ms p50; el plegado del primer turno, de 581 / 1.809 ms a 87,5 / 264 ms. La actualización de estadísticas añade 2 ms como máximo al cierre.

### Removed

- Se retira el instalador de PowerShell (`scripts/install.ps1`) y su prueba de humo. Windows PowerShell 5.1 no lo parseaba (le faltaba el BOM UTF-8), y la prueba aprobaba sus casos de rechazo sin ejecutarlo. En Windows se instala el binario del release: se descarga `mem_windows_amd64.zip`, se verifica con `checksums.txt` y se deja `mem.exe` en el PATH (comandos en `INSTALLATION.md`). El CI ejecuta esa descarga contra el último release. `mem update` sigue actualizando en Windows.

### Fixed

- Los snapshots automáticos ya no se sobrescriben cuando el reloj repite la marca de tiempo entre dos seguidos, algo habitual en Windows. Cada snapshot se crea en exclusiva y la poda retira siempre el más antiguo.

### Documentación

- Nueva edición de las guías: SDD, edición 4, y técnica, edición 2 (`docs/*-v2.37.html`); reemplaza las guías v2.36. La guía técnica describe la preparación versionada de la base; las dos documentan la instalación en Windows con el binario del release.

## [v2.36.4] - 2026-10-23

### Documentación

- Reduce el README a una guía de entrada y traslada la referencia completa de ajustes a `docs/CONFIGURATION.md`.
- Quita el badge de versión estático y el número de herramientas del badge MCP; el README conserva el badge dinámico de release y documenta el conteo junto a su fuente en `domain/mcp_tools.go`.
- Añade a CONTRIBUTING las instrucciones para medir la cobertura global, incluidos los comandos ejecutados por pruebas de contrato.

## [v2.36.3] - 2026-10-22

### Seguridad

- `install`, `update`, `setup-mcp` y `uninstall` dejan de seguir enlaces simbólicos en la configuración del proyecto: `.claude/settings.json`, `.mcp.json`, `opencode.json`, los MCP de Cursor, Windsurf y Cline, los archivos de instrucciones, `.gitignore`, `.specify/`, los plugins y los envoltorios. Un enlace hacía que gomemory escribiera, o borrara con `.claude/plugins` enlazado, un archivo fuera del repositorio, incluida la configuración global de Claude. Ahora el paso se omite con un aviso. En el ámbito de usuario el enlace se sigue y se conserva, por si viene de un repositorio de dotfiles.
- Reescribir un archivo de configuración conserva su modo: un `settings.json` con `0600` ya no pasa a `0644`.

### Corregido

- Windows:
  - `mem install` no instalaba los plugins de Claude Code y OpenCode ni la extensión de Spec Kit, porque leía los archivos embebidos con rutas `\`.
  - Las escrituras de estado que hacen los hooks en paralelo (conversación, avisos, cola de expand, snapshots del grafo) podían fallar con «Access is denied»: ahora reintentan y los lectores no bloquean el reemplazo.
  - `mem update` deja de pedir un `move` manual: publica el binario nuevo y, si el actual está en ejecución, lo aparta como `mem.exe.old` para retirarlo en la siguiente actualización. Desde una copia local ahora encuentra el binario global, que antes no veía.
  - `mem uninstall` aparta el binario en uso antes de borrarlo, así su ubicación queda libre en el acto.
  - `mem migrate --force` ya no falla al descartar el almacén previo.
  - Las anclas de memoria con raíz sin unidad (`/otro/x.go`) se aceptaban como del proyecto.
  - Shield marcaba como «fuera del home» hooks situados en un directorio del PATH.
- `mem doctor` evaluaba la salud de los canales con horas de desfase en cualquier zona horaria distinta de UTC−5.
- La compresión de salidas descartaba las líneas `FAIL` de los logs con prefijo (`gh run view --log`, `docker compose logs`) y las aserciones de cada test fallido. En un log real de CI se pasa de 6 a 275 líneas de error visibles, de 275, y de 0 a 282 aserciones, de 285.
- `mem usage` aclara que su porcentaje mide solo el contexto que entrega gomemory, no las salidas de herramientas ni el resto de la conversación.
- El guardarraíl de shell tardaba 1–2 s en bloquear una línea con miles de sustituciones anidadas; ahora la bloquea en milisegundos.

### Notas

- El checkout usa LF en todos los sistemas (`.gitattributes`), porque las plantillas embebidas se identifican por su SHA-256.
- La suite de Windows pasa de unos 250 fallos a 10. Los tests que dependen de scripts de shell se omiten en Windows y dicen por qué.

## [v2.36.2] - 2026-10-22

### Corregido

- `mem setup-mcp --scope global` ahora retira del proyecto del directorio actual los hooks de Claude Code que el global pasa a cubrir. Si un proyecto se instalaba antes que el global, sus hooks quedaban registrados en los dos ámbitos y Claude Code los ejecutaba dos veces por evento. Los demás proyectos se corrigen con `mem update`.
- La deduplicación de hooks entre ámbitos compara el subcomando completo. Antes, todos los hooks de `mem hook expand` compartían la clave `expand`, y un global con solo parte de ellos (por ejemplo, `expand stop`) hacía que el proyecto omitiera también los demás, como `expand pre-tool`, y perdiera sus protecciones.

## [v2.36.1] - 2026-10-22

### Seguridad

- Los binarios se compilan con Go 1.27.3, que corrige GO-2026-6809 (certificados intermedios comprometidos en `crypto/x509`). El toolchain se fija en `go.mod`, y el análisis `govulncheck` y GoReleaser leen esa misma versión en el workflow de release.

### Notas

- `v2.36.0` no publicó binarios: su release se detuvo en `govulncheck`. Instala `v2.36.1`, que incluye todos los cambios de `v2.36.0`.

## [v2.36.0] - 2026-10-22

### Añadido

- `mem expand`: reglas de protección, shield de configuración, cinco skills de flujo e instincts con confianza, promoción global y confirmación como preferencia.
- `mem doctor --shield` con informe redactado, archivos omitidos y códigos por severidad; `evolve` con selector de harness y aprobación previa a la instalación.
- Hooks de expand en Claude Code, Codex y OpenCode, con router determinista, interruptores, fallo abierto y observación silenciosa.

### Cambiado

- Protocolo de integración v10 con instrucciones de expand; actualización idempotente y limpieza de sus hooks y skills al desinstalar.
- `mem sanitize` cubre también estadísticas de instincts, registro global y cola transitoria.
- Skills reinterpretadas de ECC (affaan-m/ECC, MIT), con atribución en NOTICE y sin nuevas dependencias de ejecución.
- Las skills de usuario para OpenCode se instalan en `~/.config/opencode/skills`, el directorio global que OpenCode 2.x descubre siempre; al reinstalar se retira la copia de `~/.opencode/skills` solo si es idéntica y está sola en su directorio (una personalizada se conserva), y la desinstalación limpia ambas rutas.
- `mem install` entrega las cinco skills de expand también con alcance proyecto.

### Corregido

- `destructive-shell` reconoce rutas absolutas (`/bin/rm`), `sudo`/`env`/`command`/`nohup`/`nice` con sus opciones y comandos anidados en `sh|bash|zsh -c`; `git restore --staged .` ya no se bloquea.
- El plegado de instincts se ejecuta tras escribir la decisión del hook, con plazo propio, y registra `expand.timeout` o `expand.error` en vez de fallar en silencio.
- Un `instincts.db` que no abre ya no impide guardar ni leer memorias: `mem doctor` informa la degradación.
- `mem expand status|rules` y `mem doctor --shield` depuran controles de terminal en nombres de regla e informes de shield (también al leer el informe guardado); `status` muestra solo un resumen del shield.
- Shield no marca como credencial `{env:VAR}`, `{file:…}`, `%VAR%` ni `${VAR}`, y sus números de línea corresponden al archivo original.
- `destructive-shell` evalúa también el comando que sigue a un `&` de segundo plano.
- La cola de observaciones serializa inserciones, retiradas, rotación y saneamiento con un candado entre procesos: un hook concurrente ya no pierde observaciones.
- En OpenCode, las reglas `stop` reciben el texto de la última respuesta del asistente.
- Una propuesta global de instincts puede confirmarse o rechazarse en un proyecto nuevo.
- Las claves de instincts se derivan del comando protegido; `mem sanitize` reduce y fusiona los slugs antiguos con credenciales en la cola, el proyecto y el registro global.
- El saneamiento de `instincts.db` informa `SANITIZE_BUSY` cuando el checkpoint no puede vaciar el WAL, en vez de declarar éxito.
- `destructive-shell`: una redirección al inicio (`>/dev/null rm -rf x`) ya no oculta el ejecutable, y las sustituciones `$(…)` y `` `…` `` del cuerpo de un heredoc con delimitador sin citar se analizan (auditoría defensiva 2026-10-22, H-01 y H-02).
- `destructive-shell` calcula la salida real de `printf` (formatos y escapes) y de `echo` en sustituciones y tuberías: `$(printf '%s%s' r m) -rf x` se reconoce como `rm`, y un `printf` no resoluble que acabaría como comando se bloquea (ACR acr_1a0a68c0, B-001).
- `destructive-shell` evalúa las sustituciones antes de su comando y con las asignaciones vigentes en ese momento, resuelve las pegadas a otro texto (`r$(printf m)`), no analiza los comentarios `# …` y no toma una sustitución desconocida (`sh $(true)`) por un archivo de script; un pre-tool de más de 8 MB cuya herramienta no puede identificarse antes del corte se bloquea, con independencia del orden de las claves JSON (ACR acr_a635820b, C-001 a C-004).
- `mem expand --help` muestra la ayuda; el contador de herramientas de expand no pierde incrementos con hooks en paralelo; suprimir un bloqueo repetido de `stop` conserva el registro de los avisos.

### Documentación

- Guía SDD, edición 3 (Spec Kit 1.2.2, gomemory 2.36), y nueva guía técnica con la misma plantilla; cada release actualiza las dos guías vigentes.
- `expand.md`, `architecture.md` y `MANUAL.md` describen las supertools y los nuevos límites del guardarraíl de shell.

## [v2.35.0] - 2026-10-17

### Added

- Socratica can select Claude, Codex or OpenCode per flow in the CLI and TUI.
  OpenCode runs in an isolated, tool-less temporary workspace; Claude and Codex
  use schema-adapted, bounded subprocesses. Failed generations now offer an
  explicit retry, harness switch, assisted continuation, original prompt or
  cancellation path.
- Interactive console panels and safe selections for Socratica commands, plus
  terminal-control filtering for generated and stored text rendered on screen.
- `socratica_flow` to the MCP core surface (31 tools; 35 when Octopus is
  enabled), with persisted flow recovery, dispatch acknowledgement and
  project-scoped operations.
- Defensive-audit evidence and four read-only multilingual defensive audit
  reports for the current release scope.

### Changed

- The TypeScript installer limits streamed downloads even when `Content-Length`
  is absent, verifies checksums before extraction and replaces the binary
  atomically; the Unix installer applies the same bounded-download policy.
- Socratica privacy maintenance protects scalar and JSON flow/history fields,
  and dispatch uses a per-flow file lock with orphan cleanup.

### Fixed

- Terminal output from review, session and Socratica paths no longer permits
  stored or model-supplied control sequences to affect an interactive terminal.
- Generator timeouts, OpenCode availability checks and CLI/TUI hand-off paths
  fail predictably without leaving progress rendering or temporary workspaces
  behind.

## [v2.34.0] - 2026-10-16

### Added

- **Socratica** (feature 039): skill `socratica` with two modes, `/socratica` (refine a prompt) and `/socratic-q` (examine a proposal), at most three questions and explicit approval before anything runs. Distributed to Claude Code, Codex and OpenCode with thin command wrappers, removed by `mem uninstall`.
- MCP tool `socratica_flow` (single tool with `action`): persisted flow, duplicate-dispatch prevention (`approve` → `ack` with `ack_nonce` before executing), recovery of uncertain dispatches by explicit choice, bounded and privacy-protected history.
- `mem socratica refine|history|on|off|status|pending|private`, with question generation through the installed `claude`/`codex` in tool-less or read-only mode, and TUI screens (`p`).
- Opt-in automatic interception in the prompt hooks and the OpenCode plugin (`mem hook socratica-decide`); capabilities stay off until acceptance evidence exists (`--unverified` only to gather it).
- Memories saved during a Socratica turn take neither that prompt nor the previous turn's as their origin.

### Fixed

- Auditoría defensiva 2026-10-16 (H-01): `sessions.last_prompt` entra en
  `mem sanitize` como `SanitizeSessionPrompt`, con redacción al leer igual que
  `summary`; las filas anteriores a la redacción al escribir ya no quedan
  fuera del saneamiento.
- Auditoría defensiva 2026-10-16 (H-02): las mutaciones socratica
  (`ReleaseDispatch`, `MarkDispatched`, `ForceResend`, `DeleteFlow`,
  `RedactFlow`) verifican fila afectada (`ErrSocraticaNoMatch`) en vez de
  devolver éxito en vacío.
- Auditoría defensiva 2026-10-16 (H-03): `mem socratica refine -` lee stdin
  acotado (`readBounded`) y la cola del transcript usa lectura completa
  (`io.ReadFull`), sin truncados silenciosos.
- Auditoría defensiva 2026-10-16 (H-04/H-05/H-06): `reviews.target_scope`
  cubierto por el ledger, endurecimiento de permisos WAL/SHM, lock de dispatch
  con `O_NOFOLLOW` y verificación tras apertura. ACR posterior: 0 confirmadas
  en el árbol actual.

## [v2.33.1] - 2026-10-12

### Fixed

- Ledger de revisión: los lectores (`findings`, `consensus_findings`,
  `rejudgments`, `fix_rounds`) aplican `domain.ProtectText` al leer, como ya
  hacían sesiones y relaciones, y `mem sanitize --apply` cubre sus columnas
  (incluidas listas JSON, protegidas elemento a elemento). Cierra el hueco de
  la auditoría defensiva 2026-10-12 (H-01): filas anteriores a la redacción al
  escribir ya no se reemiten en crudo.
- Entrada acotada: los hooks leen stdin con tope de 8 MB (`readBounded`, error
  limpio en vez de contenido truncado) y `mem pack` con tope de 32 MB (H-02).

### Security

- Acciones de CI fijadas por SHA con comentario de versión y Dependabot
  semanal para `github-actions` (H-03).

## [v2.33.0] - 2026-10-12

### Added

- Skill `defensive-qa-audit`: auditoría defensiva multilenguaje (Go, Python,
  JS/TS, Java, PHP, Rust, Shell, SQL, CI, HTML/CSS) con prevención de fugas,
  evidencia reproducible con datos sintéticos, propuesta de cierre por hallazgo
  y tabla de cobertura. Integra memoria previa, grafo de código y derivación a
  la revisión adversarial cuando gomemory está conectado. Se instala y actualiza
  como `adversarial-consensus-review` (alcance global de `mem install`,
  `mem setup-mcp --scope global`, `mem update`) y la desinstalación de sistema
  la retira. Copia descargable en `skills/defensive-qa-audit/`.
- Prueba de sincronía entre `skills/` e `infrastructure/templates/` para las
  skills distribuidas.
- Directiva canónica `domain.CodeRetrievalPolicy` (feature 038): prioridad de
  recuperación según capacidades del agente, compartida entre el bloque de
  instalación y los recordatorios de sesión para que no diverjan.
- Puerta CI `evidence038`: verifica `pyproject.toml` y `uv.lock`, compara el
  export con hashes contra `scripts/requirements-038-test.txt`, ejecuta la
  suite pytest y la auditoría `pip_audit`.

### Changed

- Protocolo de memoria sube a v9: reemplaza la obligación incondicional del
  grafo externo por precedencia condicional según capacidad y suficiencia,
  con salvaguardas (sin inferir ahorro por nombre de herramienta, sin asumir
  cuerpos ni compresión en `get_symbol`, sin prometer compresión automática).
- Migración v8→v9 en `mem install`/`mem update`: reemplaza el bloque anterior
  conservando contenido ajeno, delimitador final e idempotencia.
- `mem install` ya no crea `AGENTS.md`/`CLAUDE.md` en la raíz del proyecto;
  el bloque viaja por MCP initialize.
- `.gitignore` excluye `.venv/`.

## [v2.32.0] - 2026-10-11

### Added

- Comando `mem sanitize` (vista previa por defecto, `--apply` para sanear):
  reescribe resúmenes, razonamientos y diagnósticos históricos con contenido
  protegido, elimina originales comprimidos no recuperables, regenera
  `.memory/context.md` y compacta la base con verificación. Informe depurado
  en `maintenance/privacy/<run_id>.json` con códigos estables
  (`SANITIZE_BUSY`, `SANITIZE_PENDING_CLEANUP`, `SANITIZE_TIMEOUT`,
  `SANITIZE_FAILED`).
- Workflow CI `test` en cada push y pull request: `go vet`, suite completa,
  detector de carreras en las rutas afectadas y umbral de cobertura del 80%.

### Changed

- Protección uniforme de texto (`ProtectText`): sesiones, relaciones,
  diagnósticos de canal, contexto, exportaciones y recuperaciones de
  originales eliminan regiones `<private>` (también anidadas o sin cierre)
  y ocultan credenciales reconocidas al escribir y al leer.
- Instaladores (`install.sh`/`install.ps1`): resuelven la versión una sola vez
  a un tag estable y verifican el SHA-256 de `checksums.txt` antes de
  extraer; cualquier fallo deja intacto el binario instalado.
- `mem update` en Linux/macOS prepara el binario completo junto al actual y
  lo publica con un único renombrado atómico; en Windows deja `mem.exe.new`
  con instrucciones para completar el cambio.
- `mem wrap` informa `WRAP_START_FAILED` sin pedir guardado cuando el comando
  nunca llegó a ejecutarse, sin filtrar argumentos ni el error crudo.
- `Builder.Build` trabaja sobre una copia local con el contador de descartes
  a cero: llamadas repetidas o concurrentes del servidor MCP ya no acumulan
  ni mezclan la línea base de tokens.

### Fixed

- `mem compare` muestra el razonamiento guardado (protegido) en vez del texto
  crudo tecleado; `mem session end` muestra el resumen guardado.
- La lectura de `channel_activity.last_error` y del último prompt de sesión
  ya no reemite contenido protegido de filas anteriores al cambio.

## [v2.31.0] - 2026-10-08

### Added

- Índice compacto de ayuda por flujos (`mem` sin argumentos) con detalle por
  familia en `mem help <comando>`.
- Flujo de instalación con pasos planificados, resumen final y siguiente
  acción en `mem index`, reportes de estado e instalación nativa.
- Autodetección de CodeGraph en el PATH con `code_graph_providers` vacío;
  `mem install` conecta su servidor MCP a los agentes que aún no lo tengan
  (con comprobación previa de identidad) y `mem doctor` informa en qué
  agentes está conectado.
- Puerta visual `scripts/visual-smoke.py`: verifica presentación por PTY
  (temas × anchos × comandos) e instalación real en un HOME aislado.

### Changed

- Listados y reportes (`search`, `list`, `compare`, `docs`, `usage`,
  `doctor`, `octopus`, revisiones, ADR) se presentan en paneles en terminal
  humana y conservan su texto plano byte a byte con salida redirigida o
  `--json`.
- La detección del MCP de CodeGraph interpreta el TOML/JSON real por agente
  (incluido el registro de Claude a nivel proyecto) en vez de
  subcadenas, y exige una entrada de servidor válida.

### Fixed

- Los flags inválidos terminan con código 2 en todos los comandos (antes
  éxito silencioso salvo en `install`); los comandos sin opciones rechazan
  argumentos con código 2.
- La cabecera con el logo ya no depende del contenido de la búsqueda y
  `--json`/`--format` la suprimen siempre.
- Cancelar una instalación detiene todo el árbol de procesos (grupo propio,
  vigilancia del padre, paridad Windows con `taskkill`): sin instaladores
  huérfanos que sigan mutando el proyecto.
- `GOMEMORY_THEME` se normaliza igual en Go y TypeScript, con casos
  compartidos verificados (`assets/theme-resolution-cases.json`).

## [v2.30.0] - 2026-10-06

### Added

- Instalación nativa en terminales ricas: `mem install` muestra un flujo en
  vivo con resumen final y `mem install --events` emite NDJSON v1 (`start`,
  `step`, `complete`) por stdout, con los registros humanos por stderr.
- Instalador TypeScript opcional (`installer/`) que consume el contrato
  NDJSON sin interpretar texto, con paridad de temas verificada contra los
  mismos casos que Go (`assets/theme-resolution-cases.json`).
- Ayuda por familias con `mem help <comando>` y `mem --no-motion <comando>`
  para desactivar animaciones conservando los colores.
- Presentación compartida de consola (jerarquía, espaciado y tablas que se
  vuelven verticales en pantallas estrechas) en ayuda, listados, sesiones,
  uso y reportes del instalador.

### Fixed

- La cabecera con el logo ya no depende del contenido de la búsqueda:
  `mem search json` la muestra igual que cualquier otra consulta y `--json`
  la suprime siempre, incluso tras `--`.
- `GOMEMORY_THEME` se normaliza (mayúsculas y espacios) y `auto` u otros
  valores infieren el tema desde `COLORFGBG`, igual en Go y TypeScript.
- `CI=1` fuerza modo no interactivo aunque haya TTY, para que la instalación
  nunca bloquee un pipeline esperando una pregunta.

## [v2.29.0] - 2026-10-05

### Added

- Identidad visual de goMemory en la consola y los instaladores, con un logo
  embebido basado en los assets oficiales y progreso visible durante `mem index`.
- Tres temas para la TUI: goMemory oscuro, goMemory claro y Matrix. En
  Configuración, `t` o Enter sobre «Tema visual» cambia el tema al instante y
  guarda la elección por proyecto.
- Selección de apariencia con `GOMEMORY_THEME` y detección opcional mediante
  `COLORFGBG`. La consola respeta `NO_COLOR`, CI y la salida redirigida.
- Variantes SVG y WebP del logo y del indicador de carga.

### Changed

- La TUI adapta marcos, formularios, selección y acciones al tamaño de la
  terminal. Las pantallas largas permiten desplazarse con PgUp/PgDown.
- La documentación describe la configuración de temas, el indexado de todos
  los proveedores externos y el comportamiento actual de los hooks.

### Fixed

- `mem index` inicializa CodeGraph cuando el proyecto aún no tiene índice;
  antes ejecutaba `index` directamente y CodeGraph requería un `init` manual.
  Los errores conservan la salida del proveedor y se valida el proyecto del índice.
- Los sondeos de CodeGraph tienen su propio plazo: el `status` final ya no
  hereda el tiempo restante del comando de indexado. Se conserva la cancelación
  del llamador.
- Los bordes de la TUI ya no desbordan por el padding de las filas o por títulos
  con caracteres de varias columnas. Un presupuesto de altura cero ya no
  muestra toda la lista.
- La lista se recorta una sola vez y las pantallas secundarias declaran su pie
  de acciones, sin deducirlo de textos como `esc` o de líneas de borde.
- El test del orden de filas de Configuración incluye el selector de tema.

## [v2.28.0] - 2026-10-04

### Fixed

- **Claude Code ya no pierde las instrucciones de gomemory** (feature 035). El
  arranque de sesión (17 777 caracteres) y el primer prompt en modo plan
  (12 944) superaban el tope de 10 000 del canal de hooks, y el modelo solo
  veía una vista previa de 2 KB: perdía el protocolo y las tools. Ahora toda
  salida se ajusta por prioridad y declara lo recortado.
- **Los hooks duplicados dejan de procesar dos veces el mismo evento.** Con los
  hooks en `~/.claude/settings.json` y en el proyecto, cada evento corría dos
  veces. `mem install` registra en el proyecto solo lo que el global no cubre,
  `mem update` retira los duplicados (también ya actualizado) y una segunda
  invocación paralela sale en silencio.
- **La compresión de salidas ya no destruye** código, resultados ni
  diagnósticos: un `sed` sobre Go llegaba sin indentación y con sentencias
  cambiadas por marcas, y una búsqueda de 30 resultados llegaba con 2.
- **El aviso de compactación ya no salta de una conversación a otra** (el
  «checkpoint pegado» de Codex): el estado por turno pertenece a la
  conversación, y las sesiones con más de 4 h inactivas se cierran solas.
- La huella de contexto ya no se corrompe con escrituras concurrentes.
- Dos procesos que abrían a la vez un almacén nuevo ya no fallan con
  `SQLITE_BUSY`.
- Los conflictos y sinapsis del contexto muestran el título de memorias
  antiguas en lugar de «(memoria previa)», y el orden ya no depende de la
  resolución de segundo de `created_at`.
- Las copias tardías del mismo `prompt_id` se descartan durante la ventana del
  recibo, incluso si la primera invocación del hook ya terminó.
- Los refrescos de CodeGraph del mismo proyecto se serializan con un lock del
  sistema operativo, aunque los inicien procesos distintos.

### Added

- Compresor de listados (`grep`, `rg`, `find`): −92,7 % en un `grep` real de
  400 líneas, con el original recuperable.
- `mem doctor`: hooks duplicados y «protecciones (7 días)».
- `mem pack savings`: columna «sin ganancia» separada de «degradaciones».
- Integración opcional del CLI de CodeGraph junto con codebase-memory-mcp, con
  snapshots independientes y ambos estados visibles en `mem context` y la TUI.
- `mem index` y la TUI reindexan todos los proveedores configurados y continúan
  si uno falla.

### Changed

- En Claude Code, el recordatorio de modo plan y la regla de Octopus ya no se
  repiten en cada turno.
- Los proveedores externos conservan el orden de `code_graph_providers`; la
  sincronización ADR y las anotaciones de hotspots eligen el proveedor por
  capacidad.

## [v2.27.1] - 2026-10-01

### Fixed

- **Codex ya no muestra «Hook failed — PostToolUse hook returned unsupported
  updatedMCPToolOutput»** en cada herramienta MCP con salida grande. Codex
  rechaza siempre el campo `updatedMCPToolOutput` (comprobado en el código de
  Codex y en codex-cli 0.159.0), así que la compresión de salidas de
  herramientas nunca funcionó en Codex: solo producía ese aviso. Ahora
  `mem hook tool-output codex` no emite nada, `mem install` deja de registrar el
  hook en `~/.codex/config.toml` y retira el que dejaron versiones anteriores
  (en cualquier ámbito y desde cualquier directorio), sin tocar los demás hooks
  ni el registro MCP. Claude Code y OpenCode no cambian.
- `mem doctor` informa de la compresión de salidas en Codex como «no soportado»
  y, si el hook antiguo sigue registrado, indica `mem install` para retirarlo.

## [v2.27.0] - 2026-09-26

### Changed

- **Un solo binario para todos los proyectos** (feature 034, US1). Con `mem` en
  el `PATH`, `mem install` ya no copia el binario al proyecto, y las copias
  `./mem` que dejaron versiones anteriores se retiran solas (al instalar o al
  abrir una sesión del agente), con un aviso visible de una línea. Así todos
  los proyectos usan la misma versión. Solo se retiran binarios de gomemory
  verificados: cualquier otro archivo llamado `mem` no se toca. Sin `mem`
  global en el `PATH`, el proyecto conserva su copia como hasta ahora.
- `mem update` ejecutado desde una copia local actualiza el binario **global**
  (antes solo actualizaba la copia y el global, el que usan hooks y MCP, se
  quedaba viejo). Si el global no admite escritura, no descarga nada e indica
  el comando exacto.
- `mem update` termina con un resumen de ✓/⚠/✗ por paso (descarga, checksum,
  binario, copia local, integración), también cuando un paso aborta. En
  `mem install`, el paso "Binario" del resumen indica la copia retirada o, si no
  se pudo retirar, la marca con ⚠ y el comando para hacerlo a mano.
- `mem uninstall --all` ya no dice que `~/.codex/config.toml` "conserva" la
  entrada de gomemory, cuando justo después la retira.
- El protocolo de memoria (v8) y el brazo extensor de spec-kit usan `mem …` del
  `PATH`; `./mem` queda solo como alternativa sin instalación global.

### Security

- `mem uninstall` borra también la memoria del proyecto en el almacén global.
  Hasta ahora anunciaba que borraba "TODA la memoria", pero solo retiraba
  `<proyecto>/.memory`: la base real (que vive en el almacén global desde la spec 005) se
  quedaba en el disco. El mensaje de confirmación enumera ahora exactamente lo
  que se borra.
- `mem export` crea el archivo con permisos `0600`. Antes quedaba legible por
  otros usuarios de la máquina, como el almacén antes de la v2.26.3.
- El workflow de release ejecuta `govulncheck` antes de GoReleaser; una
  vulnerabilidad alcanzable detiene la publicación.

### Added

- **Desinstalación sin rastros** (feature 034, US2). `mem uninstall` ofrece dos
  alcances: solo el proyecto, o todo gomemory del sistema (`--all`). Muestra un
  inventario de lo que existe realmente, permite exportar la memoria antes de
  borrarla (`~/gomemory-export-AAAAMMDD-HHMMSS/`, un archivo reimportable por
  proyecto más `index.json`, con permisos privados), conservarla
  (`--keep-memory`) o borrarla, y termina con un resumen ✓/⚠. El alcance de
  sistema retira la configuración de gomemory en Claude Code, OpenCode y Codex
  (solo sus entradas en los archivos compartidos), las habilidades y comandos
  globales, el almacén global y el binario; encuentra los proyectos por un
  registro que guarda `mem install` y, para instalaciones anteriores, buscando
  en el directorio personal (`--scan`, `--no-scan`). `--dry-run` muestra el
  inventario sin tocar nada. Sin terminal interactiva exige `--yes`.
- **Instalación guiada al estilo skills.sh** (feature 034, US3). Con terminal
  interactiva, `mem install` ofrece los agentes detectados ya marcados, la
  ubicación del binario (solo sin `mem` global), el alcance y un resumen con
  confirmación; solo configura los agentes elegidos (antes configuraba siempre
  OpenCode, Claude Code, Cursor y Codex). Flags equivalentes: `--yes`,
  `--agents`, `--scope`. La selección se guarda y `mem update` la reutiliza sin
  preguntar. `mem update` pide confirmación con terminal (`--yes` para
  omitirla). Los instaladores de consola ofrecen la instalación guiada al
  terminar dentro de un repositorio git.
- **Aviso de versión nueva** (feature 034, US4), al estilo de Codex: como mucho
  una vez cada 24 horas, `session-start` consulta en segundo plano la última
  release (sin retrasar el arranque; caché `update-check.json` con ETag) y
  muestra a la persona `gomemory vX disponible (tienes vY) → mem update`, una
  vez por versión y sesión. Solo avisa: nunca se actualiza sola. Se desactiva
  con `GOMEMORY_NO_UPDATE_CHECK=1` o `update_check_disabled` en el proyecto, y
  entonces no hay llamadas de red. `mem doctor` gana la sección
  "Versión y binario" (`binary` en `--json`).
- `mem update` verifica el SHA-256 de la descarga con el `checksums.txt` de la
  release y aborta sin tocar el binario si no coincide o no existe.
- `.env.example` documenta las variables de entorno de gomemory
  (`GOMEMORY_DATA_HOME`, `GOMEMORY_RELEASE_API_BASE`,
  `GOMEMORY_RELEASE_DOWNLOAD_BASE`, `GOMEMORY_NO_UPDATE_CHECK`,
  `GOMEMORY_BIN_DIR`).

## [v2.26.4] - 2026-09-26

### Security

- Todo el store global recupera sus permisos privados, no solo el proyecto
  que se abre. Los directorios de proyecto creados antes del hardening
  seguían en 0755, y los proyectos que no se volvían a abrir conservaban
  `mem.db`, `-wal` y `-shm` en 0644, legibles por otros usuarios. Ahora cada
  proyecto se corrige al abrirse, y `mem install` (que `mem update` ejecuta)
  recorre el store y fija 0700 en cada directorio de proyecto y 0600 en sus
  archivos `mem.db*`. Basta con actualizar.

### Changed

- Desde v2.26.3, `.memory/code_provider_snapshot.json` se crea con 0600 en
  vez de 0644, por la escritura atómica. Es una caché que solo lee el propio
  usuario.

## [v2.26.3] - 2026-09-26

### Security

- El directorio `.memory` del proyecto recupera sus permisos privados (0700)
  en cada arranque. Hasta ahora solo nacía con 0700 cuando no existía: las
  instalaciones anteriores y los hooks que lo creaban primero lo dejaban en
  0755, y `context.md` quedaba legible por otros usuarios. Las instalaciones
  existentes se corrigen solas al actualizar, sin pasos manuales.

### Fixed

- El snapshot del grafo de código se escribe de forma atómica. Varios
  refrescos en segundo plano podían coincidir, y una lectura simultánea veía
  un archivo a medias y daba el grafo por no disponible.

## [v2.26.2] - 2026-09-25

### Fixed

- El motor de compresión mide los tokens con el mismo contador que el
  contexto y `BuildContextPack`: el umbral mínimo y la decisión de ganancia ya
  no dependen de una heurística propia que podría divergir del contador real.
- El directorio `.memory` se crea siempre con permisos privados (0700),
  también cuando el refresco del grafo de código es quien lo crea primero.
- Las pruebas de contrato del hook `plan-entered` ya no fallan de forma
  intermitente al limpiar su directorio temporal.

## [v2.26.1] - 2026-09-25

### Added

- `get_context` y `get_plan_context` aceptan `full=true` (CLI: `mem context
  --full`) para recibir también lo ya enviado en la sesión. Los subagentes
  reciben la indicación de usarlo.

### Fixed

- El contexto de arranque que el host no inyecta entero (más de 10 000
  caracteres) ya no se da por entregado. Antes, `get_context` y `get_plan_context`
  devolvían marcadores «ya entregado» de entradas que el agente nunca leyó; lo
  mismo les ocurría a los subagentes. Los marcadores indican ahora cómo
  recuperar el contenido.
- La compresión de prosa deja una marca recuperable también al omitir líneas
  de lista, título o tabla duplicadas. Ningún compresor trata ya como
  duplicadas las marcas idénticas de omisión o de «ya entregado».
- El ajuste adaptativo cuenta las omisiones con el tipo de cada bloque, de
  modo que funciona también en documentos mixtos; `mem pack savings` muestra
  «—» en las filas sin usos propios.
- Las marcas dentro de JSON usan escapes JSON: un valor con caracteres de
  control ya no invalida la salida.
- `compression_originals_max_mb` no puede bajar de 8 MB, para que una llamada
  no expulse los originales que acaba de guardar.
- Instalar en un proyecto con la compresión de salidas apagada ya no retira el
  hook global de Codex que usan otros proyectos.

## [v2.26.0] - 2026-09-25

### Added

- Nuevo nivel de compresión nativa `max` para JSON, código, logs, diffs,
  tablas y prosa. Las omisiones usan referencias temporales recuperables con
  `mem pack retrieve` o `pack_retrieve`; el contenido privado no se almacena.
- `mem pack savings`, `tune` y `purge` permiten consultar el ahorro, controlar
  el ajuste adaptativo y administrar los originales recuperables.
- Compresión opt-in de salidas de herramientas y directiva opcional para
  respuestas concisas, configurables desde `mem settings` y la TUI.

### Changed

- En el corpus de referencia, el nivel `max` reduce un 72,6 % adicional frente
  a la compresión `structural`; `golist.json` pasa de 21 511 a 385 tokens
  aproximados. El nivel `structural` conserva su salida anterior.
- El contexto de sesiones con nivel `max` mantiene un prefijo estable y evita
  reenviar bloques ya entregados; la simulación de 20 turnos reduce el reenvío
  un 89,8 %.

### Fixed

- `mem pack compress` separa el informe de tokens del contenido y mantiene
  `stdout` apto para redirección.
- Las referencias recuperables permanecen disponibles tras aplicar el límite
  del almacén; si un original no cabe, se entrega la salida estructural.
- La ampliación de referencias ante colisiones conserva los valores literales
  de texto y JSON y mantiene cada marca asociada a su bloque original.

## [v2.25.0] - 2026-09-24

### Changed

- La constitución técnica por defecto pasa a la versión 2.0.0 (fecha de corte
  2026-09-24): jerarquía normativa, política Context7 y cadencia de renovación
  del catálogo, stack actualizado (Python 3.13, FastAPI 0.141, Go 1.27, Java 25
  LTS con Quarkus 3.33 LTS o Spring Boot 4.1, Vite 8.3, TypeScript 6,
  PostgreSQL 18), y secciones nuevas de resiliencia, observabilidad,
  seguridad de la cadena de suministro, CI/CD, gobierno de excepciones y
  fuentes del catálogo. Sigue sin nombrar organizaciones ni equipos.

### Added

- `mem update` entrega las plantillas nuevas de los documentos fijados sin
  pasos manuales. Una constitución o unas reglas que siguen intactas desde una
  versión anterior (se reconocen por su huella SHA-256) se actualizan solas en
  `mem update`, `mem install`, `mem seed` o al arrancar el servidor MCP. Un
  documento que el equipo editó no se toca: la siembra avisa que hay una
  versión nueva y cómo adoptarla (`mem docs export` + `mem docs reset`). La
  copia de spec-kit en `.specify/memory/constitution.md` sigue el mismo
  criterio.

## [v2.24.0] - 2026-09-24

### Fixed

- El plugin de OpenCode carga en OpenCode 2.x, que lo rechazaba con `Plugin
  must export a default definition with an id and an effect or setup
  function`. Un solo `gomemory.ts` exporta la definición v2 (`id`, `setup`) y
  conserva la de v1 (`server`), sin cambiar su comportamiento. Verificado con
  los binarios reales 1.17.0, 1.18.20, 1.18.32 y 2.0.16. En 2.x se traducen el
  fin de turno (`session.execution.*`), la compactación
  (`session.compaction.ended` y el gancho `compaction`) y las tools
  `shell`/`subagent`, así que el checkpoint, la procedencia del prompt, la
  inyección de contexto, la compactación y la captura de subagentes funcionan
  igual que en 1.x.
- El instalador ya no copia `gomemory.test.mjs` a
  `~/.config/opencode/plugins/` y retira el que dejaron instalaciones
  anteriores.

### Added

- `mem doctor` informa la versión de OpenCode, si el plugin instalado carga en
  ella (con el comando de reparación) y los plugins ajenos con la forma v1 que
  OpenCode 2.x rechaza, como `cbm-augment.ts`. Si el plugin de gomemory no
  carga en la versión detectada (o es solo v1 y la versión no se pudo
  detectar), cuenta como problema en la salida humana, en `--json` (bajo
  `opencode`) y en `--strict`. Los plugins ajenos solo se avisan: no cuentan
  como problema porque gomemory no los gestiona.

## [v2.23.3] - 2026-09-14

### Fixed

- Exportar e importar conserva `topic_key` y `source_review_id`. El formato
  del bundle sube a v2: los bundles v1 se siguen leyendo (ambos campos
  quedan vacíos, como ya valían) y los de una versión posterior se
  rechazan en vez de perder campos en silencio. Un documento fijado
  reimportado ya no pierde su identidad y una memoria promovida no
  pierde su revisión de origen.
- Todas las lecturas devuelven `source_review_id`: por id, por clave de
  tópico, listados y búsquedas FTS y LIKE. El linaje ya no es visible
  solo para el conteo de promociones.
- Los marcadores de hooks crean su directorio con permisos 0700 y el
  archivo con 0600. En un checkout fresco el marcador de primera sesión
  ya no se pierde y el bootstrap no se repite en cada prompt.

## [v2.23.2] - 2026-09-13

### Fixed

- Los guardados en paralelo ya no fallan con `SQLITE_BUSY`. Todas las
  transacciones del almacén abren en modo inmediato, porque con el modo
  diferido SQLite no invocaba el manejador de espera al pasar de lectura a
  escritura.
- El dedup conserva el linaje de la revisión: al reescribir una memoria con la
  misma `topic_key`, `source_review_id` pasa a la revisión cuyo contenido
  quedó guardado. Un guardado sin revisión mantiene el que hubiera.
- `GetMemoryByID` y las búsquedas FTS y LIKE devuelven `topic_key`, como ya
  hacían `List` y la búsqueda por clave. Leer por id ya no esconde la
  identidad fijada.
- La búsqueda LIKE trata `%`, `_` y `\` como texto literal con cláusula
  `ESCAPE`. Buscar `%` ya no devuelve todo el almacén hasta el límite.

## [v2.23.1] - 2026-09-13

### Fixed

- El aviso de duplicados compara el texto ya redactado en ambos lados y quita
  la nota de impacto al final antes de medir la similitud. Las memorias con la
  misma `topic_key` ya no generan aviso.
- La `topic_key` se guarda sin espacios laterales, como la busca la
  comprobación de duplicados. Una clave con espacios ya no crea una fila nueva
  en cada guardado.
- La sección de anclas avisa cuántas quedan fuera del tope de ocho con la
  línea `(+N anclas más; usa search_memories/get_memory)`.
- La nota de impacto vive en `domain` con un solo formato para escribirla y
  reconocerla. Reconoce símbolos con corchetes como `Cache[T]` y solo quita la
  nota exacta al final.
- Entrar en modo plan con Shift+Tab entrega el método una vez por sesión desde
  el prompt, con el mismo documento y presupuesto que `plan-entered`. Si no
  puede registrar la entrega o falta el historial, lo dice y apunta a
  `get_plan_context()`.
- El contrato de `mem mass --task` documenta que la búsqueda trae como máximo
  20 resultados y que `--top` solo limita la salida.

## [v2.23.0] - 2026-09-13

### Added

- `save_memory` y `mem save` avisan después de guardar cuando encuentran
  posibles duplicados del mismo tipo. La comparación usa similitud léxica y
  nunca bloquea la escritura.
- `get_context` informa qué anclas apuntan a archivos ausentes. Si hay un índice
  de código, distingue entre un único candidato de traslado, varios candidatos
  y una posible ancla huérfana.
- `mem mass` ordena las memorias con PageRank personalizado. Las semillas salen
  de la sesión activa, los hotspots de código o una búsqueda.
- Los paquetes de contexto pueden incluir hasta cinco memorias conectadas,
  seleccionadas por masa.

### Changed

- La sección de sinapsis ordena los enlaces por la masa de sus extremos y aclara
  que la centralidad no implica importancia ni corrección.
- La generación de contexto lee todas las relaciones antes de aplicar los
  límites de presentación. Los conflictos antiguos permanecen visibles.

### Fixed

- La detección de duplicados propaga los errores de consulta en lugar de
  interpretarlos como ausencia de coincidencias.
- La comprobación de anclas clasifica los errores de permisos y de E/S como no
  verificables, sin presentar el archivo como ausente.
- La evidencia de anclas revisa todas las memorias antes de limitar la sección
  a ocho entradas.

## [v2.22.3] - 2026-09-11

### Fixed

- Las transacciones de revisión descartan del pool cualquier conexión cuyo
  `ROLLBACK` falle, evitando que escrituras posteriores queden dentro de una
  transacción abandonada y se pierdan al cerrar la conexión.
- La creación automática de sinapsis comprueba el par de memorias en la misma
  sentencia de inserción. Esto evita relaciones duplicadas incluso si una base
  heredada no pudo crear `idx_relations_pair` por contener duplicados previos.
- `ConsensusRepository` ya no expone `UpsertFixDelta`, que permitía escribir una
  ronda sin validar estado, presupuesto ni digest. La vía de producción queda
  limitada a `RecordFixAtomically`.
- Los errores de conflicto de `RecordFixAtomically` incluyen los valores reales
  y esperados del digest o del número de rondas.
- Los tests de `adapters/primary/setup` usan un `GOMEMORY_DATA_HOME` temporal y
  ya no crean proyectos de prueba en el almacén global del usuario.

### Added

- Pruebas de regresión para el descarte de conexiones tras un `ROLLBACK`
  fallido, la idempotencia de sinapsis sin índice único y la serialización de
  listas vacías mediante la transición atómica de producción.

## [v2.22.2] - 2026-09-11

### Fixed

- `lastAnchorCache` solo avanza: inserciones concurrentes con IDs más viejos
  no retroceden el ancla de la sesión, evitando que memorias recientes
  queden fuera de la sinapsis.
- `jsonArray()` serializa nil como `[]` en vez de `null` para las columnas
  JSON de `fix_rounds` (DEFAULT `'[]'`), evitando errores de integridad.
- `tuiProvider` reutiliza `CodeProviders` de `NewContainer` con fallback,
  evitando reconstrucción innecesaria.
- Error de `RecordFixAtomically` ahora incluye el error subyacente con `%w`.

### Added

- Test `TestFormSynapse_CacheNoRetrocede` verifica que la caché no retrocede
  con inserciones concurrentes.
- Test `TestUpsertFixDelta_ListaNilSeGuardaComoArrayVacio` verifica que nil
  se persiste como `[]`.

## [v2.22.1] - 2026-09-11

### Fixed

- `Container.Close()` retorna error; `closeContainer` wrapper lo registra
  en stderr sin ensuciar stdout (hooks JSON).
- `tuiProvider` reutiliza `settings` leída una vez en `NewContainer` en
  vez de re-leer `settings.json` en cada llamada.
- Mutex en `lastAnchorCache` corrige condición de carrera bajo llamadas
  MCP paralelas.
- `formSynapse` ignora checkpoints como ancla: un checkpoint nunca gobierna
  las memorias que vienen después en la sesión.
- `StartSession` retorna `CreatedAt` usando `RETURNING` en vez de dejarlo
  vacío.
- MCP resource handler: `Get` por ID en vez de `List` de 200 memorias
  para resolver un recurso por URI.
- `dropNullUnions` recursivo en `Defs`, `AllOf`, `AnyOf` y `OneOf` —
  cierra la vía de escape que dejaba uniones con null en composición.
- `build_context`: `relTitle` formatea títulos de conflictos sin depender
  de que ambos extremos estén en la ventana cargada.
- Plugin OpenCode: `T_REVIEW_PROMOTE` renombrado a `T_REVIEW_PROMOTE_MEMORY`
  para alinearse con el nombre real de la tool MCP.

### Added

- Tests: cobertura de MCP schemas (null unions en composición), resource
  handler por ID, concurrencia de sinapsis, sesión con `CreatedAt` y
  nombres de tools OpenCode.

## [v2.22.0] - 2026-09-11

### Fixed

- MCP schemas: `addTool` elimina uniones con "null" (`["null","boolean"]` →
  `"boolean"`) de los esquemas publicados. OpenCode emitía valores vacíos
  (`"fix_authorized": .`) ante estos tipos, causando JSON inválido.
- `Container.Close()` cierra la conexión SQLite; antes quedaba abierta.
- `hookPlanGuard` retornaba después de `permit()` para evitar caer en la
  marca denegada.

### Added

- `CodeProviderSnapshot.Project` expone el nombre canónico con que el
  grafo externo indexó el repo — los agentes ya no necesitan adivinar el
  parámetro `project` de las herramientas del grafo.
- Scripts `bump_version.sh` y `sync_version.sh` para mantener
  `version/version.go` alineado con la release latest.
- CI: job `sync-version` en la workflow de release que sincroniza
  `version/version.go` tras cada publicación.

## [v2.21.0] - 2026-09-11

### Added

- Compactación sin pérdida de memoria (feature 030): la compactación de la
  conversación sigue siendo exclusiva del cliente, pero ahora ninguna
  compactación pierde lo que la memoria registró en la sesión.
  - Nueva herramienta `save_session_summary`: persiste el resumen compactado
    sin cerrar la sesión (a diferencia de `end_session`), cerrando un defecto
    real donde toda memoria guardada tras compactar quedaba sin sesión
    asociada.
  - `mem hook compaction-context` (antes de compactar) y `mem hook
    post-compact` (después) entregan la memoria de la sesión activa —no el
    proyecto entero—, con puntero `get_memory <id>` por entrada.
  - Captura pasiva de aprendizajes: al terminar un subagente, los ítems de su
    sección `## Aprendizajes clave` / `## Key Learnings` se guardan solos, sin
    duplicados.
  - Aviso opcional al agente (`compact_agent_notice`, opt-in) al superar el
    umbral de compactación, sin bloquear ni prolongar el turno.
  - `mem doctor` reporta, por agente instalado, qué capacidades de
    compactación aprovecha la integración.
  - El complemento de OpenCode corrige un defecto real: llamaba a
    `post-compact` ANTES de que la compactación existiera (dentro de
    `session.compacting`), reseteando huella y marcadores fuera de tiempo.

## [2.20.2] - 2026-09-10

### Fixed

- Aclara el contrato MCP de `review_start`: `scope` es opcional y, cuando se
  usa, debe ser un arreglo JSON de rutas relativas. OpenCode ya no recibe una
  guía que pueda inducir una ruta suelta e inválida como `scope: .`.
- El plugin de OpenCode incluye un ejemplo válido (`scope: ["."]`) y una prueba
  verifica que el esquema MCP publicado conserve esta aclaración.

## [2.20.1] - 2026-09-10

### Security

- Actualiza Go a 1.27.1 y renueva dependencias de SQLite, MCP y TUI para
  incorporar las correcciones de vulnerabilidades disponibles.
- Cierra los errores previamente descartados en rutas de persistencia, hooks,
  sincronización e inicialización, preservando el comportamiento best-effort
  donde corresponde.

## [2.20.0] - 2026-09-04

### Added

- La TUI permite eliminar una memoria individual desde la lista, incluso tras
  buscarla. Pulsa `d` y confirma con `s`; `n` o Esc cancelan la operación.

### Fixed

- Constitución y Reglas IA ahora se desplazan en la TUI con flechas, Page Up,
  Page Down, inicio y fin. El texto se ajusta al tamaño de la terminal.

## [2.19.1] - 2026-09-04

### Changed

- La constitución predeterminada ahora exige identificadores de código en
  inglés. Los comentarios necesarios se mantienen en español latino claro y
  explican el porqué sin repetir el código.
- La regla forma parte de la plantilla embebida, por lo que acompaña a cada
  binario portable de GoMemory.

## [2.19.0] - 2026-09-03

### Added

- Instala un baseline universal portable en las instrucciones globales de
  Claude Code, OpenCode y Codex. El instalador lo actualiza de forma idempotente
  sin sobrescribir instrucciones del usuario.

### Changed

- Normaliza las reglas específicas de GoMemory: conserva los invariantes
  operativos comprobados y deja las prácticas generales en el baseline común.
- El diagnóstico de activación ahora comprueba tanto el protocolo de GoMemory
  como la versión vigente del baseline universal.
- Actualiza la documentación de integración y el recuento publicado a 27
  herramientas MCP.

### Fixed

- Los hooks `Stop` y `UserPromptSubmit` de Codex declaran explícitamente JSON;
  cuando `Stop` no tiene recordatorios que emitir, devuelve un objeto JSON vacío
  válido en lugar de fallar antes de registrar el checkpoint.

## [2.18.3] - 2026-09-03

### Fixed

- Corrige los hooks `Stop` y `UserPromptSubmit` de Codex para emitir el JSON
  que el runtime valida. Se elimina `--emit=text`, que hacía fallar `Stop` con
  una salida JSON inválida cuando GoMemory inyectaba contexto.
- Reconciliar la configuración global de Codex ahora migra instalaciones que
  todavía forzaban salida de texto plano y conserva la ruta del binario.

## [2.18.2] - 2026-09-03

### Fixed

- Documenta en el esquema MCP de `review_submit` que los únicos estados válidos
  son `success` y `failure`. OpenCode ya no necesita probar valores como
  `submitted`, `complete` o `findings` antes de entregar un resultado.
- Añade pruebas de contrato y del descriptor MCP publicado para conservar esa
  guía para clientes y agentes.

## [2.18.1] - 2026-09-03

### Changed

- Aplica política de delegación Octopus AAR a todos los agentes (Claude Code, OpenCode, Codex)
- Migra hooks de Codex al sistema compartido `octopusDelegationPolicy` desde `setupCodexGlobal`

### Removed

- Retira workflow de CI independiente (`release.yml`)

## [2.16.10] - 2026-09-02

### Correcciones

#### Pegado funciona en todas las cajas de la TUI

`updateFocusedInput` enruta el pegado por pantalla en vez de recorrer el modelo
con `Focused()`. Antes, `saveContent` se enfocaba al construir el modelo y
nunca se desenfocaba al salir de la pantalla de guardar, así que se tragaba el
pegado de todas las demás cajas (ruta de import, ajustes de IA, documentos),
que quedaban sin poder pegar.

Dentro de una pantalla con varios inputs se le pasa el mensaje a todos: el
`textinput` ignora lo que recibe mientras está desenfocado.

#### Foco correcto al guardar

`saveAndReturn` usa `updateFocus()` en lugar de `Focus()` suelto para evitar
doble foco tras tabular entre cajas de la pantalla de guardado.

---

## [2.16.7] - 2026-08-30

### Correcciones

#### Un resultado `failure` ya no puede reescribir las fuentes del consenso

La corrección de 2.16.6 que devolvió el fail-closed dejaba pasar todo resultado
`failure` sin mirar su contenido. Como los hallazgos se persisten con
`ON CONFLICT DO UPDATE`, un `failure` que adjuntara un hallazgo con un
`local_id` ya existente reescribía su enunciado, su severidad o su evidencia
—sobre fuentes que el consenso ya había clasificado—. La declaración de fallo se
había convertido en la vía de escritura que la guarda existía para cerrar.

Declarar `failure` sigue siendo siempre posible, que es lo que sostiene el
fail-closed, pero fuera de la fase de recogida no puede traer hallazgos. Es la
misma regla que ya rige en las rondas de revalidación y por el mismo motivo: no
hay consenso que pueda clasificar un hallazgo que llegue ahí.

**Cambio observable:** `review_submit` rechaza un resultado `failure` con
hallazgos cuando la revisión ya salió de la fase de recogida. Envíalo sin ellos
y abre una revisión nueva para el material no clasificado.

#### El hook de fin de turno respeta el dialecto del agente

`turn-end` emitía el sobre JSON de Claude Code a cualquier agente. Codex toma el
stdout del hook como contexto tal cual y no reconoce ese sobre, así que
rechazaba la salida entera y el refuerzo de preferencias no llegaba nunca. El
campo `Emit` de la tabla de hooks de Codex existía justo para esto, y `turn-end`
—el otro hook que inyecta texto al modelo— se había registrado sin pedirlo.

Ahora `turn-end` acepta `--emit`, se registra en Codex con `Emit: "text"`, y
traduce sus dos mensajes según el destinatario: el aviso de compactación va a
quien mira la terminal y el refuerzo de preferencias al modelo. En los dialectos
planos ambos salen como texto desnudo, y el silencio es la cadena vacía en vez
de `{}`.

La salida para Claude Code no cambia.

## [2.16.6] - 2026-08-30

Correcciones surgidas de una revisión adversarial por consenso sobre 2.16.5.

### Correcciones

#### Un revisor vuelve a poder declarar `failure` con el consenso listo

La guarda de fase que 2.16.5 introdujo en el envío de resultados rechazaba
cualquier escritura fuera de la fase de recogida. Como el estado
`consensus_ready` lo escribe el propio envío del segundo revisor, la ventana se
cerraba en el mismo instante en que ambos terminaban: a partir de ahí nadie
podía declarar `failure`, y esa era la única vía a `INCOMPLETE` desde ese
estado. La transición quedaba inalcanzable aunque el dominio siguiera
declarándola legal, así que una revisión con ejecución inválida podía terminar
`APPROVED`.

Ahora la guarda deja pasar todo resultado `failure` y rechaza solo lo que debía
proteger: el reenvío que altera las fuentes sobre las que se construyó el
consenso.

#### `review_submit` recupera la idempotencia de reenvío

Por la misma guarda, un reintento con contenido idéntico tras `consensus_ready`
pasaba de no-op a error. Un reintento de transporte —un timeout cuya escritura
sí se confirmó— fallaba en el segundo intento. El reenvío vuelve a compararse
por contenido; la comparación se hace sobre los valores ya redactados, que es lo
que hay almacenado.

#### Un `failure` ya persistido no lo pisa un `success` concurrente

«Un resultado `failure` es final para la ronda» se comprobaba leyendo fuera de
toda transacción. Entre esa lectura y la escritura cabía un envío `success` que
sobrescribía el fallo y dejaba el ledger sin rastro de la ejecución fallida. La
comprobación previa sigue dando errores tempranos; la que cierra la carrera vive
ahora dentro del mismo bloqueo de escritura.

#### El digest de `--pending` deja de admitir dos lecturas

Los campos se concatenaban separados por NUL, pero ni la identidad del índice ni
el contenido del archivo tienen prohibido contenerlo: el mismo flujo de bytes
admitía más de un reparto, y dos estados pendientes distintos podían compartir
identidad congelada. Cada campo se escribe ahora precedido de su longitud.

**Cambio observable:** el digest de `--pending` cambia para el mismo árbol de
trabajo. Una revisión congelada con una versión anterior y continuada con esta
se rechaza con `target changed`; hay que volver a congelarla.

### Pruebas

Cobertura de las ramas que decidían en producción sin tenerla: la guarda de fase
y ronda del adaptador real —hasta ahora solo se afirmaba sobre la copia de esa
lógica en el doble de memoria—, las etapas 1/2/3 de un conflicto de merge en la
identidad del índice, y la colisión de reparto del digest de `--pending`.

## [2.16.5] - 2026-08-30

### Correcciones

#### La marca de resultados detecta cambios entre lecturas

`ReviewerResultsMark` resume resultados y hallazgos de una ronda como hash
SHA-256. Se usa en la comparación atómica de finalización para detectar si
alguien modificó los resultados entre la lectura y la escritura. Antes, solo
se comparaba el estado de la revisión y los re-juicios, no el contenido de los
resultados.

#### El envío de resultados comprueba fase y ronda bajo el mismo bloqueo

`UpsertReviewerResultAtomically` verifica el estado terminal, la fase y la ronda
dentro del `BEGIN IMMEDIATE` que persiste el resultado. La comprobación previa
del caso de uso sigue dando errores tempranos, pero la que cierra la carrera es
esta.

#### El linaje por revisor solo publica la ronda vigente

`ReJudgmentsByReviewer` filtra por ronda. Antes, un estado declarado en una ronda
ya superada seguía apareciendo en `review_status`, `review_rejudge` y
`mem review show`, de modo que la auditoría podía atribuir a un revisor una
postura que ya no sostenía.

#### La identidad del índice en `--pending` incluye modo y etapa

`blobsPreparados` conservaba solo el SHA del blob. El modo distingue un archivo
ejecutable de uno normal aunque los bytes sean idénticos, y la etapa distingue
las entradas 1/2/3 de un conflicto de merge, que además podían sobrescribirse
entre sí al compartir ruta.

**Cambio observable:** el digest de `--pending` cambia para el mismo árbol de
trabajo respecto de 2.16.4.

## [2.16.4] - 2026-08-29

### Correcciones

#### La finalización verifica la marca de re-juicios en el orden correcto

`FinalizeReview` leía la marca de re-juicios después de los datos de los que
se deriva el veredicto. Una retractación que aterrizara entre ambas lecturas
quedaba dentro de la marca pero el veredicto ya estaba calculado con hallazgos
obsoletos. Ahora la marca se lee primero.

`SetReviewStatusAtomically` reemplaza a `FinalizeReviewAtomically` y agrega
`ExpectedRejudgmentMark` a la comparación atómica.

#### Un hallazgo solo se muestra resuelto si lo está en la ronda vigente

`ConsensusFinding` gana `EstadoVigente` y `ResueltoEn`: un solo criterio de
vigencia. Antes, consultas de estado y de detalle leían la columna a secas y
mostraban `RESOLVED` mientras la finalización se negaba a cerrar sin decir cuál
era el hallazgo ni por qué.

## [2.16.3] - 2026-08-29

### Correcciones

#### El consenso se reemplaza de forma atómica

`BuildConsensus` ahora llama `ReplaceConsensusRound`: una sola transacción que
verifica que no existe y escribe todas las filas. Antes eran dos pasos separados
(listar y luego upserts individuales), y dos llamadas concurrentes veían el
ledger vacío y las dos escribían, mezclando la ronda.

#### La finalización ya no pierde actualizaciones concurrentes

`FinalizeReview` usa compare-and-swap: lee el estado antes de la transacción,
verifica que no cambió, y actualiza solo si coincide. `UpdateReview` reescribía
todas las columnas, así que una ronda de corrección entre la lectura y la
escritura se perdía sin aviso.

## [2.16.1] - 2026-08-29

### Correcciones

Cuatro defectos encontrados al revisar la versión 2.16.0 con su propio protocolo. Tres
viven en la misma ruta —qué ocurre DESPUÉS de una corrección— y ninguna prueba los veía,
porque todos los fixtures enviaban los resultados post-corrección sin hallazgos. El
cuarto solo aparece con los dos revisores trabajando de verdad en paralelo.

#### El consenso ya no se sobrescribe entre rondas

`BuildConsensus` aceptaba reconstruir el consenso en una ronda posterior. Los
`ConsensusLocalID` se generan por posición (C-001, C-002…) y son únicos por revisión, no
por ronda, así que el segundo cálculo reasignaba los identificadores y sobrescribía los
hallazgos confirmados de la ronda anterior: un `CONFIRMED HIGH` se convertía en un
`CONFIRMED LOW` con otras fuentes, y el `addressed_consensus_ids` de la corrección
quedaba apuntando a un hallazgo distinto del que decía haber arreglado.

Ahora el consenso se construye una sola vez, en la ronda de descubrimiento. Las rondas
posteriores revalidan con `review_rejudge`.

#### Una revisión con hallazgos post-corrección ya puede terminar

`SubmitReviewerResult` aceptaba hallazgos nuevos en rondas de revalidación. Quedaban sin
clasificación de consenso —y no podían tenerla, por el defecto anterior—, así que
`DeriveVerdict` los veía sin clasificar y la revisión no alcanzaba ningún estado
terminal: ni aprobada ni escalada, nunca.

Ahora esas rondas rechazan hallazgos y encaminan a `review_rejudge`, que es el canal que
el protocolo ya definía para revalidar.

#### Los re-juicios en paralelo ya no se pierden

`UpsertReJudgment` abría su transacción con el `BEGIN` diferido de `database/sql`, que
en modo WAL toma el bloqueo de escritura en el primer `INSERT`. El perdedor recibía
`SQLITE_BUSY`, que el `busy_timeout` no reintenta para una transacción que ya leyó.

Afectaba al flujo normal del protocolo, no a un caso raro: dos revisores independientes
re-juzgando a la vez es exactamente para lo que existe. Con 16 re-juicios simultáneos
fallaban 11 con «database is locked». Ahora usa una conexión dedicada con
`BEGIN IMMEDIATE`, el mismo patrón que ya empleaba el registro de correcciones.

#### La unanimidad ya no cruza rondas

`AggregateReJudgment` recibía todos los re-juicios de un hallazgo, de todas las rondas.
Un `RESOLVED` emitido sobre una corrección anterior completaba la unanimidad de una
posterior que ese revisor nunca vio, y la revisión terminaba aprobada. Se añadió
`AggregateReJudgmentForRound`, que filtra por ronda antes de agregar.

## [2.16.0] - 2026-08-29

### Correcciones

#### La revisión adversarial ya no puede aprobar con un defecto oculto

Tres defectos confirmados por dos revisores independientes sobre la propia
implementación de `mem review`. Los tres permitían que el protocolo diera por buena
una revisión que no lo estaba.

**Consenso parcial y severidad degradada.** La clasificación de consenso se validaba
hallazgo a hallazgo mientras se recorría la entrada, así que nadie comprobaba el
conjunto: omitir un hallazgo grave no producía ningún error, simplemente no se
mencionaba. Y la severidad venía del orquestador, de modo que un `HIGH` corroborado
por ambos revisores podía persistirse como `LOW` y desaparecer del veredicto. Ahora
la clasificación debe cubrir cada hallazgo de la ronda exactamente una vez, y la
severidad se deriva como el máximo de sus fuentes.

**La revisión de solo lectura se quedaba bloqueada.** Una revisión con un defecto
grave confirmado y sin autorización para corregir devolvía «review is not ready to
finalize» indefinidamente, porque el presupuesto de rondas permitía una corrección
que su alcance prohibía. `--read-only` y `review_fix_authorized` declaran ese alcance,
y esas revisiones terminan `ESCALATED` en una sola llamada.

**Las métricas no cumplían su propio contrato.** `review_finalize` prometía ocho
campos en `snake_case` y emitía cinco en PascalCase, porque el struct no tenía
etiquetas JSON. Faltaban `duration`, `memory_promoted` y `memory_deduplicated`.

### Otras correcciones del mismo trabajo

- **Correcciones concurrentes se perdían.** Registrar una ronda eran cuatro
  operaciones sueltas: dos procesos derivaban la misma ronda y el segundo
  sobrescribía al primero sin dejar rastro. Ahora es una transacción `BEGIN
  IMMEDIATE` que revalida dentro lo que se leyó fuera.
- **Los estados terminales no lo eran.** La máquina de estados existía y era
  correcta, pero ningún caso de uso la usaba: una revisión `APPROVED` aceptaba
  resultados, consenso y correcciones nuevas.
- **La política del proyecto no se aplicaba.** `review_max_fix_rounds` y
  `review_auto_fix_severities` existían en `settings.json` desde la versión anterior
  y nadie los leía; el código reimplantaba los defectos a mano. Además, guardar
  cualquier otra preferencia **borraba** esa política, porque no viajaba en la
  estructura que se reescribe al guardar.
- **Se podía promover aprendizaje desde una revisión sin aprobar.**
- **La identidad de los revisores no se comprobaba**, así que la independencia que
  la revisión declaraba no era verificable.
- **El re-juicio no distinguía quién lo emitía**: un solo revisor bastaba para marcar
  un defecto como resuelto. Ahora se registra por revisor, con evidencia, y `RESOLVED`
  exige unanimidad.

### Novedades

- `mem review --pending` congela **todo** el trabajo pendiente —preparado, sin
  preparar y archivos nuevos—, con una identidad reproducible. `--diff` usa `git
  diff`, que no ve los archivos sin seguimiento, así que congelaba menos de lo que
  parecía.
- `mem review --read-only` para revisiones que validan sin corregir.
- `mem review show` reconstruye el linaje completo de cada hallazgo: fuentes, ronda
  de corrección, lo que declaró cada revisor y el estado agregado.
- `review_status` devuelve target original y vigente, política, revisores, recuentos
  por clasificación/severidad/re-juicio y el linaje de cada hallazgo.

### Migración

Aditiva. Una base anterior abre sin cambios: su target vigente se interpreta como el
original y sus revisiones siguen autorizando corrección, que es su comportamiento
histórico. Ningún hallazgo se borra.

## [2.15.0] - 2026-08-29

### Novedades

#### Revisión adversarial por consenso

Nuevo protocolo de revisión con dos revisores independientes de solo lectura. Cada revisión trabaja sobre un target congelado (hash SHA-256 del contenido a evaluar), lo que garantiza que ambos revisores ven exactamente lo mismo sin importar cuándo ejecuten.

El flujo: `review start` congela el target, `review submit` recibe los hallazgos de cada revisor de forma independiente, `review consensus` cruza los hallazgos buscando convergencia, y `review finalize` decide si el cambio se aprueba, escala o corrige. Los hallazgos deconvergentes quedan registrados para análisis posterior.

Las herramientas MCP disponibles son `review_start`, `review_submit`, `review_consensus`, `review_finalize` y `review_fix`. Cada una maneja su fase del protocolo y valida las transiciones de estado.

#### El ledger de revisión redacta secretos

Los campos `claim`, `evidence` y `verification` de cada hallazgo pasan por el mismo pipeline de redacción que usa la memoria persistente. Cuando `redact_secrets_enabled` está activo (lo es por defecto), los tokens, credenciales y datos sensibles se reemplazan por `<private>` antes de escribirse al ledger.

#### Promoción de hallazgos a memorias

`review promote` convierte un hallazgo de consenso en una memoria persistente del proyecto. Esto cierra el ciclo entre la revisión y el sistema de memoria: un problema encontrado por los revisores termina como conocimiento estructurado que los agentes pueden consultar.

### Cambios

#### Limpieza del repositorio

`.agents/`, `.specify/` y `.github/workflows/` se retiraron del tracking de git. Son generados localmente por speckit y las herramientas de setup, y no deberían versionarse.

## [2.14.0] - 2026-08-29

### Novedades

#### Codex recibe contexto en cada turno, no solo al arrancar

Hasta ahora Codex obtenía el protocolo de memoria una sola vez al inicio de la sesión, en un archivo estático que se diluye a medida que crece la conversación. Se verificó en sesión interactiva real (Codex 0.151.0) que el evento `UserPromptSubmit` dispara y su stdout llega al modelo como contexto, el mismo canal que usa Claude Code para la inyección por turno. La matriz de canales declaraba que Codex no exponía esto, y sobrevivió porque una celda marcada "no aplicable" no la revisa nadie.

Ahora `mem setup-mcp --scope global` escribe el enganche `user-prompt-submit` con `--emit=text` en el config de Codex, y el hook traduce su salida al dialecto correcto por agente.

#### Habilidades disponibles en los tres agentes

Antes, gomemory escribía habilidades únicamente en `.claude/skills/`. Codex y OpenCode nunca recibieron la descomposición atómica ni la constitución como habilidad descubrible. Ahora un instalador genérico (`InstallAgentSkill`) distribuye cada skill al directorio de habilidades de cada agente que lo exponga: `.claude/skills/`, `.codex/skills/`, `.opencode/skills/`. Cada skill tiene un solo dueño que la escribe, y los agentes que no usan ese canal no reciben archivos sobrantes.

#### `mem review`: revisión adversarial por consenso (ACR)

Protocolo completo de revisión con dos revisores independientes sobre un target congelado (diff, commit o archivo). Un defecto solo se confirma cuando ambos lo encuentran por separado; la corrección automática queda limitada a hallazgos confirmados de severidad CRITICAL o HIGH, con un presupuesto de rondas configurable (2 por defecto). El agente propone la clasificación de consenso y la corrección; gomemory valida cada paso contra el estado persistido y deriva el veredicto (APPROVED, ESCALATED, INCOMPLETE) — el veredicto no se puede pasar como parámetro.

Un defecto confirmado y resuelto puede promoverse a memoria del proyecto (problema, causa raíz, resolución, verificación), reutilizando el mismo dedup por `topic_key` de las memorias normales en vez de un almacén paralelo.

CLI: `mem review --diff|--commit|--file`, `status`, `history`, `show`. Siete tools MCP nuevas (`review_start`, `_submit`, `_consensus`, `_fix_record`, `_rejudge`, `_finalize`, `_promote_memory`) más `review_status` de solo lectura. Detalle completo en [`specs/027-adversarial-consensus-review`](specs/027-adversarial-consensus-review/).

Se distribuye además como habilidad (`SKILL.md`) en los tres agentes, agnóstica de proveedor y de las herramientas de gomemory, para que un agente sin este servidor MCP conectado igual conozca el protocolo.

### Correcciones

#### `mem setup-mcp --scope global` sin argumentos registraba solo dos agentes

El valor por defecto de `--agents` era `opencode,claude`. Codex estaba en `globalScopeAgents` pero ausente del defecto, así que quien no supiera pasar `--agents codex` se quedaba sin su ciclo de memoria entero, y el comando terminaba en éxito sin advertir nada.

Ahora el defecto es `opencode,claude,codex`, con un test que fija la invariante: todo agente del defecto debe existir en `globalScopeAgents`.

#### macOS mataba el binario recién instalado con SIGKILL

El script de instalación usaba `cp` directo para reemplazar el binario. En macOS la firma de código se cachea por inodo, y `cp` sobre un ejecutable existente reutiliza el mismo inodo. El resultado era un `mem` que moría con exit 137 en cada invocación, sin mensaje ni error de firma.

Ahora el fallback elimina el binario antiguo antes de copiar, forzando un inodo nuevo. `codesign -v` no reporta problema con ninguno de los dos estados; la diferencia solo se ve al ejecutar.

#### Los tests modificaban el config real de Codex

`go test ./...` escribía en `~/.codex/config.toml` de quien ejecutaba la suite. Varios tests lanzan el binario como subproceso fijando `cmd.Dir` pero no `cmd.Env`, y el hijo hereda el HOME del proceso de test. `t.Setenv("HOME", ...)` protege al código in-process, no a un subproceso sin `Env` explícito.

Los `TestMain` de `tests/contract` y `tests/integration` ahora aíslan HOME y USERPROFILE además del `GOMEMORY_DATA_HOME`. `anclarCachesDeGo()` fija GOCACHE y GOMODCACHE antes de mover el HOME, o cada ejecución recompilaría el mundo.

#### `mem review` podía aprobar una revisión sin calcular el consenso

`review_finalize` derivaba el veredicto sobre los hallazgos de consenso; si ese paso fallaba o simplemente no se ejecutaba, la lista de hallazgos quedaba vacía y `DeriveVerdict` aprobaba con "0 hallazgos" aunque ambos revisores hubieran reportado un defecto. Lo destapó ejecutar el protocolo completo contra el servidor MCP real, no la suite de tests.

`DeriveVerdict` ahora exige que exista consenso calculado cuando algún revisor reportó hallazgos; sin eso, la revisión no está lista para finalizar. De paso, `review_consensus` dejó de exigir `unmatched` cuando no hay hallazgos sueltos que declarar.

#### El ledger de revisión guardaba secretos en claro

Las memorias se redactan al guardarse desde hace varias versiones; las tablas de `mem review` (`findings`, `consensus_findings`, `fix_rounds`) no. Un revisor cita el código que analiza, así que una credencial en esa línea quedaba persistida tal cual y se servía después por `mem review show`.

Ahora `claim`, `evidence` y `verification` pasan por el mismo `RedactSecrets`/`RedactPrivate` que ya protege a las memorias.

## [2.13.0] - 2026-08-25

### Novedades

#### Codex ejerce el ciclo de memoria, no solo lo consulta

Hasta ahora Codex recibía el registro del servidor MCP y nada más: podía consultar la memoria si se le pedía, pero
gomemory no inyectaba contexto al arrancar la sesión ni registraba la actividad al cerrar cada turno. `mem doctor`
tampoco reportaba una sola de sus filas, así que el hueco no era visible: el diagnóstico cerraba con «Sin problemas»
mientras el agente funcionaba con una fracción de sus canales.

La instalación ahora registra en `~/.codex/config.toml` los enganches del ciclo —contexto al arrancar, recuperación
tras compactar y registro de actividad al cerrar el turno— y escribe el bloque de protocolo en `~/.codex/AGENTS.md`.
La escritura preserva íntegra la configuración ajena: solo se añaden los enganches que faltan, reinstalar no los
duplica, y las referencias de confianza las genera Codex al autorizarlos. La bandera `[features] hooks` queda activa,
sin la cual Codex ignoraría la sección entera.

Codex aparece además en `mem doctor` con sus canales propios, y las dos capacidades que su ciclo no ofrece —los bordes
de entrada y salida del modo plan— se reportan como degradaciones declaradas, con su motivo, en vez de desaparecer del
informe.

#### El diagnóstico distingue disponibilidad de ejercicio

Un canal nuevo separa que un agente PUEDA consultar la memoria de que gomemory EJERZA su ciclo en él. Antes, un
registro MCP presente bastaba para dar por sano a un agente que no recibía contexto ni generaba checkpoints.

### Correcciones

#### El contexto vuelve a caber en su presupuesto

El registro automático de actividad era la única sección que no respetaba el techo de `budget`: se emitía completo, sin
acotar. En un proyecto con historia acumulada llegó a ocupar el 68 % de un documento que más que duplicaba su
presupuesto. Ahora se acota como cualquier otra sección, con un puntero `get_memory` al detalle completo.

#### La actividad automática deja de crecer sin límite

Un checkpoint guardaba hasta cinco comandos por turno, pero sin límite de largo: un comando que incluyera un archivo
entero se almacenaba literal. Cada comando queda acotado y se declara cuánto se omitió.

Los checkpoints también se deduplican por contenido. La actividad de un mismo turno se registraba dos veces —una por
el agente principal y otra por el subagente— con títulos distintos y cuerpo idéntico, y esas copias se acumulaban sin
que nada las fundiera.

#### El historial deja de viajar dos veces en la misma sesión

Al entrar en modo plan, gomemory sustituye el historial por un aviso si ya se entregó en esa sesión. La sustitución no
llegaba a aplicarse cuando el contexto lo entregaba el enganche de arranque, que es la vía habitual: la entrega no
quedaba anotada y no había con qué compararla.

#### La versión informada corresponde al código

Una compilación desde el árbol reportaba una versión anterior a la publicada, porque la constante interna se había
quedado atrás respecto de las etiquetas de release.
## [2.18.0] - 2026-09-03

### Novedades

#### Octopus AAR, enrutador adaptativo de agentes

- Añade un módulo opt-in que decide si una unidad se ejecuta en el agente principal o se delega, con presupuesto de contexto y contrato de ejecución. Octopus produce la política; el runtime ejecuta e informa el coste real.
- La política es determinista: aplica 13 reglas en orden fijo y devuelve una razón de un catálogo cerrado.
- Permite enrutar planes con dependencias y grupos paralelos, reutiliza el Context Optimization Engine, protege una reserva de validación y registra resultados para comparar estimaciones con consumo real.
- El módulo empieza apagado. Se activa desde Configuración en la TUI o con `octopus_enabled: true` en `.memory/settings.json`. Mientras está apagado no registra sus tools MCP, no altera el protocolo ni el bootstrap de ToolSearch y no escribe filas en la base.
- Añade `mem octopus plan|route|status|usage|history` y las tools MCP `octopus_route_task`, `octopus_route_plan`, `octopus_report` y `octopus_status`.
- La tabla `octopus_executions` guarda identificadores, enums y cifras, sin columnas de texto libre alimentadas por contenido. Una prueba de contrato protege ese esquema.

## [2.12.0] - 2026-08-25

### Novedades

#### Codex consolida automáticamente sus hooks en una sola fuente

`mem install` y `mem setup-mcp --scope global --agents codex` migran los hooks heredados de
`~/.codex/hooks.json` a `~/.codex/config.toml`. La migración interpreta ambos formatos, conserva eventos, filtros,
comandos, límites y campos compatibles, y elimina grupos semánticamente equivalentes sin distinguir entre GoMemory,
Herdr u otros proveedores.

Antes de modificar los archivos se crean respaldos recuperables. `hooks.json` solo se retira después de serializar y
validar el TOML consolidado; si el JSON es inválido, permanece intacto y el registro MCP de GoMemory continúa por
separado. Las referencias de confianza asociadas a posiciones anteriores se eliminan para que Codex vuelva a autorizar
los hooks desde su ubicación vigente.

## [2.11.1] - 2026-08-24

### Correcciones

#### Constitución predeterminada completamente agnóstica

La constitución ya no conserva referencias al ecosistema Speckit ni al proyecto Kolmena Core. Su texto inicial ahora describe criterios reutilizables para cualquier proyecto nuevo, sin atribuirlos a una organización, proyecto o persona.

Una prueba protege la plantilla contra la reintroducción de esas referencias. Otra verifica que restaurar una constitución solo ocurre mediante una acción explícita, preservando las personalizaciones del equipo durante la instalación y la actualización.

### Documentación y seguridad

La mejora quedó formalizada en la especificación 025, con plan, contratos, modelo de datos, guía de validación y tareas convergidas con el código publicado. El archivo local `.env` se añade a las exclusiones de Git para impedir que una credencial de desarrollo se incorpore por accidente.

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
