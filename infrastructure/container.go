package main

import (
	"os"
	"path/filepath"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/tui"
	"mem/adapters/secondary/codegraph/codebasememory"
	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/speckit"
	"mem/adapters/secondary/tokens"
	"mem/adapters/secondary/usage"
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
	Compressor      ports.Compressor
	TokenCounter    ports.TokenCounter
	SpecKitReader   ports.SpecKitReader
	// UsageRepo/UsageRecorder (feature 020): opcionales — admiten nil sin que
	// ningún emisor se entere. La etiqueta de canal viaja SOLO en la
	// construcción de UsageRecorder (research.md §1); este es el ÚNICO lugar
	// del proyecto donde se nombra un canal.
	UsageRepo     ports.UsageRepository
	UsageRecorder ports.UsageRecorder
}

// NewContainer construye el composition root. channel es la etiqueta del
// canal por el que este proceso emite ("mcp", "cli" o "tui") — un dato de
// construcción, no una autorización: un valor no reconocido se acepta igual
// (feature 020, FR-004).
func NewContainer(root, channel string) (*Container, error) {
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
	// Sinapsis (aristas de co-activación): ON por defecto; se desactiva con
	// synapse_disabled en settings.json para reducir queries por save.
	persistence.SetSynapseEnabled(!settings.SynapseDisabled)
	contextBuilder := usecases.New(memRepo, sessRepo, relRepo, root, project)
	contextBuilder.Graph = codeGraphRepo
	// Presupuesto de contexto (feature 008): techo blando de get_context para no
	// inflar la ventana del agente. Normalizado en ReadSettings (default si 0).
	contextBuilder.Budget = settings.Budget
	// Modo índice (feature 020, fase C): ausente/false = modo completo, el
	// comportamiento histórico (FR-034).
	contextBuilder.IndexMode = settings.ContextIndexMode
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

	// Registro de uso (feature 020): el repositorio y el grabador son
	// opcionales en toda dependencia que los reciba. sessRepo.Active se
	// resuelve EN EL MOMENTO de registrar (no aquí), porque la sesión puede
	// empezar después de construir el Container.
	usageRepo := persistence.NewUsageRepository(db)
	usageRecorder := usage.NewRecorder(usageRepo, project, channel, func() string {
		if sess, _ := sessRepo.Active(project); sess != nil {
			return sess.ID
		}
		return ""
	})
	// get_context es el único emisor que va DENTRO de Build() (T027):
	// ports.ContextBuilder solo expone Build()/WriteFile(), así que el
	// registro vive en el propio Builder concreto, no en un método nuevo del
	// puerto.
	contextBuilder.Counter = tokens.ApproximateTokenCounter{}
	contextBuilder.Recorder = usageRecorder

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
		Compressor:      compression.StructuralCompressor{},
		TokenCounter:    tokens.ApproximateTokenCounter{},
		SpecKitReader:   speckit.Reader{},
		UsageRepo:       usageRepo,
		UsageRecorder:   usageRecorder,
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
		Compressor:      c.Compressor,
		TokenCounter:    c.TokenCounter,
		SpecKitReader:   c.SpecKitReader,
		UsageRepo:       c.UsageRepo,
		UsageRecorder:   c.UsageRecorder,
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
	return tui.Run(c.MemoryRepo, c.RelationRepo, c.SettingsRepo, c.MaintenanceRepo, c.tuiProvider(), c.Root, c.Project, tui.UsageDeps{
		SessionRepo:   c.SessionRepo,
		UsageRepo:     c.UsageRepo,
		TokenCounter:  c.TokenCounter,
		Compressor:    c.Compressor,
		SpecKitReader: c.SpecKitReader,
	})
}

func isMockMode() bool {
	return os.Getenv("USE_MOCK_ADAPTERS") == "true"
}
