# Notas de implementación — 038

Fecha: 2026-10-12. Todas las tareas pendientes al inicio.

## T001 — Procedencia y dependencias de prueba

- Revisión: `79c5351238782de005f7b8ed67b1322a177e933c`
- Go: `go1.27.2 darwin/arm64` (coincide con `go.mod`).
- Python del sistema: `3.9.6`; Python de pruebas: `3.13` (`/opt/homebrew/bin/python3.13`).
- uv: `0.11.23`.
- Codex: `codex-cli 0.162.1` (antecedente aportado, memoria 450).
- Cambios ajenos al inicio: solo `specs/038-fix-codex-compression/` sin versionar.
- Alcance: textos de instrucciones + evaluador offline; sin motor, esquemas MCP ni hooks nuevos.
- Dependencias (cambio separado): `pyproject.toml` mínimo sin paquete instalable, grupo `dev` con `pytest==9.1.1` y `pip-audit==2.10.1`; `uv lock` genera `uv.lock` (33 paquetes, CPython 3.13.16); `uv export --locked --only-dev` genera `scripts/requirements-038-test.txt` con hashes, reproducible byte a byte (verificado por doble export + diff).
- Fuentes: Context7 `/pytest-dev/pytest` (reputación alta); PyPI `pytest 9.1.1` (≥3.10) y `pip-audit 2.10.1` (≥3.10) según research §6; `uv sync --locked --only-dev` + `pytest --version`/`pip_audit --version` verificados en entorno aislado.
- Auditoría: `pip-audit --require-hashes -r scripts/requirements-038-test.txt` → sin vulnerabilidades conocidas.
- Reversión: borrar `pyproject.toml`, `uv.lock`, `scripts/requirements-038-test.txt` y `.venv/`; ningún código de producción depende de ellos.

## T002 — Aislamiento e inventario temprano (solo lectura)

- Aislamiento para pruebas de instalación: `HOME=$TMP/038-home`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `GOMEMORY_DATA_HOME` temporales; `PATH` mínimo (`/usr/bin:/bin:/usr/sbin:/sbin`) más rutas explícitas al binario bajo prueba. Nunca `mem install/update` sobre el HOME real. Base: `scripts/audit_037_smoke.py`, endurecido según lección 438 (aislar también `PATH`).
- Codex objetivo: `codex-cli 0.162.1`.
- Candidato de interfaz por llamada: `codex debug prompt-input` (declara lista de entrada visible al modelo como JSON; por ayuda local no identifica exportación de resultados de herramientas por llamada en TUI).
- Candidato de oráculo independiente: pendiente de identificar (trazas locales de sesión existen bajo `~/.codex/sessions`, pero su correspondencia por llamada con la entrada del modelo no está comprobada).
- Riesgo registrado: `measurement_unavailable` para SC-003 hasta T024. Esta inspección no certifica la captura.

## T005/T006 — Pruebas US1 (TDD)

- T005 `TestIntegrationBlock_PrioridadCondicionalPorCapacidad`: rojo antes de T007/T008 (5 fallos: 4 fragmentos ausentes + exigencia externa incondicional presente); verde tras T008.
- T006 `TestCodeToolsPolicy_SuperficieEstructuralSinCuerpos`: verde desde su creación; fija la superficie existente (`search_code` con ubicaciones sin marcas sin compresor; `get_symbol` sin cuerpo literal). No requiere cambio de producción (plan: referencia).
- Cambio legítimo de comportamiento en la misma entrega (constitución §10): `TestIntegrationBlock_OrientaALaCompresionEnOrigen` (T076/035) actualizado de `comprimen en origen` a `recuperación comprimida en origen`.
- Regresión legada preservada: `TestComposeAgentFile_ReemplazaV6SinDejarRestos` sigue en verde porque el bloque mantiene una línea condicional de disponibilidad del proveedor externo (nombre + descubrimiento), sin exigencia incondicional.

## T010 — Conjunto dirigido US1

- `go test ./adapters/primary/cli -run 'Test.*(IntegrationBlock|ComposeAgentFile|RetrievalPolicy|RewriteToolOutput|CodeToolsPolicy)'` → ok.
- Límite: demuestra contrato y suficiencia de respuestas, no elección autónoma del agente ni ahorro (eso es US3).

## T011–T017 — US2: instrucciones consistentes y migración

- T011 `TestComposeAgentFile_MigraV8AV9SinDejarRestos`: rojo antes de T015 (sin cambios al componer v8); verde tras el bump a v9, con delimitador, baseline, bloques ajenos y doble composición idempotente.
- T012 `TestInstallAtomicPlanGlobal_CodexPreservaAjenoYEsIdempotente`: verde; HOME aislado (`homeFalso`), destino `.codex/AGENTS.md` de prueba, contenido ajeno intacto y segunda pasada sin cambios. No escribe el HOME real.
- T013 `TestRecordatorio_SigueLaMismaPoliticaCanonica`: rojo antes de T014 (2 fallos); verde tras consumir la directiva en `memoryProtocolReminder`.
- T014/T015: recordatorio comparte `domain.CodeRetrievalPolicy`; marcador v8→v9 en `domain/protocol.go` (fuente única). Test vigente `TestProtocolVersionMarker_SubioAV8` actualizado a v9 como cambio legítimo en la misma entrega.
- T016 `TestInstall038_BloqueV9SinAgentsYRetiradaCodex`: verde con binario real (marcador v9, sin AGENTS por proyecto, `CodexToolOutputHook` fuera del ciclo).
- T017: `go test ./adapters/primary/cli/ ./adapters/primary/setup/` → ok (cli 16s, setup 0.3s). Rewrite Codex sigue deshabilitado.

## T019–T022 — Evaluador offline (stdlib, pytest)

- T019: 21 pruebas en `scripts/test_validate_038_evidence.py`; rojas antes de T020 (módulo ausente), 21/21 en verde tras T020/T021 (`uv run --locked --only-dev python -m pytest -q`).
- T020/T021: `scripts/validate_038_evidence.py` con parse estricto (UTF-8, sin claves duplicadas, sin NaN/Infinity), validación de forma v1, integridad de IDs/claves, digests de manifiesto y trazas, codec `codex-tui-visible-text-utf8-v1` sin normalizar, certificación TUI con adaptador ≠ oráculo e igualdad byte a byte, y agregados con deltas firmados. Sin red, sin ejecución de texto, sin dependencias.
- T022: CLI `--input/--output` con exits 0/1/2; ejemplo sintético del contrato → `blocked` exit 2 como exige el contrato.

## T030 — Controles transversales

- `go test ./... -count=1` → 16 paquetes ok, 0 fallos.
- `go test -race ./adapters/primary/cli ./adapters/primary/setup` → ok (exit 0).
- `go vet ./...` limpio; `gofmt` limpio; `git diff --check` limpio.
- pytest 21/21 + `pip-audit --require-hashes` sin vulnerabilidades (grupo `dev` aislado).
- Puerta CI añadida en `.github/workflows/test.yml` (job `evidence038`): exige lock/export vigentes, suite y auditoría en verde. Cobertura ≥80 % verificada por el job existente de cobertura en CI, sin bajar baseline.

## T031 — Impacto de superficies (grafo actual)

- `buildIntegrationBlock` (14 llamadores en `cmd_mcp_setup.go`/`cmd_mcp.go`): contenido v9 con directiva; cubierto por tests de protocolo, migración e initialize.
- `memoryProtocolReminder` (2 llamadores en `cmd_hook.go` + entregas MCP en `cmd_mcp.go`): mismo fragmento canónico; cubierto por T013/T017.
- `composeAgentFile`: sin cambios de lógica (7 llamadores intactos).
- Sin cambios en motor, umbrales, esquemas MCP, permisos, tabla de hooks ni `CodexToolOutputHook` (sigue solo para retirada).

## T032 — Reconciliación final

- `quickstart.md`: comandos, rutas (`scripts/validate_038_evidence.py`, `requirements-038-test.txt`, `evidence/`, `implementation-notes.md`) y tests verificados contra lo implementado; documenta diseño/implementación/aceptación por separado.
- `evidence/validation.md`: estado `blocked` con motivo, hashes y pendientes; no declara cierre.
- Resultado final: diseño e implementación completos salvo aceptación TUI (T025–T028 pendientes por `measurement_unavailable`). Cobertura FR-001–011/SC-001–006 según tabla de `tasks.md`; SC-002–005 sin demostrar en sesión real.

- `domain/code_retrieval_policy.go` (nuevo): constante pura sin I/O; consumidores `buildIntegrationBlock` y `memoryProtocolReminder` únicamente.

## Correcciones ACR (solo informe previo, 9 SUSPECT, 0 CONFIRMED)

- C-005: `_resolve_in_bank` confina trazas/extractores/fixtures/revisión a `.memory/validation/038`; `_resolve_inside` queda solo para manifiesto y certificado.
- C-006: llamadas sin comparación bloquean (`calls sin comparación`); +test `test_llamada_sin_comparar_bloquea`.
- C-007: IDs únicos en hechos/referencias; +test `test_hechos_duplicados_bloquean`.
- C-009: IDs/marcadores/call_id únicos entre probes; +test de marcador repetido.
- C-005/C-009/C-007: +test `test_ruta_con_escape_bloquea`.
- C-001: fixture v8 con orden fiel (marcador antes que título) + conteo de cabecera == 1 en el test de migración.
- C-002: stub T012 con rama endMarker y regla H2 como el real; idempotencia incondicional.
- C-003: test renombrado a `SubioAV9` con nota anti-drift.
- C-004: T005 ejecuta los 8 casos del fixture (marcas exigidas/vetadas por caso + prohibición de ahorro por nombre).
- Verificación: pytest 25/25, cli/setup/domain ok, integración 038 ok, diff limpio. Ledger ACR sin fix_record (sin CONFIRMED no admite correcciones; no se fuerza).
