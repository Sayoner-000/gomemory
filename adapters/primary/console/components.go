package console

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// StepState es el estado visual de una fila de pasos. Cada estado tiene un
// glifo y un rol de color fijos para que el significado no cambie entre
// comandos ni entre temas.
type StepState int

const (
	StatePending StepState = iota
	StateRunning
	StateDone
	StateWarn
	StateFail
	StateSkip
)

// maxPanelWidth evita paneles de lado a lado en terminales muy anchas, donde
// la meta alineada a la derecha queda lejos de su fila.
const maxPanelWidth = 100

// maxStepLogs es la cola visible de salida del paso en curso.
const maxStepLogs = 3

func stateGlyph(s StepState) string {
	switch s {
	case StateRunning:
		return "●"
	case StateDone:
		return "✓"
	case StateWarn:
		return "⚠"
	case StateFail:
		return "✗"
	case StateSkip:
		return "−"
	}
	return "○"
}

// StateColor asigna cada estado a un token existente de la paleta.
func (p Palette) StateColor(s StepState) string {
	switch s {
	case StateRunning:
		return p.Primary
	case StateDone:
		return p.Secondary
	case StateWarn:
		return p.Warning
	case StateFail:
		return p.Error
	}
	return p.Muted
}

func (l Layout) bold(text, color string) string {
	if !styledTerminal(l.env) {
		return text
	}
	return "\x1b[1m" + foreground(color) + text + "\x1b[0m"
}

func (l Layout) panelWidth() int { return min(l.Width(), maxPanelWidth) }

// fit recorta o rellena una línea (con o sin ANSI) a exactamente width columnas.
func fit(line string, width int) string {
	if ansi.StringWidth(line) > width {
		line = ansi.Truncate(line, width, "…")
	}
	return line + strings.Repeat(" ", max(0, width-ansi.StringWidth(line)))
}

// Panel enmarca un bloque con el título incrustado en el borde superior y un
// dato breve (tiempo, total) alineado a la derecha. Sin terminal estilizada
// devuelve el mismo contenido como texto plano.
func (l Layout) Panel(title, meta, right string, body []string) string {
	head := title
	if meta != "" {
		head += " · " + meta
	}
	var b strings.Builder
	if !styledTerminal(l.env) {
		if right != "" {
			head += " · " + right
		}
		b.WriteString(head + "\n")
		for _, line := range body {
			b.WriteString("  " + line + "\n")
		}
		return b.String()
	}
	width := l.panelWidth()
	tail := "─╮"
	if right != "" {
		tail = " " + right + " ─╮"
	}
	full := head
	head = ansi.Truncate(head, max(1, width-ansi.StringWidth(tail)-5), "…")
	fill := max(1, width-3-ansi.StringWidth(head)-1-ansi.StringWidth(tail))
	// El título resalta y la meta se atenúa; si no cabe, el recorte va entero.
	styledHead := l.bold(head, l.palette.Primary)
	if head == full && meta != "" {
		styledHead = l.bold(title, l.palette.Primary) + l.Muted(" · "+meta)
	}
	b.WriteString(l.Muted("╭─ ") + styledHead + " " + l.Muted(strings.Repeat("─", fill)+tail) + "\n")
	inner := width - 4
	narrow := l.WithWidth(inner)
	for _, line := range body {
		// Una línea larga se ajusta dentro del panel, alineada bajo su texto,
		// en vez de recortarse: el detalle (rutas, comandos) debe leerse entero.
		indent := ansi.Strip(line)
		indent = indent[:len(indent)-len(strings.TrimLeft(indent, " "))]
		for _, part := range strings.Split(narrow.wrapHanging(line, indent), "\n") {
			b.WriteString(l.Muted("│") + " " + fit(part, inner) + " " + l.Muted("│") + "\n")
		}
	}
	b.WriteString(l.Muted("╰"+strings.Repeat("─", width-2)+"╯") + "\n")
	return b.String()
}

// StepRow es una fila de una lista de pasos tipo checklist.
type StepRow struct {
	Label, Detail, Meta string
	State               StepState
	Logs                []string
}

// StepLines dibuja las filas a width columnas: glifo de estado, etiqueta,
// detalle atenuado y meta alineada a la derecha. Solo la fila en curso
// muestra su cola de logs. frame sustituye al glifo ● mientras hay animación.
func (l Layout) StepLines(rows []StepRow, frame string, width int) []string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		glyph := stateGlyph(row.State)
		if row.State == StateRunning && frame != "" {
			glyph = frame
		}
		label := row.Label
		if row.State == StatePending || row.State == StateSkip {
			label = l.Muted(label)
		}
		left := l.color(glyph, l.palette.StateColor(row.State)) + " " + label
		if row.Detail != "" {
			left += l.Muted(" · " + row.Detail)
		}
		meta := ""
		if row.Meta != "" {
			meta = l.Muted(row.Meta)
		}
		gap := width - ansi.StringWidth(left) - ansi.StringWidth(meta)
		if meta != "" && gap < 2 {
			left = ansi.Truncate(left, max(1, width-ansi.StringWidth(meta)-2), "…")
			gap = width - ansi.StringWidth(left) - ansi.StringWidth(meta)
		}
		if meta == "" {
			lines = append(lines, fit(left, width))
		} else {
			lines = append(lines, left+strings.Repeat(" ", max(1, gap))+meta)
		}
		if row.State == StateRunning {
			logs := row.Logs
			if len(logs) > maxStepLogs {
				logs = logs[len(logs)-maxStepLogs:]
			}
			for _, log := range logs {
				lines = append(lines, fit(l.Muted("    "+log), width))
			}
		}
	}
	return lines
}

// Meter dibuja un medidor de cells celdas con el porcentaje entero.
func (l Layout) Meter(done, total, cells int) string {
	if total <= 0 || cells <= 0 {
		return ""
	}
	done = min(max(done, 0), total)
	return l.Bar(done, total, cells) + " " + strconv.Itoa(done*100/total) + "%"
}

// Bar es solo la parte gráfica del medidor, para cuando la cifra ya se
// muestra antes y la barra puede recortarse en pantallas estrechas.
func (l Layout) Bar(done, total, cells int) string {
	if total <= 0 || cells <= 0 {
		return ""
	}
	filled := min(max(done, 0), total) * cells / total
	return l.color(strings.Repeat("▰", filled), l.palette.Primary) + l.Muted(strings.Repeat("▱", cells-filled))
}

// StatusBar une segmentos no vacíos con ◦ en una sola línea que nunca
// desborda la terminal.
func (l Layout) StatusBar(segments ...string) string {
	kept := make([]string, 0, len(segments))
	for _, s := range segments {
		if s != "" {
			kept = append(kept, s)
		}
	}
	line := strings.Join(kept, l.Muted(" ◦ "))
	// Sin espacio se descartan segmentos enteros desde el final: un número
	// recortado a la mitad informa peor que un segmento ausente.
	for len(kept) > 1 && ansi.StringWidth(line) > l.Width() {
		kept = kept[:len(kept)-1]
		line = strings.Join(kept, l.Muted(" ◦ "))
	}
	if ansi.StringWidth(line) > l.Width() {
		line = ansi.Truncate(line, l.Width(), "…")
	}
	return line
}

// FormatInt agrupa los miles con espacio, legible en cualquier locale.
func FormatInt(n int) string {
	sign, digits := "", strconv.Itoa(n)
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}
