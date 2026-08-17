package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeRawSettings escribe un settings.json arbitrario (JSON crudo) para
// simular bases existentes que no conocen los campos nuevos de esta feature.
func writeRawSettings(t *testing.T, root string, raw map[string]any) {
	t.Helper()
	if err := EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(SettingsPath(root), data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestDefaultSettings_CodeGraphEvolutionDefaults(t *testing.T) {
	s := DefaultSettings()
	if s.AdrSyncEnabled {
		t.Error("AdrSyncEnabled debería ser false por defecto (opt-in explícito)")
	}
	if s.CodeImpactAnnotationDisabled {
		t.Error("CodeImpactAnnotationDisabled debería ser false por defecto (la anotación queda activa)")
	}
	if len(s.CodeGraphProviders) != 0 {
		t.Errorf("CodeGraphProviders debería estar vacío por defecto, tiene %v", s.CodeGraphProviders)
	}
}

func TestReadSettings_MissingFile_UsesDefaults(t *testing.T) {
	root := t.TempDir()
	s := ReadSettings(root)
	want := DefaultSettings()
	if s.AdrSyncEnabled != want.AdrSyncEnabled || s.CodeImpactAnnotationDisabled != want.CodeImpactAnnotationDisabled {
		t.Fatalf("sin settings.json, se esperaban los defaults: %+v, hubo: %+v", want, s)
	}
}

// Retrocompatibilidad: una base existente que solo conoce code_graph_command
// (singular, campo legado) debe seguir funcionando — ReadSettings normaliza
// ese valor a una lista de un elemento en CodeGraphProviders.
func TestReadSettings_LegacyCodeGraphCommand_NormalizesToProvidersList(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{
		"code_graph_command": "mi-binario-legado",
	})

	s := ReadSettings(root)

	want := []string{"mi-binario-legado"}
	if !reflect.DeepEqual(s.CodeGraphProviders, want) {
		t.Fatalf("CodeGraphProviders = %v, se esperaba %v (normalizado desde code_graph_command)", s.CodeGraphProviders, want)
	}
	// El campo legado se conserva tal cual para no romper lectores viejos.
	if s.CodeGraphCommand != "mi-binario-legado" {
		t.Errorf("CodeGraphCommand no debería perderse: %q", s.CodeGraphCommand)
	}
}

// Si code_graph_providers ya viene poblado explícitamente, NO se pisa con el
// legado (aunque ambos estén presentes) — la lista explícita manda.
func TestReadSettings_ExplicitProvidersList_TakesPrecedenceOverLegacy(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{
		"code_graph_command":   "legado-ignorado",
		"code_graph_providers": []string{"cmd-a", "cmd-b"},
	})

	s := ReadSettings(root)

	want := []string{"cmd-a", "cmd-b"}
	if !reflect.DeepEqual(s.CodeGraphProviders, want) {
		t.Fatalf("CodeGraphProviders = %v, se esperaba %v (la lista explícita no debe pisarse)", s.CodeGraphProviders, want)
	}
}

// Ni code_graph_command ni code_graph_providers presentes: la lista queda
// vacía (autodetección en PATH, comportamiento ya existente).
func TestReadSettings_NoProviderConfigured_EmptyList(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{
		"auto_approve": true,
	})

	s := ReadSettings(root)

	if len(s.CodeGraphProviders) != 0 {
		t.Errorf("CodeGraphProviders debería quedar vacío sin configuración, hubo %v", s.CodeGraphProviders)
	}
}

// adr_sync_enabled y code_impact_annotation_disabled son bool simples: deben
// preservar el valor explícito del JSON (incluido "true" para el disabled,
// el caso que probaría un default mal implementado con puntero olvidado).
func TestReadSettings_PreservesExplicitBooleans(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{
		"adr_sync_enabled":                true,
		"code_impact_annotation_disabled": true,
	})

	s := ReadSettings(root)

	if !s.AdrSyncEnabled {
		t.Error("adr_sync_enabled=true explícito debería preservarse")
	}
	if !s.CodeImpactAnnotationDisabled {
		t.Error("code_impact_annotation_disabled=true explícito debería preservarse")
	}
}

// speckit_context_disabled (feature 011) sigue el mismo patrón opt-out que
// code_graph_disabled: ausente en el JSON ⇒ activado (false); explícito en
// true ⇒ se preserva. Se prueba tanto leyendo un JSON crudo como el
// roundtrip completo WriteSettings/ReadSettings, porque el gate real (el
// script del hook de la extensión gomemory-context) lee este campo
// directo del archivo, sin pasar por mem settings.
func TestReadSettings_SpeckitContextDisabled_AbsentDefaultsToEnabled(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{})

	s := ReadSettings(root)

	if s.SpeckitContextDisabled {
		t.Error("speckit_context_disabled ausente debería resultar en activado (false)")
	}
}

func TestReadSettings_SpeckitContextDisabled_ExplicitTruePreserved(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{
		"speckit_context_disabled": true,
	})

	s := ReadSettings(root)

	if !s.SpeckitContextDisabled {
		t.Error("speckit_context_disabled=true explícito debería preservarse")
	}
}

func TestWriteReadSettings_SpeckitContextDisabled_Roundtrip(t *testing.T) {
	root := t.TempDir()
	s := DefaultSettings()
	s.SpeckitContextDisabled = true

	if err := WriteSettings(root, s); err != nil {
		t.Fatalf("WriteSettings: %v", err)
	}

	got := ReadSettings(root)
	if !got.SpeckitContextDisabled {
		t.Error("SpeckitContextDisabled=true debería sobrevivir un roundtrip write/read")
	}
}

func TestSettingsPath_UnusedImportsGuard(t *testing.T) {
	// Solo para dejar filepath usado si algún linter se queja del import;
	// SettingsPath ya se ejercita indirectamente arriba.
	if filepath.Base(SettingsPath("/tmp/x")) != "settings.json" {
		t.Fatal("SettingsPath debería terminar en settings.json")
	}
}

// --- Feature 013: modo plan atómico ---

// TestReadSettings_AtomicPlanDisabled_AusenteEsFalse cubre la retrocompatibilidad
// exigida por FR-025: un settings.json escrito antes de esta feature no tiene la
// clave, y debe deserializar a false — es decir, la planificación atómica queda
// ACTIVA sin que nadie tenga que optar por ella. Es el mismo patrón opt-out que
// SpeckitContextDisabled y SynapseDisabled.
func TestReadSettings_AtomicPlanDisabled_AusenteEsFalse(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{"auto_approve": false})

	if got := ReadSettings(root).AtomicPlanDisabled; got {
		t.Errorf("AtomicPlanDisabled con la clave ausente = %v, se esperaba false (funcionalidad activa)", got)
	}
}

func TestReadSettings_AtomicPlanDisabled_TrueSeRespeta(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{"atomic_plan_disabled": true})

	if got := ReadSettings(root).AtomicPlanDisabled; !got {
		t.Errorf("AtomicPlanDisabled = %v, se esperaba true", got)
	}
}

// TestSettingsRepository_AtomicPlanDisabled_RoundTrip verifica que el campo
// sobrevive el mapeo persistence.Settings <-> ports.SettingsData en ambos
// sentidos. Sin esto, el ajuste se leería bien pero se perdería al guardarlo
// desde la TUI (que escribe vía el repositorio, no vía WriteSettings).
func TestSettingsRepository_AtomicPlanDisabled_RoundTrip(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()

	s := repo.Read(root)
	s.AtomicPlanDisabled = true
	if err := repo.Write(root, s); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := repo.Read(root).AtomicPlanDisabled; !got {
		t.Errorf("tras el round-trip AtomicPlanDisabled = %v, se esperaba true", got)
	}
}

// --- Feature 015: Context Optimization & Budgeting ---

func TestReadSettings_ContextDefaults_AbsentUsesFactoryDefaults(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{"auto_approve": false})

	s := ReadSettings(root)

	if s.ContextDefaultBudget != DefaultContextBudget {
		t.Errorf("ContextDefaultBudget = %d, se esperaba el default %d", s.ContextDefaultBudget, DefaultContextBudget)
	}
	if s.ContextMinRelevance != DefaultContextMinRelevance {
		t.Errorf("ContextMinRelevance = %v, se esperaba el default %v", s.ContextMinRelevance, DefaultContextMinRelevance)
	}
	if s.ContextMaxItems != DefaultContextMaxItems {
		t.Errorf("ContextMaxItems = %d, se esperaba el default %d", s.ContextMaxItems, DefaultContextMaxItems)
	}
	if s.ContextCompressionDisabled {
		t.Error("ContextCompressionDisabled debería ser false por defecto (compresión activa)")
	}
	if s.ContextDedupDisabled {
		t.Error("ContextDedupDisabled debería ser false por defecto (dedup activo)")
	}
}

// Un valor negativo es un opt-out explícito (sin filtro/sin tope) y NO debe
// pisarse con el default — mismo criterio que Budget/CompactThreshold.
func TestReadSettings_ContextDefaults_NegativeIsExplicitOptOutNotOverwritten(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{
		"context_min_relevance": -1,
		"context_max_items":     -1,
	})

	s := ReadSettings(root)

	if s.ContextMinRelevance != -1 {
		t.Errorf("ContextMinRelevance = %v, un valor negativo explícito no debería pisarse con el default", s.ContextMinRelevance)
	}
	if s.ContextMaxItems != -1 {
		t.Errorf("ContextMaxItems = %d, un valor negativo explícito no debería pisarse con el default", s.ContextMaxItems)
	}
}

func TestSettingsRepository_ContextFields_RoundTrip(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()

	s := repo.Read(root)
	s.ContextDefaultBudget = 8000
	s.ContextMinRelevance = 0.5
	s.ContextMaxItems = 30
	s.ContextCompressionDisabled = true
	s.ContextDedupDisabled = true
	if err := repo.Write(root, s); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := repo.Read(root)
	if got.ContextDefaultBudget != 8000 {
		t.Errorf("ContextDefaultBudget tras roundtrip = %d, se esperaba 8000", got.ContextDefaultBudget)
	}
	if got.ContextMinRelevance != 0.5 {
		t.Errorf("ContextMinRelevance tras roundtrip = %v, se esperaba 0.5", got.ContextMinRelevance)
	}
	if got.ContextMaxItems != 30 {
		t.Errorf("ContextMaxItems tras roundtrip = %d, se esperaba 30", got.ContextMaxItems)
	}
	if !got.ContextCompressionDisabled {
		t.Error("ContextCompressionDisabled=true debería sobrevivir un roundtrip write/read")
	}
	if !got.ContextDedupDisabled {
		t.Error("ContextDedupDisabled=true debería sobrevivir un roundtrip write/read")
	}
}

// --- Feature 019: activación determinista del modo plan atómico ---

// TestReadSettings_PlanGuardDisabled_AusenteEsFalse cubre la retrocompatibilidad:
// un settings.json escrito antes de esta feature no tiene la clave, y debe
// deserializar a false — la exigencia de forma del plan queda ACTIVA sin que
// nadie tenga que optar por ella. Mismo patrón opt-out que AtomicPlanDisabled.
func TestReadSettings_PlanGuardDisabled_AusenteEsFalse(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{"auto_approve": false})

	if got := ReadSettings(root).PlanGuardDisabled; got {
		t.Errorf("PlanGuardDisabled con la clave ausente = %v, se esperaba false (funcionalidad activa)", got)
	}
}

func TestReadSettings_PlanGuardDisabled_TrueSeRespeta(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{"plan_guard_disabled": true})

	if got := ReadSettings(root).PlanGuardDisabled; !got {
		t.Errorf("PlanGuardDisabled = %v, se esperaba true", got)
	}
}

// TestSettingsRepository_PlanGuardDisabled_RoundTrip verifica que el campo
// sobrevive el mapeo persistence.Settings <-> ports.SettingsData en ambos
// sentidos. Sin esto, el ajuste se leería bien pero se perdería al guardarlo
// desde la TUI (que escribe vía el repositorio, no vía WriteSettings).
func TestSettingsRepository_PlanGuardDisabled_RoundTrip(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()

	s := repo.Read(root)
	s.PlanGuardDisabled = true
	if err := repo.Write(root, s); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := repo.Read(root).PlanGuardDisabled; !got {
		t.Errorf("tras el round-trip PlanGuardDisabled = %v, se esperaba true", got)
	}
}
