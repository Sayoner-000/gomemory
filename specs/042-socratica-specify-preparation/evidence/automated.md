# Puerta automática de 042

**Fecha**: 2026-10-24. **Árbol**: `main` sobre `67d4e9b` con los cambios de 042 sin confirmar. **Go**: go1.27.3 darwin/arm64.

Este documento registra **solo la puerta automática**. No acredita las demostraciones reales con Claude Code, Codex y OpenCode (T063–T065), que siguen pendientes; véase [`acceptance.md`](acceptance.md) cuando exista.

## Resultado

| Comprobación | Orden | Resultado |
| --- | --- | --- |
| Formato | `gofmt -l .` | sin archivos |
| Compilación | `go build ./...` | correcto |
| Análisis estático | `go vet ./...` | correcto |
| Pruebas y cobertura combinada | `bash scripts/coverage.sh` (`go test ./... -count=1 -coverpkg=./...` más `tests/contract` con cobertura de binario) | salida 0; todos los paquetes con pruebas `ok` |
| Carrera | `go test -race ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli ./adapters/primary/tui ./adapters/primary/setup -count=1` | los seis `ok` |

**Cobertura global combinada: 82,3 %** (mínimo constitucional: 80 %).

## Línea base frente a resultado

Línea base de T001 (`evidence/baseline.md`) frente a la medición final, cobertura unitaria de cada paquete modificado:

| Paquete | Antes | Después |
| --- | --- | --- |
| `mem/domain` | 91,4 % | 92,4 % |
| `mem/application/usecases` | 75,7 % | 76,7 % |
| `mem/adapters/secondary/persistence` | 73,2 % | 74,5 % |

Ningún paquete baja de su línea base. Cobertura media por función del código nuevo (perfil combinado):

| Archivo | Cobertura | Funciones bajo 80 % |
| --- | --- | --- |
| `domain/socratica_specify.go` | 98,9 % | 0 |
| `domain/socratica_specify_render.go` | 100,0 % | 0 |
| `domain/socratica_specify_validation.go` | 99,7 % | 0 |
| `application/usecases/socratica_specify.go` | 97,9 % | 0 |
| `adapters/secondary/persistence/socratica_ephemeral.go` | 97,2 % | 0 |

Durante la tarea `domain` llegó a bajar a 90,1 %: faltaban pruebas directas de `report` y `force_resend` en el propio paquete (solo las cubría el caso de uso). Se añadieron en `domain/socratica_specify_outcome_test.go` y no se rebajó ninguna puerta.

## Casos nuevos exigidos por el quickstart

Cada familia exigida tiene casos propios; la lista enlaza el archivo que los contiene.

| Familia | Pruebas |
| --- | --- |
| Validación por etapas, referencias, bloqueos y marcadores | `domain/socratica_specify_validation_test.go`, `domain/socratica_specify_stages_test.go` |
| Límite acumulativo de preguntas (no se reinicia al editar ni retomar) | `domain/socratica_specify_questions_test.go`, `adapters/primary/cli/cmd_mcp_socratica_specify_draft_test.go`, `tests/integration/socratica_specify_dispatch_test.go` (caso B) |
| Revisión obsoleta y edición concurrente | `domain/socratica_specify_review_test.go`, `application/usecases/socratica_specify_review_test.go`, `adapters/primary/cli/cmd_mcp_socratica_specify_review_test.go` |
| Identidad exacta del render y huella de bytes | `domain/socratica_specify_render_test.go`, `domain/socratica_specify_review_test.go` |
| Replay con concesión única, rollback y carrera | `adapters/secondary/persistence/socratica_specify_ack_test.go`, `application/usecases/socratica_specify_dispatch_test.go`, `adapters/primary/cli/cmd_mcp_socratica_specify_dispatch_test.go` |
| Resultado comunicado (`report`) | `domain/socratica_specify_outcome_test.go`, `application/usecases/socratica_specify_report_test.go` |
| Migración desde catálogo v2, base vacía y reversión | `adapters/secondary/persistence/socratica_specify_migration_test.go`, `adapters/secondary/persistence/testdata/schema.golden` |
| Payload privado, reserva sin texto y política fija | `adapters/secondary/persistence/socratica_ephemeral_test.go`, `adapters/secondary/persistence/socratica_specify_retention_test.go` |
| Reserva entre procesos y pérdida de instancia | `tests/integration/socratica_specify_private_test.go` |
| Sanitización y retención | `adapters/secondary/persistence/socratica_specify_sanitize_test.go`, `adapters/secondary/persistence/socratica_specify_retention_test.go` |
| Consulta local sin ejecución (CLI y TUI) | `adapters/primary/cli/cmd_socratica_specify_test.go`, `adapters/primary/tui/tui_socratica_specify_test.go` |
| Alias, paridad de skills, instalación y retirada | `adapters/primary/setup/socratica_install_test.go`, `adapters/primary/setup/socratica_specify_skill_test.go`, `adapters/primary/setup/agent_skills_setup_test.go`, `tests/contract/lifecycle_uninstall_test.go`, `tests/contract/lifecycle_update_test.go` |
| Reconocimiento de alias, forma Codex y petición natural | `domain/socratica_interception_test.go`, `adapters/primary/cli/hook_socratica_test.go` |
| Modos previos intactos | `adapters/primary/cli/cmd_mcp_socratica_tool_test.go`, `application/usecases/socratica_flow_test.go`, pruebas de contrato de salida de `refine/scrutiny` en los archivos de `cmd_mcp_socratica_specify_*` |

## Verificaciones que mordían

Para no aceptar pruebas que pasan en vacío se hicieron mutaciones y se restauró el código:

- Retirar `rewrite` de las rutas cerradas del modo: el contrato MCP lo detectó.
- Dejar de borrar el texto de la reserva en el almacén efímero: seis comprobaciones fallaron.
- Desactivar la comprobación de criterio ajeno a la entrega y la de bloqueo silencioso en los validadores: fallaron los casos correspondientes.
- Control del escáner de residuos de la prueba entre procesos: encuentra el texto de un flujo persistente, de modo que «cero hallazgos» en el privado significa algo.

## Límites

- Las pruebas de integración usan un agente Spec Kit sintético (proceso auxiliar contado). Demuestran quién puede ejecutar, cuántas veces y con qué texto exacto; no la calidad de una generación real.
- `TestRealSocraticaHarness` solo ejercita el generador de `refine`; no se usa como evidencia de este traspaso.
- No se ejecutó ningún cliente real. T063–T065 permanecen pendientes.
