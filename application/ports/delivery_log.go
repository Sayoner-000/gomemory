package ports

// DeliveryLog registra qué material ya recibió el agente en la sesión en curso.
//
// Es un puerto y no una llamada directa a persistencia porque la decisión de
// suprimir material vive en la capa de aplicación, y esa capa no importa
// infraestructura (constitución, principio I). También hace comprobable la
// supresión con un doble de prueba, sin base de datos.
type DeliveryLog interface {
	// Last devuelve el identificador del contenido que un canal entregó en la
	// sesión activa. ok=false significa que ese canal no entregó nada, que es
	// distinto de haber entregado algo vacío.
	Last(kind string) (hash string, ok bool)
	// Record anota lo que un canal acaba de entregar.
	Record(kind, hash string) error
}

// Canales del registro de entregas.
const (
	DeliveryContext     = "context"
	DeliveryPlanContext = "plan_context"
)
