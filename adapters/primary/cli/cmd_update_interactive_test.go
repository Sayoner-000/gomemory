package cli

import (
	"bytes"
	"strings"
	"testing"

	"mem/adapters/primary/console"
)

// FR-025: update muestra Actual → Disponible y pide confirmación con TTY.
func TestConfirmUpdate(t *testing.T) {
	var out bytes.Buffer
	ok, err := confirmUpdate(console.NewPlain(strings.NewReader("s\n"), &out), "v2.26.4", "v2.27.0", "/home/x/.local/bin/mem")
	if err != nil || !ok {
		t.Fatalf("confirmUpdate = %v, %v", ok, err)
	}
	if !strings.Contains(out.String(), "v2.26.4 → v2.27.0") || !strings.Contains(out.String(), "/home/x/.local/bin/mem") {
		t.Errorf("salida:\n%s", out.String())
	}
	ok, _ = confirmUpdate(console.NewPlain(strings.NewReader("n\n"), &bytes.Buffer{}), "v2.26.4", "v2.27.0", "/x")
	if ok {
		t.Error("n cancela la actualización")
	}
}

// Sin UI (--yes o sin TTY) no se pregunta.
func TestConfirmUpdate_SinUI(t *testing.T) {
	if ok, err := confirmUpdate(nil, "v1.0.0", "v2.0.0", "/x"); !ok || err != nil {
		t.Fatalf("sin UI se continúa: %v %v", ok, err)
	}
}
