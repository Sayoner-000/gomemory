package domain

import "testing"

// El catálogo cerrado de razones es lo que hace explicable el enrutamiento sin
// exponer razonamiento privado (FR-007, INV-AAR-013). Una razón sin texto sería
// una decisión sin explicación; dos códigos con el mismo texto harían
// indistinguibles dos políticas distintas.
func TestReason_CatalogoCerrado(t *testing.T) {
	razones := AllReasons()
	if len(razones) == 0 {
		t.Fatal("el catálogo de razones no puede estar vacío")
	}

	vistosCodigo := map[Reason]bool{}
	vistosTexto := map[string]Reason{}

	for _, r := range razones {
		if vistosCodigo[r] {
			t.Errorf("código de razón duplicado: %q", r)
		}
		vistosCodigo[r] = true

		texto := r.Text()
		if texto == "" {
			t.Errorf("la razón %q no tiene texto", r)
		}
		if previa, ok := vistosTexto[texto]; ok {
			t.Errorf("las razones %q y %q comparten el mismo texto", previa, r)
		}
		vistosTexto[texto] = r
	}
}

// Una razón fuera del catálogo no puede colarse con texto inventado: Text()
// devuelve cadena vacía, y la prueba de la política exige razón no vacía.
func TestReason_FueraDelCatalogo(t *testing.T) {
	if Reason("inventada").Text() != "" {
		t.Error("una razón fuera del catálogo no debería tener texto")
	}
}

func TestRoute_Valores(t *testing.T) {
	todas := AllRoutes()
	esperadas := map[Route]bool{
		RouteInline:   true,
		RouteDelegate: true,
		RouteParallel: true,
		RouteWait:     true,
		RouteReject:   true,
	}
	if len(todas) != len(esperadas) {
		t.Fatalf("AllRoutes devuelve %d rutas, esperaba %d", len(todas), len(esperadas))
	}
	for _, r := range todas {
		if !esperadas[r] {
			t.Errorf("ruta inesperada en el catálogo: %q", r)
		}
	}
}

// Delegada agrupa DELEGATE y PARALLEL: las dos implican un subagente y ambas
// deben pasar por las validaciones de presupuesto, contrato y permisos.
func TestRoute_Delegada(t *testing.T) {
	casos := map[Route]bool{
		RouteInline:   false,
		RouteDelegate: true,
		RouteParallel: true,
		RouteWait:     false,
		RouteReject:   false,
	}
	for r, want := range casos {
		if got := r.Delegada(); got != want {
			t.Errorf("%q.Delegada() = %v, esperaba %v", r, got, want)
		}
	}
}
