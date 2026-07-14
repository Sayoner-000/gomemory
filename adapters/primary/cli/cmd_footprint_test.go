package cli

import (
	"strings"
	"testing"
)

func TestFootprint_AcumulaYResetea(t *testing.T) {
	root := t.TempDir()
	if got := footprintRead(root); got != 0 {
		t.Fatalf("huella inicial debe ser 0, got %d", got)
	}
	footprintAdd(root, 100)
	footprintAdd(root, 50)
	if got := footprintRead(root); got != 150 {
		t.Fatalf("esperaba 150 acumulado, got %d", got)
	}
	footprintReset(root)
	if got := footprintRead(root); got != 0 {
		t.Fatalf("tras reset debe ser 0, got %d", got)
	}
}

func TestComputeCompactNudge_UmbralYDebounce(t *testing.T) {
	// Bajo el umbral: silencio.
	root := t.TempDir()
	footprintAdd(root, 100)
	if _, ok := computeCompactNudge(root, 48000); ok {
		t.Fatal("bajo el umbral no debe recordar")
	}

	// threshold<=0: desactivado aun con huella enorme.
	root2 := t.TempDir()
	footprintAdd(root2, 100000)
	if _, ok := computeCompactNudge(root2, 0); ok {
		t.Fatal("threshold<=0 debe desactivar el recordatorio")
	}

	// Sobre el umbral: recuerda una vez, con mensaje neutral (sin comando de cliente).
	root3 := t.TempDir()
	footprintAdd(root3, 60000)
	msg, ok := computeCompactNudge(root3, 48000)
	if !ok {
		t.Fatal("sobre el umbral debe recordar")
	}
	if strings.Contains(msg, "/compact") || strings.Contains(strings.ToLower(msg), "/clear") {
		t.Fatalf("el recordatorio NO debe nombrar un comando de cliente: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "compact") { // "compactar" en el texto neutral
		t.Fatalf("el recordatorio debería sugerir compactar el contexto: %q", msg)
	}

	// Segundo turno inmediato: debounce → silencio.
	if _, ok := computeCompactNudge(root3, 48000); ok {
		t.Fatal("debounce: no debe repetir de inmediato")
	}
}
