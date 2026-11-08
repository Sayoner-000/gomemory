package cli

import (
	"os"
	"sync"

	secI18n "mem/adapters/secondary/i18n"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
)

var (
	appCatalog   *secI18n.Catalog
	appCatalogMu sync.Mutex
)

// GetCatalog devuelve el catálogo de localización activo (inicializándolo bajo
// demanda desde TemplatesFS o el directorio de plantillas en pruebas).
func GetCatalog() *secI18n.Catalog {
	return getCatalog()
}

func getCatalog() *secI18n.Catalog {
	appCatalogMu.Lock()
	defer appCatalogMu.Unlock()
	if appCatalog != nil {
		return appCatalog
	}

	initCat := func(cat *secI18n.Catalog) *secI18n.Catalog {
		appCatalog = cat
		persistence.SetLanguageValidator(cat.Supports)
		return appCatalog
	}

	if TemplatesFS != nil {
		cat, err := secI18n.NewCatalog(TemplatesFS, "templates/locales")
		if err == nil && len(cat.AvailableLocales()) > 0 {
			return initCat(cat)
		}
		cat, err = secI18n.NewCatalog(TemplatesFS, "locales")
		if err == nil && len(cat.AvailableLocales()) > 0 {
			return initCat(cat)
		}
	}

	// Fallback para pruebas que no inicializan TemplatesFS
	for _, dir := range []string{
		"infrastructure/templates",
		"../../../infrastructure/templates",
		"../../infrastructure/templates",
		"../infrastructure/templates",
	} {
		if _, err := os.Stat(dir); err == nil {
			cat, err := secI18n.NewCatalog(os.DirFS(dir), "locales")
			if err == nil && len(cat.AvailableLocales()) > 0 {
				return initCat(cat)
			}
		}
	}

	// Vacío por defecto
	cat, _ := secI18n.NewCatalog(nil, "")
	appCatalog = cat
	return appCatalog
}

// contextBuilderForLang devuelve un ContextBuilder aislado para esta llamada
// con el idioma y catálogo indicados, sin mutar la instancia compartida en Deps.
func contextBuilderForLang(deps *Deps, lang string) ports.ContextBuilder {
	if deps == nil || deps.ContextBuilder == nil {
		return nil
	}
	if b, ok := deps.ContextBuilder.(*usecases.Builder); ok {
		cp := b.WithLanguage(lang)
		if cp.Catalog == nil {
			cp.Catalog = getCatalog()
		}
		return cp
	}
	return deps.ContextBuilder
}
