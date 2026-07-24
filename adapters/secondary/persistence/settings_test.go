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
		"adr_sync_enabled":                 true,
		"code_impact_annotation_disabled":  true,
	})

	s := ReadSettings(root)

	if !s.AdrSyncEnabled {
		t.Error("adr_sync_enabled=true explícito debería preservarse")
	}
	if !s.CodeImpactAnnotationDisabled {
		t.Error("code_impact_annotation_disabled=true explícito debería preservarse")
	}
}

func TestSettingsPath_UnusedImportsGuard(t *testing.T) {
	// Solo para dejar filepath usado si algún linter se queja del import;
	// SettingsPath ya se ejercita indirectamente arriba.
	if filepath.Base(SettingsPath("/tmp/x")) != "settings.json" {
		t.Fatal("SettingsPath debería terminar en settings.json")
	}
}
