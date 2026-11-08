package i18n

import (
	"os"
	"testing"
)

func TestI18nDiscovery(t *testing.T) {
	// Probar contra el sistema de archivos real de infrastructure/templates/locales
	fsys := os.DirFS("../../../infrastructure/templates")
	cat, err := NewCatalog(fsys, "locales")
	if err != nil {
		t.Fatalf("NewCatalog falló: %v", err)
	}

	locales := cat.AvailableLocales()
	if len(locales) < 2 {
		t.Fatalf("se esperaban al menos 2 idiomas (es, en), se obtuvieron %d", len(locales))
	}

	if !cat.Supports("es") {
		t.Errorf("catálogo debe soportar 'es'")
	}
	if !cat.Supports("en") {
		t.Errorf("catálogo debe soportar 'en'")
	}
	if cat.Supports("fr") {
		t.Errorf("catálogo no debe soportar 'fr' aún")
	}

	def := cat.DefaultLocale()
	if def.Code != "es" {
		t.Errorf("default debe ser 'es', se obtuvo %s", def.Code)
	}

	// Verificar resolución de mensajes
	esSwitched := cat.Get("es", "lang_switched", map[string]string{"name": "Español"})
	if esSwitched != "Idioma cambiado a Español" {
		t.Errorf("resultado inesperado en es: %s", esSwitched)
	}

	enSwitched := cat.Get("en", "lang_switched", map[string]string{"name": "English"})
	if enSwitched != "Language switched to English" {
		t.Errorf("resultado inesperado en en: %s", enSwitched)
	}

	// Fallback de clave no traducida en un idioma
	fallback := cat.Get("en", "clave_inexistente")
	if fallback != "clave_inexistente" {
		t.Errorf("fallback de clave inexistente debe devolver la clave, se obtuvo: %s", fallback)
	}
}

func TestParidad(t *testing.T) {
	fsys := os.DirFS("../../../infrastructure/templates")
	cat, err := NewCatalog(fsys, "locales")
	if err != nil {
		t.Fatalf("NewCatalog falló: %v", err)
	}

	esMsgs := cat.messages["es"]
	enMsgs := cat.messages["en"]

	if len(esMsgs) == 0 {
		t.Fatal("es no tiene mensajes cargados")
	}
	if len(enMsgs) == 0 {
		t.Fatal("en no tiene mensajes cargados")
	}

	// Todas las claves de es deben estar en en
	for k := range esMsgs {
		if _, ok := enMsgs[k]; !ok {
			t.Errorf("clave %q presente en 'es' pero ausente en 'en'", k)
		}
	}

	// Todas las claves de en deben estar en es
	for k := range enMsgs {
		if _, ok := esMsgs[k]; !ok {
			t.Errorf("clave %q presente en 'en' pero ausente en 'es'", k)
		}
	}

	// Verificar paridad de plantillas
	esTmpls := cat.templates["es"]
	enTmpls := cat.templates["en"]

	for name := range esTmpls {
		if _, ok := enTmpls[name]; !ok {
			t.Errorf("plantilla %q presente en 'es' pero ausente en 'en'", name)
		}
	}
}
