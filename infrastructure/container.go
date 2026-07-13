package main

import (
	"os"
	"path/filepath"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/tui"
	"mem/adapters/secondary/codegraph/codebasememory"
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
	CodeGraphRepo   ports.CodeGraphRepository
}

func NewContainer(root string) (*Container, error) {
	db, err := persistence.Open(root)
	if err != nil {
		return nil, err
	}

	project := persistence.ProjectKey(root)

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)
	codeGraphRepo := persistence.NewCodeGraphRepository(db)

	contextBuilder := usecases.New(memRepo, sessRepo, relRepo, root, project)
	contextBuilder.Graph = codeGraphRepo
	// Proveedor(es) EXTERNO(s) de grafo, opcionales y agnósticos al agente.
	// Enchufable por settings (code_graph_disabled / code_graph_command). Si está
	// deshabilitado, el binario no está o el repo no está indexado: degrada en
	// silencio y el contexto se arma igual con el grafo propio.
	if s := persistence.ReadSettings(root); !s.CodeGraphDisabled {
		contextBuilder.CodeProviders = []ports.CodeGraphProvider{
			codebasememory.New(root, filepath.Join(root, persistence.MemDir), s.CodeGraphCommand),
		}
	}

	c := &Container{
		Root:    root,
		Project: project,

		MemoryRepo:      memRepo,
		SessionRepo:     sessRepo,
		RelationRepo:    relRepo,
		SettingsRepo:    persistence.NewSettingsRepository(),
		ProjectRepo:     persistence.NewProjectRepository(),
		ContextBuilder:  contextBuilder,
		MaintenanceRepo: persistence.NewMaintenanceRepository(db, persistence.DbPath(root)),
		CodeGraphRepo:   codeGraphRepo,
	}

	return c, nil
}

func (c *Container) Close() {
}

func (c *Container) ToDeps() *cli.Deps {
	return &cli.Deps{
		Root:            c.Root,
		Project:         c.Project,
		MemoryRepo:      c.MemoryRepo,
		SessionRepo:     c.SessionRepo,
		RelationRepo:    c.RelationRepo,
		SettingsRepo:    c.SettingsRepo,
		ProjectRepo:     c.ProjectRepo,
		ContextBuilder:  c.ContextBuilder,
		MaintenanceRepo: c.MaintenanceRepo,
		CodeGraphRepo:   c.CodeGraphRepo,
	}
}

func (c *Container) RunTUI() error {
	return tui.Run(c.MemoryRepo, c.SettingsRepo, c.MaintenanceRepo, c.Root, c.Project)
}

func isMockMode() bool {
	return os.Getenv("USE_MOCK_ADAPTERS") == "true"
}
