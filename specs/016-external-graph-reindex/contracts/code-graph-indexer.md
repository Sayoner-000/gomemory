# Contrato: `ports.CodeGraphIndexer`

Único contrato nuevo que esta funcionalidad expone entre capas. No es una API HTTP ni un
protocolo de red — es el puerto de aplicación que CLI y TUI (adaptadores primarios) usan para
invocar el reindexado del proveedor de grafo externo, sin conocer el adaptador concreto.

## Interfaz

```go
package ports

var ErrIndexerNotInstalled = errors.New("code graph indexer: proveedor no instalado")

type CodeGraphIndexer interface {
    Name() string
    IndexRepository(ctx context.Context, mode string) (nodes, edges int, err error)
}
```

## Precondiciones

- El llamador ya tiene una referencia a un valor cuyo tipo dinámico puede o no implementar esta
  interfaz (aserción de tipo `.(ports.CodeGraphIndexer)`, `ok=false` sin panic si no la implementa).
- `mode` se pasa como `"full"` en todos los llamadores actuales (CLI y TUI); el contrato no valida
  el valor de `mode`, solo lo reenvía al proceso externo.

## Postcondiciones

| Escenario | `nodes`/`edges` | `err` |
|---|---|---|
| Proveedor no instalado | `0, 0` | `ports.ErrIndexerNotInstalled` (verificable con `errors.Is`) |
| Indexado exitoso | conteos reales del grafo resultante | `nil` |
| Fallo del proceso externo o respuesta inesperada | `0, 0` | error envuelto con contexto (`fmt.Errorf("index_repository: %w", ...)`), NUNCA el sentinel anterior |

## Garantías de comportamiento

- **Bloqueante**: a diferencia de `ports.CodeGraphProvider` (contrato estricto de no-bloqueo),
  `IndexRepository` SÍ bloquea hasta 10 minutos (`indexTimeout`). Los llamadores son responsables
  de no invocarlo desde rutas que deban responder rápido (hooks por turno, `MaybeRefresh`).
- **Idempotente en efecto, no en costo**: invocarlo dos veces seguidas no corrompe estado, pero
  cada invocación repite el trabajo completo (`mode="full"` siempre) — por eso la TUI aplica una
  guardia para no disparar dos reindexados concurrentes sobre el mismo proyecto.
- **Registro implícito del proyecto**: la primera invocación sobre un proyecto que el proveedor
  externo no conocía aún lo registra como efecto del propio indexado (ver `research.md` §2).

## Consumidores de este contrato

| Consumidor | Cuándo lo invoca | Qué hace con el resultado |
|---|---|---|
| `adapters/primary/cli/cmd_index.go` | Tras el indexado nativo Go, salvo `--skip-graph` | Imprime nodos/aristas o mensaje informativo/advertencia; nunca hace fallar el comando (exit code sigue 0) |
| `adapters/primary/tui/tui.go` | Acción "Reindexar grafo externo" en la pantalla de configuración | Actualiza `statusMsg` de la TUI vía `externalReindexDoneMsg`, sin bloquear el bucle de eventos |

## Implementación de referencia

`adapters/secondary/codegraph/codebasememory.Provider` — invoca
`cli index_repository {"repo_path":root,"mode":mode}` sobre el binario externo resuelto, con el
timeout dedicado `indexTimeout`, y parsea la respuesta con `parseIndexRepositoryResponse`
(fixture de referencia: `testdata/index_repository.json`).
