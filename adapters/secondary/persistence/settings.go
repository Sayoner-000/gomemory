package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"

	"mem/domain"
)

type Settings struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
	// CodeGraphDisabled apaga el proveedor de grafo de código EXTERNO
	// (codebase-memory-mcp). Ausente/false = auto-detección activada.
	CodeGraphDisabled bool `json:"code_graph_disabled,omitempty"`
	// CodeGraphCommand permite apuntar a otro binario del proveedor. Vacío =
	// se busca "codebase-memory-mcp" en el PATH. Campo LEGADO: se conserva
	// por compatibilidad, pero ReadSettings lo normaliza a CodeGraphProviders
	// cuando esta lista viene vacía (feature 010).
	CodeGraphCommand string `json:"code_graph_command,omitempty"`
	// CodeGraphProviders es la lista ordenada (prioridad) de proveedores de
	// grafo de código candidatos. Vacía ⇒ se usa CodeGraphCommand (si lo hay)
	// como lista de 1 elemento; si tampoco hay, autodetección en PATH.
	CodeGraphProviders []string `json:"code_graph_providers,omitempty"`
	// AdrSyncEnabled activa la sincronización bidireccional de ADR (feature
	// 010, Historia 2). Default false: opt-in explícito.
	AdrSyncEnabled bool `json:"adr_sync_enabled,omitempty"`
	// CodeImpactAnnotationDisabled apaga la anotación de impacto al guardar
	// una memoria con filepath (feature 010, Historia 1). Ausente/false =
	// activada por defecto — mismo patrón "disabled" que CodeGraphDisabled,
	// necesario porque un bool JSON no distingue "ausente" de "false" y la
	// anotación debe quedar ON sin que el usuario tenga que optar por ella.
	CodeImpactAnnotationDisabled bool `json:"code_impact_annotation_disabled,omitempty"`
	// Budget: techo blando (CARACTERES) de get_context. Semántica normalizada en
	// ReadSettings: ausente/0 → default; negativo → sin límite (opt-out).
	Budget int `json:"budget,omitempty"`
	// CompactThreshold: huella (caracteres emitidos/sesión) que dispara el
	// recordatorio de compactación. Ausente/0 → default; negativo → desactivado.
	CompactThreshold int `json:"compact_threshold,omitempty"`
	// DedupWindowDays: ventana (días) del dedup por identidad. Ausente/0 →
	// default; negativo → sin dedup por identidad.
	DedupWindowDays int `json:"dedup_window_days,omitempty"`
	// SynapseDisabled apaga la formación automática de sinapsis (aristas de
	// co-activación en sesión) al guardar una memoria. Ausente/false = activada
	// (misma lógica opt-out que CodeImpactAnnotationDisabled).
	SynapseDisabled bool `json:"synapse_disabled,omitempty"`
	// SpeckitContextDisabled apaga el brazo extensor hacia spec-kit (feature
	// 011): el hook before_specify de la extensión gomemory-context deja de
	// incorporar el resumen de mem context. Ausente/false = activado, mismo
	// patrón opt-out que CodeGraphDisabled. Se lee directo de este JSON desde
	// el script del hook (sin pasar por mem settings), así el gate no depende
	// de que la CLI/TUI ya lo expongan.
	SpeckitContextDisabled bool `json:"speckit_context_disabled,omitempty"`
	// ReviewMaxFixRounds: presupuesto de rondas de corrección de la revisión
	// adversarial (feature 027, INV-009). Ausente/0 → default. A diferencia de
	// los tunables de huella, un valor negativo NO significa «sin límite»:
	// applyReviewDefaults lo normaliza al defecto, porque una revisión sin
	// techo de rondas es exactamente el bucle infinito que la invariante existe
	// para impedir.
	ReviewMaxFixRounds int `json:"review_max_fix_rounds,omitempty"`
	// ReviewAutoFixSeverities: severidades que pueden corregirse sin
	// autorización explícita caso a caso. Ausente/vacía → CRITICAL y HIGH.
	// Una lista vacía nunca se interpreta como «todas»: convertiría un hallazgo
	// informativo en material corregible automáticamente.
	ReviewAutoFixSeverities []string `json:"review_auto_fix_severities,omitempty"`
	// ReviewFixAuthorized declara si las revisiones del proyecto pueden corregir
	// hallazgos confirmados o son de solo lectura (feature 028, FR-018). Puntero
	// a propósito: un bool JSON no distingue "ausente" de "false", y ausente debe
	// significar autorizado —el comportamiento de todas las revisiones anteriores—
	// mientras que un false explícito debe respetarse.
	ReviewFixAuthorized *bool `json:"review_fix_authorized,omitempty"`
	// AtomicPlanDisabled apaga la planificación atómica en modo plan (feature
	// 013): `mem plan-context` / get_plan_context terminan sin salida. Ausente/
	// false = activada, mismo patrón opt-out que SpeckitContextDisabled. Un
	// bool JSON no distingue "ausente" de "false", y la funcionalidad debe
	// quedar ON sin que el usuario tenga que optar por ella.
	AtomicPlanDisabled bool `json:"atomic_plan_disabled,omitempty"`
	// PlanGuardDisabled apaga la exigencia de forma del plan (feature 019):
	// `mem hook plan-guard` deja de evaluar y devolver planes sin forma de
	// árbol, permitiendo siempre. Ausente/false = activada, mismo patrón
	// opt-out que AtomicPlanDisabled.
	PlanGuardDisabled bool `json:"plan_guard_disabled,omitempty"`
	// ContextDefaultBudget/ContextMinRelevance/ContextMaxItems/
	// ContextCompressionDisabled/ContextDedupDisabled: ajustes de
	// BuildContextPack (feature 015). Mismo patrón que Budget/CompactThreshold
	// (0 → default, negativo → opt-out explícito) para los numéricos, y mismo
	// patrón opt-out `...Disabled` que SpeckitContextDisabled/AtomicPlanDisabled
	// para los booleanos.
	ContextDefaultBudget       int     `json:"context_default_budget,omitempty"`
	ContextMinRelevance        float64 `json:"context_min_relevance,omitempty"`
	ContextMaxItems            int     `json:"context_max_items,omitempty"`
	ContextCompressionDisabled bool    `json:"context_compression_disabled,omitempty"`
	ContextDedupDisabled       bool    `json:"context_dedup_disabled,omitempty"`
	// UsageWindowTokens: ventana de referencia (tokens) para `mem usage`.
	// Ausente/0 = SIN ventana — a diferencia de Budget/ContextDefaultBudget,
	// 0 NO se normaliza a ningún default: ningún valor por defecto puede
	// corresponder a la ventana de un agente concreto (feature 020, FR-014).
	UsageWindowTokens int `json:"usage_window_tokens,omitempty"`
	// ContextIndexMode: emisión de contexto en modo índice. Ausente/false =
	// modo completo, el comportamiento actual (feature 020, FR-034).
	ContextIndexMode bool `json:"context_index_mode,omitempty"`
	// OctopusEnabled activa el módulo Octopus AAR: el enrutador adaptativo que
	// decide si una unidad de trabajo se ejecuta inline o se delega a un
	// subagente (feature 027). Ausente/false = APAGADO.
	//
	// Polaridad en POSITIVO, a diferencia de los ajustes vecinos `...Disabled`:
	// aquellos refinan un flujo que ya existe y debe seguir activo sin que nadie
	// opte por él; Octopus es un flujo nuevo completo y grande, y apagado debe
	// significar huella observable cero (INV-AAR-019). Mismo patrón opt-in que
	// AdrSyncEnabled.
	OctopusEnabled bool `json:"octopus_enabled,omitempty"`
	// --- Octopus AAR: topes y reparto del presupuesto (feature 027) ---
	//
	// Misma semántica de ausencia que Budget/CompactThreshold: 0 o ausente = el
	// valor de fábrica que declara el dominio. No se repiten aquí las cifras;
	// la única fuente son las constantes de domain/octopus_policy.go y
	// domain/octopus_budget.go.
	OctopusMaxSubagents int `json:"octopus_max_subagents,omitempty"`
	OctopusMaxParallel  int `json:"octopus_max_parallel,omitempty"`
	OctopusMaxDepth     int `json:"octopus_max_depth,omitempty"`
	OctopusMaxRetries   int `json:"octopus_max_retries,omitempty"`
	// Reparto porcentual de la sesión. Si los tres no suman 100, el dominio cae
	// al reparto de fábrica en vez de producir un presupuesto roto.
	OctopusMainAgentPct  int `json:"octopus_main_agent_pct,omitempty"`
	OctopusDelegationPct int `json:"octopus_delegation_pct,omitempty"`
	OctopusValidationPct int `json:"octopus_validation_pct,omitempty"`
}

// Defaults de la huella de contexto (feature 008). En CARACTERES emitidos salvo
// DedupWindowDays (días). ~24k chars ≈ 6k tokens; ~48k chars ≈ 12k tokens.
const (
	DefaultBudget           = 24000
	DefaultCompactThreshold = 48000
	DefaultDedupWindowDays  = 7
)

// Defaults del Context Optimization Engine (feature 015). ContextMinRelevance
// en escala 0–1; el resto en tokens/cantidad de items.
const (
	DefaultContextBudget       = 4000
	DefaultContextMinRelevance = 0.65
	DefaultContextMaxItems     = 20
)

func DefaultSettings() Settings {
	return Settings{
		AutoApprove: false,
		// Derivado de domain: son las tools que ApplyAutoApprove escribe como
		// `autoApprove` para Cursor, Windsurf y Cline. Cuando esta lista estaba
		// escrita a mano se quedó sin get_plan_context ni las 5 del grafo, así que
		// esos agentes pedían permiso justo en la acción que debía ser automática.
		// forget_memory queda fuera por destructiva (MCPAutoApprovableTools).
		AutoApproveTools:     domain.MCPAutoApprovableTools(),
		Budget:               DefaultBudget,
		CompactThreshold:     DefaultCompactThreshold,
		DedupWindowDays:      DefaultDedupWindowDays,
		ContextDefaultBudget: DefaultContextBudget,
		ContextMinRelevance:  DefaultContextMinRelevance,
		ContextMaxItems:      DefaultContextMaxItems,

		ReviewMaxFixRounds:      domain.DefaultMaxFixRounds,
		ReviewAutoFixSeverities: defaultReviewAutoFixSeverities(),
	}
}

// defaultReviewAutoFixSeverities deriva la política por defecto del dominio en
// vez de escribir ["CRITICAL","HIGH"] a mano: si mañana cambia qué cuenta como
// severo, este defecto lo sigue solo. Es el mismo motivo por el que
// AutoApproveTools se deriva de domain.MCPAutoApprovableTools().
func defaultReviewAutoFixSeverities() []string {
	severas := []domain.Severity{domain.SeverityCritical, domain.SeverityHigh}
	out := make([]string, 0, len(severas))
	for _, s := range severas {
		out = append(out, string(s))
	}
	return out
}

// applyReviewDefaults normaliza la política de revisión tras leer un
// settings.json que puede no traer las claves nuevas.
//
// Nota deliberada: aquí un valor negativo NO se conserva como opt-out, al
// revés que en applyFootprintDefaults. Allí «sin límite» es una elección
// razonable sobre cuánto contexto emitir; aquí sería una revisión que puede
// corregir indefinidamente sin escalar nunca.
func applyReviewDefaults(s *Settings) {
	if s.ReviewMaxFixRounds <= 0 {
		s.ReviewMaxFixRounds = domain.DefaultMaxFixRounds
	}
	if len(s.ReviewAutoFixSeverities) == 0 {
		s.ReviewAutoFixSeverities = defaultReviewAutoFixSeverities()
	}
}

// applyFootprintDefaults normaliza los tunables de la feature 008 tras leer un
// settings.json que puede no traer las claves nuevas: valor 0 (ausente) toma el
// default; un valor negativo se conserva (opt-out explícito). Así la reducción
// de huella queda activa por defecto para bases previas sin romper el opt-out.
func applyFootprintDefaults(s *Settings) {
	if s.Budget == 0 {
		s.Budget = DefaultBudget
	}
	if s.CompactThreshold == 0 {
		s.CompactThreshold = DefaultCompactThreshold
	}
	if s.DedupWindowDays == 0 {
		s.DedupWindowDays = DefaultDedupWindowDays
	}
}

// applyContextDefaults normaliza los ajustes de BuildContextPack (feature
// 015) tras leer un settings.json que puede no traer las claves nuevas:
// ausente/0 toma el default; negativo se conserva tal cual (opt-out
// explícito que el caso de uso interpreta como "sin filtro"/"sin tope").
func applyContextDefaults(s *Settings) {
	if s.ContextDefaultBudget == 0 {
		s.ContextDefaultBudget = DefaultContextBudget
	}
	if s.ContextMinRelevance == 0 {
		s.ContextMinRelevance = DefaultContextMinRelevance
	}
	if s.ContextMaxItems == 0 {
		s.ContextMaxItems = DefaultContextMaxItems
	}
}

func SettingsPath(root string) string {
	return filepath.Join(root, MemDir, "settings.json")
}

func ReadSettings(root string) Settings {
	path := SettingsPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultSettings()
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings()
	}
	applyFootprintDefaults(&s)
	applyCodeGraphProvidersDefault(&s)
	applyContextDefaults(&s)
	applyReviewDefaults(&s)
	return s
}

// applyCodeGraphProvidersDefault normaliza el campo legado CodeGraphCommand
// (singular) a CodeGraphProviders (lista) cuando esta última no viene
// explícita en el settings.json — así una base existente que solo conoce el
// campo viejo sigue funcionando sin migración manual (feature 010).
func applyCodeGraphProvidersDefault(s *Settings) {
	if len(s.CodeGraphProviders) == 0 && s.CodeGraphCommand != "" {
		s.CodeGraphProviders = []string{s.CodeGraphCommand}
	}
}

func WriteSettings(root string, s Settings) error {
	path := SettingsPath(root)
	if err := EnsureDir(root); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ApplyAutoApprove(root string, s Settings) {
	if !s.AutoApprove || len(s.AutoApproveTools) == 0 {
		return
	}
	tools := s.AutoApproveTools
	setAAP := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}
		ms, ok := cfg["mcpServers"].(map[string]interface{})
		if !ok {
			return
		}
		entry, ok := ms["gomemory"].(map[string]interface{})
		if !ok {
			return
		}
		entry["autoApprove"] = tools
		ms["gomemory"] = entry
		cfg["mcpServers"] = ms
		out, _ := json.MarshalIndent(cfg, "", "  ")
		os.WriteFile(path, out, 0644)
	}
	removeAAP := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}
		ms, ok := cfg["mcpServers"].(map[string]interface{})
		if !ok {
			return
		}
		entry, ok := ms["gomemory"].(map[string]interface{})
		if !ok {
			return
		}
		delete(entry, "autoApprove")
		ms["gomemory"] = entry
		cfg["mcpServers"] = ms
		out, _ := json.MarshalIndent(cfg, "", "  ")
		os.WriteFile(path, out, 0644)
	}

	paths := []string{
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, ".cursor", "mcp.json"),
		filepath.Join(root, ".windsurf", "mcp_config.json"),
		filepath.Join(root, ".cline", "mcp_settings.json"),
	}
	for _, p := range paths {
		if s.AutoApprove {
			setAAP(p)
		} else {
			removeAAP(p)
		}
	}
}

// ReviewPolicy traduce los ajustes del proyecto a la política que consume el dominio.
//
// Existe para que StartReview no tenga que conocer la forma del settings.json ni
// reimplantar defectos: hasta la funcionalidad 028 los reimplantaba a mano y la
// configuración del proyecto no tenía ningún efecto observable (FR-017).
func (s Settings) ReviewPolicy() domain.ReviewPolicy {
	severidades := make([]domain.Severity, 0, len(s.ReviewAutoFixSeverities))
	for _, severidad := range s.ReviewAutoFixSeverities {
		severidades = append(severidades, domain.Severity(severidad))
	}
	politica := domain.ReviewPolicy{
		FixAuthorized:     true,
		MaxFixRounds:      s.ReviewMaxFixRounds,
		AutoFixSeverities: severidades,
	}
	if s.ReviewFixAuthorized != nil {
		politica.FixAuthorized = *s.ReviewFixAuthorized
	}
	return politica
}
