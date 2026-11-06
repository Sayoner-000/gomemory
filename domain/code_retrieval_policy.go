package domain

// CodeRetrievalPolicy es la directiva canónica de prioridad de recuperación
// según capacidades (feature 038). Texto puro, sin I/O ni dependencias: la
// comparten buildIntegrationBlock y memoryProtocolReminder para que las
// instrucciones permanentes y los recordatorios de sesión no diverjan.
const CodeRetrievalPolicy = "Prioridad de recuperación según capacidades: " +
	"prioriza herramientas de grafo frente a lecturas manuales cuando cubran el proyecto y la consulta. " +
	"Si el agente no admite compresión de resultados externos, como Codex en las versiones verificadas o " +
	"Antigravity (agy), cuyos hooks no reciben la salida de las herramientas, prioriza recuperación comprimida " +
	"en origen disponible y suficiente antes que lecturas o grep: get_context y search_memories para memoria, " +
	"y search_code, cuyas salidas comprime gomemory según su nivel de compresión; usa search_code para búsquedas Go " +
	"cubiertas y get_symbol para firma, ubicación y relaciones, sin asumir que entrega cuerpos o compresión. " +
	"Si la cobertura o la evidencia no basta, usa un proveedor externo disponible para el proyecto y, si " +
	"tampoco basta, una lectura focalizada, indicando la limitación. En agentes con compresión externa compatible " +
	"conserva su ruta compatible, sin asumir que todas las herramientas o lecturas exactas se comprimen. " +
	"Si la capacidad es desconocida, no prometas compresión automática. No infieras ahorro de una herramienta " +
	"por su nombre; pack_build recupera memoria/Spec Kit y pack_compress no elimina el coste del texto ya recibido."

// CodeRetrievalPolicyShort es CodeRetrievalPolicy en tres frases, con la misma
// prioridad. La usan los recordatorios de sesión, que se repiten en el primer
// prompt y al final de get_context y get_plan_context; las instrucciones MCP
// conservan la larga (HCD-040, Q2). Si cambia la prioridad, cambian las dos.
const CodeRetrievalPolicyShort = "Prioriza el grafo de código frente a lecturas manuales cuando cubra la consulta. " +
	"Sin compresión de resultados externos (Codex, agy), usa antes get_context, search_memories, search_code y " +
	"get_symbol, y después un proveedor externo o una lectura focalizada, indicando la limitación. " +
	"No prometas compresión ni infieras ahorro por el nombre de una herramienta."
