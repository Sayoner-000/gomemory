package console

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDocumentPreservesPipedOutputAndFitsHumanTerminals(t *testing.T) {
	text := "RESUMEN\n  Operación de contexto con símbolos 界 y acentos: información completa\n"
	for _, width := range []int{20, 40, 80, 120} {
		e := Env{StdoutTTY: true, Width: width, Getenv: func(string) string { return "" }}
		out := NewLayout(e).Document(text)
		for _, line := range strings.Split(out, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("ancho %d: línea desbordada %q", width, line)
			}
		}
		if !strings.Contains(ansi.Strip(out), "RESUMEN") || !strings.Contains(out, "información") {
			t.Fatalf("se perdió contenido: %q", out)
		}
	}
	for _, env := range []Env{{Getenv: func(string) string { return "" }}, {StdoutTTY: true, Getenv: func(k string) string {
		if k == "CI" {
			return "1"
		}
		return ""
	}}} {
		if out := NewLayout(env).Document(text); out != text {
			t.Fatalf("salida automática alterada: %q", out)
		}
	}
}

func TestLayoutThemeAndPlainMode(t *testing.T) {
	for _, theme := range []string{"dark", "light", "matrix"} {
		e := Env{StdoutTTY: true, Width: 80, Getenv: func(k string) string {
			if k == "GOMEMORY_THEME" {
				return theme
			}
			return ""
		}}
		out := NewLayout(e).Section("Memoria")
		if !strings.Contains(out, foreground(Theme(e.Getenv).Primary)) {
			t.Fatalf("sección sin paleta %s: %q", theme, out)
		}
	}
	for _, key := range []string{"NO_COLOR", "TERM"} {
		e := Env{StdoutTTY: true, Width: 40, Getenv: func(k string) string {
			if k == key {
				if key == "TERM" {
					return "dumb"
				}
				return "1"
			}
			return ""
		}}
		if out := NewLayout(e).Document("RESUMEN\n  mem usage\n"); strings.Contains(out, "\x1b") {
			t.Fatalf("ANSI en modo plano: %q", out)
		}
	}
}

func TestTableKeepsLongValuesOnNarrowScreens(t *testing.T) {
	l := NewLayout(Env{StdoutTTY: true, Width: 30, Getenv: func(string) string { return "" }})
	out := ansi.Strip(l.Table([]string{"Canal", "Tokens"}, [][]string{{"canal-extraordinariamente-largo", "12345"}}))
	if !strings.Contains(out, "12345") || !strings.Contains(strings.ReplaceAll(out, "\n", ""), "extraordinariamente") {
		t.Fatalf("tabla pierde valores: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if ansi.StringWidth(line) > 30 {
			t.Fatalf("tabla desbordada: %q", line)
		}
	}
}

func TestHumanCategoriesAndStatusesHaveVisualRoles(t *testing.T) {
	e := Env{StdoutTTY: true, Width: 80, Getenv: func(string) string { return "" }}
	l := NewLayout(e)
	out := l.Document("  Memoria diaria\n  ✅ Memoria guardada\n  ⚠️  Falta integración\n  ❌ Falló la operación\n")
	if !strings.Contains(out, foreground(l.palette.Primary)+"  Memoria diaria") {
		t.Fatalf("categoría sin énfasis: %q", out)
	}
	plain := ansi.Strip(out)
	for _, want := range []string{"✓ Memoria guardada", "⚠ Falta integración", "✗ Falló la operación"} {
		if !strings.Contains(plain, want) {
			t.Errorf("estado inconsistente %q: %q", want, plain)
		}
	}
}

func TestHelpIndentationAndTrailingSpacesCannotOverflow(t *testing.T) {
	l := NewLayout(Env{StdoutTTY: true, Width: 40, Getenv: func(string) string { return "" }})
	out := l.Document("    --agents a,b  --scope project|global  --yes  --events\n                                    Elegir agentes y alcance\n")
	for _, line := range strings.Split(out, "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatalf("línea desborda 40: %q", line)
		}
	}
}

func TestSummaryUsesThemeStatusColors(t *testing.T) {
	t.Setenv("GOMEMORY_THEME", "light")
	out := RenderSummary([]StepResult{{Name: "Memoria", Status: StepOK}, {Name: "MCP", Status: StepWarn}, {Name: "Descarga", Status: StepFail}}, true)
	for _, color := range []string{Violeta, "#866000", "#b42336"} {
		if !strings.Contains(out, foreground(color)) {
			t.Fatalf("resumen sin color de tema %s: %q", color, out)
		}
	}
}
