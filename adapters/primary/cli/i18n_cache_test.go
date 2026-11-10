package cli

import (
	"os"
	"testing"
)

// Un catálogo vacío no se cachea: si la primera llamada llega sin plantillas
// (un test con el cwd en un directorio temporal y sin TemplatesFS), las
// siguientes deben poder cargar los idiomas. Con el vacío en caché, la consola
// de install dejaba de preguntar el idioma según el orden de los tests (CI de
// Windows, v2.54.0).
func TestGetCatalog_NoCacheaUnCatalogoVacio(t *testing.T) {
	appCatalogMu.Lock()
	prevCat := appCatalog
	appCatalog = nil
	appCatalogMu.Unlock()
	prevFS := TemplatesFS
	TemplatesFS = nil
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		TemplatesFS = prevFS
		appCatalogMu.Lock()
		appCatalog = prevCat
		appCatalogMu.Unlock()
	})

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if n := len(getCatalog().AvailableLocales()); n != 0 {
		t.Fatalf("sin plantillas a la vista no debería haber idiomas: %d", n)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	if n := len(getCatalog().AvailableLocales()); n < 2 {
		t.Fatalf("tras volver al paquete, el catálogo debe cargar es y en: %d idiomas", n)
	}
}
