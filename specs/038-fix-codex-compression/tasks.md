# Tasks: Compresión compatible según capacidades del agente

**Input**: documentos de `specs/038-fix-codex-compression/`.

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/retrieval-policy.md](contracts/retrieval-policy.md), [contracts/validation-evidence.md](contracts/validation-evidence.md), [quickstart.md](quickstart.md) y constitución v2.0.0.

**Tests**: incluidos por los escenarios de aceptación y FR-007–011; el plan exige pruebas de prioridad, migración, preservación y evaluación. Las pruebas de comportamiento T005, T006, T011–T013 y T019 se escriben y fallan antes de implementar. T009 y T016 son verificaciones de integración posteriores, indicadas expresamente como tales; no alterar expectativas solo para ocultar fallos.

**Organization**: seis fases; tareas por historia. Todas están pendientes. No cambiar motor, umbrales, esquemas MCP, permisos o hooks compatibles; no reinstalar rewrite Codex ni generar AGENTS por proyecto.

## Format: `[ID] [P?] [Story] Description`

- `[P]`: ejecutable en paralelo con tareas de su misma oleada después de cumplir los prerrequisitos de fase; archivos distintos y sin dependencias pendientes entre ellas.
- `[US1]`, `[US2]`, `[US3]`: corresponden a las historias de la especificación.
- Rutas relativas a la raíz del repositorio. Las rutas nuevas son entregables, no archivos que se afirme que ya existen.
- `implementation-notes.md` es bitácora saneada de resultados/comandos, no transcripción ni almacén de secretos.
- Las capturas reales se guardan localmente bajo `.memory/validation/038/` con acceso restrictivo y no se versionan; solo se publica un resumen saneado con hashes.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: fijar procedencia y un entorno de pruebas seguro sin inicializar un proyecto nuevo ni añadir dependencias de producción.

- [X] T001 Registrar revisión del checkout, versiones Go/Python/Codex y cambios ajenos en `specs/038-fix-codex-compression/implementation-notes.md`. En un cambio de dependencias separado, crear `pyproject.toml` mínimo sin paquete instalable, con grupo `dev` que fije `pytest==9.1.1` y `pip-audit==2.10.1`; generar/versionar `uv.lock` mediante `uv lock` y exportar `scripts/requirements-038-test.txt` con todas las transitivas y hashes desde ese lock. Documentar Context7, fuentes oficiales, Python 3.13, `pip-audit --require-hashes`, reversión y resultado. Comprobar que `uv export --locked --only-dev --format requirements.txt` reproduce el export versionado y sincronizar el grupo `dev` en entorno aislado antes de T019; si falla, dejar la puerta pendiente. No modificar configuración personal.
- [X] T002 Documentar el aislamiento de HOME, XDG, GOMEMORY_DATA_HOME y PATH para pruebas de instalación en `specs/038-fix-codex-compression/implementation-notes.md`, tomando helpers existentes como base y sin invocar install/update sobre HOME real. Hacer inventario temprano, solo lectura, de la versión Codex objetivo: candidato de interfaz por llamada para resultados TUI y candidato de oráculo independiente de entrada del modelo, con referencia de implementación; si alguno falta, registrar `measurement_unavailable` como riesgo de bloqueo de SC-003 antes de construir el evaluador. Esta inspección no certifica la captura; T024 sigue siendo la puerta definitiva.

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: fijar fixtures compartidos que permiten probar prioridades y preservación sin red, credenciales o nuevas tablas.

- [X] T003 [P] Crear en `tests/fixtures/038/capability-profiles.json` casos sintéticos Codex incompatible, rewrite compatible con exclusiones, capacidad desconocida, índice Go ausente, otro lenguaje, cuerpo requerido y compresión sin ganancia; declarar para cada caso prioridad, alternativa y afirmaciones prohibidas según `contracts/retrieval-policy.md`.
- [X] T004 [P] Crear `tests/fixtures/038/protocol-v8.md` con un bloque gomemory v8 y su delimitador final, baseline universal y bloques ajenos antes/después de CodeGraph y grafo externo, además de marcadores de contenido público para verificar preservación en migración y repetición.

**Checkpoint**: fixtures públicos disponibles; no hay nuevos ports, esquemas o dependencias. Ninguna tarea de historia empieza antes de esta fase.

## Phase 3: User Story 1 - Obtener contexto comprimido por una vía compatible (Priority: P1) 🎯 MVP

**Goal**: el protocolo que reciben las sesiones nuevas establece prioridad por capacidad y suficiencia, no una obligación externa contradictoria.

**Independent Test**: inicializar el servidor MCP de prueba y revisar su protocolo frente a los perfiles; verificar una búsqueda Go elegible y la alternativa para cuerpo/lenguaje no cubierto, conservando evidencia necesaria. La selección autónoma en TUI y el ahorro final se demuestran en US3, no se deducen de estos tests.

### Tests for User Story 1

- [X] T005 [P] [US1] Ampliar `adapters/primary/cli/protocol_codex_test.go` con pruebas rojas de la precedencia descrita en el contrato: fuente comprimida suficiente antes del proveedor externo en Codex, alternativa explícita por insuficiencia, perfiles compatibles/desconocidos y ausencia de obligación externa incondicional; usar los casos de T003.
- [X] T006 [P] [US1] Añadir `adapters/primary/cli/cmd_mcp_code_tools_policy_test.go` con pruebas de la superficie existente: `search_code` entrega resultado íntegro sin ganancia/compresor o con nivel none y conserva referencias con max; `get_symbol` solo ofrece firma/ubicación/relaciones y no se trata como cuerpo equivalente; usar servidor MCP real con dobles locales del repositorio, sin alterar motor.

### Implementation for User Story 1

- [X] T007 [US1] Definir la constante pura `CodeRetrievalPolicy` en `domain/code_retrieval_policy.go` con la regla canónica de `contracts/retrieval-policy.md`: capacidad, cobertura Go y suficiencia; límites de get_symbol, pack_build y pack_compress; degradación explícita y ausencia de promesas de compresión desconocida (dep: T005, T006).
- [X] T008 [US1] Sustituir los dos párrafos contradictorios de `buildIntegrationBlock()` en `adapters/primary/cli/cmd_install.go` por `domain.CodeRetrievalPolicy`, conservando el resto del protocolo, los nombres MCP y el bootstrap externo disponible; no introducir un detector runtime ni modificar templates universales (dep: T007).
- [X] T009 [US1] Añadir en `adapters/primary/cli/code_retrieval_policy_test.go` una verificación de integración posterior al cambio mediante MCP initialize con `newMCPServer()`: comprobar que `ServerOptions.Instructions` entrega el mismo fragmento canónico y una sola precedencia, sin duplicar registro de herramientas (dep: T008).
- [X] T010 [US1] Ejecutar las pruebas dirigidas US1 y registrar sus resultados y límites en `specs/038-fix-codex-compression/implementation-notes.md`; demostrar contrato y suficiencia de las respuestas, sin afirmar ahorro o elección autónoma del agente (dep: T009).

**Checkpoint**: MVP del protocolo en sesiones nuevas verificable. Instalaciones globales v8 y recordatorios siguen pendientes de US2; no publicar el cierre completo con US1 sola.

## Phase 4: User Story 2 - Recibir instrucciones consistentes durante la sesión (Priority: P2)

**Goal**: instrucciones permanentes gestionadas, initialize y recordatorios comparten la misma política; actualizar instalaciones existentes sin destruir contenido ajeno.

**Independent Test**: componer/migrar el fixture v8, reinstalar dos veces en HOME aislado y comparar el bloque global, initialize y recordatorios de usuario/subagente; comprobar preservación, idempotencia y ausencia del hook incompatible.

### Tests for User Story 2

- [X] T011 [P] [US2] Añadir en `adapters/primary/cli/cmd_install_protocol_test.go` pruebas rojas v8→v9, conservación del delimitador final y baseline, bloques ajenos antes/después intactos y segunda composición idempotente; conservar los casos legados v5/v6 y cargar T004.
- [X] T012 [P] [US2] Ampliar `adapters/primary/setup/atomic_plan_global_test.go` para comprobar instalación global repetida con contenido ajeno preservado, HOME/XDG/PATH aislados y configuración ausente; cubrir el destino Codex sin escribir `~/.codex/AGENTS.md` real.
- [X] T013 [P] [US2] Ampliar `adapters/primary/cli/code_retrieval_policy_test.go` con pruebas rojas de fragmento canónico compartido en `memoryProtocolReminder`, recordatorio de usuario y subagente, y ausencia de promesa automática en perfiles desconocidos; comprobar que descubrir herramientas externas sigue permitido para alternativas.

### Implementation for User Story 2

- [X] T014 [US2] Reemplazar únicamente el párrafo externo contradictorio de `memoryProtocolReminder` en `adapters/primary/cli/cmd_hook.go` por `domain.CodeRetrievalPolicy`, conservando reglas de memoria, privacidad, bootstrap y presupuesto/dialecto de hooks (dep: T013).
- [X] T015 [US2] Incrementar `ProtocolVersionMarker` de v8 a v9 en `domain/protocol.go` para activar la actualización por `composeAgentFile`, manteniendo el marcador universal v1 y el patrón de reconocimiento de versiones anteriores (dep: T011, T012, T014).
- [X] T016 [US2] Añadir una verificación de integración posterior al cambio en `tests/integration/protocol_block_test.go`: bloque nuevo servido por MCP, no crear AGENTS por proyecto, conservar retirada con respaldo de archivos legados y mantener retirada de `CodexToolOutputHook`; operar solo con fixtures/entorno aislado (dep: T015).
- [X] T017 [US2] Ejecutar contratos de composición, instalación global, recordatorios y hooks Codex/Claude/OpenCode, y registrar preservación, idempotencia y ausencia de rewrite Codex en `specs/038-fix-codex-compression/implementation-notes.md` (dep: T016).
- [X] T018 [US2] Actualizar `specs/038-fix-codex-compression/quickstart.md` con los nombres de tests y procedimiento real de actualización global aislada verificados en T017; explicar que conflictos de instrucciones externas ajenas se advierten y no se corrigen sobrescribiéndolas.

**Checkpoint**: política coherente en todas las superficies gestionadas y migración verificada. US3 todavía debe comprobar conducta y ahorro en una sesión real.

## Phase 5: User Story 3 - Demostrar uso real y ahorro atribuible (Priority: P3)

**Goal**: evaluar evidencia real mediante una unidad común, respuestas completas y referencias equivalentes; conservar un estado pendiente/bloqueado si no se demuestra el cierre.

**Independent Test**: el evaluador se prueba offline con casos válidos/adversos sin depender de US1/US2; su aceptación integrada exige la versión implementada de ambas historias, una TUI real y tres consultas fijadas antes de la sesión. Fixtures simulados nunca cuentan como aceptación interactiva.

### Tests for User Story 3

- [X] T019 [US3] Añadir pruebas rojas con pytest en `scripts/test_validate_038_evidence.py` a partir de `contracts/validation-evidence.schema.json`, `.example.json` y `capture-certification.schema.json`: forma raíz/campos/tipos, referencias y IDs, hashes/rutas, ahorro válido, overhead y recuperaciones adicionales, Unicode/CRLF, deltas negativos/nulos, firma vs cuerpo, pérdida de hechos, claves de comparación distintas, coaching, sesión exec/sintética, métricas históricas, capturas incompletas/no textuales, clave JSON duplicada y `capture_verified=true` con ruta distinta de la fijada, certificado ausente, corrupto, autofirmado, lados incompletos o trazas divergentes; no copiar secretos en fixtures. Ejecutar `python3 -m pytest -q scripts/test_validate_038_evidence.py` y comprobar el fallo pertinente antes de T020 (dep: T001).

### Implementation for User Story 3

- [X] T020 [US3] Implementar en `scripts/validate_038_evidence.py` carga JSON UTF-8 estricta y validación stdlib de los esquemas de evidencia y certificado y sus invariantes cruzados: campos/tipos, claves duplicadas, IDs y vínculos, claves de comparación, hashes/rutas locales, procedencia TUI y checks. Para `capture_verified`, resolver su `evidence_ref` dentro de `.memory/validation/038/`, validar `capture-certification.json`, existencia y SHA-256 de fixtures/extractores/trazas y referencia de revisión, lados observed/reference con IDs y marcadores distintos, versión/MCP iguales, oráculo distinto del adaptador e igualdad byte a byte por probe; un booleano afirmativo sin certificado válido produce `blocked`. Rechazar entrada inválida sin ejecutar texto, acceder a red o importar dependencia externa (dep: T019).
- [X] T021 [US3] Implementar en `scripts/validate_038_evidence.py` el codec `codex-tui-visible-text-utf8-v1`: abrir cada `trace_path` como bytes de un único payload, verificar SHA-256, decodificación UTF-8 estricta, igualdad exacta con `visible_text`, punto `codex_tui_model_input` y versión/binario de ambos lados; contar bytes sin normalizar, sumar llamadas observadas adicionales una vez y conservar deltas firmados. Bloquear formatos estructurados/no textuales o referencias sin el mismo punto de captura (dep: T020).
- [X] T022 [US3] Implementar en `scripts/validate_038_evidence.py` la CLI `--input/--output`, agregados, causas y estados `closed/pending/blocked` con exits 0/1/2 según el contrato; exigir tres consultas autónomas, hechos preservados, pruebas de superficies/regresión y ahorro positivo para cerrar; ejecutar `python3 -m pytest -q scripts/test_validate_038_evidence.py` y registrar resultado en `specs/038-fix-codex-compression/implementation-notes.md` (dep: T021).
- [X] T023 [US3] Fijar `specs/038-fix-codex-compression/evidence/query-manifest.json` con raíz `{schema_version:1, queries:[...]}` mediante preflight externo a la aceptación: tres prompts naturales elegibles sin nombres de herramientas, `comparison_key` y hechos/referencias requeridos según el esquema, más SHA-256 de los bytes del archivo; usar los ejemplos de quickstart y ajustar solo antes de la sesión, sin escoger consultas tras fallos. Fijar aparte en `evidence/fallback-manifest.json` un prompt natural que necesite el cuerpo literal de una función Go conocida, frente a la ruta comprimida estructural que solo entrega firma/relaciones; declarar evidencia esperada de fuente alternativa, motivo comunicado y prohibición de afirmar compresión sin prueba. Fijar su hash antes de la TUI (dep: T022).
- [X] T024 [US3] Preparar `.memory/validation/038/environment.json` con binario/revisión/MCP/hash, protocolo v9, compresión max y versión Codex. Antes de T025, identificar API/evento/campo exacto de la versión instalada **y oráculo independiente de entrada del modelo**; documentar procedencia y extractores. Ejecutar dos probes MCP controlados en TUI (observed y reference `compression=none`, marcadores distintos, Unicode, CRLF, encabezado, metadatos y aviso), comparar trazas adaptador/oráculo byte a byte por llamada y guardar `.memory/validation/038/capture-certification.json` conforme al esquema con hashes y revisión responsable. `surface_checks.capture_verified.evidence_ref` apunta a ese archivo. `codex exec --json`, `codex debug prompt-input` y pantalla no certifican por sí solos la TUI. Si falta interfaz/oráculo o falla igualdad, registrar `unsupported`/`measurement_unavailable` y bloqueo; no iniciar T025–T028 ni inferir bytes MCP crudos (dep: T017, T023).
- [ ] T025 [US3] Ejecutar una sesión Codex TUI real sobre T023 y guardar en `.memory/validation/038/observations.json` la raíz `{agent_profile, session, calls}` con solo llamadas observadas: argumentos originales, claves, rutas, `visible_text` y trazas/hash realmente visibles mediante **el adaptador certificado en T024**, incluidos originales necesarios; no nombrar herramientas ni corregir su elección. Ejecutar además el caso conductual negativo prefijado en T023, separado de las tres consultas de ahorro: observar respuesta final, alternativa elegida y explicación de la limitación, y guardar un registro saneado en `.memory/validation/038/fallback-observation.json`. Si falta TUI/captura, registrar bloqueo y dejar esta tarea pendiente (dep: T024).
- [ ] T026 [US3] Capturar fuera de la sesión de aceptación en `.memory/validation/038/references.json` la raíz `{calls}` con solo referencias `compression=none`, mediante el mismo punto de entrada del modelo, codec, versión Codex y binario MCP de T025; conservar misma clave de comparación y metadatos variables entregados, y restaurar configuración aislada. Una respuesta MCP directa sin captura visible no satisface esta tarea (dep: T025).
- [ ] T027 [US3] Verificar hechos y referencias predefinidos de cada respuesta observada y ensamblar `.memory/validation/038/evidence.json` conforme al esquema v1: manifiesto y hash, perfil/sesión, llamadas y trazas, pares, llamadas adicionales, revisión responsable y checks de superficies/regresión con `pytest_ci` en estado pendiente; registrar que la puerta CI la aporta T030 y la concilia T032. Revisar por separado `fallback-observation.json`: debe mostrar uso de alternativa, motivo comunicado y ausencia de afirmación de compresión no demostrada; un incumplimiento deja la aceptación `pending` y se documenta en T029, sin introducir el caso negativo en los agregados de ahorro (dep: T026).
- [ ] T028 [US3] Ejecutar `python3 scripts/validate_038_evidence.py --input .memory/validation/038/evidence.json --output .memory/validation/038/report.json` y registrar estado, exit, cantidades antes/después, diferencia y porcentaje sin ocultar overhead, llamadas caras o resultados adversos; no reemplazar evidencia por pack_savings acumulado (dep: T027).
- [X] T029 [US3] Publicar un resumen saneado en `specs/038-fix-codex-compression/evidence/validation.md` con hashes, revisión, método, verificación por consulta, resultado de T028 y resultado conductual negativo independiente; si recursos previos faltan, publicar el motivo `blocked` sin fabricar capturas y mantener T025–T028 sin marcar; no afirmar consumo facturado ni fallo del motor sin evidencia.

**Checkpoint**: US3 solo se acepta con `closed` sustentado por sesión TUI real y tres pares válidos. Un evaluador implementado o un informe `blocked` no satisface FR-007–011; conserva el trabajo realizado sin declarar la feature cerrada.

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: cerrar documentación y controles de la implementación sin confundirlos con aceptación interactiva.

- [X] T030 Añadir a `.github/workflows/test.yml` una puerta obligatoria para 038: Python 3.13 y uv en entorno aislado; exigir `pyproject.toml`/`uv.lock` vigentes, comprobar que el export `uv export --locked --only-dev --format requirements.txt` coincide byte a byte con `scripts/requirements-038-test.txt` y conserva hashes, sincronizar desde el lock, ejecutar `python3 -m pytest -q scripts/test_validate_038_evidence.py` y `pip-audit --require-hashes -r scripts/requirements-038-test.txt`; fallar ante lock/export ausente o desactualizado, instalación, suite o auditoría roja conforme a la constitución. Ejecutar además `go test ./...`, race dirigido, formato/diff, cobertura CI ≥80 % sin bajar baseline y demás controles aplicables; registrar resultado local y CI, o bloqueo con propuesta de cierre, en `specs/038-fix-codex-compression/implementation-notes.md` (dep: T001, T017, T022).
- [X] T031 Examinar llamadores y cambios de las superficies `domain/protocol.go`, `domain/code_retrieval_policy.go`, `adapters/primary/cli/cmd_install.go` y `adapters/primary/cli/cmd_hook.go` mediante el grafo actual, y registrar impacto comprobado en `specs/038-fix-codex-compression/implementation-notes.md`; usar detect_changes de codebase-memory-mcp solo si tiene índice del checkout actual, nunca el proyecto histórico distinto (dep: T030).
- [X] T032 Reconciliar comandos reales, enlaces, requisitos FR-001–011 y criterios SC-001–006 en `specs/038-fix-codex-compression/quickstart.md` y `specs/038-fix-codex-compression/evidence/validation.md`; documentar resultado final de diseño/implementación/aceptación por separado y no declarar cierre completo mientras exista aceptación bloqueada o pendiente (dep: T029, T031).

## Dependencies & Execution Order

### Phase Dependencies

```text
Setup T001 → T002
    ↓
Foundational T003 ∥ T004
    ├─ US1 T005 ∥ T006 → T007 → T008 → T009 → T010
    │       ↓
    │  US2 T011 ∥ T012 ∥ T013 → T014 → T015 → T016 → T017 → T018
    │                                                ↓
    └─ US3 offline T019 → T020 → T021 → T022 → T023 → T024
                                                        ↓
                        T025 → T026 → T027 → T028 → T029
    T001 + US2 T017 + US3 T022 → T030 → T031
    T026 → T027
    T027 + T029 + T030 + T031 → T032
```

La fase US2 empieza tras US1; no exige el evaluador. El tramo offline US3 puede avanzar tras Foundational en paralelo con US1/US2 porque solo toca scripts distintos. Su tramo interactivo T024 depende de T017 y T023, y no sustituye las historias anteriores.

T029 puede ejecutarse para documentar bloqueo si faltan recursos de T024–T028, pero no convierte tareas omitidas en completadas. T032 puede cerrar la documentación con estado pendiente/bloqueado; no representa aceptación exitosa de la feature.

### Within Each User Story

- US1: pruebas rojas de política/superficie antes de directiva y consumo; T009 verifica initialize tras integrar el cambio y antes de registrar MVP.
- US2: pruebas rojas de migración/global/recordatorios antes de cambios; T016 verifica integración tras actualizar el marcador.
- US3: pruebas del evaluador antes de implementación; manifiesto previo antes de TUI; referencias equivalentes fuera de esa sesión; evaluación después de verificar evidencia necesaria.
- No dos tareas que escriben el mismo archivo se ejecutan simultáneamente. Bitácora compartida siempre serializada.

## Parallel Execution Examples

### User Story 1

Después de Foundational: T005 en `protocol_codex_test.go` ∥ T006 en `cmd_mcp_code_tools_policy_test.go`. Esperar ambas antes de T007. T019, de US3, puede avanzar en una rama de trabajo independiente sin tocar esos archivos.

### User Story 2

Después de US1: T011 en `cmd_install_protocol_test.go` ∥ T012 en `atomic_plan_global_test.go` ∥ T013 en `code_retrieval_policy_test.go`. Serializar cambios productivos y la bitácora; no ejecutar dos instaladores sobre el mismo HOME de prueba.

### User Story 3

Después de Foundational: el tramo T019–T022 puede avanzar en paralelo con US1/US2, con un único escritor de los scripts. La captura TUI, referencias y evaluación son secuenciales y no se delegan como sustituto de una sesión real. T030–T031 pueden ejecutarse después de T017/T022 mientras se gestiona acceso a TUI, sin declarar ahorro.

La marca `[P]` no autoriza subagentes por sí misma: antes de delegar se aplica la política Octopus vigente.

## Implementation Strategy

### MVP First

Setup + Foundational + US1 (T001–T010). Validar protocolo en initialize y suficiencia de la ruta sin afirmar que el modelo ya la sigue. El MVP no es despliegue global terminado: US2 actualiza bloques v8 y recordatorios.

### Incremental Delivery

1. US1: directiva de prioridad y límites verdaderos, conservando alternativas.
2. US2: misma directiva durante sesión y migración global preservadora; regresiones compatibles.
3. US3: evaluador probado y aceptación con evidencia real; recursos ausentes producen bloqueo explícito.
4. Controles transversales y documentación final sin ocultar pendientes.

### Requirement Coverage

| Requisito/criterio | Tareas principales |
|-------------------|--------------------|
| FR-001, FR-002 | T003, T005, T007–T010 |
| FR-003 | T009, T013, T014, T017 |
| FR-004 | T003, T005, T013, T017 |
| FR-005 | T005–T008, T018, T023–T025, T027, T029 |
| FR-006 | T004, T011, T012, T015–T017 |
| FR-007 | T023–T025 |
| FR-008 | T019–T022, T026–T028 |
| FR-009 | T006, T019, T023, T027 |
| FR-010, FR-011 | T019–T022, T028, T029, T032 |
| SC-001, SC-006 | T009, T011–T017 |
| SC-005 | T003, T005, T019–T022, T023–T025, T027, T029 |
| SC-002, SC-003, SC-004 | T023–T029 |

## Notes

- 32 tareas: Setup 2, Foundational 2, US1 6, US2 8, US3 11, controles transversales 3.
- Siete tareas con `[P]`: T003–T006 y T011–T013, en oleadas independientes con prerrequisitos satisfechos. Paralelismo adicional por tramo entre historias se detalla sin etiquetar falsamente pasos dependientes.
- T001 fija la dependencia de pruebas en un cambio separado; T030 incorpora y verifica la puerta obligatoria de pytest en CI. Ninguna ejecución local sustituye ese resultado.
- Los perfiles y mediciones son modelos lógicos/fixtures: no requieren migraciones de base de datos.
- Obtener cuerpo literal o un lenguaje fuera de Go es alternativa legítima, no fallo de compresión por sí mismo. No cambiar contratos de get_symbol para satisfacer pruebas inventadas de cuerpos.
- No ejecutar install/update sobre entorno personal, no descartar cambios ajenos y no guardar autenticación o trazas privadas en Git.
- Este archivo solo define trabajo: ninguna tarea fue ejecutada al generarlo.
