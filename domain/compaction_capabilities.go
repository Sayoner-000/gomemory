package domain

// CompactionCapability es una de las capacidades que un cliente puede ofrecer
// alrededor de la compactación de su propia conversación (feature 030,
// spec.md «Capacidades del cliente»). El vocabulario es deliberadamente
// agnóstico: no nombra agentes ni comandos de cliente. La correspondencia
// concreta agente→capacidad vive en KnownAgents (domain/agents.go).
type CompactionCapability string

const (
	// CompactionPreCompactInput (C1): el cliente permite añadir texto que el
	// compresor tiene en cuenta al generar el resumen compactado.
	CompactionPreCompactInput CompactionCapability = "pre_compact_input"
	// CompactionPostCompactChannel (C2): el cliente entrega al agente un
	// texto que lee al reanudar tras compactar.
	CompactionPostCompactChannel CompactionCapability = "post_compact_channel"
	// CompactionSubagentFinalText (C3): el cliente entrega, al terminar un
	// subagente, el mensaje final que produjo.
	CompactionSubagentFinalText CompactionCapability = "subagent_final_text"
	// CompactionAgentTurnChannel (C4): el cliente entrega al agente un texto
	// breve en el fin de turno o en el turno siguiente, sin interrumpirlo.
	CompactionAgentTurnChannel CompactionCapability = "agent_turn_channel"
	// CompactionHumanChannel (C5): el cliente muestra un aviso visible a la
	// persona.
	CompactionHumanChannel CompactionCapability = "human_channel"
	// CompactionSummaryInput (C6): la integración recibe el texto del resumen
	// que produjo la compactación.
	CompactionSummaryInput CompactionCapability = "compact_summary_input"
)

// AllCompactionCapabilities devuelve las 6 capacidades en orden C1-C6. Única
// fuente de la lista completa: el test de contrato INV-C1 y `mem doctor` la
// recorren desde aquí, en vez de mantener la enumeración por su cuenta.
func AllCompactionCapabilities() []CompactionCapability {
	return []CompactionCapability{
		CompactionPreCompactInput,
		CompactionPostCompactChannel,
		CompactionSubagentFinalText,
		CompactionAgentTurnChannel,
		CompactionHumanChannel,
		CompactionSummaryInput,
	}
}
