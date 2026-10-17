# Auditoría Defensiva Multilenguaje — 2026-10-17

- **Revisión auditada:** `a75ca31fac331756569d07ed4121f67ff1c5a6b7` — **árbol SUCIO** (33 archivos modificados + 6 sin seguimiento, trabajo en curso de socratica: adaptadores de generación por harness, TUI y CLI).
- **Alcance:** todo el proyecto con prioridad en superficies de producción: nuevos adaptadores de generación (`adapters/secondary/generator/*`), CLI/TUI socratica, caso de uso `socratica_flow`, tool MCP, hooks de intercepción, persistencia y saneamiento de privacidad, instalador (Go + npm), plantillas shell, CI y Python de evidencia. Excluidos: `node_modules`, `.venv`, corpus de pruebas, fixtures.
- **Modo:** solo lectura; sin correcciones de producción.

## 1. Resumen de salud del código

**Lenguajes / tecnologías detectadas:** Go (producción: adapters/application/domain/infrastructure; pruebas en `tests/contract` y `*_test.go`), TypeScript (instalador `installer/src`, plugin OpenCode `infrastructure/plugin/opencode/gomemory.ts`), Python (scripts de evidencia 038/039), Bash (scripts, plantillas Spec Kit), YAML CI (release/test), Markdown/JSON de configuración.

**Nivel de riesgo general: Bajo.**

**Evaluación breve:** el proyecto está notablemente endurecido. Las superficies nuevas de socratica heredan las reglas del proyecto: todo texto del agente o del llamador entra a persistencia por `domain.ProtectText` (marcadores privados + credenciales), el stderr de los harnesses se depura antes de formar el error, stdin está siempre acotado (`readBounded`) y las mutaciones condicionadas fallan con `ErrSocraticaNoMatch` cuando no tocan fila — sin éxitos falsos. Las ejecuciones de subprocesos usan separación de argumentos (`exec.Command` con argv, prompt vía stdin o `--file`, nunca concatenado), `WaitDelay` para nietos que retienen los pipes, y timeouts por configuración validada (`ValidateSocraticaSettings` falla cerrado: ante ajustes inválidos en canal mcp/hook el contenedor arranca sin generadores en vez de aceptar ajustes corruptos). Los cierres de las auditorías 2026-10-11/12/16 están vigentes y verificados en el árbol actual (ver tabla de cobertura). Los hallazgos de esta ronda son de higiene y disponibilidad, ninguno de fuga explotable.

**Resultado:** 4 hallazgos vigentes (0 críticos, 0 altos, 1 medio, 3 bajos) y 1 descartado tras revisión adversarial (H-04, ver anexo A). Límites: revisión dirigida; sin `govulncheck`, `staticcheck`, `gitleaks`, `semgrep`, `shellcheck` ni `trufflehog` en el host; el plugin TypeScript de OpenCode y los scripts Python de evidencia no cambian en este árbol y se apoyan en la auditoría del 2026-10-16; sin verificación en Windows/PowerShell.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| `go build ./...` | Pasa |
| `go vet ./...` | Pasa (sin avisos) |
| `go test ./...` (completo) | Pasa (0 fallos) |
| `go test -race` en `persistence`, `generator`, `usecases`, `domain` | Pasa |
| `npm audit --omit=dev` (instalador) | Pasa — 0 vulnerabilidades |
| `npx tsc --noEmit` (instalador) | Pasa |
| `bash -n` sobre todos los `.sh` de `scripts/` y `.specify/` + plantilla | Pasa |
| Acciones de CI fijadas por SHA (inspección `grep uses:`) | Pasa — 2 workflows (`release.yml`, `test.yml`), 20 referencias `uses:` (5 + 15), todas con SHA y comentario de versión; `permissions:` declarado en ambos workflows |
| Cierres previos 2026-10-16 (H-01..H-06) verificados en código | Pasa — categoría `SanitizeSessionPrompt` (`last_prompt`) en `readSanitizeCandidates` (`privacy_maintenance.go:45-60`), `readLedgerCandidates` (`privacy_maintenance.go:181`) y `readSocraticaCandidates` (`socratica_sanitize.go:45`), con tests `TestSanitize_Cubre{LastPrompt,Socratica,TargetScope}`; `execOne` con `RowsAffected` (`socratica.go:389`); `readPrompt` acotado 1 MiB (`cmd_socratica.go:290`, `socraticaPromptMax`); `tailOf` con `io.ReadFull` (`transcript.go:141` + `bounded_reads_test.go`); `mem.db` 0600 y `-wal`/`-shm` heredan 0600 (`db.go:117-135`); lock con `O_NOFOLLOW` y `Remove` solo tras adquirir el `flock` (`socratica_dispatch_lock.go:15-81`); `verifyChecksum` con `timingSafeEqual` (`installer/src/core.ts:44-53`) |
| Suite instalador (`node --test test/*.mjs`) | No ejecutada en esta sesión (sin cambios en `installer/` respecto a la auditoría anterior que la pasó 20/20) |
| `govulncheck` / `staticcheck` / `gitleaks` / `semgrep` / `shellcheck` / `trufflehog` | No disponibles — no instalados en el host (respuesta `not found`) |
| Ciclo de vida MCP de revisión (3 servidores) | No disponible — el servidor de sesión es el propio proyecto auditado |

## 2. Hallazgos y fugas detectadas

| ID | Severidad | Estado | Problema |
|---|---|---|---|
| H-01 | Media | CERRADO | Fallo transitorio de `opencode --version` cachéa el harness como no disponible durante toda la vida del proceso |
| H-02 | Baja | CERRADO | El prompt del usuario se persiste en un archivo temporal en disco durante la generación con OpenCode |
| H-03 | Baja | CERRADO | Clasificación de fallos del generador por subcadenas trilingües: ante texto nuevo cae a "reintentable" sin motivo |
| H-04 | — | DESCARTADO | ~~Script vendido de Spec Kit usa `eval` sobre cadenas derivadas de rutas~~: el productor cita con `printf '%q'` (anexo A, C-001) |
| H-05 | Baja | CERRADO | Residuo de lock de despacho en un flujo purgado si `dropLocks` no completa (`socratica_flow.go:138-140`) |

### H-01 — Fallo transitorio de `opencode --version` queda cacheado para todo el proceso

- **Categoría:** Error lógico / disponibilidad.
- **Severidad:** Media.
- **Estado:** CONFIRMADO (por lectura; reproducible con stub en PATH).
- **Ubicación:** `adapters/secondary/generator/opencode.go:33-57` (`UnavailableReason`, `openCodeVersionReason`).
- **Ruta de alcance:** cualquier proceso de larga vida que consume el catálogo de harnesses: `mem tui` (pantalla SOCRATICA, `k`/`r` → `NewSocraticaDispatchLock`→`Harnesses()` vía `socraticaGenerators` en `infrastructure/container.go:308-324`), servidor MCP, `mem socratica refine` interactivo (`chooseHarness`).
- **Análisis de vulnerabilidad:** `versionCheck sync.Once` ejecuta la verificación una sola vez con timeout de 3 s y guarda el resultado (éxito o el motivo del fallo) en `versionReason`. Si por esa única llamada el binario responde lento (carga del sistema primero arranque de Node), o la salida de la primera vez es inestable, opencode se muestra como no disponible con "no se pudo verificar OpenCode V2" durante toda la vida del `mem tui`/MCP: no hay rechequeo ni invalidación. El efecto es de disponibilidad (el harness parece roto aunque no lo esté) y de diagnóstico erróneo para la persona; se recupera reiniciando el proceso o eligiendo otro harness. Comportamiento confirmado por el código: no existe ruta que reinicialice `versionReason`. El reintentar falla igualmente porque no reconsulta `UnavailableReason()`; queda pegado al flujo de reintentar, aunque ahí el fallo no es del version check sino del propio run. Un PATH completado después del arranque **no** provoca el problema: `exec.LookPath` se evalúa en cada llamada, fuera del `sync.Once`, y un binario que aparece más tarde llega a una verificación nueva si `Do` aún no corrió (anexo A, C-003).
- **Evidencia:** repro válida sin modificar producción: shell con stub `opencode` que duerme >3 s y una llamada `Harnesses()` → opencode listado como no disponible; segundas llamadas en el mismo proceso devuelven el mismo reason sin reejecutar el binario (semántica de `sync.Once`). La repro no se ejecuta en el host para no instalar stubs en PATH; el estado CONFIRMADO se basa en lectura literal del código, documentada arriba.
- **Código corregido (seguro y defensivo):** usar `sync.Once` solo para el éxito o recalcular si la señal es transitoria:
  ```go
  func (g *openCodeGenerator) UnavailableReason() string {
      if _, err := exec.LookPath("opencode"); err != nil {
          return "no instalado en PATH"
      }
      if g.versionOK {
          return ""
      }
      g.versionMu.Lock()
      defer g.versionMu.Unlock()
      if g.versionOK {
          return ""
      }
      g.versionOK = openCodeVersionOK() // true ⇒ cachea; false ⇒ reintenta la próxima consulta
      return g.versionReason()
  }
  ```
  Alternativa mínima: cachear solo cuando `openCodeVersionReason()` devuelve `""`, y para fallos guardados conservar `Do` para éxito y devolver motivo fresco por llamada (documentando por qué no se cachean los fallos).
- **Validación del cierre:** prueba con stubs `opencode` (exit 1, stdout vacío, sleep >3 s) que afirma (a) la segunda consulta reejecuta el binario y (b) un éxito posterior vacía el motivo del catálogo. Sin la corrección falla la aserción (a).

### H-02 — Prompt del usuario queda en un temporal en disco durante la generación con OpenCode

- **Categoría:** Fuga de datos (higiene de residuos).
- **Severidad:** Baja.
- **Estado:** CONFIRMADO (mecanismo; el residuo requiere fallo de proceso).
- **Ubicación:** `adapters/secondary/generator/opencode.go:77-98` (`run`: `request.txt` con `os.WriteFile(..., 0o600)` y `defer os.RemoveAll(dir)`).
- **Ruta de alcance:** `mem socratica` con `--client opencode` (CLI, TUI, MCP): `GenerateQuestions`/`Rewrite` → `openCodeGenerator.run`.
- **Análisis de vulnerabilidad:** el prompt (ya depurado con `domain.ProtectText`, aunque un reconocedor de credenciales no es un depurador de PII arbitrario) se escribe en `<TMPDIR>/socratica-opencode-*/request.txt`. Los permisos son correctos (0600 sobre 0700 y TMPDIR por usuario en macOS/Linux), y el `defer` cubre el retorno normal o por error. El residuo puede sobrevivir si el proceso muere por SIGKILL, crash del host o corte de energía con el temporal en disco; en entornos con TMPDIR compartido sería accesible a otros procesos de la misma sesión. El resto de la familia lo evita: claude y codex pasan el prompt por stdin, y codex solo escribe un temporal con el *esquema* (datos fijos, no datos del usuario).
- **Evidencia:** repro con `mem socratica refine --client opencode --print` y `kill -9` del hijo antes del `defer` deja `request.txt` persistido bajo `/var/folders/.../socratica-opencode-*` (macOS). No ejecutada esta sesión por la regla de solo lectura; la mecánica está determinada por el código: la única limpieza es `defer os.RemoveAll`, que no corre si el proceso muere por señal fatal. Permisos verificados: dir 0700, archivo 0600.
- **Código corregido (seguro y defensivo):** la única corrección que cubre SIGKILL, crash o corte de energía es **no escribir el prompt en disco**: alinear OpenCode con stdin como los otros dos adaptadores. Ningún `defer`, `finally` ni manejador de señales corre ante SIGKILL, así que endurecer la limpieza no cierra el hallazgo (anexo A, C-007):
  ```go
  // Si OpenCode V2 acepta el mensaje por stdin, no se crea request.txt.
  cmd := exec.CommandContext(ctx, "opencode", args...) // sin --file
  cmd.Stdin = strings.NewReader(prompt)
  ```
  Si OpenCode V2 no admite stdin, el residual se acepta y se documenta: se mantienen los permisos 0700/0600 y el `defer os.RemoveAll(dir)`, que cubre todo retorno salvo la muerte por señal fatal del propio proceso `mem`.
- **Validación del cierre:** prueba con un stub de `opencode` que registra sus argumentos y su stdin, y afirma (a) que no recibe `--file`, (b) que el prompt llega por stdin y (c) que no existe ningún `socratica-opencode-*` bajo `TMPDIR` mientras corre. Con el código actual falla (a). Nota: un stub que sale con 137 **no** sirve como validación. Que muera el hijo no mata al proceso Go, así que el `defer` se ejecuta y la prueba pasaría también con el código actual.

### H-03 — `ExplainFailure` clasifica por subcadenas: ante texto nuevo, error irreintentable se ofrece como reintentable

- **Categoría:** Error lógico (higiene, misma clase que lo señalado en 038 para `validate_038_evidence.py`).
- **Severidad:** Baja.
- **Estado:** CONFIRMADO por lectura (sin repro binaria).
- **Ubicación:** `adapters/secondary/generator/failure.go:9-21` (`ExplainFailure`).
- **Ruta de alcance:** `mem socratica` CLI (`failed()` `cmd_socratica.go:586-646`) y TUI: el mensaje crudo(`f.LastError`, ya protegido en escritura y lectura) se clasifica para decidir si se ofrece reintentar.
- **Análisis de vulnerabilidad:** las tres cláusulas `strings.Contains` (`failure.go:11,14,17`), seguidas del default incondicional (`:20`), comparan subcadenas concretas de las redacciones **actuales** de claude/codex/opencode, en inglés y español. Difusión normal y esperada del texto de error de estas herramientas (nuevas versiones, localizaciones, migración 3.x) hará que 403 de free y auth fallen sin match: el mensaje pasa por el default con `Retryable: true` y la UI ofrecerá "[r]eintentar" aunque sea un fallo estructural, quemando llamadas de API (interval/budget) y devolviendo la misma pantalla sin progreso. El patrón —clasificar por subcadena en vez de por el motivo en el momento en que se conoce— es el mismo que la auditoría 038 ya marcó en `validate_038_evidence.py`. No es fuga: el mensaje mostrado pasa por `domain.ProtectText(raw)`.
- **Evidencia:** lectura del fichero; el default `return ports.SocraticFailure{Message: message, Retryable: true}` es la rama por omisión. `TestProviderFailureExplainer` (`harness_test.go:150-161`) sí ejercita `ExplainFailure`, pero solo con las redacciones **actuales** (`provider.auth`/free tier, `invalid_json_schema`, `Not logged in`) y con `"timeout"` para el default. No hay ningún caso con una redacción nueva de un fallo estructural, que es justo la brecha del hallazgo (anexo A, C-002).
- **Código corregido (seguro y defensivo):** propagar una `cause` en el momento de conocerse, no re-derivarla del texto:
  ```go
  type GenerationError struct {
      Kind       ports.SocraticFailureKind // freeTierUnrecoverable | providerAuth | schemaUnsupported | retryable
      Underlying error
  }
  ```
  Los adaptadores (`exec.go`, `opencode.go`) ya conocen el tipo en el punto de generación (`parseOpenCodeOutput` maneja `"type":"error"` con el JSON crudo), así que asignan `Kind` allí; `ExplainFailure` solo traduce. Como red de seguridad, el default pasa a `Retryable: false` salvo clasificación explícita de timeout (fallar cerrado: preferible no ofrecer reintento a ofrecerlo sistemáticamente).
- **Validación del cierre:** tabla de pruebas con redacciones nuevas de error de harness (mensajes inventados para los mismos escenarios 403/auth/esquema) que afirmen `Retryable=false` y el mensaje asignado por `Kind`; con el código actual esas aserciones fallan (default reintentable).

### H-04 — DESCARTADO: `eval` de Spec Kit sobre salida citada con `printf '%q'`

- **Estado:** DESCARTADO tras la revisión adversarial (anexo A, C-001, confirmado por ambos revisores). Se mantiene el identificador para la trazabilidad.
- **Afirmación original (incorrecta):** `get_feature_paths` imprimía líneas `KEY="value"` a partir de rutas crudas y `eval "$_paths_output"` (`.specify/scripts/bash/setup-plan.sh:31-32`) las volvería a ejecutar como código shell.
- **Hechos verificados:** `get_feature_paths` (`.specify/scripts/bash/common.sh:189-200`) emite cada asignación con `printf 'CLAVE=%q\n'`, con un comentario explícito contra la inyección. `%q` escapa los metacaracteres, así que el `eval` solo reconstruye el valor literal. Réplica en bash con un valor hostil: `d='/tmp/we ird/$(touch /tmp/pwned_acr)"`id`'; eval "$(printf 'V=%q\n' "$d")"`. Resultado: `$V` es idéntico a `$d` y `/tmp/pwned_acr` no se crea.
- **Corrección propuesta original (retirada):** el `sed … | tr -d '"'` malinterpretaba el formato (`%q` escapa un espacio como `\ `, no lo pone entre comillas) y habría corrompido rutas válidas. La guardia de metacaracteres habría rechazado rutas legítimas ya citadas. No se debe aplicar ninguna de las dos.
- **Acción:** ninguna en código. Lección de método: antes de afirmar que un `eval` permite inyección, hay que leer el productor de la cadena, no solo el consumidor.

### H-05 — Locks de despacho huérfanos si el proceso muere entre el commit de purga y `dropLocks`

- **Categoría:** Integridad / recursos huérfanos (uso de disco, no seguridad).
- **Severidad:** Baja.
- **Estado:** SOSPECHOSO (sin repro; el cierre H-06 de 2026-10-16 dotó al lock de `Remove` solo tras adquirirlo, que mitiga el borrado de locks retenidos, no el de locks nunca tocados).
- **Ubicación:** `adapters/secondary/persistence/socratica.go:480-527` (purga por ids) frente a `application/usecases/socratica_flow.go` (PurgeStale/PurgeTerminal → `dropLocks` tras el commit).
- **Ruta de alcance:** `mem gc` / `mem socratica` (caso de uso de purga → `deleteReturningIDs` → el llamador borra los ficheros `.memory/socratica/locks/<id>.lock` **fuera** de la transacción).
- **Análisis de vulnerabilidad:** la limpieza de locks es un paso posterior al commit; si el proceso muere entre ambos, los `.lock` de los flujos purgados quedan huérfanos indefinidamente (no existe GC de locks). No es una vulnerabilidad: el lock huérfano no bloquea flujos nuevos porque el id ya no existe — solo acumula ficheros. Nota de higiene: es la variante socratica del problema de locks que el cierre de 2026-10-16 resolvió para el lock de sesión; esta ronda no encontró en el árbol ninguna limpieza de `.memory/socratica/locks` más allá de `dropLocks` en línea.
- **Evidencia:** lectura de las llamadas de purga y `dropLocks` en `socratica_flow.go` (ejecutado tras el commit en `PurgeStale`/`PurgeTerminal`); sin test que cubra el estado "proceso muerto antes de `dropLocks`".
- **Código corregido (seguro y defensivo):** una pasada de GC de locks al arranque del TUI/CLI que recorra `.memory/socratica/locks/` y elimine los de ids no presentes en la base (comparación en memoria contra consulta de ids activos). Ese mismo punto es el sitio natural para la política de retención.
- **Validación del cierre:** prueba de integración que purga flujos con el borrado de locks forzado a fallar (error inyectado) y verifica que, tras un `mem gc` posterior, el directorio de locks queda vacío sin tocar locks de flujos activos; con el código actual falla (los huérfanos no se recuperan).

## 3. Recomendaciones de resiliencia

1. **`go test -race` en CI incluida en `test.yml`** (está en `evidence038` y carreras cubiertas en esta ronda a mano): añadir un job corto `-race` sobre `persistence`, `generator`, `usecases`, `domain` (la cabecera del run en CI cubre solo `-tags` determinados; si ya está, documentar esa cobertura en el workflow).
2. **Stubs de harness para pruebas de generación.** Una prueba de banco con stubs de `claude`/`codex`/`opencode` (exit codes y redacciones de errores) fijaría el contrato `ExplainFailure`/`run` sin salir del host; hoy `harness_test.go` cubre mosto del parseo pero no las rutas de `cmd.Run` error, `WaitDelay` ni el reintentar después de `run` fallido.
3. **Clasificar en origen (H-03).** La convención del propio repo ya la aplica a SQLite (`classifySQLite` en `privacy_maintenance.go:399`): firmar errores con `Kind`/`domain.ErrValidation` en el punto de emisión. Aplicar el mismo criterio al mapeo de errores de los harnesses.
4. **`govulncheck` ya está en `release.yml`** (`golang/govulncheck-action@032d4…`); añadirlo en `test.yml` para que falle el PR antes que el release si entra una dependencia vulnerable.
5. **Regla del portfolio que salió bien y mantener:** "toda columna de texto nueva entra en sanitize y toda vía stdin nueva usa `readBounded` en el mismo cambio" (memoria [564]). Los adaptadores socratica la respetan sin desviarse; plasmarla en `docs/conventions` o en un checklist de PR para que no dependa de la memoria de la persona.
6. **Sin configurado `socratica_private`: la TUI/CLI imprimen el prompt original y las respuestas en consola** es diseño deliberado (los prompts son del propio usuario), pero conviene un aviso en `mem socratica --help` de que las respuestas y el approved prompt se entregan en claro en modo `--print`, para claridad de decisiones en contextos con third-parties observando la TTY.

---

## Anexo A — Revisión adversarial por consenso del informe

- **Review ID:** `acr_55914540-0605-4f1c-8709-be7cba654d6a` · veredicto **APPROVED** · independencia **degradada** (los dos revisores usan el mismo modelo y proveedor, en contextos aislados).
- **Objetivo:** este informe (digest `8af1bd78…`) sobre la revisión `a75ca31`. Las correcciones de abajo se aplicaron al informe después del ACR y se verificaron contra el código del árbol actual.

| ACR | Severidad | Hallazgo afectado | Corrección aplicada |
|---|---|---|---|
| C-001 | Media (confirmado) | H-04 | Descartado: `printf '%q'` en `common.sh:189-200` neutraliza el `eval`; réplica con `$(…)` y backticks sin ejecución |
| C-002 | Baja | H-03 | La evidencia reconoce `TestProviderFailureExplainer` y acota la brecha a las redacciones nuevas |
| C-003 | Baja | H-01 | Retirada la subcausa «PATH completado después del arranque» (`LookPath` va fuera del `Once`) |
| C-004 | Baja | Cobertura | 2 workflows y 20 referencias `uses:` (5 + 15) |
| C-005 | Info | H-03 | Tres cláusulas `Contains` más el default |
| C-006 | Info/Baja | Cobertura | Ubicaciones exactas de `readSanitizeCandidates`, `readLedgerCandidates` y `readSocraticaCandidates` |
| C-007 | Info | H-02 | Retirado el snippet de `defer` contradictorio; el cierre es solo por stdin |
| (propio) | Info | H-02 | Su validación (stub con exit 137) también pasaba con el código actual; se sustituye por aserciones sobre argv, stdin y `TMPDIR` |

---

## Anexo B — Cierres aplicados (2026-10-17)

Cada prueba nueva se ejecutó primero contra el comportamiento anterior y falló. Con la corrección pasa, y también `go test ./...` completo y `-race` sobre `persistence`, `generator`, `usecases`, `domain`, `cli` y `tui`.

| ID | Cierre | Prueba que lo fija |
|---|---|---|
| H-01 | `openCodeGenerator.UnavailableReason` cachea solo el éxito (mutex + `versionOK`); un fallo se reconsulta en la siguiente llamada | `TestOpenCodeVersionCheckRetriesUntilSuccess`: el primer chequeo falla, el segundo da disponible y el tercero no vuelve a ejecutar el binario |
| H-02 | OpenCode recibe el prompt por stdin (sin `--file` ni `request.txt`); en el directorio temporal solo queda el perfil fijo del agente | `TestOpenCodeIsolatedAgentAndCleanup`: sin `--file`, el prompt fuera de argv, el workspace contiene solo `.opencode` y el prompt llega por stdin. Verificado con OpenCode v2.0.32 real: `run` acepta el mensaje por stdin |
| H-03 | `domain.RetryableGenerationFailure`: solo `ErrGenerationTimeout` y `ErrInvalidGeneration` (textos estables que emiten los propios adaptadores) son reintentables; el resto falla cerrado, también en el fallback del caso de uso | `TestProviderFailureExplainer`: redacciones nuevas de 403 y de sesión expirada no se ofrecen para reintento |
| H-05 | `PurgeStale` (`mem gc`) recorre los locks con `SocraticaDispatchLock.List` y elimina los de flujos cuya fila no existe (`FlowExists`). Un error de consulta no cuenta como «no existe», y `Remove` no borra un lock retenido | `TestSocraticaPurge_RecuperaLocksHuerfanos` (SQLite y FS reales: borra el huérfano y conserva el lock vivo, el retenido y un fichero ajeno) y el subtest de caso de uso con fakes |

Límite de la verificación real: `TestRealSocraticaHarness` con OpenCode devuelve `FreeTierError` (403) porque la capa gratuita rechaza los agentes personalizados. Ocurre igual con la invocación anterior (`--file`) y con la nueva (stdin), así que no es una regresión del cierre. Una generación completa de extremo a extremo requiere un proveedor no gratuito configurado en OpenCode.

---

Auditoría realizada en modo solo lectura; los cierres del anexo B se aplicaron después. H-01, H-02, H-03 y H-05 están cerrados; H-04 está descartado.
