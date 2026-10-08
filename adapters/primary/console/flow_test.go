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

func TestFlow_LineaConPlanCierraConConteosYSiguiente(t *testing.T) {
	var out bytes.Buffer
	flow := NewFlow(&out, plainEnv(), "Indexar")
	flow.Plan("Código Go", "codegraph", "codebase-memory-mcp")
	flow.Next("mem context")
	flow.Done(StepResult{Name: "Código Go", Meta: "3 nodos · 1ms"})
	flow.Done(StepResult{Name: "codegraph", Status: StepSkip, Detail: "no instalado"})
	flow.Done(StepResult{Name: "codebase-memory-mcp", Status: StepWarn, Detail: "falló", Manual: "codebase-memory-mcp index"})
	flow.End("warn")
	text := out.String()
	for _, want := range []string{"✓ Código Go · 3 nodos · 1ms", "− codegraph: no instalado", "⚠ codebase-memory-mcp: falló → codebase-memory-mcp index", "1 ok · 1 aviso · 1 omitido", "siguiente → mem context"} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q en:\n%s", want, text)
		}
	}
}

func TestFlow_VivoDibujaPanelConAvanceYResumen(t *testing.T) {
	var out bytes.Buffer
	flow := NewFlow(&out, styledEnv(80), "Indexar")
	flow.Plan("Código Go", "codegraph")
	flow.Progress("Código Go")
	if block := ansi.Strip(flow.render("⠋")); !strings.Contains(block, "╭─ Indexar · 0 de 2") || !strings.Contains(block, "⠋ Código Go") || !strings.Contains(block, "○ codegraph") {
		t.Fatalf("panel en curso:\n%s", block)
	}
	flow.Done(StepResult{Name: "Código Go", Meta: "3 nodos"})
	flow.Progress("codegraph")
	flow.Log("init --yes")
	if block := ansi.Strip(flow.render("⠋")); !strings.Contains(block, "1 de 2") || !strings.Contains(block, "▰▰▰▰▰▰▱▱▱▱▱▱ 50%") || !strings.Contains(block, "    init --yes") {
		t.Fatalf("avance a mitad:\n%s", block)
	}
	flow.Done(StepResult{Name: "codegraph", Status: StepFail, Detail: "sin índice", Manual: "codegraph init"})
	flow.End("fail")
	text := ansi.Strip(out.String())
	for _, want := range []string{"2 de 2", "✗ codegraph", "✗ codegraph: sin índice → codegraph init", "Interrumpido"} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q en:\n%s", want, text)
		}
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if ansi.StringWidth(line) > 80 {
			t.Fatalf("línea desbordada: %q", ansi.Strip(line))
		}
	}
}

func TestFlow_LineaEstrechaConservaLaGuia(t *testing.T) {
	var out bytes.Buffer
	flow := NewFlow(&out, Env{Width: 30, Getenv: func(string) string { return "" }}, "Indexar")
	flow.Next("mem context · mem tui")
	flow.Done(StepResult{Name: "codebase-memory-mcp", Meta: "43 nodos · 43 aristas · 9ms"})
	flow.End("")
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	for i, line := range lines {
		if ansi.StringWidth(line) > 30 {
			t.Fatalf("línea desbordada: %q", line)
		}
		if i > 0 && !strings.HasPrefix(line, "│") && !strings.HasPrefix(line, "└") && !strings.HasPrefix(line, "  ") {
			t.Fatalf("continuación sin guía: %q\n%s", line, out.String())
		}
	}
	if text := out.String(); !strings.Contains(text, "siguiente → mem context") || !strings.Contains(text, "mem tui") {
		t.Fatalf("siguiente acción recortada:\n%s", out.String())
	}
}

func TestPrintSummary_ContratoPlanoYPanelEstilizado(t *testing.T) {
	results := []StepResult{{Name: "Descarga", Detail: "mem.tar.gz v9"}, {Name: "Checksum", Status: StepFail, Detail: "no coincide"}}
	var plain bytes.Buffer
	PrintSummary(&plain, plainEnv(), "Actualizar", results, "")
	if got := plain.String(); got != "\nResumen:\n  ✓ Descarga: mem.tar.gz v9\n  ✗ Checksum: no coincide\n" {
		t.Fatalf("contrato FR-024 alterado: %q", got)
	}
	var styled bytes.Buffer
	PrintSummary(&styled, styledEnv(80), "Actualizar", results, "mem version")
	text := ansi.Strip(styled.String())
	for _, want := range []string{"╭─ Actualizar · 2 pasos", "✓ Descarga · mem.tar.gz v9", "✗ Checksum: no coincide", "Interrumpido", "siguiente → mem version"} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q:\n%s", want, text)
		}
	}
}
