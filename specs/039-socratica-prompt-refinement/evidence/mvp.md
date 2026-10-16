# Evidencia MVP (T060): US1 + US2 + US3

- Fecha: 2026-10-16 · macOS (Darwin 27.0.0) · Go del repositorio · Node v26.11.1 (plugin)
- Árbol de trabajo sin commit sobre `ccd0e7e`.

## Regresión automatizada

| Orden | Resultado |
|---|---|
| `go vet ./...` | sin salida, código 0 |
| `go test ./... -count=1` (incluye el subconjunto MVP: `./domain/ ./application/... ./adapters/secondary/persistence/ ./adapters/primary/cli/ ./adapters/primary/setup/ ./tests/...`) | código 0; 20 paquetes `ok`, ningún `FAIL` |
| `go test -race` dirigido a dominio, caso de uso, persistencia, generador, lanzador, CLI y TUI de Socratica | 7 paquetes `ok` |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | 46 pruebas, 46 pass |
| `bash scripts/coverage.sh` | ver `README.md` (T070) |

La primera ejecución completa encontró tres defectos que las pruebas dirigidas no veían, ya cerrados:

1. `socratica_flow` no figuraba en `domain.MCPAllTools()`: quedaba fuera del bootstrap de ToolSearch y un agente con herramientas diferidas no podía invocarla (`TestDomainReflejaLasToolsRegistradas`).
2. Las superficies esperadas de Octopus derivan de esa lista (`TestOctopusApagado_*`, `TestOctopusEncendido_*`).
3. El cambio de `chat.message` alteraba la fábrica v1 del plugin (`TestOpenCodeV2_FabricaV1IdenticaALaReferencia`). Era deliberado (T059); se actualizó la referencia de la feature 032 solo en `chat.message`, con nota de trazabilidad. Con la intercepción apagada el mensaje llega igual.

Además, al revisar la rama 2.x del plugin se cerró un defecto propio: la decisión se aplicaba a un `output` sintético y no llegaba al prompt real; ahora se devuelve a `ev.prompt.text` (prueba `v2 socratica: the decision reaches the real prompt text`).

## Contra el binario real (no simulado)

- `tests/integration/socratica_refine_test.go` y `socratica_scrutiny_test.go`: recorridos refine y scrutiny por `mem mcp --root` (stdio), con HOME aislado; los directorios de habilidades no cambian (FR-025).
- `tests/integration/socratica_hook_test.go`: `mem hook user-prompt-submit` y `mem hook socratica-decide` con la intercepción apagada y con `--unverified`.
- `tests/contract/lifecycle_uninstall_test.go`: `setup-mcp --scope global` deja la habilidad y los tres envoltorios; `uninstall --all` los retira.
- `tests/contract/lifecycle_update_test.go` (`TestUpdate_ReinstalaSocratica`): `mem update` con el binario real publicado como release reinstala habilidad y envoltorio.
- Sondeo manual del binario: `mem socratica status` (0), `on --client claude` sin evidencia (1, «no soportada»), `refine -` sin generador en PATH (2, original repetido).

## Aceptación por agente (quickstart §6, pasos 1–3 y 5)

**No ejecutada.** Requiere sesiones interactivas reales en Claude Code 2.1.300, Codex 0.162.1 y OpenCode 2.0.27 con la persona (y los 5 prompts de SC-005 de `README.md`). Hasta registrarla, T060 queda abierta y todas las capacidades siguen en `false`.
