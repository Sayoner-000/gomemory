package cli

import (
	"strings"
	"testing"

	"mem/application/ports"
)

func TestApplyCompressionLevel(t *testing.T) {
	var s ports.SettingsData
	if err := applyCompressionLevel(&s, " MAX "); err != nil || s.ContextCompressionLevel != "max" {
		t.Fatalf("max válido rechazado: %v %q", err, s.ContextCompressionLevel)
	}
	if err := applyCompressionLevel(&s, "maxima"); err == nil {
		t.Fatal("un nivel inválido debe rechazarse")
	}
	if s.ContextCompressionLevel != "max" {
		t.Error("un valor rechazado no debe tocar el ajuste")
	}
}

func TestFormatCompressionSettings(t *testing.T) {
	cases := []struct {
		in   ports.SettingsData
		want string
	}{
		{ports.SettingsData{}, "Compresión de contexto: structural (por defecto)"},
		{ports.SettingsData{ContextCompressionDisabled: true}, "Compresión de contexto: none (heredado"},
		{ports.SettingsData{ContextCompressionLevel: "max", ToolOutputCompression: true}, "Compresión de contexto: max (ajuste)"},
	}
	for _, c := range cases {
		if got := formatCompressionSettings(c.in); !strings.Contains(got, c.want) {
			t.Errorf("salida %q no contiene %q", got, c.want)
		}
	}
	if got := formatCompressionSettings(ports.SettingsData{ToolOutputCompression: true}); !strings.Contains(got, "salidas de herramientas: true") {
		t.Errorf("falta el interruptor de salidas: %q", got)
	}
}
