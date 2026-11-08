package cli

import (
	"testing"
)

func TestParseInstallArgsLanguage(t *testing.T) {
	// 1. Probar flag --lang=en
	opts, err := parseInstallArgs([]string{"--lang=en"})
	if err != nil {
		t.Fatalf("parseInstallArgs con --lang=en falló: %v", err)
	}
	if opts.language != "en" || !opts.languageSet {
		t.Errorf("se esperaba opts.language == 'en' y languageSet true, se obtuvo %q, %v", opts.language, opts.languageSet)
	}

	// 2. Probar flag --lang es (espacio)
	opts, err = parseInstallArgs([]string{".", "--lang", "es"})
	if err != nil {
		t.Fatalf("parseInstallArgs con --lang es falló: %v", err)
	}
	if opts.language != "es" || !opts.languageSet {
		t.Errorf("se esperaba opts.language == 'es', se obtuvo %q", opts.language)
	}

	// 3. Probar código de idioma no soportado
	_, err = parseInstallArgs([]string{"--lang=invalido"})
	if err == nil {
		t.Errorf("se esperaba error con código de idioma no soportado")
	}
}
