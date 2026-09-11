package ports

// SessionSummaryUpdater persiste el resumen de una sesión de memoria SIN
// cerrarla (feature 030, US2). Es un puerto estrecho, deliberadamente
// separado de SessionRepository: ese puerto ya tiene dobles de prueba en
// adapters/primary/tui/tui_usage_test.go y adapters/primary/cli/cmd_save_test.go,
// y añadirle un método rompería su compilación sin autorización (constitución,
// principio III). El repositorio concreto de sesión implementa también esta
// interfaz estrecha.
type SessionSummaryUpdater interface {
	// UpdateSummary reemplaza el resumen de la sesión id, dejando ended_at
	// intacto. Devuelve error si la sesión no existe o ya está cerrada —
	// misma semántica de fallo explícito que SessionRepository.End.
	UpdateSummary(id, summary string) error
}
