package console

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Layout comparte jerarquía, espaciado y ancho visible entre las salidas humanas.
// Las salidas redirigidas conservan su texto original, byte a byte.
type Layout struct {
	env     Env
	palette Palette
}

func NewLayout(e Env) Layout { return Layout{env: e, palette: Theme(e.Getenv)} }

func (l Layout) Width() int {
	if l.env.Width > 0 {
		return l.env.Width
	}
	return 80
}

func (l Layout) color(text, color string) string {
	if !styledTerminal(l.env) {
		return text
	}
	return foreground(color) + text + "\x1b[0m"
}

func (l Layout) Section(title string) string {
	return l.color(title, l.palette.Primary)
}

func (l Layout) Muted(text string) string { return l.color(text, l.palette.Muted) }

func (l Layout) Header(action string) string {
	return l.color("go", l.palette.Secondary) + l.color("Memory", l.palette.Text) + " › " + l.Section(action)
}

// Document añade énfasis sin interpretar ni cambiar los datos del reporte.
func (l Layout) Document(text string) string {
	if !visualTerminal(l.env) {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.NewReplacer("✅ ", "✓ ", "⚠️  ", "⚠ ", "⚠️ ", "⚠ ", "❌ ", "✗ ").Replace(line)
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed != "" && strings.Trim(trimmed, "=─-") == "":
			line = l.Muted(strings.Repeat("─", min(l.Width(), 64)))
		case strings.HasPrefix(trimmed, "✓ "):
			line = l.color(line, l.palette.Secondary)
		case strings.HasPrefix(trimmed, "⚠ "):
			line = l.color(line, l.palette.WarningColor())
		case strings.HasPrefix(trimmed, "✗ "):
			line = l.color(line, l.palette.ErrorColor())
		case trimmed != "" && !strings.HasPrefix(line, " "):
			line = l.Section(line)
		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && !strings.HasPrefix(trimmed, "mem ") && !strings.HasPrefix(trimmed, "-"):
			line = l.Section(line)
		case strings.HasPrefix(trimmed, "mem "):
			// El comando y sus parámetros quedan distinguibles de la descripción.
			if pos := strings.Index(trimmed, "  "); pos >= 0 {
				indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
				line = indent + l.color(trimmed[:pos], l.palette.Text) + trimmed[pos:]
			}
		}
		lines[i] = ansi.Hardwrap(ansi.Wrap(line, l.Width(), ""), l.Width(), false)
	}
	return strings.Join(lines, "\n")
}

// Printf y Println son los puntos de entrada para mensajes humanos; los
// serializadores y los documentos exportados siguen escribiendo directamente.
func Printf(format string, args ...any) {
	fmt.Print(NewLayout(DetectEnv()).Document(fmt.Sprintf(format, args...)))
}
func Println(args ...any) { fmt.Print(NewLayout(DetectEnv()).Document(fmt.Sprintln(args...))) }

// Table usa columnas en pantallas amplias y fichas verticales si no caben.
func (l Layout) Table(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = ansi.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				widths[i] = max(widths[i], ansi.StringWidth(cell))
			}
		}
	}
	total := max(0, len(headers)-1) * 2
	for _, w := range widths {
		total += w
	}
	var b strings.Builder
	if total > l.Width() {
		for _, row := range rows {
			for i, cell := range row {
				if i < len(headers) {
					fmt.Fprintf(&b, "%s: %s\n", l.Muted(headers[i]), cell)
				}
			}
			b.WriteByte('\n')
		}
	} else {
		writeRow := func(row []string, heading bool) {
			for i, cell := range row {
				if i >= len(widths) {
					break
				}
				if i > 0 {
					b.WriteString("  ")
				}
				pad := strings.Repeat(" ", widths[i]-ansi.StringWidth(cell))
				if heading {
					cell = l.Section(cell)
				}
				b.WriteString(cell)
				if i < len(row)-1 {
					b.WriteString(pad)
				}
			}
			b.WriteByte('\n')
		}
		writeRow(headers, true)
		for _, row := range rows {
			writeRow(row, false)
		}
	}
	return ansi.Hardwrap(ansi.Wrap(b.String(), l.Width(), ""), l.Width(), false)
}
