package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsLanguage(t *testing.T) {
	tmp := t.TempDir()

	// Default settings debe tener Language "es"
	s := ReadSettings(tmp)
	if s.Language != "es" {
		t.Errorf("Language default debe ser 'es', se obtuvo %q", s.Language)
	}

	// Guardar idioma "en" y releer
	s.Language = "en"
	if err := WriteSettings(tmp, s); err != nil {
		t.Fatalf("WriteSettings falló: %v", err)
	}

	reloaded := ReadSettings(tmp)
	if reloaded.Language != "en" {
		t.Errorf("Language recargado debe ser 'en', se obtuvo %q", reloaded.Language)
	}

	// Validar que el archivo settings.json contenga "language": "en"
	data, err := os.ReadFile(filepath.Join(tmp, MemDir, "settings.json"))
	if err != nil {
		t.Fatalf("leer settings.json falló: %v", err)
	}
	if !containsString(string(data), `"language": "en"`) {
		t.Errorf("settings.json no contiene \"language\": \"en\"; contenido: %s", string(data))
	}

	// Repositorio
	repo := NewSettingsRepository()
	sData := repo.Read(tmp)
	if sData.Language != "en" {
		t.Errorf("repo.Read Language debe ser 'en', se obtuvo %q", sData.Language)
	}

	sData.Language = "es"
	if err := repo.Write(tmp, sData); err != nil {
		t.Fatalf("repo.Write falló: %v", err)
	}
	reloaded2 := repo.Read(tmp)
	if reloaded2.Language != "es" {
		t.Errorf("repo.Read tras write debe ser 'es', se obtuvo %q", reloaded2.Language)
	}

	// Validación contra códigos soportados: un código desconocido cae a "es".
	SetLanguageValidator(func(code string) bool {
		return code == "es" || code == "en"
	})
	defer SetLanguageValidator(nil)
	sData.Language = "fr-no-soportado"
	if err := repo.Write(tmp, sData); err != nil {
		t.Fatalf("repo.Write falló: %v", err)
	}
	reloaded3 := repo.Read(tmp)
	if reloaded3.Language != "es" {
		t.Errorf("idioma no soportado debe normalizarse a 'es', se obtuvo %q", reloaded3.Language)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && filepath.Base(s) != "" && stringContains(s, substr)))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
