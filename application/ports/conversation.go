package ports

// ConversationResolver dice a qué conversación de agente pertenece el proceso
// actual (HCD-036). Cada agente que trabaja en el proyecto tiene la suya, y con
// ella su sesión de gomemory, su registro de entregas y su estado por turno.
type ConversationResolver interface {
	// Current devuelve el id de la conversación del proceso, o "" si no hay
	// ninguna identificable: entonces se conserva el comportamiento anterior.
	Current() string
	// Adopt fija el id que trae el payload de un hook: es el del host y manda
	// sobre el enlace y el id local.
	Adopt(hookID string)
}

// HostBinding enlaza el proceso host de un agente (Claude Code, Codex,
// OpenCode) con la conversación que atiende. Los hooks publican el enlace y el
// servidor MCP y la CLI lo leen a través de sus procesos ancestros.
type HostBinding interface {
	// Publish anota que el host del proceso actual atiende la conversación id.
	Publish(conversationID string) error
	// Lookup devuelve la conversación enlazada al host del proceso actual.
	Lookup() (string, bool)
}
