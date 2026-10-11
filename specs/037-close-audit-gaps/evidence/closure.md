# Registro de cierre — feature 037

Fuente: [auditoría defensiva](../../../docs/audits/2026-10-11-defensive-qa.md), revisión base `30528515597292ecfcd42c7c08283cae0fbbbd12`.

Valores de `result`: `confirmed_open` (defecto reproducido, sin corregir), `corrected_pending_verification` (corregido, falta evidencia de plataforma/host requerida), `closed` (corregido y validado en el entorno requerido). Sin payloads reales ni credenciales.

| finding_id | requirement_ids | scenario_ids | revision | environment | command | expected | observed | result | evidence_paths |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| H-01 | FR-001, FR-002, FR-005–008 | US1.1–3, US1.6–7, US2.1–5 | 3052851 + cambios 037 (sin commit) | macOS arm64, Go 1.27.2, Python 3.13 | `audit_037_smoke.py --case privacy` y `--case history`; `TestAudit037_SanitizeHistorico` | escrituras, salidas e historia sin marcadores; residuos verificados | privacy: base 11/19, corregido 19/19; history: base 2/10, corregido 10/10 | closed | privacy/*.json, history/*.json |
| H-02 | FR-003, FR-004 | US1.4–6 | 3052851 + cambios 037 (sin commit) | macOS arm64, Go 1.27.2, Python 3.13 | `go test ./domain`; `audit_037_smoke.py --case privacy` | anidados/incompletos eliminados; sin original privado | base guarda original con apertura sin cierre; corregido 19/19 | closed | privacy/base-3052851.json, privacy/fixed.json |
| H-03 | FR-009, FR-010 | US3.1–3 | 3052851 + cambios 037 (sin commit) | macOS arm64, Go 1.27.2, Python 3.13 | `go test -race` (TestBuild_*, TestMCPGetContext_OchoSolapadasSinMezcla); `audit_037_smoke.py --case context-wrap` | 8 emisiones MCP solapadas = aislada (8420/655) | base: 8420→62703 acumulado y DATA RACE; corregido: 8/8 iguales | closed | context-wrap/*.json |
| H-04 | FR-011, FR-012 | US4.4–5 | 3052851 + cambios 037 (sin commit) | macOS arm64, Go 1.27.2, Python 3.13 | `go test -run TestReplaceSelfUnix`; `audit_037_smoke.py --case installation` (sonda update.never_absent) | ninguna invocación ve el binario ausente o parcial; fallos previos al rename conservan el anterior | base: miles de lecturas de tamaño 0/ausente durante `mem update`; corregido: 0 en ~600 000 lecturas | closed | install-unix/*.json |
| H-05 | FR-016–018 | US5.1–3 | 3052851 + cambios 037 (sin commit) | macOS arm64, Node v26.11.1; host local OpenCode 1.18.35 (no v2) | `node --test infrastructure/plugin/opencode/gomemory.test.mjs`; `audit_037_opencode_smoke.mjs` | event.result reducido en content string, Part[] mixto y output string; no-ops intactos | simulado: base falla (temporal sin publicar), corregido 39/39. Host v2: no disponible | corrected_pending_verification (falta OpenCode v2 real) | opencode-v2/local-1.18.35.json |
| H-06 | FR-019–021 | US5.4–6 | 3052851 + cambios 037 (sin commit) | macOS arm64, Node v26.11.1 | `node --test …gomemory.test.mjs` (rollback por registro, cierre con IO, plazo, dispose fallido, subproceso real) | rollback sin cerrar sesión ajena; cierre ≤5 s sin operaciones pendientes; fallos depurados | simulado y subproceso real: corregido 39/39. Host v2: no disponible | corrected_pending_verification (falta OpenCode v2 real) | opencode-v2/local-1.18.35.json |
| H-07 | FR-022, FR-023 | US6.1–2 | 3052851 + cambios 037 (sin commit) | macOS arm64, Go 1.27.2, Python 3.13 | `TestAudit037_WrapDistingueArranqueFallido`; `audit_037_smoke.py --case context-wrap` | arranque fallido → 1 con WRAP_START_FAILED y sin diálogo; hijo iniciado conserva salida y código | base: inexistente/sin permiso salen 0 con diálogo; corregido 11/11 | closed | context-wrap/*.json |
| H-08 | FR-013–015 | US4.1–3, US4.6 | 3052851 + cambios 037 (sin commit) | Unix: macOS arm64, bash 3.2 del sistema. Windows: no disponible | `audit_037_smoke.py --case installation`; instalación real `GOMEMORY_VERSION=v2.31.0 scripts/install.sh` desde GitHub; Windows: `audit_037_windows_smoke.ps1 -Case all` | matriz de integridad: íntegro aceptado, 100 % de inválidos rechazados con el previo intacto | Unix: base 5/20, corregido 20/20; descarga real verificada (SHA-256) e instalada. Windows: sin ejecutar | corrected_pending_verification (falta Windows real) | install-unix/*.json |
| H-09 | FR-024, FR-025 | US7.1–3 | 3052851 + cambios 037 (sin commit) | macOS arm64, Node v26.11.1, Go 1.27.2 | `node --test …gomemory.test.mjs`; mutación sin `--conversation`; `.github/workflows/test.yml` (pasos ejecutados en local) | fixtures verifican comando, conversación y proyecto; fallos inyectados efectivos; CI con plugin, núcleo, instalador y Windows | base 22/24 → corregido 39/39; mutación detectada (2 fallos); CI escrito, sin ejecución remota todavía | closed (fixtures); ejecución remota de CI pendiente de push | baseline.md |

## Notas por hallazgo

### H-01 / H-02 (US1)

- Causa H-02: `RedactPrivate` usaba la regex no codiciosa `(?is)<private>.*?</private>`: con anidamiento cortaba en el primer cierre y dejaba texto privado; una apertura sin cierre no se eliminaba. Corregido con recorrido de etiquetas con profundidad y plegado ASCII; `IsPrivateContent` hereda la regla.
- Causa H-01 (escrituras/salidas): `EndSession`, `UpdateSessionSummary`, `InsertRelation`/`UpdateRelation`/`ImportRelation` y `RecordChannelError` guardaban crudo; sus lectores y `ExportProject` lo reemitían. Ahora todo pasa por `domain.ProtectText` al escribir y al leer.
- Hallazgo adicional del runner (no estaba en la auditoría): `mem compare -m` imprimía el razonamiento crudo porque `RecordVerdict` devolvía el argumento original. Corregido en el caso de uso; prueba `TestRecordVerdict_DevuelveRazonamientoProtegido`.
- Remanente histórico: una memoria guardada antes de la 037 con bloques anidados conserva texto privado seguido de un `</private>` huérfano (la regex vieja eliminó la apertura). El texto no se puede delimitar automáticamente, pero el cierre huérfano sí se detecta: `mem sanitize` lo reporta como categoría `memory_review` (ids, sin contenido) para revisión manual, sin modificarlo.
- US2 (`mem sanitize`): vista previa sin efectos; `--apply` publica el informe `pending_cleanup` antes de la transacción inmediata con `secure_delete`, actualiza solo el campo permitido si el valor sigue siendo el leído, elimina originales protegidos, reescribe snapshots por temporal+rename, regenera `context.md`, ejecuta `VACUUM` + `wal_checkpoint(TRUNCATE)` verificando `busy` y solo entonces declara `completed`. Escritor simultáneo, lector que bloquea el checkpoint, copia inaccesible y cancelación dejan `pending_cleanup` con salida 1; el reintento completa y cierra los informes anteriores.
- Hallazgo del runner durante US2: el remanente `memory_review` hacía que el `context.md` regenerado se marcara siempre como residuo (bucle sin fin de `pending_cleanup`). Causa: la verificación usaba `IsPrivateContent`, que cuenta un cierre huérfano como privado. Corregido con `domain.ContainsProtected` (apertura o credencial); regresión fijada en adaptador y contrato (mutación detectada).
- No cubierto por automatización: falta de espacio en disco simulada (no reproducible de forma portable sin privilegios). El camino de error es el mismo que una escritura fallida de artefacto (`write_failed` → pendiente), probado con la copia inaccesible.
- Límites declarados en cada informe: copias externas, snapshots del sistema y borrado físico en SSD quedan fuera; los clientes antiguos deben detenerse antes del cierre físico.

### H-03 (US3)

- Causa: `Builder.discardedChars` vivía en la instancia compartida; `acota`/`fits`/`descarta` lo incrementaban y `Build` nunca lo reiniciaba. El servidor MCP reutiliza el mismo `Builder`, así que la línea base crecía en cada llamada y las concurrentes se escribían el contador entre sí (carrera).
- Corrección: `Build` opera sobre una copia local del `Builder` con el contador a cero; misma firma pública, sin mutex global. El recorder real de uso no presentó carreras bajo `-race`.
- Nota de método: `mem context` (CLI) y `get_context` (MCP) no construyen el mismo documento; el equivalente aislado de una llamada MCP es otra llamada MCP en un proceso nuevo.

### H-04 / H-08 (US4)

- Causa H-04: `replaceSelf` renombraba `mem` a `mem.old` y después copiaba el nuevo sobre la ruta pública: entre ambos pasos la ruta no existía y durante la copia el archivo estaba incompleto. Ahora se prepara un temporal completo en el mismo directorio (copia, permisos, `sync`, `close`) y se publica con un único `rename`. Windows conserva su política.
- Causa H-08: `install.sh` e `install.ps1` descargaban `latest/download` y extraían sin verificar `checksums.txt`, aceptando enlaces, binarios vacíos o ajenos a la plataforma, y `install.sh` salía 0 aunque el destino no admitiera escritura. Ahora: tag estable resuelto una vez, entrada única exacta y digest de 64 hex, SHA-256 antes de extraer, solo el miembro `mem`/`mem.exe`, validación de cabecera ELF/Mach-O/PE sin ejecutar, publicación atómica y conservación del previo ante cualquier fallo.
- Incidente durante la validación: la sonda de `mem update` heredaba el `PATH` del operador y, en un cwd sin git, `mem update` resolvió y reemplazó el `~/.local/bin/mem` real. Se corrigió el runner (PATH mínimo y comprobación previa de que `mem` resuelve al binario del banco) y se restauró el release oficial v2.31.0 con el instalador endurecido, verificando su SHA-256. Ninguna configuración global se modificó.
- Pendiente: ejecutar `scripts/audit_037_windows_smoke.ps1 -MemBin <mem.exe> -WorkDir <dir> -Case all` en Windows real (o en el job windows de `.github/workflows/test.yml`) y registrar su `result.json`. Mientras tanto, H-08 permanece `corrected_pending_verification`.

### H-05 / H-06 (US5)

- Causa H-05: el gancho v2 `execute.after` pasaba a la fábrica v1 un objeto temporal `{ output }` y nunca copiaba la reducción a `event.result`, que es lo que lee el consumidor. Ahora `withReducedText` publica en la misma representación (content string, `Part[]` conservando la primera parte textual con sus metadatos y las no textuales en orden, u output string); un `output` no textual nunca se reemplaza y una reducción que termina tras el cierre no se publica.
- Causa H-06: `cleanup` abortaba el iterador sin esperar nada, `execFileExec` no aceptaba cancelación (30 s fijos) y un fallo de registro en el setup dejaba la suscripción viva. Ahora hay ciclo `initializing → active → closing → closed`, operaciones rastreadas, `AbortSignal` hasta el subproceso, plazo global de 5 s, cierre de la sesión propia con un canal independiente de la señal abortada, `cleanup` idempotente y `OPENCODE_CLEANUP_FAILED` depurado cuando algo no se libera. El rollback de setup no llama a `hooks.dispose()` (no cierra la sesión de memoria).
- Hallazgo con un subproceso real (no visible con el `exec` simulado): `execFile` descarta `detached`, así que un descendiente del hijo heredaba la salida y mantenía viva la llamada hasta su timeout de 30 s pese a `SIGKILL`. Ahora se usa `spawn` con grupo de procesos propio en Unix y la cancelación señala al grupo; la prueba mide SIGTERM inmediato y SIGKILL tras la gracia de 1 s.
- Pendiente: ejecutar `node scripts/audit_037_opencode_smoke.mjs --mem-bin <mem> --opencode-bin <opencode v2> --out-dir <dir> --model-ready` con un OpenCode 2.x en banco separado y credenciales de prueba. El host local es 1.18.35: el runner devuelve `pending_verification` (código 2).

### H-07 (US6)

- Causa: `CmdWrap` solo trataba `*exec.ExitError`; cualquier otro error de `Run` (comando inexistente, sin permiso) dejaba `exitCode = 0` y abría el diálogo de guardado como si el comando hubiera terminado bien.
- Corrección: un error que no es `ExitError` emite `WRAP_START_FAILED` con una causa útil (no encontrado / sin permiso) y solo el nombre del comando protegido, y sale 1 antes del diálogo. Los hijos iniciados conservan su salida, su código y el diálogo; una terminación por señal sigue siendo no cero.

### H-09 y validación transversal (US7)

- Causa H-09: dos aserciones comparaban `args.join(' ') === 'session start'` cuando producción añade `--conversation=<id>`, y la simulación de fallo exigía la cadena completa, así que con argumentos extra el fallo nunca se inyectaba. Ahora se compara por prefijo semántico, se exige identidad de conversación y proyecto, el fallo inyectado deja un contador que la prueba verifica y `afterEach` libera las instancias vivas aunque falle un assert.
- CI: `.github/workflows/test.yml` (push y PR) ejecuta vet, suite completa, carreras dirigidas, cobertura combinada con umbral 80 %, el runner Python (`--case all` en Linux, `installation` en macOS), el plugin con Node 22.20.0 (mínimo declarado), el instalador y, en Windows, `go test` y el runner de PowerShell 5.1 y 7. No modifica `release.yml` ni añade lockfiles.
- Runtimes mínimos (FR-027): Python ≥3.12 (probado con el 3.9.6 del sistema → `RUNTIME_UNSUPPORTED`, salida 2), Node ≥22.20.0 en el runner de OpenCode, PowerShell ≥5.1 y Windows en el runner de PowerShell.

## Puerta de cobertura

| Momento | Cobertura combinada | Resultado |
| --- | --- | --- |
| Línea base (3052851) | 78,6 % | falla (umbral 80 %) |
| Tras US1–US6 | 78,8 % | falla |
| Con pruebas de herramientas MCP de código y de revisión, migración legada y presentación de ahorro | 80,4 % | pasa |

Las pruebas añadidas para cerrar la brecha verifican comportamiento observable (respuestas de las herramientas, veredicto `APPROVED`, migración y normalización de datos, cifras del informe), no solo ejecución. El umbral no se modificó.

## Resumen

| Hallazgo | Estado |
| --- | --- |
| H-01, H-02, H-03, H-04, H-07, H-09 | closed |
| H-05, H-06 | corrected_pending_verification: falta OpenCode 2.x real |
| H-08 | corrected_pending_verification: falta Windows real (Unix cerrado) |

## Impacto del cambio (T075)

`codebase-memory-mcp` se desconectó durante la sesión, así que `detect_changes` no pudo ejecutarse. El impacto se revisó con CodeGraph (índice del proyecto): `Builder.Build` tiene 22 llamadores vía `ports.ContextBuilder` y conserva su firma; `OriginalStoreRepository.Put/Get` tienen 2 llamadores cada uno (el motor nativo ya descarta lo privado antes de `Put` y `RetrieveOriginal` envuelve el error sin contenido); `EndSession`, `RecordVerdict`, `replaceSelf` y `CmdWrap` no cambiaron de firma. Las suites de todos los llamadores pasan (`go test ./...`, contratos, integración y carreras dirigidas). Pendiente: repetir con `detect_changes` cuando el servidor vuelva a estar disponible.


## Revisión acr_9ba278e2 (correcciones posteriores)

La revisión adversarial del diff de la 037 (solo informe, `fix_authorized=false`) dejó dos hallazgos confirmados y dos sospechosos. Se corrigieron fuera del protocolo de esa revisión, con pruebas en rojo antes de cada cambio:

| ID | Problema | Corrección | Prueba |
| --- | --- | --- | --- |
| C-001 | `<private >`, `<PRIVATE  >`, `<private atributo="x">` y `</private >` no se reconocían como etiquetas | `matchPrivateTag` único para redactor y detectores: apertura con `>` directo o espacio y atributos hasta `>` (sin `<` intermedio, máx. 256 bytes); cierre con espacios opcionales. `<privateX>` sigue sin ser etiqueta | `TestRedactPrivate_VariantesDeEtiqueta`; runner con variante: base 39/60, corregido 60/60 |
| C-002 | `PendingReports` descartaba en silencio informes ilegibles | Devuelve los ilegibles como artefactos `unreadable_report`; la vista previa los avisa y `Apply` no declara `completed` mientras existan | `TestPrivacyArtifacts_InformeIlegibleNoSeOculta`, `TestSanitize_InformeIlegibleImpideCompleted` |
| S-01 | Categorías del informe tomadas antes de la transacción | Se recalculan con los candidatos releídos dentro de la transacción | `TestSanitize_CategoriasReflejanLoAplicado` |
| S-02 | Un snapshot con campos de otra versión quedaba pendiente para siempre (`unknown_format`) | Recorrido JSON genérico que protege todos los textos, conserva campos desconocidos y números exactos; solo un archivo que no es JSON queda pendiente, con `invalid_json` (distinto de `protected_content`) | `TestPrivacyArtifacts_DepuraSnapshots` |

Validación tras las correcciones: `go vet`, `go test ./...`, carreras dirigidas en verde; cobertura combinada 80,4 %; `audit_037_smoke.py --case all` 60/60 (evidencia en `acr-9ba278e2/`).
