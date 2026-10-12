# Guía de validación — 038

Esta guía se ejecuta después de implementar. Las rutas de evaluador y pruebas nuevas son entregables propuestos, no archivos disponibles todavía. No usar estos pasos como evidencia de que la feature ya funciona.

## Requisitos

Go del `go.mod`, Python 3.13, uv, `pyproject.toml` y `uv.lock` de pruebas creados por T001, export con hashes `scripts/requirements-038-test.txt`, Codex TUI con autenticación legítima, MCP gomemory apuntando al binario evaluado y grafo Go disponible. El evaluador usa solo biblioteca estándar; pytest y pip-audit pertenecen al grupo `dev`. Registrar revisión y versiones. Si faltan lock, export, auditoría o puerta CI, mantener pendiente la validación. Mantener aprobaciones y sandbox normales.

```bash
git rev-parse HEAD
go version
codex --version
uv --version
uv run --locked --only-dev python -m pytest --version
```

T001 crea un proyecto Python mínimo de pruebas sin paquete instalable, ejecuta `uv lock`, versiona `uv.lock` y genera el export con hashes mediante `uv export --locked --only-dev --format requirements.txt --output-file scripts/requirements-038-test.txt`. Antes de T019, comprobar que repetir el export no cambia el archivo, sincronizar el grupo `dev` con `uv sync --locked --only-dev` y ejecutar `uv run --locked --only-dev python -m pip_audit --require-hashes -r scripts/requirements-038-test.txt`. T030 exige lo mismo en `.github/workflows/test.yml`; el job debe fallar si falta/desactualiza lock o export, si la auditoría falla o si las pruebas fallan. Comprobar `codex --help` y capacidades locales antes de usar captura específica. La documentación actual no garantiza flags/eventos de una versión histórica.

## 1. Pruebas dirigidas

```bash
go test ./adapters/primary/cli -run 'Test.*(IntegrationBlock|ComposeAgentFile|RetrievalPolicy|RewriteToolOutput)'
go test ./adapters/primary/setup -run 'Test.*(AtomicPlan|Global|Codex)'
go test ./tests/contract ./tests/integration
uv run --locked --only-dev python -m pytest -q scripts/test_validate_038_evidence.py
```

Resultados esperados: prioridad común, migración v8→v9, bloques externos intactos, repetición idempotente, protocolo en initialize y recordatorios coherentes, hook de rewrite Codex no reinstalado. Tests verificados en T010/T017: `TestIntegrationBlock_PrioridadCondicionalPorCapacidad`, `TestMCPInitialize_EntregaPoliticaCanonica`, `TestComposeAgentFile_MigraV8AV9SinDejarRestos`, `TestRecordatorio_SigueLaMismaPoliticaCanonica`, `TestInstallAtomicPlanGlobal_CodexPreservaAjenoYEsIdempotente`, `TestInstall038_BloqueV9SinAgentsYRetiradaCodex`. Aislar HOME/XDG/datos y PATH dentro del banco de instalación, sin escribir configuración personal. No ejecutar mem install/update sobre el HOME real como smoke test. Los conflictos con instrucciones externas ajenas se advierten y no se corrigen sobrescribiéndolas.

## 2. Verificar artefacto servido

Construir el binario de implementación según el procedimiento vigente del repositorio y colocarlo en el entorno aislado del banco. Confirmar que los comandos de hook y el servidor MCP apuntan a ese mismo binario; registrar ruta, hash y revisión. Verificar inicialización y contenido del bloque v9 global de prueba, sin generar AGENTS por proyecto.

No basta con inspeccionar código fuente: la TUI debe recibir las instrucciones del binario evaluado. La preparación de autenticación/configuración aislada se realiza sin copiar secretos a artefactos versionados; si no hay entorno válido, registrar bloqueo.

## 3. Fijar manifiesto antes de aceptación

Preparar tres consultas naturales que pidan listas de funciones Go relacionadas con hooks, paquetes de contexto y entrega de contexto. Ejemplos iniciales:

- «Enumera las funciones relacionadas con el ciclo de hooks y sus ubicaciones».
- «Enumera los símbolos relacionados con paquetes de contexto y sus ubicaciones».
- «Enumera las funciones de entrega de contexto y sus ubicaciones».

El preflight fuera de la sesión debe confirmar suficiencia, cobertura y posibilidad de reducción de cada consulta. Fijar prompts finales, `comparison_key`, hechos y referencias necesarios, y SHA-256 del manifiesto. Si un ejemplo no es elegible, ajustar el banco ANTES de iniciar aceptación y registrar el motivo. No pedir cuerpos si la vía disponible solo entrega estructura, ni elegir nuevos prompts después de un resultado adverso. La entrada consolidada seguirá [el esquema v1](contracts/validation-evidence.schema.json) y [el contrato de captura](contracts/validation-evidence.md).

Fijar por separado `evidence/fallback-manifest.json` con una solicitud natural del cuerpo literal de una función Go conocida, el símbolo/archivo esperado y los criterios de alternativa, explicación de insuficiencia y ausencia de promesa de compresión. Registrar su hash antes de iniciar la TUI. Este caso no forma parte de las tres comparaciones de ahorro.

## 4. Sesión TUI real

Antes de la sesión de aceptación, completar la [certificación del adaptador TUI](contracts/validation-evidence.md#certificación-del-adaptador-tui-antes-de-medir) de T024 con dos resultados MCP sintéticos, una fuente independiente de entrada del modelo por llamada, igualdad byte a byte y prueba de referencia `compression=none`. Guardar `.memory/validation/038/capture-certification.json` conforme al [esquema](contracts/capture-certification.schema.json) y registrar interfaz, evento/campo, extractores/versiones, revisión responsable, comandos y hashes. El evaluador comprueba ese archivo aunque `capture_verified.passed` sea `true`. En Codex 0.162.1, `codex debug prompt-input` solo declara renderizar la lista de entrada inicial; `codex exec --json` corresponde al modo no interactivo. Ninguno certifica por sí solo los resultados de herramientas de la TUI. Si falta la interfaz verificable, registrar `measurement_unavailable`, publicar bloqueo y dejar T025–T028 pendientes.

En el proyecto de prueba y configuración aislada, iniciar el comando interactivo normal:

```bash
codex
```

Enviar las tres solicitudes del manifiesto sin nombrar herramientas ni corregir su elección. Capturar identificador de sesión, llamadas, argumentos y `visible_text` completo mediante el adaptador certificado, incluido cualquier `pack_retrieve` necesario. Después enviar el prompt negativo prefijado, sin indicar la ruta: conservar llamada alternativa, respuesta final y explicación de la limitación en `fallback-observation.json`. Conservar trazas locales y sus SHA-256. Confirmar presencia de protocolo y recordatorios v9. Sesión `codex exec`, pruebas simuladas o trazas incompletas no satisfacen esta etapa.

Usar captura disponible y comprobada para la versión instalada. Guardar un archivo de payload UTF-8 exacto por entrega, sin envoltura ni salto añadido, y su SHA-256; el evaluador coteja esos bytes con `visible_text`. Registrar en `surface_checks.capture_verified` el preflight que demuestra el punto de captura. Un log de diagnóstico TUI o un `CallToolResult` crudo no demuestran el texto visto por el modelo; si falta el punto exacto o hay bloques no textuales sin representación fiel, declarar bloqueo. Tratar los payloads como sensibles y registrar solo evidencia saneada.

## 5. Capturar referencias y evaluar

Fuera de la sesión de aceptación, repetir las mismas recuperaciones con compresión `none` mediante una ejecución controlada que pase por `codex_tui_model_input`, la misma versión de Codex, binario MCP y codec `codex-tui-visible-text-utf8-v1`. Conservar fuente, revisión, filtros, límites y todo byte visible. Una respuesta MCP directa que no pase por ese punto no sirve como referencia. No comparar listas propias con snippets externos de cuerpos. Restaurar configuración aislada tras capturar referencias.

Guardar manifiesto, observaciones, referencias y revisión humana conforme al [contrato de evidencia](contracts/validation-evidence.md), y ejecutar el evaluador una vez implementado:

```bash
python3 scripts/validate_038_evidence.py --input /ruta/local/evidence.json --output /ruta/local/report.json
```

Resultado requerido: exit 0 y `closed`, tres consultas sin coaching, hechos preservados, respuestas completas comparables y ahorro agregado positivo; además, revisión conductual negativa satisfactoria y documentada por separado. Exit 1 significa pendiente; exit 2, bloqueado/entrada inválida. No modificar consultas, omitir overhead ni invocar métricas históricas para convertir un incumplimiento en cierre.

## 6. Regresión y cierre

```bash
go test ./...
go test -race ./adapters/primary/cli ./adapters/primary/setup
git diff --check
```

Exigir además el resultado del job CI con la nueva puerta pytest y la cobertura global ≥80 %, sin bajar la línea base. Inspeccionar llamadores/impacto con el grafo actual; usar codebase-memory-mcp si dispone de índice correspondiente a este checkout, no un proyecto histórico distinto. Documentar revisión y resultado real.

Versionar únicamente informe saneado cuando exista. Antes de cerrar, confirmar estados y puertas de [data-model.md](data-model.md); si faltan recursos o ahorro, dejar el cierre explícitamente pendiente.
