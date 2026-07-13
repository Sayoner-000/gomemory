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
	CodeProviders   []ports.CodeGraphProvider
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
	var codeProviders []ports.CodeGraphProvider
	if s := persistence.ReadSettings(root); !s.CodeGraphDisabled {
		codeProviders = []ports.CodeGraphProvider{
			codebasememory.New(root, filepath.Join(root, persistence.MemDir), s.CodeGraphCommand),
		}
	}
	contextBuilder.CodeProviders = codeProviders

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
		CodeProviders:   codeProviders,
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
		CodeProviders:   c.CodeProviders,
		TUIProvider:     c.tuiProvider(),
	}
}

// tuiProvider construye el proveedor de grafo externo para la TUI. Se construye
// SIEMPRE (independiente del toggle), para poder mostrar el estado del grafo
// externo aunque esté desactivado. Snapshot() solo lee el archivo cacheado:
// nunca bloquea.
func (c *Container) tuiProvider() ports.CodeGraphProvider {
	s := persistence.ReadSettings(c.Root)
	return codebasememory.New(c.Root, filepath.Join(c.Root, persistence.MemDir), s.CodeGraphCommand)
}

func (c *Container) RunTUI() error {
	return tui.Run(c.MemoryRepo, c.RelationRepo, c.SettingsRepo, c.MaintenanceRepo, c.tuiProvider(), c.Root, c.Project)
}

func isMockMode() bool {
	return os.Getenv("USE_MOCK_ADAPTERS") == "true"
}
