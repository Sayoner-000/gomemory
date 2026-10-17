# Auditoría Defensiva Multilenguaje (ronda 2) — 2026-10-17

- **Revisión auditada:** `a75ca31fac331756569d07ed4121f67ff1c5a6b7` — **árbol SUCIO** (33 archivos modificados + 6 sin seguimiento, trabajo en curso de socratica). Mismo contenido de código que la ronda 1 (`docs/audits/2026-10-17-defensive-qa.md`); ninguna escritura de producción entre rondas.
- **Alcance de esta ronda:** superficies que la ronda 1 dejó fuera de foco — **plugin TypeScript de OpenCode** (`infrastructure/plugin/opencode/gomemory.ts`), **instalador Node** (`installer/src/*.ts`), **instaladores shell** (`scripts/install.sh`, `scripts/install.ps1`), **scripts Python de evidencia**, **superficie MCP** (`adapters/primary/cli/cmd_mcp*.go`: tools, recursos y plantillas de recurso), **persistencia** (construcción de SQL dinámico), **rendering de consola/TUI** de contenido almacenado, y **workflows CI / goreleaser**. Se reverifican además los cierres previos.
- **Excluidos:** `node_modules`, `.venv`, corpus de pruebas, fixtures, `docs/audits/*` previos.
- **Modo:** solo lectura; sin correcciones de producción. Este documento es el único archivo creado.

## 1. Resumen de salud del código

**Lenguajes / tecnologías:** Go (producción), TypeScript (instalador `installer/src`, plugin `infrastructure/plugin/opencode/gomemory.ts`), Python (scripts de evidencia 037/038/039), Bash (scripts, plantillas Spec Kit), PowerShell (`scripts/install.ps1`), YAML CI (release/test), Markdown/JSON.

**Nivel de riesgo general: Bajo.**

**Evaluación breve:** el código sigue notablemente endurecido y las superficies nuevas de esta ronda están bien construidas. El instalador Node resuelve la versión una sola vez, valida `tag_name` con regex, verifica SHA-256 **antes** de extraer y restringe la extracción al miembro exacto (`filter` por nombre y tipo `File`; `unzipSync` por nombre), publica con patrón *stage + rename* y restaura el binario anterior en Windows. El shell hace lo mismo (solo extrae `mem`, valida que sea ejecutable de la plataforma, y no instala nada si el checksum no cuadra). El **plugin TypeScript no registra logs ni escribe archivos** y ejecuta `mem` con separación de argumentos (sin shell). La **construcción de SQL dinámico** (`privacy_maintenance.go`, `socratica_sanitize.go`, `globalstore.go`) usa **únicamente literales fijos/whitelist** (`ledgerColumns`, `legacyProjectTables`) — nunca entrada de usuario. Las **acciones de CI están fijadas por SHA**, `permissions` está declarado y acotado, no hay `pull_request_target`/`workflow_run`, y no se interpola `github.event.*` dentro de `run:`. Los hallazgos de esta ronda son de **higiene defensiva**; ninguno es una fuga explotable remota.

**Resultado:** 3 hallazgos (0 críticos, 0 altos, 0 medios, 2 bajos, 1 informativo/sospechoso).

### Cobertura y validación (esta ronda)

| Comprobación | Resultado |
|---|---|
| `go build ./...` + `go vet ./...` | Pasa |
| `go test ./...` (completo) | Pasa (0 fallos) |
| `go test -race` en `persistence`, `usecases`, `cli`, `domain` | Pasa, **salvo 1 ejecución de `./adapters/primary/cli` que falló (1/5)** — ver H-03 |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa — 46/46 |
| `npm audit --omit=dev` (instalador) | Pasa — 0 vulnerabilidades |
| `npx tsc --noEmit` (instalador) | Pasa |
| `bash -n` sobre `scripts/*.sh` | Pasa |
| `python3 -m py_compile scripts/*.py` | Falla **solo** en `scripts/acr-smoke.py` con el intérprete del host (3.9.6): usa f-strings con comillas anidadas del mismo tipo (PEP 701, ≥3.12). El mínimo declarado del proyecto es 3.12 (`test.yml:26`), así que no es defecto en el piso declarado (ver §4) |
| Acciones CI fijadas por SHA + `permissions` + disparadores | Pasa (`release.yml`: tag-only, `contents: write` con job `govulncheck` en `read`; `test.yml`: push/PR, `contents: read`) |
| Sin `pull_request_target` / `workflow_run` / `github.event.*` en `run:` | Pasa |
| SQL dinámico acotado a whitelist | Pasa (`ledgerColumns`, `legacyProjectTables`) |
| Extracción de archivos (zip-slip) | Pasa — miembro exacto en TS, `unzipSync` por nombre, `tar x` con `filter` `path===name && type==='File'`, y en sh solo `tar -xzf … "$BIN_NAME"` |
| Cierres previos 2026-10-16 (H-01..H-06) vigentes | Pasa — reverificados por lectura en el árbol actual |

## 2. Hallazgos

| ID | Severidad | Estado | Problema |
|---|---|---|---|
| H-01 | Baja | CERRADO | El contenido almacenado se renderiza en consola/TUI sin depurar secuencias de control (inyección de escape de terminal) |
| H-02 | Baja | CERRADO | Los instaladores no acotan el tamaño de descarga/descompresión antes de verificar la integridad |
| H-03 | Informativo | NO REPRODUCIDO | `go test -race ./adapters/primary/cli` falla de forma intermitente (1/5); hipótesis: carrera sobre el global `os.Stdout` en `captureStdout` |

### H-01 — Inyección de secuencias de control al renderizar contenido almacenado

- **Categoría:** Inyección de escape / endurecimiento de salida (CWE-150).
- **Severidad:** Baja.
- **Estado:** CONFIRMADO (por lectura; determinista).
- **Ubicación:** `adapters/primary/cli/cmd_mcp_render.go:50` (`renderMemoryDetail` imprime `m.Content` crudo), `adapters/primary/cli/cmd_get.go:29` (`mem get`), `adapters/primary/cli/cmd_search.go:51` (`ansi.Truncate` es *ANSI-aware*: conserva las secuencias), `adapters/primary/console/components.go:72-76` (`fit`/`Panel` dejan pasar la línea), `adapters/primary/tui/tui.go:816` (`ansi.Wrap(m.selected.Content, …)`) y `adapters/primary/tui/tui.go:2130` (`copyableText`).
- **Ruta de alcance:** `mem get`, `mem search`, `get_context`/`mem://context` y la pantalla de detalle del TUI, sobre cualquier memoria cuyo texto contenga `ESC` (`\x1b`) u otros caracteres de control.
- **Análisis:** la ruta de guardado solo **redacta secretos y bloques `<private>`** (`adapters/secondary/persistence/memory.go:71-72`; `domain.ProtectText` = `RedactSecrets`+`RedactPrivate`, `domain/redact.go:166`), pero **no elimina caracteres de control C0/C1**. No existe ningún `ansi.Strip`/`IsControl` sobre el contenido almacenado en las rutas de render (grep en `domain/`, `application/`, `adapters/`). Como consecuencia, una memoria que contenga una secuencia OSC — p. ej. `\x1b]52;c;<base64>\x07` (reescritura del portapapeles), `\x1b]0;…\x07` (título de ventana) o `\x1b[…` (estado del cursor/terminal) — se **emite tal cual** al terminal al mostrarla. La ingesta no requiere privilegio: contenido pegado, `mem import`, o el store global compartido entre proyectos.
- **Evidencia:**
  - `renderMemoryDetail` concatena `m.Content` sin depurar (`cmd_mcp_render.go:50`).
  - `cmd_search.go:51`: `content = ansi.Truncate(content, 60, "...")` — la truncación ANSI conserva los escapes.
  - `components.go:72-76`: `fit` solo trunca/rellena con `ansi.Truncate`/`ansi.StringWidth`; no neutraliza.
  - Búsqueda global: no aparece `ansi.Strip` ni `unicode.IsControl`/`IsPrint` sobre contenido en el árbol de producción (solo `ansi.Strip` para **medir** anchos en `layout.go`/`components.go` y en dos puntos del TUI).
  - Contraste: el camino de **portapapeles** sí protege (`cmd_socratica.go:526` aplica `domain.ProtectText` y codifica en base64, lo que además neutraliza el escape) y está **gated** por `c.sio.tty && TERM != dumb` — no hay hallazgo aquí; el hueco está en el render directo a terminal.
- **Código corregido (defensivo, mínimo):** depurar en el choke point de render, p. ej. una función `sanitizeTerminal(s string) string` que elimine `C0`/`C1` salvo `\t` y `\n` (y opcionalmente neutralice `ESC`), aplicada en `renderMemoryDetail`, en `Panel`/`memoryPanel` y en el detalle del TUI. Alternativamente, depurar una vez en `insertMemory` (cubre import/checkpoint), conservando el original si se necesita el crudo.
- **Validación propuesta:** guardar una memoria cuyo contenido incluya `"\x1b]0;PWNED\x07"` y verificar que `mem get <id>` la muestra **literal** (sin cambiar el título de la terminal) en terminal real; test de regresión que falle con el código actual.

### H-02 — Sin cota de tamaño en la descarga/descompresión del instalador

- **Categoría:** Consumo de recursos / endurecimiento de cadena de suministro.
- **Severidad:** Baja.
- **Estado:** CONFIRMADO (por lectura).
- **Ubicación:** `installer/src/bootstrap.ts:12-17` (`get` → `response.arrayBuffer()`, sin límite), `installer/src/bootstrap.ts:82-83` (descarga `asset` + `checksums.txt` y **después** verifica), `installer/src/bootstrap.ts:52` (`unzipSync(archive, …)`: aunque filtra por nombre, la entrada seleccionada se infla completa), `scripts/install.sh:195-202` (`curl -fsSL … -o "$tmp/$asset"` sin `--max-filesize`).
- **Análisis:** la integridad se verifica con SHA-256, pero **después** de materializar el archivo completo: el instalador Node bufferea la respuesta entera en memoria (`arrayBuffer`) y, en Windows, la descomprime en memoria con `unzipSync` **antes** de que exista ninguna cota; el shell escribe a un temporal en disco sin tope. Un asset sobredimensionado o una bomba de descompresión (origen comprometido, proxy/CDN intermedio, o fichero manipulado) consume memoria/disco antes de que el checksum pueda rechazarlo, abortando la instalación con OOM/lleno de disco en vez de fallar limpio. Mitigaciones ya presentes: TLS a `github.com`, `tag_name` validado por regex y checksum del mismo release (que el propio script documenta como «integridad publicada, no firma independiente»). Es, por tanto, un endurecimiento barato, no una fuga.
- **Evidencia:** `bootstrap.ts:14` (`AbortSignal.timeout` pero sin tope de bytes); `bootstrap.ts:82` descarga en `Promise.all` y `:83` verifica después; `install.sh:195` sin `--max-filesize`.
- **Código corregido (defensivo, mínimo):** en `get`, rechazar cuando el `Content-Length` o el acumulado superen un tope (p. ej. 128 MiB), y cortar el stream; en `unzipSync`, comprobar `size`/ratio antes de inflar; en `install.sh`, añadir `--max-filesize`.
- **Validación propuesta:** test unitario de `bootstrap` con un `fetch` simulado que devuelve `Content-Length` enorme → debe fallar antes de bufferear; repro shell con un asset sobredimensionado y `curl --max-filesize`.

### H-03 — Fallo intermitente de `go test -race ./adapters/primary/cli` (sospechoso)

- **Categoría:** Calidad de pruebas / no determinismo (posible carrera).
- **Severidad:** Informativo (impacta CI).
- **Estado:** SOSPECHOSO — **1 fallo observado en 5 ejecuciones**; no reproducido en las 4 ejecuciones siguientes del paquete completo ni en 10 ejecuciones del subconjunto de desinstalación (`-run 'TestRemoveProjectIntegration|TestRemoveOpenCodeArtifacts|TestUninstall'`).
- **Ubicación:** `adapters/primary/cli/cmd_plan_context_test.go:35-62` (`captureStdout`), CI en `.github/workflows/test.yml:38` (ejecuta `-race` sobre `./adapters/primary/cli`).
- **Análisis:** `captureStdout` **sustituye el global `os.Stdout`** (`os.Stdout = w`) y lo restaura al finalizar, con una goroutine lectora del pipe. Cualquier goroutine que siga viva y escriba a `os.Stdout` (p. ej. `hook_tool_output.go:204` (`withBudget`, que «puede seguir en segundo plano» tras vencer el plazo) o `install_process_unix.go:61,84` (`watchParent`/`watchOutput`)) provoca que el detector de carreras marque el acceso concurrente al global `os.Stdout`. La salida capturada en el fallo correspondía a la prueba de desinstalación de proyecto (`uninstall_plan.go:216-227`), que además corre **sin aislar `HOME`** (`cmd_uninstall_test.go:10-24`) y por tanto **lee el HOME real** — su nota de regresión advierte justamente de ese riesgo (`cmd_uninstall_opencode_test.go:150-158`).
- **Evidencia:** ejecución `go test -race ./adapters/primary/cli -count=1` → `FAIL mem/adapters/primary/cli` (42.9 s) en la 1ª de 5; salida final con las líneas de `uninstall` sobre `/Users/<usuario>/.config/opencode/plugins/gomemory.ts`; 4 ejecuciones posteriores `ok`. La línea exacta del banner (`--- FAIL: Test…` o `WARNING: DATA RACE`) **no quedó capturada**, por eso el estado es SOSPECHOSO y no CONFIRMADO.
- **Cierre propuesto:** aislar `HOME` (`t.Setenv("HOME", t.TempDir())`) en `TestRemoveProjectIntegration_*`, y sustituir el swap del global `os.Stdout` por un `*os.File` inyectado (o `t.Setenv`-style con `os.Pipe` por canal) para que el detector de carreras no pueda marcar el global. Validación: 20 ejecuciones de `go test -race ./adapters/primary/cli -count=1` sin fallos y sin bajar de 0 avisos de carrera.

## 3. Superficies revisadas sin hallazgo (evidencia)

- **Plugin TypeScript** (`infrastructure/plugin/opencode/gomemory.ts`): sin `console.*`, sin `writeFile`/`appendFile`, sin `import()` dinámico; `mem` se ejecuta con separación de argumentos vía `$`${BIN} ${args}`` (interpolación de array, sin shell) y `execFileExec` usa `spawn` con `detached` para cancelar el árbol, `EXEC_TIMEOUT_MS`/`EXEC_MAX_BUFFER` y `stderr` a `ignore` (no filtra diagnóstico ni credenciales). Suite `node --test`: 46/46.
- **Instalador Node** (`installer/src/*.ts`): resolución única de versión + regex de `tag_name`; `verifyChecksum` con `timingSafeEqual` y regex de 64 hex (misma longitud → no lanza); extracción por miembro exacto y tipo `File`; `placeBinary` con *aside/restore* en Windows; `--binary` solo comprueba `X_OK`/`isFile` (confianza local, por diseño). `tsc` y `npm audit` limpios.
- **Instaladores shell** (`scripts/install.sh`, `scripts/install.ps1`): verifican checksum antes de extraer; extraen solo `mem`; validan ejecutable/PE (magic `MZ`); `install.ps1` selecciona la entrada `mem.exe` exacta (sin traversal) y publica con *replace* atómico.
- **Python de evidencia**: `subprocess` con listas de argumentos (sin `shell=True`), sin `extractall` sobre entrada no confiable (`audit_037_smoke.py` **crea** archivos con symlinks hostiles como *fixture* para probar las defensas del instalador).
- **Superficie MCP** (`cmd_mcp.go`): recursos `mem://context` y plantilla `mem://memory/{id}` resuelven por `Get` con id numérico (`Sscanf("%d")`), sin rutas de archivo ni traversal; `pack_retrieve` resuelve `ref` contra la tabla `compression_originals` con consultas parametrizadas.
- **Persistencia**: identificadores SQL dinámicos provenientes de constantes/whitelist (`ledgerColumns`, `legacyProjectTables`); el resto son consultas parametrizadas; aperturas con `busy_timeout` + `journal_mode(WAL)` (y `_txlock=immediate` en la DB de proyecto).
- **CI / goreleaser**: acciones fijadas por SHA; `permissions` acotado por workflow y por job; release solo con tags; `sync-version` usa `env:` (no interpolación en `run:`); Dependabot con comentario de versión.

## 4. Observaciones sin hallazgo

- `scripts/acr-smoke.py` (línea 80) usa f-strings con comillas del mismo tipo anidadas (PEP 701), que requieren **Python ≥3.12**; con el Python 3.9.6 del host no compila. El piso declarado del proyecto es 3.12 (`test.yml:26`), así que no es defecto en el mínimo soportado; se anota porque el script **no está cubierto por CI** y no documenta su propio mínimo.
- El árbol de `docs/audits/` cambió entre rondas (el informe de la ronda 1 fue actualizado por la persona con el anexo A del ACR); **el código no cambió**.

## 5. Límites de esta ronda

- Herramientas no disponibles en el host: `govulncheck`, `staticcheck`, `gitleaks`, `semgrep`, `shellcheck`, `trufflehog` (no instaladas). La verificación de vulnerabilidades de dependencias se apoya en `npm audit --omit=dev` y en el job `govulncheck` de `release.yml`.
- Sin ejecución en Windows/PowerShell real (solo lectura del script y del flujo CI `windows`).
- El host tiene **Python 3.9.6**, por debajo del mínimo declarado 3.12: `py_compile` no es representativo para los scripts que usen sintaxis ≥3.12.
- H-03 no se reprodujo de forma determinista; el veredicto SOSPECHOSO es deliberado hasta capturar el banner del fallo.
- Revisión dirigida por superficies; sin `staticcheck`/análisis de flujo no exhaustivo.

## Anexo — Cierres aplicados (2026-10-17)

| ID | Cierre | Validación |
|---|---|---|
| H-01 | `domain.StripTerminalControls` (quita C0 salvo `\t`/`\n`, DEL y C1; CRLF→LF; UTF-8 inválido→U+FFFD) se aplica **al leer** en los 6 escaneos de `persistence/memory.go` vía `finishScannedMemory`, sobre título, contenido, ruta, prompt originante y clave de tópico. Al leer también quedan cubiertas las filas antiguas y todos los consumidores (CLI, TUI, MCP). No va en `ProtectText` porque esta protege los prompts de Socratica, que se conservan byte a byte | `TestStripTerminalControls` y `TestMemoryRead_NeutralizaSecuenciasDeTerminal` (fila escrita en crudo, como la dejaría una versión anterior; falla sin la guarda). Binario real con `GOMEMORY_DATA_HOME` aislado: `mem get`, `mem search` y `mem list` muestran `]0;PWNED` sin ESC/BEL |
| H-02 | `installer/src/bootstrap.ts`: `download(url, limit)` rechaza un `Content-Length` mayor que el tope sin leer el cuerpo y corta la lectura acumulada. Topes: metadatos y checksums 1 MiB, archivo 128 MiB (los assets reales pesan unos 8 MB), binario 256 MiB (`originalSize` del zip y `entry.size` del tar). `scripts/install.sh`: `curl --max-filesize` en las tres descargas | 3 pruebas nuevas en `bootstrap.test.mjs` (suite 24/24). `curl --max-filesize` contra el release v2.34.0 real: con tope de 1 MiB, exit 56; con 128 MiB descarga |
| H-03 | Sin cambio de código: no se reprodujo en 28 pasadas de `-race` del paquete (8 en orden normal y 20 con `-test.shuffle=on`). La hipótesis no se sostiene por lectura: el paquete no tiene pruebas `t.Parallel()` y `withBudget` devuelve bytes en vez de escribir en `os.Stdout`. `TestRemoveProjectIntegration_*` no escribe en `HOME`: un `HOME` señuelo con `.codex/config.toml`, `.claude.json`, `.claude/settings.json` y el plugin de OpenCode quedó idéntico byte a byte | Si vuelve a fallar, el banner (`WARNING: DATA RACE` o `--- FAIL`) quedará en el log del job `-race` de `test.yml`; con él se reabre con evidencia en lugar de aplicar el cambio especulativo sobre `captureStdout` (usado en 14 archivos) |

**Brecha abierta:** `scripts/install.ps1` sigue sin tope de tamaño (`Invoke-WebRequest -OutFile`). Propuesta: antes de descargar, una petición `HEAD` que rechace un `Content-Length` mayor de 128 MiB, y después de descargar, un tope sobre `(Get-Item $zip).Length`. No se aplicó porque el host no tiene PowerShell para validarlo; requiere verificación en Windows.
