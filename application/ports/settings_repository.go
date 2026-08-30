package ports

type SettingsData struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
	// CodeGraphDisabled apaga el proveedor de grafo de código externo
	// (codebase-memory-mcp). Ausente/false = auto-detección activada.
	CodeGraphDisabled bool `json:"code_graph_disabled,omitempty"`
	// CodeGraphCommand apunta a otro binario del proveedor. Vacío = default.
	// Legado: ver CodeGraphProviders (feature 010).
	CodeGraphCommand string `json:"code_graph_command,omitempty"`
	// CodeGraphProviders es la lista ordenada (prioridad) de proveedores de
	// grafo de código candidatos (feature 010, Historia 3).
	CodeGraphProviders []string `json:"code_graph_providers,omitempty"`
	// AdrSyncEnabled activa la sincronización bidireccional de ADR (feature
	// 010, Historia 2). Default false.
	AdrSyncEnabled bool `json:"adr_sync_enabled,omitempty"`
	// CodeImpactAnnotationDisabled apaga la anotación de impacto al guardar
	// (feature 010, Historia 1). Ausente/false = activada.
	CodeImpactAnnotationDisabled bool `json:"code_impact_annotation_disabled,omitempty"`
	// Budget es el techo blando (en CARACTERES emitidos) de get_context. <=0 =
	// sin límite (opt-in). Reduce la huella persistente del contexto de arranque.
	Budget int `json:"budget,omitempty"`
	// CompactThreshold es la huella (en caracteres emitidos por gomemory en la
	// sesión) a partir de la cual el hook de fin de turno sugiere compactar. <=0 =
	// desactivado.
	CompactThreshold int `json:"compact_threshold,omitempty"`
	// DedupWindowDays es la ventana (días) para consolidar memorias equivalentes
	// (mismo proyecto+tipo+título) en vez de crear filas nuevas. <=0 = sin dedup
	// por identidad.
	DedupWindowDays int `json:"dedup_window_days,omitempty"`
	// SynapseDisabled apaga la formación automática de sinapsis (aristas de
	// co-activación en sesión). Ausente/false = activada.
	SynapseDisabled bool `json:"synapse_disabled,omitempty"`
	// SpeckitContextDisabled apaga el brazo extensor hacia spec-kit (feature
	// 011, historia 4). Ausente/false = activado.
	SpeckitContextDisabled bool `json:"speckit_context_disabled,omitempty"`
	// AtomicPlanDisabled apaga la planificación atómica en modo plan (feature
	// 013, historia 4): get_plan_context deja de emitir método y contexto.
	// Ausente/false = activada, mismo patrón opt-out que SpeckitContextDisabled.
	AtomicPlanDisabled bool `json:"atomic_plan_disabled,omitempty"`
	// PlanGuardDisabled apaga la exigencia de forma del plan (feature 019,
	// historia 1): `mem hook plan-guard` deja de evaluar y devolver planes sin
	// forma de árbol, permitiendo siempre. Ausente/false = activada, mismo
	// patrón opt-out que AtomicPlanDisabled.
	PlanGuardDisabled bool `json:"plan_guard_disabled,omitempty"`
	// ContextDefaultBudget es el presupuesto de tokens por defecto para
	// `mem pack build` cuando el ajuste se consulta programáticamente (el CLI
	// exige --max-tokens explícito). Ausente/0 = default de fábrica; negativo
	// no tiene sentido (un presupuesto no puede ser "ilimitado") y se trata
	// como ausente (feature 015).
	ContextDefaultBudget int `json:"context_default_budget,omitempty"`
	// ContextMinRelevance es el umbral mínimo de relevancia (0–1) para que un
	// candidato entre a un ContextPack. Ausente/0 = default de fábrica;
	// negativo = sin filtro de relevancia (opt-out explícito, feature 015).
	ContextMinRelevance float64 `json:"context_min_relevance,omitempty"`
	// ContextMaxItems es el tope de candidatos a considerar antes de rankear.
	// Ausente/0 = default de fábrica; negativo = sin tope (feature 015).
	ContextMaxItems int `json:"context_max_items,omitempty"`
	// ContextCompressionDisabled apaga la compresión estructural de
	// BuildContextPack (Compression=None en vez de Structural). Ausente/false
	// = activada, mismo patrón opt-out que SpeckitContextDisabled
	// (feature 015).
	ContextCompressionDisabled bool `json:"context_compression_disabled,omitempty"`
	// ContextDedupDisabled apaga la deduplicación de BuildContextPack.
	// Ausente/false = activada (feature 015).
	ContextDedupDisabled bool `json:"context_dedup_disabled,omitempty"`
	// UsageWindowTokens es la ventana de referencia (en tokens) contra la que
	// `mem usage` expresa el ahorro como porcentaje. Ausente/0 = sin ventana:
	// esa línea del reporte no se muestra. Es un valor que PROVEE el usuario,
	// nunca una lectura ni un default que presuma la ventana de ningún agente
	// concreto (feature 020, FR-014).
	UsageWindowTokens int `json:"usage_window_tokens,omitempty"`
	// ContextIndexMode activa la emisión de contexto en modo índice: protocolo
	// íntegro + una línea por memoria (id, tipo, título), detalle bajo demanda
	// con get_memory(id). Ausente/false = modo completo, el comportamiento
	// actual (feature 020, FR-034).
	ContextIndexMode bool `json:"context_index_mode,omitempty"`
	// ReviewMaxFixRounds, ReviewAutoFixSeverities y ReviewFixAuthorized son la
	// política de revisión adversarial del proyecto (feature 028, FR-017).
	//
	// Viajan en SettingsData y no solo en el Settings de persistencia por una razón
	// concreta: Write reconstruye el Settings COMPLETO a partir de esta struct, así
	// que un campo que no esté aquí se pierde al guardar cualquier otra preferencia.
	// La política de revisión se estaba borrando así, en silencio.
	ReviewMaxFixRounds      int      `json:"review_max_fix_rounds,omitempty"`
	ReviewAutoFixSeverities []string `json:"review_auto_fix_severities,omitempty"`
	// ReviewFixAuthorized es un puntero porque un bool JSON no distingue "ausente"
	// de "false", y aquí la diferencia decide si una revisión puede corregir.
	ReviewFixAuthorized *bool `json:"review_fix_authorized,omitempty"`
}

type SettingsRepository interface {
	Read(root string) SettingsData
	Write(root string, s SettingsData) error
	ApplyAutoApprove(root string, s SettingsData)
}
