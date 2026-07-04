package cli

import "mem/application/ports"

type Deps struct {
	// Root y Project son la raíz y el identificador de proyecto ya resueltos
	// por el composition root (infrastructure.NewContainer) antes de
	// construir el resto de repos — la única fuente de verdad para "a qué
	// proyecto pertenece esta invocación", en vez de que cada comando los
	// recalcule por su cuenta.
	Root    string
	Project string

	MemoryRepo      ports.MemoryRepository
	SessionRepo     ports.SessionRepository
	RelationRepo    ports.RelationRepository
	SettingsRepo    ports.SettingsRepository
	ProjectRepo     ports.ProjectRepository
	ContextBuilder  ports.ContextBuilder
	MaintenanceRepo ports.MaintenanceRepository
	CodeGraphRepo   ports.CodeGraphRepository
}
