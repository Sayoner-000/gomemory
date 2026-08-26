package cli

import (
	"strings"
	"testing"
)

// TestFormatCheckpoint_AcotaComandosLargos cubre la causa raíz medida en
// producción: el límite de 5 comandos por turno no acotaba el LARGO de cada
// uno, así que un heredoc con un archivo de test entero se persistía literal.
// El checkpoint más grande del proyecto medía 25 478 caracteres, más que el
// presupuesto completo del contexto.
func TestFormatCheckpoint_AcotaComandosLargos(t *testing.T) {
	heredoc := "cat > archivo_test.go <<'EOF'\n" + strings.Repeat("linea de un archivo Go completo\n", 640)

	got := formatCheckpoint(turnActivity{
		Files:    []string{"adapters/primary/cli/cmd_hook.go"},
		Commands: []string{heredoc},
	})

	if len(got) > 2000 {
		t.Fatalf("el checkpoint debe quedar acotado, mide %d caracteres", len(got))
	}
	if !strings.Contains(got, "cat > archivo_test.go") {
		t.Fatalf("el inicio del comando debe conservarse, got: %q", got)
	}
	if !strings.Contains(got, "caracteres omitidos") {
		t.Fatalf("lo omitido debe declararse, nunca recortarse en silencio, got: %q", got)
	}
	if !strings.Contains(got, "Editó: adapters/primary/cli/cmd_hook.go") {
		t.Fatalf("la lista de archivos editados no debe verse afectada, got: %q", got)
	}
}

// TestFormatCheckpoint_ComandoCortoIntacto: acotar no puede degradar el caso
// normal, que es la inmensa mayoría de los turnos.
func TestFormatCheckpoint_ComandoCortoIntacto(t *testing.T) {
	got := formatCheckpoint(turnActivity{Commands: []string{"go test ./...", "git status"}})

	if got != "Comandos: go test ./...; git status" {
		t.Fatalf("un comando corto debe llegar íntegro y sin marcas, got: %q", got)
	}
}
