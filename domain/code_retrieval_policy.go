package domain

// CodeRetrievalPolicy es la directiva canónica de prioridad de recuperación
// según capacidades (feature 038). Texto puro, sin I/O ni dependencias: la
// comparten buildIntegrationBlock y memoryProtocolReminder para que las
// instrucciones permanentes y los recordatorios de sesión no diverjan.
const CodeRetrievalPolicy = "Prioridad de recuperación según capacidades: " +
	"prioriza herramientas de grafo frente a lecturas manuales cuando cubran el proyecto y la consulta. " +
	"Si el agente no admite compresión de resultados externos, como Codex en las versiones verificadas, " +
	"prioriza recuperación comprimida en origen disponible y suficiente; usa search_code para búsquedas Go " +
	"cubiertas y get_symbol para firma, ubicación y relaciones, sin asumir que entrega cuerpos o compresión. " +
	"Si la cobertura o la evidencia no basta, usa un proveedor externo disponible para el proyecto y, si " +
	"tampoco basta, una lectura focalizada, indicando la limitación. En agentes con compresión externa compatible " +
	"conserva su ruta compatible, sin asumir que todas las herramientas o lecturas exactas se comprimen. " +
	"Si la capacidad es desconocida, no prometas compresión automática. No infieras ahorro de una herramienta " +
	"por su nombre; pack_build recupera memoria/Spec Kit y pack_compress no elimina el coste del texto ya recibido."
