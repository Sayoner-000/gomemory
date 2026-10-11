# Tasks: 037-close-audit-gaps — Cierre de brechas de auditoría defensiva

**Input**: Design documents from `/specs/037-close-audit-gaps/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (privacy-maintenance, context-wrap, installation, opencode-lifecycle), quickstart.md

**Tests**: Incluidos. El plan exige "pruebas dirigidas en rojo antes de corregir cada comportamiento" y FR-025 pide reproducir cada hallazgo. Cada tarea de prueba DEBE fallar contra la revisión `30528515597292ecfcd42c7c08283cae0fbbbd12` antes de su corrección; una selección que ejecuta cero casos no cuenta como evidencia.

**Organization**: Tareas agrupadas por historia de usuario (US1–US7 de spec.md). Las hojas del árbol del plan ([1.1]–[7.4]) se indican entre corchetes al final de cada tarea para trazabilidad.

**Reglas transversales** (constitución + plan):

- Sin dependencias nuevas, sin migraciones de esquema, sin SQL fuera de `adapters/secondary/persistence`.
- Solo marcadores y credenciales sintéticas; ninguna prueba toca el almacén ni la configuración habituales (HOME/XDG/`GOMEMORY_DATA_HOME` temporales).
- Salidas JSON/NDJSON sin banners; contratos públicos de consola, MCP y compresión intactos.
- Evidencia ausente de Windows u OpenCode v2 real → `corrected_pending_verification`, nunca `closed`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Línea base medida y andamiaje de evidencia antes de tocar código.

- [X] T001 Medir línea base en la revisión actual y registrarla en specs/037-close-audit-gaps/evidence/baseline.md: `go build ./...`, `go test ./... -count=1`, `go vet ./...`, `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (se espera 22/24 por H-09), `npm --prefix installer ci && npm --prefix installer run typecheck && npm --prefix installer test`, cobertura combinada con `GOMEMORY_COVERAGE_OUTPUT=<tmp> bash scripts/coverage.sh`, más versiones de Go/Node/Python y `git rev-parse HEAD`
- [X] T002 [P] Crear el registro de cierre specs/037-close-audit-gaps/evidence/closure.md con una fila por H-01–H-09 y los campos del data-model (finding_id, requirement_ids, scenario_ids, revision, environment, command, expected, observed, result, evidence_paths), todas con `result=confirmed_open`
- [X] T003 [P] Crear el esqueleto del runner scripts/audit_037_smoke.py (solo stdlib): argumentos `--mem-bin --work-dir --data-home --out-dir --case {privacy,history,context-wrap,installation,all}`, entorno hijo aislado (HOME, XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME, XDG_CACHE_HOME, GOMEMORY_DATA_HOME temporales), conteo de casos, salida 0 solo si todos los casos seleccionados se ejecutaron y aprobaron, 1 si se reproduce un defecto, 2 por preparación inválida o cero casos; cada caso queda como función vacía que devuelve "no implementado" (→ 2)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Regla única de texto protegido (H-02) que consumen US1 y US2, y fixtures fiables del plugin (H-09) sobre los que US5 añade pruebas.

**⚠️ CRITICAL**: US1, US2 y US5 no pueden empezar hasta cerrar esta fase.

- [X] T004 Escribir pruebas en rojo en domain/redact_test.go para `RedactPrivate`: bloques anidados (2 y 3 niveles), consecutivos, vacíos, mayúsculas mixtas (`<PRIVATE>`, `</Private>`), texto Unicode antes/dentro/después, apertura sin cierre (privado hasta fin de texto), cierre aislado (se descarta sin ocultar texto posterior), texto público sin etiquetas idéntico byte a byte y segunda aplicación estable [1.1]
- [X] T005 [P] Escribir pruebas en rojo en domain/compression_test.go para `IsPrivateContent`: apertura incompleta y anidamiento detectados con la misma regla que `RedactPrivate`; texto público → false [1.1]
- [X] T006 Reemplazar la regex de domain/redact.go por un recorrido de etiquetas ASCII con profundidad: índices calculados sobre el texto original (sin pasar toda la cadena a minúsculas), comparación ASCII sin distinguir mayúsculas, apertura sin cierre elimina hasta el final, cierre aislado se descarta; dejar T004 en verde [1.1]
- [X] T007 Hacer que `IsPrivateContent` en domain/compression.go reutilice el recorrido de T006 (sin regla duplicada); dejar T005 en verde [1.1]
- [X] T008 Añadir en domain/redact.go la composición única `ProtectText(s) = RedactSecrets(RedactPrivate(s))` con prueba de idempotencia y de texto público intacto en domain/redact_test.go; es el único punto de entrada que usarán escrituras y lecturas de US1/US2 [1.1]
- [X] T009 [P] Corregir los fixtures de infrastructure/plugin/opencode/gomemory.test.mjs (H-09, FR-024): comparar `session start` por prefijo semántico de argumentos y exigir además `--conversation=<id>` y `cwd` del proyecto; la simulación de fallo de inicio debe activarse con argumentos adicionales y dejar un contador de activaciones que la prueba compruebe; mover la liberación de recursos a `finally`. Resultado esperado: 24/24 sin tocar producción [7.1]

**Checkpoint**: `go test ./domain -count=1` y `node --test infrastructure/plugin/opencode/gomemory.test.mjs` en verde.

---

## Phase 3: User Story 1 - Mantener privados los datos de memoria y sesión (Priority: P1) 🎯 MVP

**Goal**: Ninguna escritura nueva ni salida (respuesta, consulta, contexto, exportación, recuperación de originales) reemite contenido protegido de memorias, resúmenes, razonamientos de relación o diagnósticos, incluidos registros históricos sin sanear (H-01 parcial, H-02).

**Independent Test**: Con almacén aislado y marcadores sintéticos por cada entrada afectada (CLI, MCP, hooks, import), inspeccionar SQLite real y stdout/stderr: cero marcadores protegidos, texto público e identidades conservados (`audit_037_smoke.py --case privacy`).

### Tests for User Story 1 ⚠️ (escribir primero, deben fallar)

- [X] T010 [P] [US1] Pruebas en rojo en adapters/secondary/persistence/session_summary_test.go: `EndSession` y `UpdateSessionSummary` persisten el resumen protegido; `ActiveSession` y `RecentSessions` depuran una fila histórica insertada en crudo por SQL de fixture; id, proyecto, conversación, created_at y ended_at sin cambios [1.2, 1.3]
- [X] T011 [P] [US1] Pruebas en rojo en adapters/secondary/persistence/relation_privacy_test.go (nuevo): `InsertRelation` y `UpdateRelation` depuran `reasoning`; los lectores de relaciones depuran filas históricas crudas; extremos, tipo, confidence y created_at intactos [1.2, 1.3]
- [X] T012 [P] [US1] Pruebas en rojo en adapters/secondary/persistence/channel_activity_test.go: `RecordChannelError` guarda `last_error` depurado conservando una causa útil y metadatos; la consulta de diagnóstico depura filas históricas crudas (FR-005) [1.2, 1.3]
- [X] T013 [P] [US1] Pruebas en rojo en adapters/secondary/persistence/compression_originals_test.go: `Put` rechaza contenido con región privada completa o incompleta sin escribir; `Get` de un original histórico protegido devuelve error depurado sin bytes privados; un original público se recupera exacto con su referencia vigente (FR-004) [1.4]
- [X] T014 [P] [US1] Pruebas en rojo en application/usecases/portability_test.go: `ExportProject` no reemite contenido protegido de memorias ni de `reasoning` históricos; la importación de relaciones depura `reasoning` y conserva fechas originales [1.2, 1.3]
- [X] T015 [P] [US1] Prueba en rojo en application/usecases/build_context_test.go: un resumen de sesión histórico con bloque privado que cruza el punto de recorte se emite depurado (protección antes de `acota`/recorte) y conserva el texto público [1.3]
- [X] T016 [P] [US1] Pruebas en rojo de interfaces en adapters/primary/cli/cmd_mcp_session_summary_test.go: herramientas MCP `end_session` y `save_session_summary` responden y persisten el resumen depurado [1.2]
- [X] T017 [P] [US1] Prueba de contrato en rojo en tests/contract/session_privacy_cli_test.go (nuevo, binario compilado): `mem session end -s "<público> <private>X</private> <credencial sintética>"` imprime y guarda solo texto público con la credencial oculta; `mem context` posterior conserva el público [1.2]
- [X] T018 [P] [US1] Pruebas de integración en rojo en tests/integration/compact_summary_hook_integration_test.go: `mem hook compact-summary` y `mem hook channel-error` con marcador privado anidado/incompleto y credencial sintética no persisten ni muestran el contenido protegido [1.2]

### Implementation for User Story 1

- [X] T019 [US1] Aplicar `domain.ProtectText` en escrituras (`EndSession`, `UpdateSessionSummary`) y en la materialización de lectores (`ActiveSession`, `RecentSessions` y demás lectores de `summary`) en adapters/secondary/persistence/session.go; sin persistir cambios al leer; dejar T010 en verde [1.2, 1.3]
- [X] T020 [US1] Aplicar `domain.ProtectText` a `reasoning` en `InsertRelation`, `UpdateRelation` y lectores de adapters/secondary/persistence/relation.go, y revisar los envoltorios de adapters/secondary/persistence/repositories.go para que ninguna ruta guarde crudo; dejar T011 en verde [1.2, 1.3]
- [X] T021 [US1] Aplicar `domain.ProtectText` en `RecordChannelError` y en la lectura de diagnósticos de adapters/secondary/persistence/channel_activity.go; dejar T012 en verde [1.2, 1.3]
- [X] T022 [US1] Rechazar en `Put` y en `Get` de adapters/secondary/persistence/compression_originals.go el contenido que `IsPrivateContent` reconozca, con error depurado y sin alterar originales públicos; dejar T013 en verde [1.4]
- [X] T023 [US1] Proteger memorias y `reasoning` en exportación e importación de application/usecases/portability.go; dejar T014 en verde [1.2, 1.3]
- [X] T024 [US1] Proteger resúmenes antes de recortarlos en application/usecases/build_context.go (y application/usecases/build_context_pack.go si emite resúmenes); dejar T015 en verde [1.3]
- [X] T025 [US1] Depurar la respuesta inmediata de `session end`/`session summary` en adapters/primary/cli/cmd_session.go y de las herramientas de sesión en adapters/primary/cli/cmd_mcp.go usando el valor protegido, no el argumento crudo; dejar T016 y T017 en verde [1.2]
- [X] T026 [US1] Verificar que `hookCompactSummary` y el manejador `channel-error` de adapters/primary/cli/cmd_hook.go pasan por las escrituras protegidas y no imprimen el mensaje crudo; ajustar solo si T018 sigue en rojo [1.2]
- [X] T027 [US1] Implementar el caso `privacy` en scripts/audit_037_smoke.py: memorias, sesiones CLI/MCP, compact-summary, channel-error, relaciones (crear/editar/importar), exportación, contexto y originales público/privado; inspeccionar SQLite y capturas; anidamiento, apertura incompleta, mayúsculas, Unicode y segunda aplicación [1.2–1.4]
- [X] T028 [US1] Validar US1: `go test ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli ./tests/contract ./tests/integration -count=1`, `go test -race ./adapters/secondary/persistence ./adapters/primary/cli -count=1` y `python3 scripts/audit_037_smoke.py ... --case privacy` con salida 0; anotar H-02 y la parte de escrituras/lecturas de H-01 en specs/037-close-audit-gaps/evidence/closure.md

**Checkpoint**: US1 funcional y demostrada sobre binario real; H-02 cerrable.

---

## Phase 4: User Story 2 - Sanear información histórica sin perder datos públicos (Priority: P1)

**Goal**: `mem sanitize` ofrece vista previa sin efectos y `--apply` explícito, transaccional, idempotente, con informe depurado `pending_cleanup → completed` y limpieza verificada de artefactos controlados (cierre histórico de H-01).

**Independent Test**: Almacén con registros históricos sintéticos (fixture de DB, copias JSON, originales): preview no cambia filas ni revela texto; apply preserva ids/fechas/público y limpia artefactos; segundo apply → `rows_changed=0`; interrupciones dejan `pending_cleanup` y salida 1 hasta reintento (`audit_037_smoke.py --case history`).

**Depende de**: Phase 2 y US1 (lectores depurados y `ProtectText`).

### Tests for User Story 2 ⚠️

- [X] T029 [P] [US2] Pruebas en rojo en domain/privacy_maintenance_test.go: categorías permitidas (session, relation, channel_error, y `memory_review` solo informativa para memorias con `</private>` huérfano — remanente de la regex anterior, ver evidence/closure.md) y campo permitido por categoría; transición de estados preview/pending_cleanup/completed/failed; la serialización JSON del informe nunca incluye `text` ni hashes de secretos [2.1]
- [X] T030 [P] [US2] Pruebas en rojo en application/usecases/sanitize_project_test.go con fakes de los puertos: preview no muta ni publica informe; apply publica `pending_cleanup` antes de abrir la transacción y aborta sin cambios si no puede; candidatos se releen dentro de la transacción; error o cancelación antes del commit → rollback; solo cambia el campo texto (ids/fechas/confidence intactos); fallo posterior al commit conserva `pending_cleanup` y exit 1; reintento completa; segundo apply `rows_changed=0` y verifica residuos previos; `external_copies` siempre presente [2.2, 2.3]
- [X] T031 [P] [US2] Pruebas en rojo en adapters/secondary/persistence/privacy_maintenance_test.go sobre SQLite temporal: los candidatos salen de lectura cruda (no de lectores depurados), acotados al proyecto; transacción inmediata con `secure_delete` en su conexión; sentencias fijas por categoría con parámetros; escritor simultáneo → `SANITIZE_BUSY` sin corrupción; VACUUM y `wal_checkpoint(TRUNCATE)` fuera de la transacción comprobando busy/log/checkpointed; nunca borra WAL/SHM [2.2, 2.4]
- [X] T032 [P] [US2] Pruebas en rojo en adapters/secondary/persistence/privacy_artifacts_test.go: informe publicado por temporal+rename en `DataHome/maintenance/privacy/<run_id>.json` con directorio privado y archivo 0600; reescritura depurada de JSON de `DataHome/backups/<ProjectKey>`; regeneración de `.memory/context.md`; eliminación de originales protegidos sin tocar públicos; enlace simbólico fuera de raíz, JSON desconocido o archivo inaccesible → pendiente informado sin borrado a ciegas; ninguna copia cruda nueva [2.3, 2.4]
- [X] T033 [P] [US2] Pruebas en rojo en adapters/primary/cli/cmd_sanitize_test.go: preview por defecto; `--apply`; `--json` emite un único objeto en stdout y diagnóstico depurado en stderr; `--timeout` ≤0 o inválido → 2 sin iniciar; pending/busy/timeout → 1 con `SANITIZE_PENDING_CLEANUP`/`SANITIZE_BUSY`/`SANITIZE_TIMEOUT`; sin banners en modo JSON [2.5]

### Implementation for User Story 2

- [X] T034 [US2] Crear domain/privacy_maintenance.go con `Candidate` (category, record_id string, project, field, text solo en memoria), `SanitizeReport` (campos del data-model, `schema_version=1`), estados, categorías y códigos de diagnóstico; dejar T029 en verde [2.1]
- [X] T035 [US2] Crear application/ports/privacy_maintenance.go con `PrivacyMaintenanceRepository.Preview(ctx, project)`, `PrivacyMaintenanceUnitOfWork.WithTransaction(ctx, project, func(PrivacyTransaction) error)`, `PrivacyTransaction` (listar candidatos actuales y reemplazar texto permitido, sin tipos SQL/driver) y `PrivacyArtifactsPort` (inventario, saneamiento, verificación de residuos, publicación de informe); todos los métodos de IO reciben `context.Context` [2.1]
- [X] T036 [US2] Implementar application/usecases/sanitize_project.go: orden preview → informe pending → transacción con `domain.ProtectText` → limpieza de artefactos → compactación/checkpoint → verificación → completed; reloj inyectable; dejar T030 en verde [2.2, 2.3]
- [X] T037 [US2] Implementar adapters/secondary/persistence/privacy_maintenance.go (repositorio + UnitOfWork + compactación/checkpoint verificados) reutilizando la configuración de `openDBFile` (busy timeout, transacción inmediata); dejar T031 en verde [2.2, 2.4]
- [X] T038 [US2] Implementar adapters/secondary/persistence/privacy_artifacts.go (`PrivacyArtifactsPort`) sin seguir enlaces fuera de raíces administradas; dejar T032 en verde [2.3, 2.4]
- [X] T039 [US2] Cablear repositorio, UnitOfWork, artefactos y caso de uso en infrastructure/container.go y exponerlos en adapters/primary/cli/deps.go [2.5]
- [X] T040 [US2] Implementar adapters/primary/cli/cmd_sanitize.go con componentes de consola existentes, añadir `case "sanitize"` en adapters/primary/cli/dispatcher.go y la entrada en el grupo de mantenimiento de `Usage()` en adapters/primary/cli/cli.go; sin herramienta MCP de apply; dejar T033 en verde [2.5]
- [X] T041 [US2] Prueba de contrato tests/contract/sanitize_cli_test.go sobre binario compilado con fixture de DB histórica: preview sin cambios, apply completed, segundo apply `rows_changed=0`, interrupción antes y después del commit → pending y salida 1, reintento → 0 [2.5]
- [X] T042 [US2] Implementar el caso `history` en scripts/audit_037_smoke.py (fixture de DB/JSON/originales, escritor simultáneo, lector que bloquea checkpoint, copia inaccesible, cancelación; búsqueda final de marcadores en DB, WAL, copias y derivados con clientes cerrados) [2.4, 2.5]
- [X] T043 [US2] Validar US2: `go test ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli ./tests/contract -count=1`, `go test -race ./application/usecases ./adapters/secondary/persistence -count=1`, `--case history` salida 0; completar H-01 en specs/037-close-audit-gaps/evidence/closure.md

**Checkpoint**: US1 + US2 cierran H-01 y H-02 (MVP de privacidad).

---

## Phase 5: User Story 3 - Obtener métricas de contexto consistentes (Priority: P2)

**Goal**: Cada `Build` calcula descartes, línea base, emisión y ahorro solo con sus propios datos, también con el `Builder` compartido por MCP (H-03).

**Independent Test**: Datos, configuración y reloj fijos: tres construcciones consecutivas y ocho simultáneas iguales a su equivalente aislado, bajo `-race`.

### Tests for User Story 3 ⚠️

- [X] T044 [P] [US3] Pruebas en rojo en application/usecases/build_context_test.go: tres `Build` consecutivos con contenido y par baseline/emitted idénticos; `Build` con descartes seguido de otro sin descartes sin contaminación; ocho `Build` concurrentes sobre la misma instancia iguales al aislado (ejecutar con `-race`) [3.1]
- [X] T045 [P] [US3] Prueba de integración en rojo en adapters/primary/cli/cmd_mcp_context_concurrency_test.go (en el paquete cli porque necesita el servidor MCP no exportado; el plan proponía tests/integration/build_context_concurrency_test.go): ocho llamadas MCP de contexto solapadas sobre la misma instancia cableada por infrastructure/container.go, con recorder y contador reales; cada emisión individual igual a la aislada (no comparar el total agregado de usage) [3.2]

### Implementation for User Story 3

- [X] T046 [US3] En application/usecases/build_context.go hacer que `Build` opere sobre una copia local de la configuración inmutable con `discardedChars` a cero por llamada (mismas firmas de `Build`/`WriteFile` y puertos); propagar la copia a `acota`, `fits` y a application/usecases/build_context_pack.go si usa el contador; sin mutex global; dejar T044 en verde [3.1]
- [X] T047 [US3] Si T045 revela carreras en el recorder o el contador de tokens, corregirlas en su adaptador (sin serializar todas las construcciones); dejar T045 en verde con `go test -race ./tests/integration -run BuildContextConcurrency -count=1` [3.2]
- [X] T048 [US3] Implementar la parte de contexto del caso `context-wrap` en scripts/audit_037_smoke.py (tres secuenciales y ocho MCP en la misma conexión, comparando emisiones individuales) y anotar H-03 en specs/037-close-audit-gaps/evidence/closure.md

**Checkpoint**: H-03 cerrable.

---

## Phase 6: User Story 4 - Instalar y actualizar sin aceptar artefactos dañados (Priority: P2)

**Goal**: Update Unix publica solo un ejecutable completo (H-04); `install.sh` e `install.ps1` resuelven una versión concreta, verifican SHA-256 y validan el ejecutable antes de publicar, conservando el anterior ante cualquier fallo (H-08).

**Independent Test**: Fixtures locales de release/asset/checksums sin red pública; ejecutable previo reconocible; matriz completa de contracts/installation.md en Unix (`--case installation`) y en Windows real (`audit_037_windows_smoke.ps1`).

### Tests for User Story 4 ⚠️

- [X] T049 [P] [US4] Pruebas en rojo en adapters/primary/cli/cmd_update_test.go para `replaceSelf` Unix: con barrera durante la copia, el destino sigue siendo el ejecutable anterior completo; fuente ausente, fallo de chmod/sync/close/rename y cancelación antes del rename conservan el anterior, eliminan el temporal propio y no declaran éxito; publicación correcta deja la versión nueva ejecutable; rama Windows sin cambios [4.1]
- [X] T050 [P] [US4] Crear fixtures de instalación en tests/testdata/install037/ (implementado: curl de fixture versionado en tests/testdata/install037/bin/curl; los tarballs y checksums.txt de cada escenario los genera el runner en ejecución desde --mem-bin para no versionar binarios) (release con tag estable, `latest` que cambia entre descargas, assets tar/zip con `mem`/`mem.exe`, `checksums.txt` correcto/incorrecto/ausente/inválido/sin entrada/duplicado, archivo vacío, enlace simbólico, miembro con traversal o ruta absoluta) y un `curl` de fixture para PATH, sin interruptor de producción que desactive la verificación [4.2]
- [X] T051 [US4] Implementar el caso `installation` en scripts/audit_037_smoke.py: matriz completa de contracts/installation.md para scripts/install.sh (explícita y latest) con bytes del ejecutable previo comparados tras cada fallo, y update Unix con invocaciones concurrentes del binario durante preparación/publicación; debe fallar (salida 1) contra el código actual [4.1, 4.2]

### Implementation for User Story 4

- [X] T052 [US4] Reescribir `replaceSelf` en adapters/primary/cli/cmd_update.go para Unix: temporal en el directorio del ejecutable, copia, permisos, sync, close y rename final; limpieza del temporal ante error/cancelación; conservar la política Windows existente; dejar T049 en verde [4.1]
- [X] T053 [US4] Endurecer scripts/install.sh: resolver `latest` una sola vez a tag estable; descargar asset y `checksums.txt` del mismo tag; exigir entrada única con nombre exacto y digest SHA-256 válido (asterisco opcional); `sha256sum` o `shasum -a 256`, fallar si no hay ninguno; comparar antes de extraer; extraer solo `mem`; validar archivo regular, no vacío, no enlace; publicar sin tocar destino/PATH ante fallo; limpiar temporal [4.2]
- [X] T054 [US4] Endurecer scripts/install.ps1 con la misma política usando `Get-FileHash -Algorithm SHA256` (comparación sin distinguir mayúsculas), extracción solo de `mem.exe`, rechazo de reparse points/rutas inseguras y conservación del destino previo [4.3]
- [X] T055 [US4] Crear scripts/audit_037_windows_smoke.ps1 (`-MemBin -WorkDir -Case`): LOCALAPPDATA temporal, interceptores de red de prueba sin saltar hash, matriz correcto/incorrecto/ausente/duplicado/red/permisos/archivo inválido/ejecutable previo, conteo de casos y códigos 0/1/2 como el runner Python [4.3]
- [X] T056 [US4] Validar US4: `go test ./adapters/primary/cli -run ReplaceSelf -count=1`, `go test ./tests/contract -run Update -count=1`, `--case installation` salida 0; ejecutar T055 en Windows real o registrar H-08 (parte PowerShell) como `corrected_pending_verification` en specs/037-close-audit-gaps/evidence/closure.md; anotar H-04

**Checkpoint**: H-04 cerrable; H-08 cerrable con evidencia Windows.

---

## Phase 7: User Story 5 - Recibir compresión efectiva y liberar la integración OpenCode (Priority: P2)

**Goal**: En OpenCode v2 el consumidor recibe el texto reducido en `event.result` (H-05) y la integración tiene un lifecycle cancelable con rollback de setup y cierre ≤5 s (H-06); export dual y rama v1 intactos.

**Independent Test**: Suite `node --test` con contrato simulado para las tres formas de resultado, no-ops, fallo de cada registro y cierre con evento en curso; después, host OpenCode v2 real con `audit_037_opencode_smoke.mjs`.

**Depende de**: T009 (fixtures fiables).

### Tests for User Story 5 ⚠️

- [X] T057 [P] [US5] Pruebas en rojo en infrastructure/plugin/opencode/gomemory.test.mjs para `execute.after` v2: el consumidor observa `event.result` reducido para `content: string`, `content: Part[]` mixto (primera parte textual sustituida con sus metadatos, demás textuales retiradas, no textuales en su orden) y `output: string`; no-op con compresión desactivada, sin ganancia, contenido protegido, estado no completado, sesión ajena o propietario no resuelto, sin atribuir ahorro; re-comprobación de `closing` antes de publicar [5.1]
- [X] T058 [P] [US5] Pruebas en rojo en infrastructure/plugin/opencode/gomemory.test.mjs para el lifecycle: fallo inyectado en cada registro de setup → suscripción abortada, registros previos liberados, error original conservado y ninguna sesión ajena cerrada; cierre con evento e IO en curso termina ≤5 s sin hijos ni operaciones pendientes ni publicación tardía; callbacks tras `closing` no hacen IO; cleanup repetido comparte resultado; fallo de dispose o rechazo de suscripción continúa con el resto y reporta `OPENCODE_CLEANUP_FAILED` depurado [5.2]

### Implementation for User Story 5

- [X] T059 [US5] En infrastructure/plugin/opencode/gomemory.ts extraer el texto por representación (misma prioridad que `resultText`), pasar al hook v1 un resultado mutable y publicar la reducción en `event.result` según la tabla de contracts/opencode-lifecycle.md, sin serializar objetos `output` a string; dejar T057 en verde [5.1]
- [X] T060 [US5] En infrastructure/plugin/opencode/gomemory.ts hacer que `execFileExec` acepte `AbortSignal`, no arranque si ya está abortada, cancele el hijo en curso con gracia de 1 s y terminación forzada, y confirme salida esperando `close` (no solo el rechazo); propagar `requestOptions.signal` a las requests de sesión v2 [5.2]
- [X] T061 [US5] En infrastructure/plugin/opencode/gomemory.ts implementar el estado `initializing/active/closing/closed` con lista de registros, promesa de suscripción retenida, conjunto de operaciones rastreadas y promesa única de cleanup con deadline global de 5 s; rollback de setup sin dispose v1 que cierre sesiones ajenas; cierre de sesión propia con presupuesto restante y canal independiente de la señal abortada; dejar T058 en verde [5.2]
- [X] T062 [US5] Verificar que adapters/primary/setup/opencode_setup.go instala la versión actualizada del plugin y que tests/contract/opencode_v2_shape_test.go, tests/contract/opencode_hooks_test.go y tests/contract/opencode_plugin_command_test.go siguen en verde (export dual y rama v1 sin regresión) [5.1, 5.2]
- [X] T063 [US5] Crear scripts/audit_037_opencode_smoke.mjs (`--mem-bin --opencode-bin --out-dir`): valida major=2, rutas HOME/XDG/config/DB aisladas, servidor `--standalone` propio, observa el resultado posterior al hook, repite las tres formas, no-ops, propiedad ajena, fallo de registros y cierre con evento en curso; sin host/modelo → código 2 con `status=pending_verification`; nunca toca el servicio habitual [5.3]
- [X] T064 [US5] Ejecutar T063 contra un host OpenCode v2 real con versión fijada y registrar H-05/H-06 en specs/037-close-audit-gaps/evidence/closure.md; sin host → `corrected_pending_verification` con instrucciones para completar [5.3]

**Checkpoint**: H-05/H-06 corregidos; cerrados solo con evidencia v2 real.

---

## Phase 8: User Story 6 - Distinguir ejecución real de un fallo de arranque (Priority: P2)

**Goal**: `mem wrap` sale 1 con `WRAP_START_FAILED` depurado y sin diálogo si el comando no arranca, y preserva salida y código del hijo iniciado (H-07).

**Independent Test**: Binario real con stdin cerrado: comando inexistente, archivo sin permiso de ejecución, hijo 0, hijo 7 y hijo 1 (distinguible del fallo de arranque).

### Tests for User Story 6 ⚠️

- [X] T065 [P] [US6] Prueba de contrato en rojo en tests/contract/wrap_start_failure_test.go (nuevo, binario compilado, stdin cerrado): inexistente y sin permisos → código 1, `WRAP_START_FAILED` en stderr sin argumentos completos ni error crudo, sin diálogo de guardado; hijo 0 → 0 y hijo 7 → 7 con stdout/stderr preservados; hijo que sale 1 no emite `WRAP_START_FAILED`; terminación por señal no produce éxito [6.1]

### Implementation for User Story 6

- [X] T066 [US6] En adapters/primary/cli/cmd_wrap.go clasificar el error de `Run`: `exec.ExitError` conserva la política actual; cualquier otro error de arranque → diagnóstico depurado `WRAP_START_FAILED` con causa útil (ruta/permisos), código 1 y retorno antes del diálogo; auto-inicio de sesión sin cambios; dejar T065 en verde [6.1]
- [X] T067 [US6] Implementar la parte wrap del caso `context-wrap` en scripts/audit_037_smoke.py y anotar H-07 en specs/037-close-audit-gaps/evidence/closure.md [6.1]

**Checkpoint**: H-07 cerrable.

---

## Phase 9: User Story 7 - Confiar en las regresiones que permiten cerrar la auditoría (Priority: P3)

**Goal**: CI de pruebas en push/PR con núcleo, plugin, instalador y Windows; runtimes mínimos comprobados antes de ejecutar; registro de cierre 9/9 (H-09 y evidencia transversal). La corrección de fixtures de H-09 ya se hizo en T009.

**Independent Test**: El workflow ejecuta todas las suites sin omisiones; un runtime por debajo del mínimo falla con mensaje diferenciado; closure.md relaciona cada hallazgo con requisitos, escenarios y evidencia.

- [X] T068 [P] [US7] Añadir preflight de runtimes mínimos (Python ≥3.12 en scripts/audit_037_smoke.py, Node ≥22.20.0 en scripts/audit_037_opencode_smoke.mjs, versión de PowerShell registrada en scripts/audit_037_windows_smoke.ps1) que falle con código 2 y mensaje `RUNTIME_UNSUPPORTED` antes de ejecutar casos; probar ejecutando cada runner con un intérprete simulado inferior o variable de prueba [7.3]
- [X] T069 [US7] Crear .github/workflows/test.yml (push y pull_request; sin tocar release.yml ni añadir lockfile): job Linux con Go según `go.mod`, `go vet ./...`, `go test ./... -count=1`, `go test -race` sobre usecases/persistence/cli, `scripts/coverage.sh` con umbral ≥80 %, `node --test infrastructure/plugin/opencode/gomemory.test.mjs`, `npm --prefix installer ci/typecheck/test`, Python 3.13 con `audit_037_smoke.py --case all`; job macOS con `--case installation`; job windows-latest con `go test ./...` y `audit_037_windows_smoke.ps1 -Case all` [7.2]
- [ ] T070 [US7] (PARCIAL 2026-10-11: sin act/actionlint; YAML validado y cada paso equivalente ejecutado en local en verde salvo Windows; falta el enlace a una ejecución real, que requiere push autorizado) Verificar el workflow localmente (`act` si está disponible, o revisión de sintaxis + ejecución manual de cada paso) y, tras push autorizado por la persona, registrar el enlace a la ejecución real como evidencia en specs/037-close-audit-gaps/evidence/closure.md [7.2]
- [X] T071 [US7] Completar specs/037-close-audit-gaps/evidence/closure.md: H-09 con T009 + CI; revisar las nueve filas (revisión, entorno, comando, esperado, observado, rutas de evidencia depurada) y fijar `closed` solo donde exista la validación correspondiente [7.4]

---

## Phase 10: Polish & Cross-Cutting Concerns

- [X] T072 [P] Documentar `mem sanitize` (preview/apply, códigos, ventana de mantenimiento con clientes antiguos detenidos, límites de copias externas y borrado físico) en docs/MANUAL.md y la verificación de integridad de los scripts en INSTALLATION.md
- [X] T073 [P] Añadir `mem sanitize` y la tabla de informe en docs/DATABASE.md (sin esquema nuevo; informe en `DataHome/maintenance/privacy`)
- [X] T074 Ejecutar quickstart.md Q1 completo (suites, vet, race, plugin, instalador, cobertura ≥80 % sin bajar la línea base de T001, `git diff --check`) y Q2–Q5 con `--case all`; registrar cifras actuales en specs/037-close-audit-gaps/evidence/baseline.md junto a la línea base
- [X] T075 Medir impacto y llamadores del diff con codebase-memory-mcp `detect_changes` y revisar llamadores de `RedactPrivate`, `IsPrivateContent`, `Builder.Build`, `replaceSelf` y `CmdWrap`; registrar el resultado en specs/037-close-audit-gaps/evidence/closure.md
- [X] T076 Actualizar el estado de cada hallazgo en docs/audits/2026-10-11-defensive-qa.md con enlace a specs/037-close-audit-gaps/evidence/closure.md, sin incluir payloads ni credenciales

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: depende de Setup; bloquea US1, US2 y US5.
- **US1 (Phase 3, P1)**: depende de Phase 2. MVP.
- **US2 (Phase 4, P1)**: depende de US1 (usa `ProtectText` y necesita lectores depurados para no reemitir lo que sanea).
- **US3 (Phase 5)**, **US4 (Phase 6)**, **US6 (Phase 8)**: dependen solo de Setup (T003 para su caso del runner); independientes entre sí y de US1/US2.
- **US5 (Phase 7)**: depende de T009.
- **US7 (Phase 9)**: T068 independiente; T069–T071 dependen de US1–US6.
- **Polish (Phase 10)**: depende de todas las historias.

### Within Each User Story

- Pruebas en rojo antes de la implementación; demostrar el rojo antes de corregir.
- Dominio → puertos → caso de uso → adaptadores → cableado → CLI → contrato → runner → evidencia.
- Las tareas que editan el mismo archivo no se marcan [P]: scripts/audit_037_smoke.py (T027, T042, T048, T051, T067) y infrastructure/plugin/opencode/gomemory.ts (T059–T061) son secuenciales.

### Hallazgo → tareas

| Hallazgo | Tareas |
| --- | --- |
| H-01 | T010–T028, T029–T043 |
| H-02 | T004–T008, T013, T022 |
| H-03 | T044–T048 |
| H-04 | T049, T051, T052, T056 |
| H-05 | T057, T059, T062–T064 |
| H-06 | T058, T060–T064 |
| H-07 | T065–T067 |
| H-08 | T050, T051, T053–T056 |
| H-09 | T009, T069–T071 |

---

## Parallel Example: User Story 1

```text
# Pruebas en rojo de US1 (archivos distintos):
T010 adapters/secondary/persistence/session_summary_test.go
T011 adapters/secondary/persistence/relation_privacy_test.go
T012 adapters/secondary/persistence/channel_activity_test.go
T013 adapters/secondary/persistence/compression_originals_test.go
T014 application/usecases/portability_test.go
T015 application/usecases/build_context_test.go
T016 adapters/primary/cli/cmd_mcp_session_summary_test.go
T017 tests/contract/session_privacy_cli_test.go
T018 tests/integration/compact_summary_hook_integration_test.go
```

## Parallel Example: tras Phase 2

```text
US1 (privacidad) ∥ US3 (T044–T046) ∥ US4 (T049–T053) ∥ US5 (T057–T061) ∥ US6 (T065–T066)
```

`∥` indica independencia lógica; la regla del proyecto es ejecución secuencial y delegar solo con ruta DELEGATE de Octopus.

---

## Implementation Strategy

### MVP First (User Story 1)

1. Phase 1 + Phase 2 (regla única de texto protegido y fixtures fiables).
2. Phase 3 (US1): escrituras y lecturas protegidas en todas las interfaces.
3. **Detenerse y validar** con T028 sobre binario real antes de continuar.

### Incremental Delivery

1. US1 → H-02 y H-01 (nuevas escrituras/lecturas).
2. US2 → H-01 histórico (`mem sanitize`).
3. US3, US6 → cambios acotados de Go (H-03, H-07).
4. US4 → H-04, H-08 (Unix; Windows con evidencia real).
5. US5 → H-05, H-06 (cerrados solo con host v2 real).
6. US7 + Polish → CI, registro 9/9, documentación.

### Criterio de entrega

Ninguna historia se declara terminada con tests verdes únicamente: se exige el runner sobre el binario compilado y la fila correspondiente en closure.md. Cobertura combinada <80 % o evidencia Windows/OpenCode v2 ausente mantienen la entrega abierta.
