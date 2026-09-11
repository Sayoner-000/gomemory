package ports

import "mem/domain"

// SessionMemoryLister lista las memorias asociadas a una sesión concreta
// (feature 030, US1). Es un puerto estrecho, deliberadamente separado de
// MemoryRepository: ese puerto tiene dobles de prueba en
// adapters/primary/tui/tui_test.go, adapters/primary/cli/cmd_save_test.go,
// adapters/primary/cli/cmd_footprint_test.go y
// application/usecases/import_adrs_test.go, y añadirle un método rompería su
// compilación sin autorización (constitución, principio III). El repositorio
// concreto de memorias implementa también esta interfaz estrecha.
type SessionMemoryLister interface {
	// ListBySession devuelve las memorias de esa sesión y proyecto, de la más
	// reciente a la más antigua, respetando limit. sessionID vacío devuelve
	// una lista vacía y nil (nunca todas las memorias del proyecto).
	ListBySession(project, sessionID string, limit int) ([]domain.Memory, error)
}
