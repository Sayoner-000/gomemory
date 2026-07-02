package main

import (
	"os"
	"path/filepath"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/mcp"
	"mem/adapters/primary/tui"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
)

type Container struct {
	Root    string
	Project string

	MemoryRepo      ports.MemoryRepository
	SessionRepo     ports.SessionRepository
	RelationRepo    ports.RelationRepository
	SettingsRepo    ports.SettingsRepository
	ProjectRepo     ports.ProjectRepository
	ContextBuilder  ports.ContextBuilder
	MaintenanceRepo ports.MaintenanceRepository

	MCPServer *mcp.Server
}

func NewContainer(root string) (*Container, error) {
	db, err := persistence.Open(root)
	if err != nil {
		return nil, err
	}

	project := filepath.Base(root)

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	c := &Container{
		Root:    root,
		Project: project,

		MemoryRepo:      memRepo,
		SessionRepo:     sessRepo,
		RelationRepo:    relRepo,
		SettingsRepo:    persistence.NewSettingsRepository(),
		ProjectRepo:     persistence.NewProjectRepository(),
		ContextBuilder:  usecases.New(memRepo, sessRepo, relRepo, root, project),
		MaintenanceRepo: persistence.NewMaintenanceRepository(db, persistence.DbPath(root)),

		MCPServer: mcp.NewWithRepos(memRepo, sessRepo, project, 0),
	}

	return c, nil
}

func (c *Container) Close() {
}

func (c *Container) ToDeps() *cli.Deps {
	return &cli.Deps{
		MemoryRepo:      c.MemoryRepo,
		SessionRepo:     c.SessionRepo,
		RelationRepo:    c.RelationRepo,
		SettingsRepo:    c.SettingsRepo,
		ProjectRepo:     c.ProjectRepo,
		ContextBuilder:  c.ContextBuilder,
		MaintenanceRepo: c.MaintenanceRepo,
	}
}

func (c *Container) RunTUI() error {
	return tui.Run(c.MemoryRepo, c.SettingsRepo, c.MaintenanceRepo, c.Root, c.Project)
}

func isMockMode() bool {
	return os.Getenv("USE_MOCK_ADAPTERS") == "true"
}
