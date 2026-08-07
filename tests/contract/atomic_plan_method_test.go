package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func atomicPlanMethod(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	p := filepath.Join(wd, "..", "..", "infrastructure", "templates", "atomic-plan-method.md")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("leer atomic-plan-method.md: %v", err)
	}
	return string(data)
}

// TestAtomicPlanMethod_CubreLaLineaBase verifica que el método conserva lo que
// la versión optimizada del usuario ya resolvía (reference-ads-baseline.md):
// test de atomicidad, procedimiento, notación de dependencias y formato de
// árbol. Sin esto, una edición futura podría vaciar el método sin que nada
// falle.
func TestAtomicPlanMethod_CubreLaLineaBase(t *testing.T) {
	c := atomicPlanMethod(t)

	obligatorios := map[string]string{
		"test de atomicidad":     "atómica",
		"verbo de acción":        "verbo",
		"resultado verificable":  "verificable",
		"límite de profundidad":  "6 niveles",
		"umbral de priorización": "25",
		"notación de dependencia": "dep:",
		"marca de hoja atómica":  "✓",
	}
	for nombre, aguja := range obligatorios {
		if !strings.Contains(c, aguja) {
			t.Errorf("el método debe cubrir %s (falta %q)", nombre, aguja)
		}
	}
}

// TestAtomicPlanMethod_UsaElHistorial cubre FR-016: el método debe apoyarse en
// el historial del proyecto al descomponer. Es la adición que distingue este
// método del documento ADS genérico — sin ella, la Historia 1 carga contexto
// que nadie usa.
func TestAtomicPlanMethod_UsaElHistorial(t *testing.T) {
	c := strings.ToLower(atomicPlanMethod(t))

	for _, aguja := range []string{"historial", "decision", "convenci", "causa ra"} {
		if !strings.Contains(c, aguja) {
			t.Errorf("FR-016: el método debe instruir a usar el historial del proyecto (falta %q)", aguja)
		}
	}
}

// TestAtomicPlanMethod_Autoverificacion cubre FR-018: el agente contrasta cada
// hoja contra el test de atomicidad ANTES de entregar el plan. Es la decisión
// D5 de la especificación: autovalidación, sin compuerta externa.
func TestAtomicPlanMethod_Autoverificacion(t *testing.T) {
	c := strings.ToLower(atomicPlanMethod(t))

	if !strings.Contains(c, "autoverificaci") {
		t.Error("FR-018: el método debe incluir un paso de autoverificación previo a la entrega")
	}
	if !strings.Contains(c, "antes de") {
		t.Error("FR-018: la autoverificación debe ocurrir ANTES de presentar el plan")
	}
}

// TestAtomicPlanMethod_MarcadoDeNoAtomica cubre FR-019: una hoja que no puede
// atomizarse se entrega marcada con su motivo, sin bloquear el plan completo.
func TestAtomicPlanMethod_MarcadoDeNoAtomica(t *testing.T) {
	c := strings.ToLower(atomicPlanMethod(t))

	if !strings.Contains(c, "no atómica") && !strings.Contains(c, "no atomica") {
		t.Error("FR-019: el método debe explicar cómo marcar una hoja no atómica")
	}
	if !strings.Contains(c, "motivo") {
		t.Error("FR-019: el marcado debe exigir declarar el motivo")
	}
}

// TestAtomicPlanMethod_SeDetieneEnModoPlan cubre FR-020 y FR-021: en modo plan
// el método entrega el árbol y se detiene. La ejecución sigue siendo
// responsabilidad de /speckit-implement, no de esta feature.
func TestAtomicPlanMethod_SeDetieneEnModoPlan(t *testing.T) {
	c := strings.ToLower(atomicPlanMethod(t))

	if !strings.Contains(c, "modo plan") {
		t.Error("FR-020: el método debe distinguir explícitamente el modo plan")
	}
	if !strings.Contains(c, "no ejecutes") && !strings.Contains(c, "det") {
		t.Error("FR-020/FR-021: en modo plan el método debe ordenar entregar y detenerse")
	}
}

// TestAtomicPlanMethod_EsBreve protege la decisión D5: el método viaja por la
// llamada, no por el bloque de protocolo, precisamente para no inflar la huella
// de contexto. Si crece sin control, esa ventaja se pierde.
func TestAtomicPlanMethod_EsBreve(t *testing.T) {
	if n := len(atomicPlanMethod(t)); n > 6000 {
		t.Errorf("el método ocupa %d caracteres; se esperaba <= 6000 para no inflar el contexto de planificación", n)
	}
}
