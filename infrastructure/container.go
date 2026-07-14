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

	settings := persistence.ReadSettings(root)
	// Dedup en la fuente (feature 008): la ventana de identidad se toma de settings
	// (singleton de proceso). <=0 desactiva el dedup por identidad.
	persistence.SetDedupWindowDays(settings.DedupWindowDays)
	contextBuilder := usecases.New(memRepo, sessRepo, relRepo, root, project)
	contextBuilder.Graph = codeGraphRepo
	// Presupuesto de contexto (feature 008): techo blando de get_context para no
	// inflar la ventana del agente. Normalizado en ReadSettings (default si 0).
	contextBuilder.Budget = settings.Budget
	// Proveedor(es) EXTERNO(s) de grafo, opcionales y agnósticos al agente.
	// Enchufable por settings (code_graph_disabled / code_graph_command). Si está
	// deshabilitado, el binario no está o el repo no está indexado: degrada en
	// silencio y el contexto se arma igual con el grafo propio.
	var codeProviders []ports.CodeGraphProvider
	if !settings.CodeGraphDisabled {
		codeProviders = []ports.CodeGraphProvider{
			codebasememory.New(root, filepath.Join(root, persistence.MemDir), settings.CodeGraphCommand),
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
