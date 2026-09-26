# Investigación (fase 0): ciclo de vida de gomemory en consola

**Feature**: [spec.md](spec.md) · **Fecha**: 2026-09-26

Cada decisión incluye el motivo y las alternativas descartadas. Las rutas y líneas se verificaron contra `main` en `351215e` (v2.26.4).

---

## R1. Cómo se decide si hay un binario global

**Decisión**: se usa `exec.LookPath("mem")` (`mem.exe` en Windows) y se compara con `os.SameFile` contra `<proyecto>/mem`. Hay global si LookPath resuelve **y** no es el mismo archivo que la copia del proyecto. La lógica vive en un único punto, que amplía `binRefFor` (`adapters/primary/cli/binref.go:45`).

**Motivo**: `binRefFor` ya decide con LookPath cómo referencian hooks y MCP al binario. Tener una segunda regla distinta permitiría que install y los hooks discreparan sobre qué binario usar.

**Alternativas descartadas**: rutas fijas (`~/.local/bin`, `/usr/local/bin`): fallan con `GOMEMORY_BIN_DIR` y con instalaciones de Homebrew. Leer la ruta desde un archivo de estado: puede quedar obsoleta cuando la persona mueve el binario.

## R2. Cómo se identifica una copia local de gomemory antes de borrarla

**Decisión**: se ejecuta `<copia> version` con un tiempo límite de 2 s y el entorno mínimo, y se acepta solo si la salida empieza por `gomemory ` seguida de una versión semántica. Antes se comprueba que sea un archivo regular ejecutable, no un enlace simbólico ni un directorio.

**Motivo**: FR-003 prohíbe tocar archivos ajenos llamados `mem`. Ejecutar un binario desconocido tiene un riesgo acotado: el archivo ya está en el árbol del proyecto y los hooks de la persona lo ejecutarían igual. El tiempo límite evita bloqueos.

**Alternativas descartadas**: firma o hash de binarios conocidos: no se sostiene con 5 plataformas y versiones antiguas sin checksums locales. Solo el tamaño o la cabecera ELF/Mach-O: no distingue gomemory de otro binario Go.

**Nota Windows**: si el archivo está bloqueado, `os.Remove` falla. Se ignora el error y se reintenta en la siguiente ocasión (caso límite de la spec).

## R3. Aviso visible para la persona en `session-start` (FR-004a, FR-030)

**Decisión**: nuevo renderizador `renderSessionStart(dialect, contexto, aviso)` en `hook_dialect.go`, con el patrón de `renderTurnEnd` (`hook_dialect.go:235`):
- **Claude**: JSON `{"systemMessage": aviso, "hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": contexto}}`. `systemMessage` lo ve la persona; el modelo no (ver el comentario en `cmd_hook.go:478-489`).
- **JSON (Codex)**: `systemMessage` con el aviso. El contexto sigue en el canal que ya usa Codex.
- **Neutral/Text (OpenCode y otros)**: un solo canal. El aviso se antepone al contexto como una instrucción al agente: «Informa a la persona: …».

Si no hay aviso, la salida es **idéntica** a la actual (texto plano), así que las pruebas de contrato de `session-start` no cambian.

**Detección del dialecto en `session-start`** (descubierto al implementar): el payload de `SessionStart` trae `hook_event_name` pero no `tool_name`, así que `detectDialect` devuelve *neutral* también en Claude Code. Registrar `session-start --emit=claude` obligaría a modificar una prueba existente (`setup/claude_code_hooks_test.go:39`). La señal que se usa es `CLAUDE_PROJECT_DIR`, que Claude Code exporta al entorno de sus hooks. El proyecto ya depende de ella: `binRefFor` registra comandos `${CLAUDE_PROJECT_DIR}/mem`. Orden: `--emit` explícito → `claude` si existe `CLAUDE_PROJECT_DIR` → neutral.

**Motivo**: hoy `hookSessionStart` (`cmd_hook.go:109`) imprime el contexto como texto plano. Cambiar siempre a JSON rompería los dialectos planos. Emitir JSON solo cuando hay aviso en Claude mantiene la huella cero cuando no hay nada que decir.

**Alternativas descartadas**: `additionalContext` solo para el modelo: el aviso no llegaría a la persona, en contra de FR-030. Imprimir por stderr: Claude Code no lo muestra en la sesión.

**Dedupe "una vez por versión y sesión"**: marcador `.memory/.update-notice` con la versión ya anunciada y el ID de sesión. Sigue el patrón de los marcadores de `sessionMarkerPath`, que se borran al iniciar sesión (`cmd_hook.go:123`). Para las retiradas de copias no hace falta marcador: el aviso sale en la misma ejecución que borra el archivo, y ese archivo ya no existe la siguiente vez.

## R4. Consulta de versión en segundo plano (FR-028, FR-029)

**Decisión**:
- Subcomando nuevo `mem update-check`, despachado sin contenedor, igual que el fast-path de `code-refresh`.
- Hace una consulta HTTP con un tiempo límite de 5 s a la API de releases (reutiliza `latestReleaseTag`, `cmd_update.go:137`) con `If-None-Match` del `etag` guardado. Un 304 renueva solo `checked_at`.
- La caché es `<DataHome>/update-check.json` (0600), escrita de forma atómica (temporal + rename, el mismo patrón que `writeSnapshot` en `codebasememory/provider.go`).
- `session-start` lee la caché. Si falta o tiene más de 24 h, lanza el proceso con `detach` y reaping (el patrón de `MaybeRefresh`, `provider.go:128`) y no espera.

**Motivo**: la constitución (§8) exige un tiempo límite en toda llamada externa. El hook no puede depender de la red (SC-006). Con ETag, las consultas repetidas no consumen el límite de peticiones de la API de GitHub.

**Alternativas descartadas**: consultar dentro del hook con un tiempo límite corto: añade latencia a cada arranque sin red. Un demonio o una tarea programada: añade piezas y permisos sin necesidad.

**Comparación de versiones**: semver sin prefijo `v`. Las prerreleases no generan aviso, porque `releases/latest` de GitHub las excluye.

## R5. Verificación de integridad en `mem update` (FR-032)

**Decisión**: descargar `checksums.txt` del mismo release, calcular SHA-256 del archivo descargado con `crypto/sha256` y comparar con la línea `<hash>  <asset>`. Si falta el archivo, falta la línea o el hash no coincide, se aborta **antes** de `extractBinary` y `replaceSelf`.

**Motivo**: goreleaser ya publica `checksums.txt` en cada release (verificado en v2.26.3 y v2.26.4). La constitución §15 pide verificar la cadena de suministro y prohíbe criptografía propia (se usa la biblioteca estándar).

**Alternativas descartadas**: firma con cosign o minisign: requiere clave, cambios en el pipeline y una dependencia. Se deja como mejora futura, no bloquea.

## R6. `update` desde una copia local actualiza el global (FR-006)

**Decisión**: `CmdUpdate` calcula el destino como `global` si R1 dice que hay uno y `os.Executable()` es otro archivo. Si no, usa `os.Executable()` (el comportamiento actual). Tras sustituir el global, retira la copia con la regla de R2. Si el global no admite escritura (`EACCES` al crear el temporal junto a él), no descarga nada: informa del comando exacto (`sudo mem update` o `curl … | sh`) y termina con un código distinto de cero.

**Motivo**: hoy `./mem update` deja el global viejo, que es justo el síntoma que se reportó.

## R7. Consola interactiva sin dependencias nuevas (FR-020, FR-025, FR-027)

**Decisión**: componentes propios pequeños sobre `charm.land/bubbletea/v2`, `bubbles/v2` y `lipgloss/v2` (ya directos en `go.mod`, los usa la TUI): selección múltiple, selección única, confirmación, confirmación escrita (`gomemory`), lista de pasos con indicador de progreso y resumen ✓/⚠. Viven en un paquete nuevo `adapters/primary/console/`. La detección de TTY usa `github.com/charmbracelet/x/term` (`term.IsTerminal`), que ya está en `go.mod` como indirecta y pasa a directa sin añadir ningún módulo nuevo al grafo.

**Degradación (FR-027)**: si `NO_COLOR` está definida, `TERM=dumb` o el ancho es menor de 60 columnas, se usa el renderizador de texto plano. Sin TTY no se instancia bubbletea: se usa el modo equivalente a `--yes`.

**Alternativas descartadas**: `charmbracelet/huh`: es justo el tipo de componente que se necesita, pero sería una dependencia nueva (§20 prohíbe añadirla sin necesidad). Los componentes necesarios son pocos y simples. Prompts con `bufio`: no permiten selección múltiple usable.

## R8. Detección de agentes instalados (FR-020, suposición de la spec)

**Decisión**: para cada agente de `domain.KnownAgents` (`domain/agents.go:107`), hay dos señales: su ejecutable en el PATH (`claude`, `codex`, `opencode`, `cursor`) o su directorio de configuración de usuario (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.cursor`). Windsurf y Cline se ofrecen sin marcar, porque la feature 021 los sacó de la instalación automática.

**Motivo**: `KnownAgents` es el registro único de agentes y capacidades. Usarlo evita una segunda lista.

## R9. Persistencia de la selección (FR-023)

**Decisión**: campos nuevos en `persistence.Settings` (`settings.go`): `agents []string` y `agent_scope string` (`project`/`global`), ambos con `omitempty`. Un proyecto sin `agents` usa los detectados (R8), así que las instalaciones existentes no cambian de golpe. `mem update` ya lanza `install` como subproceso (`cmd_update.go:115`) y, sin TTY, leerá la selección guardada.

## R10. Registro de proyectos y escaneo (FR-013)

**Decisión**:
- **Registro**: `install` escribe `<DataHome>/projects/<key>/root` (0600) con la ruta absoluta, de forma atómica e idempotente, mediante una función nueva de `persistence` junto a `HardenGlobalStore`.
- **Escaneo**: `filepath.WalkDir` desde la raíz. Se detiene a 6 niveles, **no** sigue enlaces simbólicos, omite `.git`, `node_modules`, `vendor`, `.venv`, `target`, `dist`, `build`, `Library` (macOS) y los directorios ocultos de caché (`.cache`, `.npm`, `.cargo`, …). Cuenta como proyecto el que tenga `.memory/settings.json` legible con claves de gomemory. Los errores de permiso se acumulan para el resumen (caso límite de la spec).
- **Unión**: registrados ∪ escaneados, sin duplicados por `ProjectKey`.

**Motivo**: hoy la clave es slug + hash (`globalstore.go:82`) y no se puede invertir. Un escaneo acotado cumple la aclaración de la sesión del 2026-09-26.

## R11. Desinstalación: inventario y retirada (FR-007…FR-019)

**Decisión**:
- **Inventario**: los canales de activación de usuario y de proyecto se obtienen con `ActivationInspector.Inspect` (`setup/activation_inspect.go:42`), que ya recorre MCP, hooks, instrucciones, el plugin de OpenCode y la entrada de Codex por agente y ámbito. A eso se suman el binario (R1), el almacén (`DataHome`, con tamaños) y la exportación. Un solo recorrido alimenta tanto `--dry-run` como la ejecución real (SC-004).
- **Retirada por proyecto**: se reutilizan las funciones actuales de `cmd_uninstall.go` (`removeIntegrationBlocks`, `removeMCPEntries`, `removeClaudePlugin`, `removeClaudePermissions`, `removeOpenCodeArtifacts`, `removeNativeWrappers`) y se añade `DataHome/projects/<key>` (corrige FR-010).
- **Retirada global**: las mismas funciones de edición parcial, aplicadas a las rutas de usuario (`~/.claude.json`, `~/.claude/settings.json`, `~/.config/opencode/…`, `~/.codex/config.toml`), más los envoltorios globales de `atomic_plan_global.go`. Nunca se borra un archivo compartido entero (FR-012). Si un archivo no se puede interpretar, se deja como está y se marca con ⚠.
- **Orden (FR-014)**: exportación, luego proyectos, configuración global, `DataHome` y, al final, el binario propio. En Windows el binario se borra de forma diferida con `cmd /c ping -n 3 127.0.0.1 >nul & del "<ruta>"` lanzado con `detach`.
- **Hoy la política de proyecto no toca la configuración global de Codex** (`cmd_mcp_setup.go:846-852`), para no quitársela a otros proyectos. Se mantiene: la configuración global solo se retira en el alcance de sistema.

**Desviaciones al implementar (2026-09-26)**:
- El inventario global se deriva de la **matriz de canales** (`domain.CellsForActivity`), no de `ActivationInspector`: la matriz es la declaración única de artefactos por agente y ámbito, y `uninstall` ya la consumía. Se añadió la actividad `ActivityUninstallGlobal` (ámbito de usuario), gemela de `ActivityInstallGlobal`, con su prueba de simetría. Faltaba la celda `~/.claude.json` (servidor MCP de Claude a nivel de usuario, que escribe `setup-mcp --scope global`): sin ella ninguna desinstalación lo retiraba.
- El planificador vive en el adaptador primario (`cli/uninstall_plan.go`) y no en `application/usecases`: necesita hechos del disco y utilidades de `cli` (`resolveGlobalBinary`, `identifyLocalCopy`). La lógica de negocio (orden FR-014, continuidad ante ⚠, estados) está en `domain.UninstallPlan`. La exportación por lotes sí es un caso de uso (`usecases.ExportProjects`).
- `uninstall` pasa a los comandos sin contenedor (`rootIndependentCommands`): antes abría la base del proyecto actual justo antes de borrar el almacén. `CmdUninstall` devuelve el código de salida y el despachador hace `os.Exit`, para que las pruebas en proceso sigan funcionando.
- La instalación global deja respaldos `~/.codex/*.gomemory-*.bak` y el bloque de instrucciones universales en los archivos de instrucciones globales: ambos se retiran.

## R12. Exportación con permisos privados (FR-009a) — defecto latente

**Hallazgo**: `CmdExport` (`cmd_export.go:34`) crea el archivo con `os.Create`, que con la umask habitual deja 0644. Una exportación de memoria queda legible por otros usuarios, la misma clase de fuga que se cerró en v2.26.3 y v2.26.4.

**Decisión**: `CmdExport` (y la exportación por lotes) crea el archivo con `os.OpenFile(..., 0o600)`. La exportación por lotes crea su directorio con 0700. Se corrige en el mismo cambio, porque esta feature la reutiliza (regla 7: un defecto latente que activa el propio cambio se paga en ese cambio).

**Exportar proyectos sin ruta conocida**: `ExportProject` recibe repositorios abiertos por proyecto. Se añade una apertura del almacén por **clave** (`persistence.OpenByKey`, gemela de `Open`), para exportar los proyectos del almacén cuya ruta no se conoce.

## R13. Variables de entorno documentadas (FR-031, constitución §9)

**Decisión**: crear `.env.example` en la raíz con `GOMEMORY_DATA_HOME`, `GOMEMORY_RELEASE_DOWNLOAD_BASE`, `GOMEMORY_NO_UPDATE_CHECK` (nueva) y `GOMEMORY_BIN_DIR` (instalador), cada una con propósito, ejemplo seguro, obligatoriedad y unidad. `NO_COLOR` se respeta como convención estándar y no se documenta como propia.

**Motivo**: §9 exige documentarlas en `.env.example` y hoy ese archivo no existe (brecha previa que se cierra aquí).

## R14. Instaladores de consola (FR-026)

**Decisión**: al final de `scripts/install.sh`, si `[ -t 1 ] && [ -r /dev/tty ]` y el directorio actual es un repositorio git, se pregunta `¿Configurar gomemory en <dir>? [S/n]`, leyendo de `/dev/tty`, porque con `curl | sh` stdin es la tubería. Si la respuesta es sí, se ejecuta `mem install . </dev/tty`. `install.ps1` hace lo equivalente con `[Environment]::UserInteractive` y `Read-Host`. Sin TTY, se mantiene el mensaje actual del siguiente paso.
