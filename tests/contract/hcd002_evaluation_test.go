package main_test

import (
	"os/exec"
	"testing"
)

// TestHCD002EvaluationCapture ejecuta el self-test del ejecutor de HCD-002:
// contabilidad UTF-8, p95 por rango más cercano, fallos conservados, literales
// obligatorios, búsqueda del id a recuperar y frontera del modelo no verificada.
func TestHCD002EvaluationCapture(t *testing.T) {
	cmd := exec.Command("python3", "../../scripts/hcd002_evaluate.py", "--self-test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("captura no conforme: %v\n%s", err, output)
	}
}
