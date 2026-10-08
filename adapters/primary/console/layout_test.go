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
		if !strings.Contains(strings.ToUpper(ansi.Strip(out)), "RESUMEN") || !strings.Contains(out, "información") {
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
	out := renderSummary([]StepResult{{Name: "Memoria", Status: StepOK}, {Name: "MCP", Status: StepWarn}, {Name: "Descarga", Status: StepFail}}, true)
	for _, color := range []string{Violeta, "#866000", "#b42336"} {
		if !strings.Contains(out, foreground(color)) {
			t.Fatalf("resumen sin color de tema %s: %q", color, out)
		}
	}
}

func TestDocumentGramaticaComun(t *testing.T) {
	l := NewLayout(styledEnv(60))
	in := "CONFIGURACIÓN\nAuto-approve: true\nRuta MCP: /x\n  ➖ gomemory: sin cambios\nUsa --auto-approve=true para cambiar\nError: subcomando requerido\n"
	out := l.Document(in)
	plain := ansi.Strip(out)
	for _, want := range []string{"╭─ Configuración", "│ Auto-approve: true", "Ruta MCP: /x", "│ − gomemory: sin cambios", "→ Usa --auto-approve=true para cambiar", "Error: subcomando requerido"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("falta %q en:\n%s", want, plain)
		}
	}
	if !strings.Contains(out, foreground(l.palette.Muted)+"Auto-approve:") {
		t.Fatalf("la clave debe ir atenuada: %q", out)
	}
	if !strings.Contains(out, foreground(l.palette.ErrorColor())+"Error: subcomando requerido") {
		t.Fatalf("el error debe usar el color de error: %q", out)
	}
	if got := ansi.Strip(l.Document("MCP Y AGENTES\n  uno\n")); !strings.Contains(got, "╭─ MCP y agentes") {
		t.Fatalf("siglas en títulos: %q", got)
	}
}

func TestDocumentContinuacionConSangriaColgante(t *testing.T) {
	l := NewLayout(styledEnv(40))
	out := ansi.Strip(l.Document("  ✓ gomemory claude user instructions hooks presentes en settings\n"))
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("se esperaba ajuste: %q", out)
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "    ") || ansi.StringWidth(line) > 40 {
			t.Fatalf("continuación sin sangría colgante: %q", out)
		}
	}
}

func TestDocumentAgrupaSeccionesEnPanelesYDejaElPieFuera(t *testing.T) {
	l := NewLayout(styledEnv(60))
	in := "Encabezado libre\n\nMEMORIA DIARIA\n  mem save        Guardar\n  mem search      Buscar\n\nSISTEMA\n  mem tui         Abrir\n\nUsa mem help <comando> para ver flags\n"
	out := ansi.Strip(l.Document(in))
	for _, want := range []string{"Encabezado libre", "╭─ Memoria diaria", "│ mem save", "╭─ Sistema", "→ Usa mem help"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "│ → Usa") {
		t.Fatalf("el pie quedó dentro del panel:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if ansi.StringWidth(line) > 60 {
			t.Fatalf("desborde: %q", line)
		}
	}
	if plain := NewLayout(plainEnv()).Document(in); plain != in {
		t.Fatalf("salida no humana alterada: %q", plain)
	}
}

func TestDocumentSeccionConLineaEnBlancoInicial(t *testing.T) {
	out := ansi.Strip(NewLayout(styledEnv(60)).Document("CONFIGURACIÓN\n\nAuto-approve: true\nTema: dark\n\nUsa --tema para cambiar\n"))
	if !strings.Contains(out, "│ Auto-approve: true") || !strings.Contains(out, "│ Tema: dark") || strings.Contains(out, "│ → Usa") {
		t.Fatalf("sección mal agrupada:\n%s", out)
	}
}

func TestReport_PanelConClavesAlineadasYSubtitulos(t *testing.T) {
	l := NewLayout(styledEnv(60))
	out := ansi.Strip(l.Report("Octopus AAR", "estado", "Topes efectivos\n  Agentes por plan: 4\n  Concurrencia: 3\n\nTasa de éxito: 100 %\n"))
	for _, want := range []string{"╭─ Octopus AAR · estado", "│ Topes efectivos", "│   Agentes por plan: 4", "│   Concurrencia:     3", "│ Tasa de éxito: 100 %"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q:\n%s", want, out)
		}
	}
	if plain := NewLayout(plainEnv()).Report("x", "", "a: 1\n"); plain != "a: 1\n" {
		t.Fatalf("Report fuera de terminal debe dejar el texto intacto: %q", plain)
	}
}

func TestDocumentTituloConDosPuntosAbreSeccion(t *testing.T) {
	out := ansi.Strip(NewLayout(styledEnv(60)).Document("Compresión:\n  nivel: max\n  originales: 0.1 MB\n\nOpenCode: v1 detectado\n\nDegradaciones declaradas (no requieren acción):\n  - claude: x\n"))
	for _, want := range []string{"╭─ Compresión", "│ nivel:", "OpenCode: v1 detectado", "╭─ Degradaciones declaradas · no requieren acción"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "│ OpenCode") {
		t.Fatalf("una línea clave-valor suelta no es sección:\n%s", out)
	}
}
