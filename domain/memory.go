package domain

type MemoryType string

const (
	Learning     MemoryType = "learning"
	Decision     MemoryType = "decision"
	Architecture MemoryType = "architecture"
	Bugfix       MemoryType = "bugfix"
	Pattern      MemoryType = "pattern"
	Discovery    MemoryType = "discovery"
	// Preference captura preferencias de interacción del usuario (cómo quiere
	// que le hable el agente, correcciones de estilo/flujo, confirmación de
	// un enfoque) — distinto de conocimiento sobre el código.
	Preference MemoryType = "preference"
	// Checkpoint es un registro automático de actividad de un turno (archivos
	// tocados, comandos corridos), generado por el hook turn-end sin
	// intervención del agente — no por save_memory.
	Checkpoint MemoryType = "checkpoint"
)

func ValidMemoryType(s string) MemoryType {
	switch MemoryType(s) {
	case Learning, Decision, Architecture, Bugfix, Pattern, Discovery, Preference, Checkpoint:
		return MemoryType(s)
	default:
		return Learning
	}
}

type Memory struct {
	ID        int64      `json:"id"`
	Project   string     `json:"project"`
	SessionID string     `json:"session_id,omitempty"`
	Type      MemoryType `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Filepath  string     `json:"filepath,omitempty"`
	// OriginPrompt es el prompt del usuario que originó este guardado, adjuntado
	// automáticamente en el momento del Insert desde el último prompt de la sesión
	// activa (ver SetSessionLastPrompt). Da trazabilidad: por qué se guardó esto.
	// Vacío cuando el agente no expone el prompt (p. ej. clientes MCP sin hooks).
	OriginPrompt string `json:"origin_prompt,omitempty"`
	// TopicKey agrupa memorias del mismo tópico para el upsert de deduplicación
	// (feature 008): guardar con un TopicKey ya existente en el proyecto actualiza
	// la memoria previa en vez de crear una fila nueva. Vacío = sin agrupación.
	TopicKey  string `json:"topic_key,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
