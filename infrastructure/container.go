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
	ADRSyncProvider ports.ADRSyncProvider
	ADRSyncRepo     ports.ADRSyncRepository
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
		codeProviders = buildCodeProviders(root, settings)
	}
	contextBuilder.CodeProviders = codeProviders

	// Proveedor "activo" para los consumidores que necesitan una única
	// fuente inequívoca (a diferencia de get_context, que muestra una
	// sección por cada uno disponible): el primero de la lista cuyo
	// snapshot cacheado esté disponible (feature 010, Historia 3).
	activeProvider := usecases.FirstAvailable(codeProviders)

	// Anotación de impacto al guardar (feature 010, Historia 1). nil si no
	// hay proveedor activo o si la capacidad está apagada por settings.
	if !settings.CodeImpactAnnotationDisabled {
		persistence.SetCodeImpactProvider(activeProvider)
	} else {
		persistence.SetCodeImpactProvider(nil)
	}

	// Sincronización de ADR (feature 010, Historia 2): opt-in explícito
	// (default false). Reusa el mismo proveedor activo que Historia 1 —
	// codebasememory.Provider implementa tanto CodeGraphProvider como
	// ADRSyncProvider, así que el type assertion solo falla si algún día hay
	// un CodeGraphProvider que NO hable manage_adr (degrada a nil, sin
	// exportar/importar, sin error).
	adrSyncRepo := persistence.NewADRSyncRepository(db)
	var adrSyncProvider ports.ADRSyncProvider
	if activeProvider != nil {
		adrSyncProvider, _ = activeProvider.(ports.ADRSyncProvider)
	}
	persistence.SetAdrSyncEnabled(settings.AdrSyncEnabled)
	persistence.SetADRSync(adrSyncProvider, adrSyncRepo)

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
		ADRSyncRepo:     adrSyncRepo,
	}
	if settings.AdrSyncEnabled {
		c.ADRSyncProvider = adrSyncProvider
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
		ADRSyncProvider: c.ADRSyncProvider,
		ADRSyncRepo:     c.ADRSyncRepo,
	}
}

// tuiProvider construye el proveedor de grafo externo para la TUI. Se construye
// SIEMPRE (independiente del toggle), para poder mostrar el estado del grafo
// externo aunque esté desactivado. Snapshot() solo lee el archivo cacheado:
// nunca bloquea. Con varios candidatos configurados (Historia 3), muestra el
// primero disponible — si ninguno lo está, el primero de la lista (para que
// la TUI tenga algo que mostrar como "no disponible" en vez de nada).
func (c *Container) tuiProvider() ports.CodeGraphProvider {
	s := persistence.ReadSettings(c.Root)
	providers := buildCodeProviders(c.Root, s)
	if active := usecases.FirstAvailable(providers); active != nil {
		return active
	}
	if len(providers) > 0 {
		return providers[0]
	}
	return nil
}

// buildCodeProviders construye un CodeGraphProvider por cada comando
// candidato en settings.CodeGraphProviders (ya normalizada por ReadSettings,
// que incluye el legado CodeGraphCommand cuando la lista viene vacía). Sin
// ningún candidato configurado, arma el único proveedor por defecto
// (autodetección en PATH) — mismo comportamiento que antes de Historia 3.
func buildCodeProviders(root string, settings persistence.Settings) []ports.CodeGraphProvider {
	memDir := filepath.Join(root, persistence.MemDir)
	if len(settings.CodeGraphProviders) == 0 {
		return []ports.CodeGraphProvider{codebasememory.New(root, memDir, "")}
	}
	providers := make([]ports.CodeGraphProvider, 0, len(settings.CodeGraphProviders))
	for _, cmd := range settings.CodeGraphProviders {
		providers = append(providers, codebasememory.New(root, memDir, cmd))
	}
	return providers
}

func (c *Container) RunTUI() error {
	return tui.Run(c.MemoryRepo, c.RelationRepo, c.SettingsRepo, c.MaintenanceRepo, c.tuiProvider(), c.Root, c.Project)
}

func isMockMode() bool {
	return os.Getenv("USE_MOCK_ADAPTERS") == "true"
}
