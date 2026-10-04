package ports

// SessionActivityReader informa la última actividad de una sesión de memoria
// (feature 035, research.md R2), para rotar las que llevan horas abiertas sin
// uso: Codex no tiene evento de fin de sesión y OpenCode solo cierra al
// terminar el proceso. Es un puerto estrecho, separado de SessionRepository por
// la misma razón que SessionSummaryUpdater: ampliar ese puerto rompería sus
// dobles de prueba sin autorización (constitución, principio III).
type SessionActivityReader interface {
	// LastActivity devuelve la última escritura de la sesión (memorias y
	// checkpoints incluidos) o su creación si no tiene memorias, en el formato
	// de timestamp del almacén. ok=false si la sesión no existe.
	LastActivity(sessionID string) (ts string, ok bool, err error)
}
