package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"mem/application/ports"
)

func TestConfigScreen_CompresionApareceYPersiste(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		width:        100,
		height:       50,
		ready:        true,
	}

	view := ansi.Strip(m.configView())
	for _, want := range []string{
		"Compresión de contexto: structural",
		"Comprimir salidas de herramientas: OFF",
		"Respuestas concisas: OFF",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("la vista no contiene %q", want)
		}
	}

	m.configCursor = configRowCompressionLevel
	updated, _ := m.updateConfig(keyMsg("enter"))
	if data.ContextCompressionLevel != "max" {
		t.Fatalf("nivel persistido = %q; se esperaba max", data.ContextCompressionLevel)
	}

	m = updated.(model)
	m.configCursor = configRowToolOutput
	updated, _ = m.updateConfig(keyMsg("enter"))
	if !data.ToolOutputCompression {
		t.Fatal("el interruptor de salidas de herramientas no se persistió")
	}
	if !strings.Contains(updated.(model).statusMsg, "mem install") {
		t.Fatal("activar el hook debe avisar que hay que ejecutar mem install")
	}

	m = updated.(model)
	m.configCursor = configRowConcise
	if _, _ = m.updateConfig(keyMsg("enter")); !data.ConciseOutputDirective {
		t.Fatal("el interruptor de respuestas concisas no se persistió")
	}
}
