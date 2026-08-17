package ports

import "mem/domain"

// ActivationInspector inspecciona los canales de activación del modo plan
// atómico instalados para un proyecto (feature 019, Historia 3). Implementado
// en el adaptador de setup, que conoce las rutas concretas de cada agente;
// este puerto mantiene al caso de uso de diagnóstico sin depender del
// filesystem (constitución, Principio I).
type ActivationInspector interface {
	// Inspect recorre domain.KnownAgents y el estado del brazo extensor de
	// grafo de código para root (ámbito proyecto) y para el directorio de
	// usuario, devolviendo un canal por combinación agente/ámbito/tipo
	// aplicable. Nunca falla: una ruta que no se puede leer se traduce en un
	// canal StateMissing, no en un error.
	Inspect(root string) []domain.ActivationChannel
}
