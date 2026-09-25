---
description: "Task list for 033-native-context-compression"
---

# Tasks: Motor nativo de compresión de contexto (prácticas de Headroom)

**Input**: documentos de diseño en `specs/033-native-context-compression/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: se incluyen, por tres motivos: la constitución (§10) exige cobertura
de al menos el 80 % y pruebas de comportamiento; los criterios SC-001 a SC-010
de la spec son medibles por máquina; y el quickstart remite a pruebas
concretas. Cada prueba se escribe **antes** de su implementación y tiene que
fallar primero.

**Organization**: tareas agrupadas por historia de usuario (US1 a US5 de la spec).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (archivo distinto, sin dependencias pendientes)
- **[Story]**: historia de la spec (US1 a US5)
- Rutas relativas a la raíz del repo (`/home/admindocker/data/gomemory`)

## Reglas transversales (aplican a TODAS las tareas)

- Arquitectura hexagonal: `domain/` no importa nada de `application/`,
  `adapters/` ni `infrastructure/`; `application/` no importa adaptadores. El
  wiring solo vive en `infrastructure/container.go`.
- Cero dependencias nuevas en `go.mod`: solo la biblioteca estándar.
- Las cifras por defecto solo se declaran en `domain/compression_policy.go`.
- SQL parametrizado, sin `SELECT *`.
- Pruebas con el store aislado: usar `GOMEMORY_DATA_HOME`
  con `t.TempDir()` y **nunca** "la base más reciente" (patrón de
  `tests/integration/octopus_module_off_test.go`).
- Comentarios y documentación en español, con la densidad de comentarios del
  código vecino.
- Todo campo nuevo de ajustes se añade en `ports.SettingsData` **y** en el
  `Settings` de `adapters/secondary/persistence/settings.go`, y se mapea en
  los dos sentidos en `adapters/secondary/persistence/repositories.go` (si
  falta en uno, `Write` lo borra en silencio).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: corpus de referencia y esqueleto de paquetes.

- [X] T001 Crear `tests/testdata/compression_corpus/` con al menos 10 muestras reales y anonimizadas: `golist.json` (`go list -json ./...` del repo), `gitlog.txt` (`git log -200 --stat`), `gotest-fail.log` (`go test -v` con un fallo inducido), `go-panic.log` (traza de `panic` de Go), `py-traceback.log`, `mem-context.md` (`./mem context`), `search-results.txt` (`./mem search compresión`), `repo.diff` (`git show 5dece4b`), `sample.go` (copia de `application/usecases/build_context.go`), `sample.py`, `sample.ts`, `sample.java` (de licencia compatible, con cabecera de origen) y `prose.md` (3 memorias largas exportadas). Añadir `README.md` con el origen de cada muestra.
- [X] T002 Crear `tests/testdata/compression_corpus/expectations.json` con `{archivo: {"type": <content_type esperado>, "min_saving_vs_structural_pct": N}}`, y los valores: `golist.json` ≥ 70 respecto al original, logs ≥ 50, código ≥ 30, prosa ≥ 0 y corpus total ≥ 40 respecto a la compresión estructural.
- [X] T003 [P] Crear el paquete `adapters/secondary/compression/native/` con `doc.go`, que explique en español el propósito del paquete y enlace `specs/033-native-context-compression/research.md`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: contrato, dominio, ajustes y esquema que usan todas las historias.

**⚠️ CRITICAL**: ninguna historia empieza antes de terminar esta fase.

- [X] T004 Escribir la prueba de regresión SC-008 en `tests/integration/compression_structural_regression_test.go`: para cada muestra del corpus, `compression.StructuralCompressor{}.Compress` con `CompressionStructural` produce la misma salida que un golden generado ahora en `tests/integration/testdata/golden_structural/<muestra>.golden` (no `.out`: `.gitignore` ignora `*.out`)` (creado en esta tarea con el código de la v2.25.0). Tiene que pasar ANTES y DESPUÉS de toda la feature.
- [X] T005 Ampliar `application/ports/compressor.go`: añadir `CompressionMax` **después** de `CompressionNone` en el `iota` (no cambia los valores 0 y 1). Añadir a `CompressionResult` los campos opcionales `Compressor string`, `ContentType string`, `StructuralTokens int`, `Refs []string`, `FallbackReason string` y `LatencyMicros int64`, con un comentario que remita a `data-model.md`. `go build ./...` y `go test ./...` en verde.
- [X] T006 [P] Crear `domain/compression.go` con: el tipo `ContentType` (constantes `json`, `code`, `log`, `diff`, `table`, `prose`, `mixed` y `unknown`, más el subtipo de lenguaje), el tipo `Omission{Ref, Summary}`, `RenderTextMarker(o) string` → `⟦mem⟧ <Summary> · ref=<Ref>`, `RenderJSONMarker(o, extra map[string]string) map[string]any`, `IsMarkerLine(string) bool` y `RefFromHash(sha256hex string, n int) string`.
- [X] T007 [P] Crear `domain/compression_policy.go` con los valores de fábrica de data-model.md (`MinTokens=64`, `OriginalsTTLDays=7`, `OriginalsMaxBytes=256<<20`, `AdaptiveThreshold=0.20`, `AdaptiveMinOmissions=20`, `ToolOutputMinTokens=400`, `MaxInputBytes=2<<20` y `HookBudget=150ms`), el tipo `Aggressiveness` y la función `Thresholds(level int) (jsonMin, codeMinBody, logMinRun, proseMaxSentences int)` con la tabla de escalones 3, 2, 1 y 0. También `ParseCompressionLevel(s string, legacyDisabled bool) (level string)` → `"none" | "structural" | "max"`, con la precedencia de R13.
- [X] T008 [P] Escribir `domain/compression_test.go` y `domain/compression_policy_test.go`: el render del marcador, `IsMarkerLine`, la ampliación de ref ante colisión (12 → 16), `Thresholds` para cada escalón y `ParseCompressionLevel` (ausente → structural, legacy disabled → none, valor inválido → structural).
- [X] T009 [P] Crear `application/ports/clock.go` (`ClockPort interface { Now() time.Time }`, sin `ctx` porque no hace I/O) y su implementación real `SystemClock` en `adapters/secondary/clock/system.go`.
- [X] T010 Añadir a `ports.SettingsData` (`application/ports/settings_repository.go`) y a `persistence.Settings` (`adapters/secondary/persistence/settings.go`) los campos `ContextCompressionLevel string` (`context_compression_level`), `ToolOutputCompression bool`, `ConciseOutputDirective bool`, `CompressionMinTokens int`, `CompressionOriginalsTTLDays int`, `CompressionOriginalsMaxMB int` y `CompressionAdaptiveThresholdPct int`, con comentarios del mismo estilo que `OctopusEnabled`. Mapearlos en los dos sentidos en `adapters/secondary/persistence/repositories.go`.
- [X] T011 Escribir `adapters/secondary/persistence/settings_compression_test.go`: guardar el nivel `max`, después escribir `AutoApprove` mediante `Write` y comprobar que el nivel sigue siendo `max` (la regresión de la política de revisión que ya ocurrió); y que un `settings.json` sin la clave devuelve `""`.
- [X] T012 Añadir a `migrate` en `adapters/secondary/persistence/db.go` las tablas `compression_originals`, `delivered_blocks`, `compression_stats` y `compression_tuning`, con las columnas, claves primarias e índice `expires_at` de data-model.md (`CREATE TABLE IF NOT EXISTS`).
- [X] T013 Escribir `adapters/secondary/persistence/db_compression_migration_test.go`: la migración desde una base vacía y desde una base creada con el esquema de la v2.25.0 (sin las tablas nuevas) deja las 4 tablas presentes y es idempotente al ejecutarse dos veces.
- [X] T014 Ampliar `adapters/primary/cli/cmd_settings.go` con las banderas `--compression-level=none|structural|max`, `--tool-output-compression=true|false` y `--concise-output=true|false`, y mostrarlas en `--show`. Rechazar un nivel inválido con un error claro. Añadir la prueba en `adapters/primary/cli/cmd_settings_compression_test.go`.
- [X] T015 Resolver en `infrastructure/container.go` el nivel efectivo con `domain.ParseCompressionLevel(settings.ContextCompressionLevel, settings.ContextCompressionDisabled)` y exponerlo en `Deps` (`adapters/primary/cli/deps.go`) como `CompressionLevel ports.CompressionLevel`. Sin cambiar todavía el compresor inyectado.

**Checkpoint**: `go test ./...` en verde; T004 en verde; el nivel es configurable y se lee en `Deps`.

---

## Phase 3: User Story 1 - Contexto comprimido al máximo y reversible (Priority: P1) 🎯 MVP

**Goal**: en el nivel `max`, todo lo que entrega gomemory pasa por el motor nativo con marcadores recuperables.

**Independent Test**: `go test ./tests/integration/ -run TestCompressionCorpus` informa de un ahorro de al menos el 40 % frente a la compresión estructural y de al menos el 70 % en `golist.json`; cada ref se recupera byte a byte (quickstart Q2, Q3, Q4 y Q6).

### Tests for User Story 1

- [X] T016 [P] [US1] Escribir `adapters/secondary/compression/native/router_test.go`: cada muestra del corpus se clasifica con el `type` de `expectations.json`; un Markdown con cercas ```` ```go ```` y ```` ```json ```` se segmenta en 3 bloques con sus tipos; un JSON inválido o truncado se clasifica como `prose` o `log`, nunca como `json`.
- [X] T017 [P] [US1] Escribir `adapters/secondary/compression/native/guard_test.go` con `TestLiteralGuard`: se acepta una salida que es subsecuencia literal más marcadores; se rechaza una salida con una línea alterada (un carácter cambiado); se rechaza un JSON con un valor conservado distinto del original; y se acepta la línea Go `func X(a int) error { /* ⟦mem⟧ … */ }` cuando el prefijo hasta `{` es literal. Añadir también `TestProtectedTokens` (SC-006): en un conjunto de textos con rutas, URLs, versiones, mensajes de error e identificadores, cada uno de esos elementos que aparece en la salida del motor aparece literal, y los que están en líneas de ERROR nunca se omiten.
- [X] T018 [P] [US1] Escribir `adapters/secondary/compression/native/json_test.go`: con un array de 200 objetos (3 con `"error"`, uno con `latency_ms` a más de 3σ), la salida es JSON válido; conserva el primero, el último, los 3 errores y el atípico; tiene un único marcador con `ref` y `campos`; y ahorra al menos el 70 % en `golist.json`.
- [X] T019 [P] [US1] Escribir `adapters/secondary/compression/native/code_test.go`: en `sample.go` conserva todas las firmas y declaraciones de tipos literales y sustituye los cuerpos largos por un marcador; las funciones de menos de 6 líneas quedan intactas; en `sample.py`, `sample.ts` y `sample.java` conserva cabeceras y docstrings; con llaves desbalanceadas → `FallbackReason="literal_guard"` o degrada a estructural.
- [X] T020 [P] [US1] Escribir `adapters/secondary/compression/native/log_test.go`: todas las líneas `ERROR`, `FATAL` y `PANIC` siguen presentes; la cabecera de la traza y sus 5 primeros marcos quedan intactos; 312 líneas `INFO` de la misma forma se colapsan en primera + última + `⟦mem⟧ ×310 …`; y hay diff y tabla (cabecera y extremos conservados).
- [X] T021 [P] [US1] Escribir `adapters/secondary/compression/native/prose_test.go`: se eliminan las frases duplicadas; ninguna frase de salida se altera respecto a la entrada; y las frases con rutas, URLs o versiones se conservan siempre.
- [X] T022 [P] [US1] Escribir `adapters/secondary/compression/native/engine_test.go` con fakes de los puertos: un bloque por debajo de `MinTokens` sale idéntico; la misma entrada procesada 100 veces da una salida idéntica (FR-003); si el resultado no mejora a la estructural → `no_gain`; una entrada de más de 2 MB → `too_large`; un pánico dentro de un compresor → estructural con `FallbackReason="error"`; un `OriginalStoreRepository` falso que devuelve error → estructural con `store_unavailable` y sin refs; un almacén que tarda más de `StoreTimeout` → la misma degradación, porque se respeta el plazo; un texto con `<private>` o un secreto → sin refs, sin llamadas a `Put` y `FallbackReason="private"`; con un `CompressionStatsRepository` falso, una llamada a `Record` por compresión (un error de `Record` no altera la salida); y `Tokens <= StructuralTokens` siempre (INV-C3).
- [X] T023 [P] [US1] Escribir `adapters/secondary/persistence/compression_originals_test.go` con un reloj falso: `Put` y `Get` devuelven el original byte a byte; dos `Put` iguales dejan una sola fila; `Get` tras el TTL devuelve «no encontrado»; al superar el tope se expulsan primero los de `last_access_at` más antiguo; y `Get` refresca `last_access_at`.
- [X] T024 [P] [US1] Escribir `tests/integration/compression_privacy_test.go` (SC-009) por **todas** las vías que comprimen: `mem pack compress --level max`, `pack_build` (`BuildContextPack` con una spec de spec-kit que contiene `<private>ghp_FAKE…</private>`) y el paquete delegado de Octopus. En cada una: no se emiten `Refs`, no se inserta ninguna fila en `compression_originals` y la salida no contiene el secreto.
- [X] T025 [P] [US1] Escribir `tests/integration/compression_corpus_test.go` (`TestCompressionCorpus`): para cada muestra comprueba los mínimos de `expectations.json`, que cada ref emitida se recupera y contiene literalmente lo omitido (SC-005), y el ahorro agregado del corpus es de al menos el 40 % frente a la estructural (SC-001). Añadir `TestMemoriesNeverAltered` (FR-015): después de `get_context`, `search_memories` y `pack_build` en `max`, el contenido de cada memoria leído del repositorio es idéntico byte a byte al guardado.
- [X] T026 [P] [US1] Escribir `tests/contract/mcp_pack_tools_test.go` levantando el servidor MCP real con la conexión abierta (igual que las pruebas de Octopus, sin `printf | mem mcp`): `pack_compress{text}` sin `level` sigue funcionando; `pack_compress{text, level:"max"}` devuelve marcadores; `pack_retrieve{ref}` devuelve el original; una ref desconocida da un error MCP; y `pack_retrieve` está en `domain.MCPAutoApprovableToolsFor`.
- [X] T027 [P] [US1] Escribir `tests/integration/plan_context_suppression_max_test.go` (regresión de I1): con nivel `max` y sesión activa, tras entregar el contexto por cada vía (`mem context`, `get_context` por MCP y `mem hook session-start`), `get_plan_context` sustituye el historial por el aviso de supresión; lo mismo con nivel `structural` (comportamiento actual).
- [X] T028 [P] [US1] Escribir `tests/integration/hook_context_compression_test.go` (G1): con nivel `max` y memorias con contenido comprimible (JSON y logs), la salida de `mem hook session-start` y de `mem hook post-compact` contiene marcadores `⟦mem⟧ … ref=` recuperables con `mem pack retrieve`; con nivel `structural`, las dos salidas son idénticas a un golden de la v2.25.0.

### Implementation for User Story 1

- [X] T029 [P] [US1] Implementar `adapters/secondary/compression/native/segment.go` y `router.go` según R2 (precedencia: cercas → JSON/JSONL → diff → log/traza → código → tabla → prosa), usando solo la biblioteca estándar. T016 en verde.
- [X] T030 [P] [US1] Implementar `adapters/secondary/compression/native/guard.go` según R7: `CheckText(orig, out string) error` y `CheckJSON(orig, out any) error`. T017 en verde.
- [X] T031 [P] [US1] Implementar `adapters/secondary/compression/native/json.go` según R3: varianza por campo, suelo de claves de error, 3σ, frecuencia ≤ 1 %, extremos, un marcador por hueco con el resumen `campos`, recursión y conservación del orden original. Recibe `jsonMin` de `domain.Thresholds`. T018 en verde.
- [X] T032 [P] [US1] Implementar `adapters/secondary/compression/native/code_go.go` (con `go/parser` y `go/ast`, importaciones usadas, firmas, tipos, constantes, doc comments y cuerpos con marcador si superan `codeMinBody`) y `code_heuristic.go` (Python por sangría; Java, TS y JS por balance de llaves; SQL con `CREATE` conservado e `INSERT` colapsados; genérico por bloques). T019 en verde.
- [X] T033 [P] [US1] Implementar `adapters/secondary/compression/native/log.go`, `diff.go` y `table.go` según R5 (normalización solo para agrupar, emitiendo siempre una línea literal). T020 en verde.
- [X] T034 [P] [US1] Implementar `adapters/secondary/compression/native/prose.go` según R6 (dedup exacta y normalizada, párrafos largos 3 + 2 y frases con elementos protegidos). T021 en verde.
- [X] T035 [US1] Crear el puerto `application/ports/original_store.go` (`OriginalStoreRepository` con `ctx context.Context` como primer parámetro de `Put`, `Get`, `Purge` y `Usage`, como en data-model.md) e implementarlo en `adapters/secondary/persistence/compression_originals.go` con gzip, `ON CONFLICT` para deduplicar, TTL desde el último acceso, expulsión LRU al insertar y consultas con `QueryRowContext`/`ExecContext`, usando `ports.ClockPort`. T023 en verde.
- [X] T036 [US1] Implementar `adapters/secondary/compression/native/engine.go`: tipo `Engine` que implementa `ports.Compressor`. Con `CompressionNone` o `CompressionStructural` delega en `compression.StructuralCompressor` (misma salida, INV-C2). Con `CompressionMax`: (1) si `domain.RedactPrivate(text) != text` o `domain.RedactSecrets(text) != text`, entrega la estructural con `FallbackReason="private"` y termina (FR-024; así lo cumple **todo** llamador del puerto, no solo `CompressContent`); (2) comprueba el tamaño y aplica el umbral; (3) segmenta, enruta y aplica el compresor con `recover()`; (4) pasa la guarda; (5) compara con la estructural (`no_gain`); (6) guarda los originales de cada omisión por el puerto, dentro de un `context.WithTimeout(domain.StoreTimeout)` (puente E-002; si falla o vence el plazo → estructural con `store_unavailable`); (7) rellena los campos nuevos de `CompressionResult` y mide la latencia; (8) si hay `CompressionStatsRepository` (puede ser nil hasta US4), registra el resultado en modo best-effort. `OriginalStoreRepository`, `CompressionStatsRepository`, `ClockPort` y la función de agresividad se inyectan en el constructor. T022 en verde.
- [X] T037 [US1] Crear `application/usecases/compress_content.go`: `CompressContent(compressor, level, text, origin) (ports.CompressionResult, error)` como fachada fina que fija el nivel y el origen para las estadísticas y registra el uso en `UsageRecorder`. La privacidad y las estadísticas **no** se implementan aquí: viven en el `Engine` (T036). T024 en verde.
- [X] T038 [US1] Crear `application/usecases/retrieve_original.go`: `RetrieveOriginal(store, ref) (string, bool, error)`.
- [X] T039 [US1] Cablear en `infrastructure/container.go`: `Compressor` pasa a ser `native.NewEngine(originalsRepo, statsRepo=nil hasta US4, tuning=fija en 3 por ahora, clock.SystemClock{})` y el nivel efectivo de T015 se propaga a los llamadores: `BuildContextPack` (`ContextRequest.Compression`), `CompressText`, el paquete delegado de Octopus (`NewPackContractUseCase`) y `OptimizeToolDescription`, que se queda en `CompressionStructural` a propósito porque la descripción de las herramientas tiene que ser estable. `go test ./...` y T004 en verde.
- [X] T040 [US1] Aplicar el motor a la salida de `get_context` y `mem context` en nivel `max`: en `adapters/primary/cli/cmd_context.go` y en el handler `get_context` de `adapters/primary/cli/cmd_mcp.go`, calcular primero `usecases.HashDeContenido(documentoCrudo)` sobre la salida **sin procesar** de `ContextBuilder.Build()`, registrarlo en `DeliveryLog` y **después** pasar el documento por `CompressContent` con el origen `context`. Si se registrara el hash de la salida comprimida, la supresión que ya hace `get_plan_context` (que compara con el hash del documento sin procesar) dejaría de funcionar. En `structural`, la salida no cambia (SC-008).
- [X] T041 [US1] Aplicar el motor, con nivel `max`, al contexto que inyectan los hooks en `adapters/primary/cli/cmd_hook.go`: en `entregaContextoDeArranque` (session-start), registrar el hash del documento **sin procesar** y comprimir después con el origen `context`; en `printRecoveryAndContext` (pre-compact y post-compact), comprimir la salida de `usecases.BuildCompactionContext` y el contexto que la acompaña, sin tocar nunca `domain.RecoverySteps`. En `structural`, las salidas no cambian. T027 y T028 en verde.
- [X] T042 [US1] Aplicar el motor a los resultados de `search_memories`, `search_code` y `mem search` en `adapters/primary/cli/cmd_mcp.go` y `adapters/primary/cli/cmd_search.go`, con el origen `search`, solo en `max`. *(Ampliado al implementar: también `get_memory` y `mem get`, que son la vía por la que llega el contenido íntegro; `get_context` solo muestra extractos de 200 caracteres.)*
- [X] T043 [US1] Ampliar `adapters/primary/cli/cmd_pack.go`: `compress` acepta `--level`, `--json` y `--compare` (la tabla de 3 niveles del contrato); **corregir el defecto R0** imprimiendo una línea en blanco antes de `tokens: <raw> → <final> (<compresor>, -<pct>%)`; y añadir el subcomando `retrieve <ref>` (stdout byte a byte; si no se encuentra, código 2 y el mensaje del contrato en stderr). Actualizar el mensaje «subcomando requerido» con los nuevos subcomandos.
- [X] T044 [US1] Ampliar `adapters/primary/cli/cmd_mcp.go`: `pack_compress` con los campos opcionales `level` y `compare`; nueva herramienta `pack_retrieve{ref}`; y añadir `pack_retrieve` a la lista de herramientas en `domain/mcp_tools.go` y a `domain.MCPAutoApprovableToolsFor`. T026 en verde.
- [X] T045 [US1] Añadir la línea de protocolo del contrato (`Marcas ⟦mem⟧ … → pack_retrieve(ref=X)`) al bloque de protocolo que emiten `get_context` y los hooks de inicio (generador compartido en `domain/protocol.go`, marcador `gomemory-protocol-v7`), **solo** con el nivel `max`.
- [X] T046 [US1] Hacer que `mem install` (`adapters/primary/cli/cmd_install.go`) escriba `context_compression_level: "max"` solo cuando el proyecto no tenía ajustes; una reinstalación sobre ajustes existentes no toca el nivel. Prueba en `adapters/primary/cli/cmd_install_compression_test.go`.

**Checkpoint**: T004 y T016 a T028 en verde; se cumplen Q1 a Q4, Q6 y Q8 del quickstart con el binario compilado. **MVP entregable.**

---

## Phase 4: User Story 2 - No pagar dos veces lo mismo en una sesión (Priority: P1)

**Goal**: en `max`, el prefijo del contexto es estable byte a byte y en la misma sesión solo viaja el delta; el registro se reinicia al compactar.

**Independent Test**: `go test ./tests/integration/ -run TestSessionDelta20Turns` da una reducción de al menos el 80 % del contenido reenviado y un prefijo idéntico en el 100 % de los turnos sin cambios (quickstart Q5).

### Tests for User Story 2

- [X] T047 [P] [US2] Escribir `adapters/secondary/persistence/delivered_blocks_test.go`: `Mark` y `Seen` quedan acotados a la sesión activa; `Reset` borra solo la sesión activa; y sin sesión activa `Seen` devuelve false y `Mark` no hace nada.
- [X] T048 [P] [US2] Escribir `application/usecases/build_context_order_test.go`: en `max`, todo lo anterior a `<!-- mem:volatile -->` es idéntico entre dos construcciones hechas con relojes distintos y actividad reciente distinta; en `structural`, la salida es igual a la de la v2.25.0 (golden generado en esta tarea con el código actual).
- [X] T049 [P] [US2] Escribir `tests/integration/session_delta_test.go` (`TestSessionDelta20Turns`): simula 20 inyecciones de `get_context`, `get_plan_context` y `search_memories` con 2 memorias nuevas entre medias, y mide los tokens reenviados frente a la v2.25.0 (al menos un 80 % menos, SC-003) y la igualdad del prefijo (SC-004); tras `hook post-compact`, la inyección siguiente no contiene `ya entregad` (FR-019).

### Implementation for User Story 2

- [X] T050 [US2] Crear el puerto `application/ports/delivered_blocks.go` (`Seen`, `Mark` y `Reset`, con `ctx context.Context` como primer parámetro) e implementarlo en `adapters/secondary/persistence/delivered_blocks.go` con consultas `*Context`, resolviendo la sesión activa en cada llamada, igual que `DeliveryLogRepository`. T047 en verde.
- [X] T051 [US2] Reordenar `Builder.Build` en `application/usecases/build_context.go` **solo** cuando el nivel sea `max` (el `Builder` recibe el nivel por un campo nuevo): primero las secciones estables (reglas fijadas, conflictos, preferencias, decisiones, patrones, bugfixes, aprendizajes y sinapsis), sin marcas de tiempo; después `<!-- mem:volatile -->` y luego actividad reciente, código indexado, memoria conectada a código activo, sesión activa, sesiones recientes y anclas sin evidencia. Desempates deterministas por `id` en cada sección. T048 en verde.
- [X] T052 [US2] Crear `application/usecases/session_delta.go`: dado un documento o una lista de bloques (cada memoria renderizada es un bloque, con la huella de su contenido), sustituye los ya vistos por `- [id] título ⟦ya entregado⟧`, agrupa las series en `⟦mem⟧ N memorias ya entregadas en esta sesión, sin cambios · get_memory(id) para reconsultar` y marca los nuevos. No toca nunca la zona anterior al primer bloque de memoria (protocolo y reglas).
- [X] T053 [US2] Aplicar `SessionDelta` en `max` a `get_context` y `mem context` (`cmd_context.go` y `cmd_mcp.go`), al contexto inyectado por los hooks (T041), a `get_plan_context` (`application/usecases/build_plan_context.go`, conservando la supresión total que ya existe por hash cuando coincide todo), a `search_memories` (delta por resultado) y al **método de planificación como bloque entero** *(ampliado al implementar: era lo que se reenviaba completo cada turno; con él el ahorro de reenvío pasa del 46 % al 89,8 %)*. `pack_build` queda fuera: sus paquetes se piden para una tarea concreta. El hash que se registra en `DeliveryLog` sigue siendo siempre el del documento **sin procesar**, antes del delta y de la compresión.
- [X] T054 [US2] Reiniciar el registro en `adapters/primary/cli/cmd_hook.go`: `hookPreCompact`, `hookPostCompact` y `session-start` (fuentes `startup`, `clear` y `compact`) llaman a `DeliveredBlocks.Reset()` **y** borran las filas de `context_deliveries` de la sesión activa (hoy `hookPostCompact` no lo hace). Añadir `ResetContextDeliveries(db, sessionID)` en `adapters/secondary/persistence/context_delivery.go` y exponerlo en el puerto `ports.DeliveryLog` como `Reset() error`.
- [X] T055 [US2] Comprobar que el plugin de OpenCode (`infrastructure/plugin/opencode/gomemory.ts`) llama a `mem hook post-compact` o `pre-compact` en `session.compacted` (en las ramas v1 y v2). Si no llama a ninguno, añadir la llamada a un subcomando que reinicie el registro sin imprimir contexto. T049 en verde.

**Checkpoint**: se cumple Q5; T048 confirma que `structural` no cambia.

---

## Phase 5: User Story 3 - Comprimir cualquier salida de herramienta (Priority: P2)

**Goal**: la entrada genérica (CLI y MCP) ya existe desde US1; aquí se añade el hook opt-in que comprime las salidas de las herramientas en los runtimes que lo permiten.

**Independent Test**: `go test ./tests/contract/ -run TestHookToolOutput` en verde; quickstart Q7 contra Claude Code 2.1.282, Codex 0.157.0 y OpenCode 1.18.32.

### Tests for User Story 3

- [X] T056 [P] [US3] Capturar los fixtures reales de `tool_response` en `tests/contract/testdata/tool_output/`: Claude Code (`Bash` con `go list -json`, `Grep`, `WebFetch` y una herramienta MCP de otro servidor), Codex (una herramienta MCP) y OpenCode (la salida de `bash`). Registrar en `README.md` la versión del runtime de la que se capturó cada uno.
- [X] T057 [P] [US3] Escribir `tests/contract/hook_tool_output_test.go` (`TestHookToolOutput`): para cada fixture, las claves y los tipos de la salida son idénticos a los de `tool_response` y los tokens de texto bajan (H2); `Read`, `Edit`, `Write`, `MultiEdit`, `NotebookEdit` y `mcp__gomemory__*` producen stdout vacío (H3); con el ajuste apagado, stdout vacío; por debajo de 400 tokens, stdout vacío; con un reloj que supera los 150 ms, stdout vacío y código 0 (H1); un texto privado no produce refs (H4); en Codex, una herramienta que no es MCP produce stdout vacío.

### Implementation for User Story 3

- [X] T058 [US3] Implementar `mem hook tool-output <claude|codex|opencode>` en `adapters/primary/cli/cmd_hook.go` (nuevo `case "tool-output"` y la función `hookToolOutput` en `adapters/primary/cli/hook_tool_output.go`): lee stdin, aplica exclusiones y umbral, recorre `tool_response` reescribiendo solo `stdout`, `stderr`, `output`, `content` (string) y `content[].text` mediante `CompressContent` con el origen `tool_output` y el nivel `max`, emite `updatedToolOutput` (Claude), `updatedMCPToolOutput` (Codex) o `{"output": …}` (OpenCode), y corta con el presupuesto `domain.HookBudget`. Añadir `--enabled`, que imprime `true` o `false`. T057 en verde.
- [X] T059 [US3] Registrar `PostToolUse` con el matcher `Bash|Grep|Glob|WebFetch|WebSearch|mcp__.*` → `tool-output claude` en `adapters/primary/setup/claude_code_setup.go`, **solo** con `ToolOutputCompression=true`; con el ajuste apagado, `mem install` elimina el registro si existe. Incluir `PostToolUse` en la lista de claves que limpia `cmd_uninstall.go` (ya está) y comprobarlo. Prueba en `adapters/primary/setup/claude_code_setup_tool_output_test.go`.
- [X] T060 [US3] Registrar `PostToolUse` → `tool-output codex` en `adapters/primary/setup/codex_setup.go` con el ajuste activo; la consolidación de hooks de Codex se hace en `setupCodexGlobal`, siguiendo la decisión registrada. Prueba en `adapters/primary/setup/codex_setup_tool_output_test.go`.
- [X] T061 [US3] En `infrastructure/plugin/opencode/gomemory.ts` (rama v1), dentro de `tool.execute.after` y sin tocar la captura de `task` que ya existe: la primera vez consulta y guarda `mem hook tool-output --enabled`; si está activo y la herramienta no está excluida, envía `{"tool", "output"}` a `mem hook tool-output opencode` y asigna `output.output` si hay respuesta. Siempre en modo best-effort, dentro de `try/catch`.
- [X] T062 [US3] Validar Q7 en OpenCode 1.18.32 (binario `/root/.opencode/bin/opencode`) y en OpenCode 2.x (el paquete que se usó en la feature 032, en un HOME aislado): anotar si la versión comprimida llega al modelo. Si en v2 mutar `ev.result` en `ctx.tool.hook("execute.after")` surte efecto, cablearlo en la rama v2 del plugin; si no, dejarla sin cablear y documentarlo. ⚠ Resultado incierto: depende del binario. **Resultado (2026-09-25):** Claude Code 2.1.282 verificado de extremo a extremo (la transcripción guarda como `tool_result` la versión comprimida; el original recuperable es el stdout que Claude pasa al hook, ya truncado a 30 000 caracteres). OpenCode 1.18.32 **sin verificar**: el proveedor Anthropic no tenía crédito y el modelo gratuito `opencode/big-pickle` agotó 240 s sin ejecutar la herramienta. OpenCode 2.x: sin verificar; la rama v2 del plugin no se cablea. Codex: sin verificar en vivo (solo MCP según el binario). La tarea se cierra con estado conservador: `mem doctor` no anuncia soporte en los runtimes sin evidencia y el manual documenta el límite.
- [X] T063 [US3] Guardar en gomemory el resultado real por runtime con `save_memory(topic_key="runtime-tool-output-rewrite")`, actualizando la memoria que ya existe.

**Checkpoint**: se cumple Q7; con el ajuste apagado no hay ningún registro nuevo en los hooks (huella cero).

---

## Phase 6: User Story 4 - Medir el ahorro y ajustarlo solo (Priority: P2)

**Goal**: estadísticas por compresor, comparación, tasa de recuperación y ajuste adaptativo de la agresividad.

**Independent Test**: `go test ./application/usecases/ -run TestAdaptiveLowersAggressiveness` y `./mem pack savings` muestran las cifras y el ajuste (quickstart Q9).

### Tests for User Story 4

- [X] T064 [P] [US4] Escribir `adapters/secondary/persistence/compression_stats_test.go`: `Record` acumula `uses`, los tokens por etapa, `omissions`, `fallbacks` y la latencia; `RecordRetrieval` suma en el par `(compressor, content_type)`; `Summary` devuelve una fila por compresor; y ninguna columna contiene texto del contenido.
- [X] T065 [P] [US4] Escribir `application/usecases/compression_tuning_test.go` (`TestAdaptiveLowersAggressiveness`) con reloj falso: 20 omisiones de `code/python` con 5 recuperaciones (25 %) bajan la agresividad a 2 con su motivo; 19 omisiones no ajustan nada; la agresividad nunca sube sola; `Reset` la devuelve a 3; y el nuevo escalón cambia los umbrales que recibe el compresor.

### Implementation for User Story 4

- [X] T066 [US4] Crear los puertos `application/ports/compression_stats.go` (`CompressionStatsRepository` y `CompressionTuningRepository`, con `ctx context.Context` como primer parámetro en todas sus operaciones) e implementarlos en `adapters/secondary/persistence/compression_stats.go` y `compression_tuning.go`, con `INSERT … ON CONFLICT DO UPDATE` y consultas `*Context`. T064 en verde.
- [X] T067 [US4] Inyectar el `CompressionStatsRepository` real en el `Engine` desde `infrastructure/container.go`, sustituyendo el nil de T039; el registro ya lo hace el `Engine` (T036) para todas las vías de compresión. Comprobar con una prueba de integración que `pack_build`, `get_context` y `mem pack compress` suman en `compression_stats`.
- [X] T068 [US4] Ampliar `RetrieveOriginal` (`application/usecases/retrieve_original.go`): sumar la recuperación y evaluar el ajuste adaptativo (`application/usecases/compression_tuning.go`: si `retrievals/omissions > umbral` y `omissions >= AdaptiveMinOmissions` en la ventana, `Lower`). T065 en verde.
- [X] T069 [US4] Sustituir la agresividad fija de T039 por `CompressionTuningRepository.Get(ctx, project, type)` en el constructor del `Engine` (`infrastructure/container.go`), con el mismo puente `context.WithTimeout(domain.StoreTimeout)` y agresividad 3 si falla la lectura.
- [X] T070 [US4] Crear `application/usecases/compression_savings.go` (el informe del contrato: tabla por compresor, ajustes vigentes y uso de originales) y exponerlo como `mem pack savings [--json]` en `adapters/primary/cli/cmd_pack.go` y como la herramienta MCP `pack_savings` en `cmd_mcp.go` y `domain/mcp_tools.go`.
- [X] T071 [US4] Añadir `mem pack tune --reset [--type T]` y `mem pack purge` a `adapters/primary/cli/cmd_pack.go` (purge usa `OriginalStoreRepository.Purge` e imprime `purgados N (X MB)`).
- [X] T072 [US4] Añadir la sección «Compresión» a `adapters/primary/cli/cmd_doctor.go`, con la clave `compression` en `--json`: el nivel y su origen, el uso de originales frente al tope (aviso a partir del 90 %), el estado del hook por runtime (`activo | inactivo | parcial (solo MCP) | no soportado`, según los resultados de T062) y los ajustes adaptativos. Con `--strict`, código distinto de 0 si el nivel es `max` y el almacén no se puede escribir. Prueba en `adapters/primary/cli/cmd_doctor_compression_test.go` (Q8).

**Checkpoint**: se cumplen Q8 y Q9.

---

## Phase 7: User Story 5 - Respuestas más breves del agente (Priority: P3)

**Goal**: una directiva de concisión opcional en la zona volátil.

**Independent Test**: con `--concise-output=true`, `mem context` contiene la directiva después de `<!-- mem:volatile -->` y el prefijo no cambia.

- [X] T073 [P] [US5] Escribir `application/usecases/build_context_concise_test.go`: directiva presente solo con el ajuste activo y siempre después del separador volátil; prefijo idéntico con y sin la directiva.
- [X] T074 [US5] Añadir la directiva (una o dos frases en español que pidan respuestas concisas sin omitir información necesaria) al final de la zona volátil en `application/usecases/build_context.go` cuando `ConciseOutputDirective=true`. Si el nivel no es `max` y no existe zona volátil, añadirla al final del documento. T073 en verde.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T075 [P] Añadir a la pantalla de configuración de `adapters/primary/tui/tui.go` la fila «Compresión: ninguna/estructural/máxima» (cíclica con enter) y los interruptores «Comprimir salidas de herramientas» y «Respuestas concisas», siguiendo el patrón de la fila de Octopus. Al activar las salidas de herramientas, avisar de que hay que ejecutar `mem install` para registrar el hook.
- [X] T076 [P] Escribir los benchmarks de `adapters/secondary/compression/native/bench_test.go` sobre cada muestra del corpus, informando de ns/op y tokens; comprobar el p95 por debajo de 50 ms por cada 10 k tokens (SC-007, Q10) y optimizar el compresor que no llegue.
- [X] T077 [P] Documentar en español en `docs/MANUAL.md` (niveles, marcadores, `mem pack retrieve|savings|tune|purge`, `mem settings --compression-level`, el hook opt-in y sus límites por runtime) y en `docs/MEMORY-PROTOCOL.md` (la línea `⟦mem⟧ → pack_retrieve`).
- [X] T078 [P] Añadir la entrada de `CHANGELOG.md` para la versión siguiente, con la línea base y el ahorro medido en el corpus.
- [X] T079 Ejecutar `go vet ./...`, `gofmt -l .` y `go test ./... -count=1 -cover`; la cobertura de `adapters/secondary/compression/native` tiene que ser de al menos el 80 %.
- [X] T080 Compilar el binario y ejecutar el quickstart Q0 a Q10 completo contra él (regla de trabajo 2), anotando en `specs/033-native-context-compression/quickstart.md` los valores reales medidos en cada paso.
- [X] T081 Ejecutar la revisión adversarial (skill `adversarial-consensus-review`) sobre el diff de la feature antes de publicar, y cerrar cada hallazgo confirmado con prueba de regresión.
- [X] T082 Guardar en gomemory las decisiones que cambien durante la implementación (`topic_key="spec-033-native-compression"`) y cerrar la sesión con `end_session`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (1)**: sin dependencias.
- **Foundational (2)**: depende de Setup y **bloquea** todas las historias.
- **US1 (3)**: depende de la fase 2. Es el MVP.
- **US2 (4)**: depende de la fase 2. Es independiente de los compresores de US1: el reorden y los deltas no necesitan el motor. Sus tareas T040, T041 y T053 tocan los mismos handlers (`cmd_context.go`, `cmd_mcp.go` y `cmd_hook.go`, que también toca T054), así que se hacen en serie si coinciden en el tiempo.
- **US3 (5)**: depende de US1 (T036 y T037: el motor y `CompressContent`) y de T010 (el ajuste).
- **US4 (6)**: depende de US1 (T035 a T038). T069 sustituye lo cableado en T039.
- **US5 (7)**: depende de T051 (zona volátil). Si US2 no está hecha, aplica el caso sin zona volátil de T074.
- **Polish (8)**: depende de las historias que se incluyan en la entrega.

### Dentro de cada historia

- Las pruebas se escriben primero y tienen que fallar; después la implementación.
- Puertos → persistencia → casos de uso → CLI, MCP y hooks → wiring.
- T004 (la regresión de `structural`) se vuelve a ejecutar al cerrar cada fase.

### Parallel Opportunities

- Fase 2: T006, T007, T008 y T009 en paralelo; T010 → T011; T012 → T013.
- US1: todas las pruebas T016 a T028 en paralelo; los compresores T029 a T034 en paralelo (cada uno en su archivo); después, en serie, T035 → T036 → T037 a T046.
- US2 en paralelo con US1 cuando termina la fase 2 (salvo el solapamiento anotado arriba).
- US3 y US4 en paralelo entre sí cuando termina US1.

---

## Parallel Example: User Story 1

```text
# Todas las pruebas de US1 a la vez:
T016 router_test.go · T017 guard_test.go · T018 json_test.go · T019 code_test.go
T020 log_test.go · T021 prose_test.go · T022 engine_test.go · T023 compression_originals_test.go
T024 compression_privacy_test.go · T025 compression_corpus_test.go · T026 mcp_pack_tools_test.go
T027 plan_context_suppression_max_test.go · T028 hook_context_compression_test.go

# Después, todos los compresores a la vez:
T029 router/segment · T030 guard · T031 json · T032 code_go + code_heuristic · T033 log/diff/table · T034 prose
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Fase 1 y fase 2 (T001 a T015).
2. Fase 3, US1 (T016 a T046).
3. **Parar y validar**: quickstart Q1 a Q4 y Q6 con el binario compilado. `structural` tiene que seguir igual (T004).
4. Ya entrega el grueso del ahorro: JSON, logs y código comprimidos y recuperables en todo lo que entrega gomemory.

### Incremental Delivery

1. MVP (US1) → release menor.
2. \+ US2 (deltas y caché) → ahorro en sesiones largas, sin pérdida.
3. \+ US4 (medición y ajuste) → decisión informada y ajuste automático.
4. \+ US3 (hook de salidas de herramientas, opt-in) → ahorro fuera de gomemory.
5. \+ US5 (concisión) → opcional.

### Notas

- La tarea T062 es la única de resultado incierto (el comportamiento real de OpenCode 2.x) y no bloquea nada más.
- Commit después de cada tarea o grupo lógico, con Conventional Commits en español, como el historial del repo.
