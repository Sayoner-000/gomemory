package ports

import (
	"context"
	"errors"
)

// ErrIndexerNotInstalled señala que el proveedor no tiene binario resuelto
// (no está en PATH / sin override configurado). Los llamadores (`mem index`,
// la TUI) lo usan para distinguir "no instalado" (línea informativa) de un
// error real de indexado (warning).
var ErrIndexerNotInstalled = errors.New("code graph indexer: proveedor no instalado")

// CodeGraphIndexer es una capacidad OPCIONAL y separada de CodeGraphProvider:
// dispara un reindexado BLOQUEANTE del proveedor externo. A diferencia de
// CodeGraphProvider (contrato estricto de no-bloqueo, ver su docstring),
// IndexRepository puede tardar minutos — por eso vive en su propia interfaz
// y SOLO deben invocarla comandos/acciones explícitas fuera del hot path
// (hoy: `mem index` y la acción "Reindexar grafo externo" de la TUI). Nunca
// debe llamarse desde hooks, MaybeRefresh, Refresh, ni ningún camino
// automático por turno.
type CodeGraphIndexer interface {
	Name() string
	// IndexRepository dispara un reindexado del proyecto en el proveedor
	// externo. mode se pasa tal cual (hoy siempre "full", sin selector).
	// Devuelve nodos/aristas del grafo resultante. Sin binario resuelto →
	// ErrIndexerNotInstalled.
	IndexRepository(ctx context.Context, mode string) (nodes, edges int, err error)
}
