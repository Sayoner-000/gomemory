package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// T076 (feature 035, FR-026) + feature 038: donde el host no permite que
// gomemory comprima las salidas de herramientas externas (Codex), el protocolo
// orienta a la recuperación comprimida en origen con precedencia condicional
// por capacidad (domain.CodeRetrievalPolicy). El bloque es uno solo para todos
// los agentes: la orientación es neutral y nombra a Codex como ejemplo, no
// como regla exclusiva.
func TestIntegrationBlock_OrientaALaCompresionEnOrigen(t *testing.T) {
	block := buildIntegrationBlock()
	for _, want := range []string{"recuperación comprimida en origen", "`search_code`", "`get_symbol`", "Codex"} {
		if !strings.Contains(block, want) {
			t.Errorf("el protocolo debe contener %q", want)
		}
	}
}

// T005 (feature 038, FR-001–005): el protocolo establece una precedencia
// condicional por capacidad y suficiencia, no una obligación externa
// incondicional. Los casos vienen de tests/fixtures/038/capability-profiles.json
// (T003). En rojo hasta T007/T008: el bloque v8 exige el grafo externo con
// "úsalo SIEMPRE" y no declara la regla canónica.
func TestIntegrationBlock_PrioridadCondicionalPorCapacidad(t *testing.T) {
	raw, err := os.ReadFile("../../../tests/fixtures/038/capability-profiles.json")
	if err != nil {
		t.Fatalf("falta el fixture T003: %v", err)
	}
	var fixture struct {
		Cases []struct {
			ID                  string   `json:"id"`
			Priority            string   `json:"priority"`
			ForbiddenAssertions []string `json:"forbidden_assertions"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("fixture T003 ilegible: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("el fixture T003 no declara casos")
	}

	block := buildIntegrationBlock()

	// C-004: cada caso del fixture se ejecuta contra el bloque. Las
	// afirmaciones prohibidas son abstractas; esta tabla fija cada una a una
	// marca vetada (debe faltar) o a la salvaguarda del texto canónico que la
	// niega (debe estar). Una afirmación sin mapear falla: no se salta.
	type marca struct{ vetada, salvaguarda string }
	prohibido := map[string]marca{
		"proveedor externo incondicional":   {vetada: "úsalo SIEMPRE para exploración de código en vez de leer archivos a mano"},
		"ahorro por nombre de herramienta":  {salvaguarda: "No infieras ahorro de una herramienta por su nombre"},
		"cuerpos entregados por get_symbol": {salvaguarda: "sin asumir que entrega cuerpos"},
		"cuerpo literal":                    {salvaguarda: "sin asumir que entrega cuerpos"},
		"compresion de get_symbol":          {salvaguarda: "sin asumir que entrega cuerpos o compresión"},
		"origen como equivalente":           {salvaguarda: "Si la cobertura o la evidencia no basta, usa un proveedor externo"},
		"ahorro atribuido":                  {salvaguarda: "No infieras ahorro de una herramienta por su nombre"},
		"ahorro-atribuido":                  {salvaguarda: "No infieras ahorro de una herramienta por su nombre"},
		"grafo propio como cobertura":       {salvaguarda: "usa search_code para búsquedas Go cubiertas"},
		"indice propio como disponible":     {salvaguarda: "cuando cubran el proyecto y la consulta"},
		"excepcion-codex-impuesta":          {salvaguarda: "En agentes con compresión externa compatible conserva su ruta compatible"},
		"lecturas-exactas-como-comprimidas": {salvaguarda: "sin asumir que todas las herramientas o lecturas exactas se comprimen"},
		"compresion-automatica-prometida":   {salvaguarda: "no prometas compresión automática"},
		"resultado-como-comprimido":         {salvaguarda: "sin asumir que todas las herramientas o lecturas exactas se comprimen"},
	}
	exigidoPorCaso := map[string][]string{
		"codex-incompatible-go-cubierto": {"recuperación comprimida en origen disponible y suficiente"},
		"codex-firma-relacion":           {"sin asumir que entrega cuerpos"},
		"codex-cuerpo-requerido":         {"Si la cobertura o la evidencia no basta"},
		"codex-otro-lenguaje":            {"Si la cobertura o la evidencia no basta"},
		"codex-indice-ausente":           {"Si la cobertura o la evidencia no basta"},
		"rewrite-compatible":             {"compresión externa compatible"},
		"capacidad-desconocida":          {"no prometas compresión automática"},
		"origen-sin-ganancia":            {"sin asumir que todas las herramientas o lecturas exactas se comprimen"},
	}
	for _, c := range fixture.Cases {
		exigidos, ok := exigidoPorCaso[c.ID]
		if !ok {
			t.Errorf("caso %q: sin marcas exigidas mapeadas", c.ID)
		}
		for _, want := range exigidos {
			if !strings.Contains(block, want) {
				t.Errorf("caso %q: el protocolo debe declarar %q", c.ID, want)
			}
		}
		for _, f := range c.ForbiddenAssertions {
			m, ok := prohibido[f]
			switch {
			case !ok:
				t.Errorf("caso %q: afirmación prohibida %q sin mapear", c.ID, f)
			case m.vetada != "" && strings.Contains(block, m.vetada):
				t.Errorf("caso %q: el protocolo contiene lo prohibido %q", c.ID, f)
			case m.salvaguarda != "" && !strings.Contains(block, m.salvaguarda):
				t.Errorf("caso %q: falta la salvaguarda contra %q: %q", c.ID, f, m.salvaguarda)
			}
		}
	}
	// La prohibición de inferir ahorro por nombre vive en el propio texto.
	if !strings.Contains(block, "No infieras ahorro de una herramienta por su nombre") {
		t.Errorf("el protocolo debe prohibir inferir ahorro por nombre de herramienta")
	}
}
