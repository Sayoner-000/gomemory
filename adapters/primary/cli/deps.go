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

	MemoryRepo     ports.MemoryRepository
	SessionRepo    ports.SessionRepository
	RelationRepo   ports.RelationRepository
	ReviewRepo     ports.ReviewRepository
	ConsensusRepo  ports.ConsensusRepository
	SettingsRepo   ports.SettingsRepository
	ProjectRepo    ports.ProjectRepository
	ContextBuilder ports.ContextBuilder
	// DeliveryLog registra qué material ya recibió el agente en la sesión en
	// curso, para no reenviarlo. Puede ser nil: sin él, los casos de uso
	// entregan el documento completo.
	DeliveryLog ports.DeliveryLog
	// ChannelActivity registra si cada canal de inyección sigue vivo. Puede
	// ser nil: sin él, el informe no reporta vitalidad.
	ChannelActivity ports.ChannelActivityLog
	MaintenanceRepo ports.MaintenanceRepository
	CodeGraphRepo   ports.CodeGraphRepository
	// CodeProviders son los proveedores EXTERNOS de grafo (opcionales). Los
	// hooks los usan para refrescar el snapshot por turno sin bloquear
	// (MaybeRefresh es fire-and-forget). Vacío si están deshabilitados.
	CodeProviders []ports.CodeGraphProvider
	// TUIProvider es el proveedor de grafo externo para la TUI. Se construye
	// SIEMPRE (independiente del toggle) para poder mostrar el estado del grafo
	// aunque esté desactivado; Snapshot() solo lee el cache y nunca bloquea.
	TUIProvider ports.CodeGraphProvider
	// ADRSyncProvider/ADRSyncRepo (feature 010, Historia 2): nil si
	// adr_sync_enabled=false o no hay proveedor disponible — el hook de
	// refresco y `mem adr-sync status` deben chequear nil antes de usarlos.
	ADRSyncProvider ports.ADRSyncProvider
	ADRSyncRepo     ports.ADRSyncRepository
	// Compressor/TokenCounter (feature 015): dependencias del Context
	// Optimization Engine (`mem pack ...` y las tools MCP pack_*). Siempre
	// construidos (sin toggle) porque son puro cómputo local, sin I/O externo.
	Compressor    ports.Compressor
	TokenCounter  ports.TokenCounter
	SpecKitReader ports.SpecKitReader
	// UsageRepo/UsageRecorder (feature 020): opcionales, admiten nil.
	UsageRepo ports.UsageRepository
	// OctopusRepo (feature 027): telemetría del enrutador adaptativo. Admite
	// nil — sin él, Octopus enruta igual, solo que sin memoria de lo ocurrido
	// (INV-AAR-015). Nunca se consulta con el módulo apagado.
	OctopusRepo   ports.OctopusRepository
	UsageRecorder ports.UsageRecorder
}
