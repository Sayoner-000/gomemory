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
}
