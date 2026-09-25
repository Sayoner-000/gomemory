package ports

import "context"

// DeliveredBlocksRepository registra, por sesión, qué bloques de contenido (una
// entrada de memoria, un resultado de búsqueda) ya recibió el agente, para no
// reenviarlos iguales en la misma sesión (feature 033, FR-018).
//
// Todas las operaciones actúan sobre la sesión ACTIVA, resuelta en cada
// llamada. Sin sesión activa son inertes: Seen devuelve false y Mark no
// escribe, así que el contenido se entrega completo.
type DeliveredBlocksRepository interface {
	// Seen devuelve, de las huellas dadas, las que ya se entregaron.
	Seen(ctx context.Context, hashes []string) (map[string]bool, error)
	Mark(ctx context.Context, hashes []string) error
	// Reset olvida lo entregado en la sesión activa: tras una compactación o
	// un arranque, el agente ya no tiene el contenido anterior (FR-019).
	Reset(ctx context.Context) error
}
