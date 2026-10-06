package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNativeInstallFlowKeepsStepsWarningsAndFits(t *testing.T) {
	for _, width := range []int{40, 80, 120} {
		var out bytes.Buffer
		e := Env{StdoutTTY: true, Width: width, Getenv: func(key string) string {
			if key == "GOMEMORY_NO_MOTION" {
				return "1"
			}
			return ""
		}}
		flow := NewFlow(&out, e, "Instalar")
		flow.Progress("Configurando el proyecto con un nombre largo")
		flow.Done(StepResult{Name: "Memoria", Status: StepOK})
		flow.Done(StepResult{Name: "Integración OpenCode", Status: StepWarn, Detail: "No se pudo configurar", Manual: "mem setup-mcp --agents opencode"})
		flow.End("ok")
		text := ansi.Strip(out.String())
		for _, want := range []string{"┌", "│", "✓ Memoria", "⚠ Integración", "mem setup-mcp", "└", "avisos"} {
			if !strings.Contains(text, want) {
				t.Errorf("flujo sin %q: %q", want, text)
			}
		}
		if strings.Contains(text, "goMemory listo") {
			t.Fatal("aviso oculto por cierre exitoso")
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("flujo desbordado %d: %q", width, line)
			}
		}
	}
}

func TestNativePromptsHaveConnectedStates(t *testing.T) {
	m := newMultiSelectModel("Agentes", []Option{{Value: "opencode", Label: "OpenCode", Checked: true}})
	if text := ansi.Strip(m.View().Content); !strings.Contains(text, "│") || !strings.Contains(text, "◆") {
		t.Fatalf("pregunta desconectada: %q", text)
	}
	m.done = true
	if text := ansi.Strip(m.View().Content); !strings.Contains(text, "◇") || !strings.Contains(text, "OpenCode") {
		t.Fatalf("respuesta sin cierre: %q", text)
	}
	m.canceled = true
	if text := ansi.Strip(m.View().Content); !strings.Contains(text, "Cancelado") {
		t.Fatalf("cancelación confundida con respuesta: %q", text)
	}
}
