package domain

// Este archivo declara, junto al canal, dos datos que el informe de estado
// necesitaba y no tenía: qué se pierde cuando un canal no funciona, y con qué
// comando se restablece.
//
// Viven aquí y no en el renderizador del informe (FR-008) para que un canal
// nuevo los traiga consigo. Si estuvieran en quien imprime, añadir un canal
// exigiría acordarse de editar también el informe, que es la clase de olvido
// que esta familia de features intenta hacer imposible.

// TodosLosCanales enumera los tipos de canal que el informe puede reportar.
func TodosLosCanales() []ChannelKind {
	return []ChannelKind{
		KindPlanEntry, KindPlanGuard, KindTurnReminder,
		KindInstructions, KindNativeWrapper, KindMCPInstructions,
		KindServerConfig, KindPermissions,
	}
}

// EfectoDelCanal describe qué deja de ocurrir cuando el canal no funciona.
//
// La redacción es deliberadamente de comportamiento y no de mecanismo: quien
// lee el informe necesita saber si le importa, no cómo está construido. Decir
// "falta el archivo X" obliga a conocer el sistema por dentro para traducirlo.
func EfectoDelCanal(k ChannelKind) string {
	switch k {
	case KindPlanEntry:
		return "al entrar en modo plan, el agente no recibe el método de descomposición ni el historial del proyecto, y planifica sin ese contexto"
	case KindPlanGuard:
		return "un plan sin forma de árbol de tareas puede llegar a ti sin que nada lo detenga antes"
	case KindTurnReminder:
		return "el agente deja de recibir el recordatorio por turno: guarda menos memorias y no avisa cuándo conviene compactar"
	case KindInstructions:
		return "el agente no lee el protocolo de memoria al arrancar y usa las herramientas solo si se lo pides"
	case KindNativeWrapper:
		return "los atajos propios del agente dejan de estar disponibles; la funcionalidad sigue accesible por comando"
	case KindMCPInstructions:
		return "el agente se conecta sin recibir el protocolo y no sabe cuándo usar la memoria"
	case KindServerConfig:
		return "el agente no encuentra el servidor de memoria: ninguna herramienta aparece disponible"
	case KindPermissions:
		return "cada llamada a una herramienta de memoria queda esperando tu aprobación, y el protocolo deja de aplicarse solo"
	default:
		return "el agente pierde una capacidad de memoria en este ámbito"
	}
}

// Correccion es la acción propuesta para restablecer un canal.
type Correccion struct {
	// Comando restablece el canal. Debe ser ejecutable tal como se muestra.
	Comando string
	// Advertencia describe el alcance cuando la corrección sale del proyecto.
	// Vacía si el efecto queda contenido en él.
	Advertencia string
}

// CorreccionPara devuelve el comando que restablece los canales de un agente en
// un ámbito.
//
// Una corrección de ámbito de usuario afecta a todos los proyectos de la
// máquina. Se advierte antes de proponerla (FR-006): proponerla como una acción
// simple ocultaría que su efecto sale del proyecto en el que se ejecuta.
func CorreccionPara(agent string, scope AgentScope) Correccion {
	if scope == ScopeUser {
		return Correccion{
			Comando:     "mem setup-mcp --scope global --agents " + agent,
			Advertencia: "afecta a todos los proyectos de esta máquina, no solo a este",
		}
	}
	return Correccion{Comando: "mem install ."}
}
