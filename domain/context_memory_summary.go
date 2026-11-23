package domain

// ContextMemorySummary contains only global-history metadata needed by context
// titles, ranking and anchors. DisplayTitle is sanitized and already includes
// the legacy content fallback when the stored title is empty. No body is exposed.
type ContextMemorySummary struct {
	ID           int64
	Type         MemoryType
	SessionID    string
	DisplayTitle string
	Filepath     string
}
