package cli

import "mem/application/ports"

type Deps struct {
	MemoryRepo     ports.MemoryRepository
	SessionRepo    ports.SessionRepository
	RelationRepo   ports.RelationRepository
	SettingsRepo   ports.SettingsRepository
	ProjectRepo    ports.ProjectRepository
	ContextBuilder ports.ContextBuilder
}
