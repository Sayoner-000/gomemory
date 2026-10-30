package ports

import "mem/domain"

// ContextPackRenderer mide la representación realmente entregada por un borde.
// Render debe ser determinista, sin I/O y sin modificar el paquete.
type ContextPackRenderer interface {
	Render(domain.ContextPack) string
}
