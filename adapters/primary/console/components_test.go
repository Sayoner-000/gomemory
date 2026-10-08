package console

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func styledEnv(width int) Env {
	return Env{StdoutTTY: true, Width: width, Getenv: func(key string) string {
		if key == "TERM" {
			return "xterm-256color"
		}
		return ""
	}}
}

func plainEnv() Env { return Env{Width: 80, Getenv: func(string) string { return "" }} }

func TestPanel_TituloEnElBordeYMetaALaDerecha(t *testing.T) {
	for _, width := range []int{40, 80, 140} {
		l := NewLayout(styledEnv(width))
		out := l.Panel("Indexar", "2 de 3", "4s", []string{"✓ Código Go", strings.Repeat("x ", 100)})
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		top := ansi.Strip(lines[0])
		if !strings.HasPrefix(top, "╭─ Indexar · 2 de 3 ") || !strings.HasSuffix(top, " 4s ─╮") {
			t.Fatalf("borde superior %d: %q", width, top)
		}
		if bottom := ansi.Strip(lines[len(lines)-1]); !strings.HasPrefix(bottom, "╰") || !strings.HasSuffix(bottom, "╯") {
			t.Fatalf("borde inferior: %q", bottom)
		}
		want := ansi.StringWidth(lines[0])
		for _, line := range lines {
			if got := ansi.StringWidth(line); got != want || got > width {
				t.Fatalf("ancho %d: línea de %d columnas (esperado %d): %q", width, got, want, ansi.Strip(line))
			}
		}
	}
}

func TestPanel_SinTerminalEsTextoPlano(t *testing.T) {
	out := NewLayout(plainEnv()).Panel("Indexar", "2 de 3", "4s", []string{"✓ Código Go"})
	if out != "Indexar · 2 de 3 · 4s\n  ✓ Código Go\n" {
		t.Fatalf("panel plano = %q", out)
	}
}

func TestStepLines_EstadosGlifosYMeta(t *testing.T) {
	l := NewLayout(plainEnv())
	rows := []StepRow{
		{Label: "Código Go", State: StateDone, Meta: "3 nodos · 1ms"},
		{Label: "codegraph", State: StateRunning, Detail: "init --yes", Logs: []string{"a", "b", "c", "d"}},
		{Label: "codebase-memory-mcp", State: StatePending},
		{Label: "otro", State: StateSkip, Detail: "no instalado"},
	}
	got := strings.Join(l.StepLines(rows, "", 50), "\n")
	for _, want := range []string{"✓ Código Go", "3 nodos · 1ms", "● codegraph · init --yes", "    b", "    d", "○ codebase-memory-mcp", "− otro · no instalado"} {
		if !strings.Contains(got, want) {
			t.Fatalf("falta %q en:\n%s", want, got)
		}
	}
	if strings.Contains(got, "    a") {
		t.Fatalf("la cola de logs debe limitarse a 3 líneas:\n%s", got)
	}
	first := l.StepLines(rows[:1], "", 50)[0]
	if ansi.StringWidth(first) != 50 || !strings.HasSuffix(first, "3 nodos · 1ms") {
		t.Fatalf("meta no alineada a la derecha: %q", first)
	}
}

func TestStatusBar_SegmentosYMedidor(t *testing.T) {
	l := NewLayout(plainEnv())
	if got := l.Meter(2, 3, 6); got != "▰▰▰▰▱▱ 66%" {
		t.Fatalf("medidor = %q", got)
	}
	if got := l.StatusBar("◆ goMemory", "", "2 ok"); got != "◆ goMemory ◦ 2 ok" {
		t.Fatalf("barra = %q", got)
	}
	narrow := NewLayout(styledEnv(40)).StatusBar(strings.Repeat("segmento ", 10), "fin")
	if ansi.StringWidth(narrow) > 40 {
		t.Fatalf("barra desbordada: %q", ansi.Strip(narrow))
	}
}

func TestFormatInt_SeparadorDeMiles(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 4835: "4 835", -89659: "-89 659", 1234567: "1 234 567"} {
		if got := FormatInt(in); got != want {
			t.Fatalf("FormatInt(%d) = %q, esperado %q", in, got, want)
		}
	}
}

func TestTable_ColumnasNumericasALaDerecha(t *testing.T) {
	l := NewLayout(styledEnv(80))
	out := ansi.Strip(l.Table([]string{"Origen", "Llamadas"}, [][]string{{"build_context", "3"}, {"mcp", "1 024"}}))
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if !strings.HasSuffix(lines[0], "Llamadas") || !strings.HasSuffix(lines[1], "       3") || !strings.HasSuffix(lines[2], "   1 024") {
		t.Fatalf("columna numérica no alineada a la derecha:\n%s", out)
	}
	if got := NewLayout(styledEnv(80)).WithWidth(30).Width(); got != 30 {
		t.Fatalf("WithWidth = %d", got)
	}
}

func TestPanel_AjustaLineasLargasSinPerderTexto(t *testing.T) {
	l := NewLayout(styledEnv(40))
	out := ansi.Strip(l.Panel("Doctor", "", "", []string{"✓ gomemory claude hooks presentes en ~/.claude/settings.json"}))
	joined := strings.Join(strings.Fields(strings.NewReplacer("│", " ").Replace(out)), " ")
	if !strings.Contains(joined, "~/.claude/settings.json") || strings.Contains(out, "…") {
		t.Fatalf("el panel recortó el detalle:\n%s", out)
	}
}

func TestStatusBar_DescartaSegmentosEnterosAlFaltarAncho(t *testing.T) {
	got := ansi.Strip(NewLayout(styledEnv(40)).StatusBar("✓ Ahorro total 135 588 tokens", "5.1 %", "2 679 366 → 2 543 778"))
	if got != "✓ Ahorro total 135 588 tokens ◦ 5.1 %" {
		t.Fatalf("barra = %q", got)
	}
}
