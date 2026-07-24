package ports

import "context"

// ADRSyncProvider habla con el documento único de ADR de un proveedor
// externo (ver domain.ADRDocument/ParseADRDocument). GetDocument/
// UpdateDocument operan sobre el `content` COMPLETO del documento — el
// parseo/merge por bloque/sección es responsabilidad de quien llama
// (dominio puro), no del adaptador, para no acoplarse al esquema interno
// del proveedor más allá de "un string de contenido".
//
// Igual que CodeGraphProvider: opcional, provider-agnóstico, y todo fallo
// (proveedor no disponible, error de red/CLI) degrada en silencio en el
// llamador — nunca hace fallar el guardado de una memoria.
type ADRSyncProvider interface {
	Name() string
	GetDocument(ctx context.Context) (string, error)
	UpdateDocument(ctx context.Context, content string) error
}
