package console

import (
	"fmt"
	"strings"
	"unicode"

	"mem/version"

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

// Banner es la cabecera común de todos los comandos humanos: marca, versión
// y comando, con una regla que separa la cabecera del contenido.
func (l Layout) Banner(command string) string {
	line := l.bold("◆ goMemory", l.palette.Primary) + " " + l.Muted(version.Version) + " " + l.Muted("›") + " " + l.color(command, l.palette.Text)
	line = ansi.Truncate(line, l.Width(), "…")
	return line + "\n" + l.Muted(strings.Repeat("─", min(l.Width(), maxPanelWidth)))
}

// Document aplica la gramática visual común a la salida humana de cualquier
// comando: títulos ▸, claves atenuadas, glifos de estado, pistas → y ajuste
// con sangría colgante. Fuera de una terminal humana devuelve el texto
// intacto, byte a byte, para agentes, pipes y CI.
func (l Layout) Document(text string) string {
	if !visualTerminal(l.env) {
		return text
	}
	in := strings.Split(text, "\n")
	isTop := func(i int) bool {
		t := in[i]
		return strings.TrimSpace(t) != "" && !strings.HasPrefix(t, " ")
	}
	// Un título de sección es una línea en mayúsculas, o una línea suelta
	// terminada en ":" cuyo contenido viene sangrado debajo.
	sectionTitle := func(i int) (string, string, bool) {
		if !isTop(i) {
			return "", "", false
		}
		t := strings.TrimSpace(in[i])
		if isHeading(t) {
			return headingCase(t), "", true
		}
		next := i + 1
		for next < len(in) && strings.TrimSpace(in[next]) == "" {
			next++
		}
		if strings.HasSuffix(t, ":") && next < len(in) && strings.HasPrefix(in[next], "  ") {
			title, meta, _ := strings.Cut(strings.TrimSuffix(t, ":"), " (")
			return title, strings.TrimSuffix(meta, ")"), true
		}
		return "", "", false
	}
	out := make([]string, 0, len(in))
	for i := 0; i < len(in); i++ {
		title, meta, isSection := sectionTitle(i)
		if !styledTerminal(l.env) || !isSection {
			out = append(out, l.styleLine(in[i]))
			continue
		}
		// Una sección es su título más todo lo que sigue hasta el próximo
		// título o hasta un párrafo libre tras una línea en blanco (el pie).
		j, hasBody := i+1, false
		for ; j < len(in); j++ {
			if _, _, next := sectionTitle(j); next {
				break
			}
			blank := strings.TrimSpace(in[j]) == ""
			if blank && hasBody && j+1 < len(in) && isTop(j+1) && !isHeading(strings.TrimSpace(in[j+1])) {
				break
			}
			hasBody = hasBody || !blank
		}
		body := make([]string, 0, j-i)
		for _, raw := range in[i+1 : j] {
			if len(body) == 0 && strings.TrimSpace(raw) == "" {
				continue
			}
			body = append(body, l.styleText(strings.TrimPrefix(raw, "  ")))
		}
		for len(body) > 0 && strings.TrimSpace(ansi.Strip(body[len(body)-1])) == "" {
			body = body[:len(body)-1]
		}
		for len(out) > 0 && strings.TrimSpace(ansi.Strip(out[len(out)-1])) == "" {
			out = out[:len(out)-1]
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, strings.TrimSuffix(l.Panel(title, meta, "", body), "\n"))
		i = j - 1
	}
	result := strings.Join(out, "\n")
	// Una sección que llega al final consume la línea vacía del salto final.
	if strings.HasSuffix(text, "\n") && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

// styleLine aplica la gramática a una línea suelta y la ajusta al ancho.
func (l Layout) styleLine(raw string) string {
	indent := raw[:len(raw)-len(strings.TrimLeft(raw, " "))]
	return l.wrapHanging(l.styleText(raw), indent)
}

// styleText aplica la gramática sin ajustar: para contenido que otro
// componente (un panel) ajusta a su propio ancho.
func (l Layout) styleText(raw string) string {
	line := glyphs.Replace(raw)
	trimmed := strings.TrimSpace(line)
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	switch {
	case trimmed != "" && strings.Trim(trimmed, "=─-") == "":
		return l.Muted(strings.Repeat("─", min(l.Width(), maxPanelWidth)))
	case indent == "" && isHeading(trimmed):
		line = l.bold("▸ "+headingCase(trimmed), l.palette.Primary)
	case statusGlyph(trimmed) >= 0:
		state := StepState(statusGlyph(trimmed))
		glyph, rest, _ := strings.Cut(trimmed, " ")
		if state == StateSkip {
			rest = l.Muted(rest)
		}
		line = indent + l.color(glyph, l.palette.StateColor(state)) + " " + rest
	case strings.HasPrefix(trimmed, "Error:"):
		line = indent + l.color(trimmed, l.palette.ErrorColor())
	case strings.HasPrefix(trimmed, "Usa ") || strings.HasPrefix(trimmed, "Ejemplo:"):
		line = indent + l.Muted("→ "+trimmed)
	case isKeyValue(trimmed):
		key, value, _ := strings.Cut(trimmed, ":")
		line = indent + l.Muted(key+":") + value
	case strings.HasPrefix(trimmed, "mem "):
		// El comando y sus parámetros quedan distinguibles de la descripción.
		if pos := strings.Index(trimmed, "  "); pos >= 0 {
			line = indent + l.color(trimmed[:pos], l.palette.Text) + l.Muted(trimmed[pos:])
		}
	case trimmed != "" && indent == "":
		line = l.Section(line)
	case indent == "  " && !strings.HasPrefix(trimmed, "-"):
		line = l.Section(line)
	}
	return line
}

// Report presenta la salida de un comando de estado dentro de un panel:
// subtítulos resaltados, claves alineadas por bloque y ajuste con sangría.
// Sin terminal estilizada equivale a Document (texto intacto fuera de TTY).
func (l Layout) Report(title, meta, text string) string {
	if !styledTerminal(l.env) {
		return l.Document(text)
	}
	lines := strings.Split(strings.Trim(text, "\n"), "\n")
	keyWidth := make([]int, len(lines))
	for start := 0; start < len(lines); {
		end := start
		for end < len(lines) && isKeyValue(strings.TrimSpace(lines[end])) {
			end++
		}
		width := 0
		for i := start; i < end; i++ {
			key, _, _ := strings.Cut(strings.TrimSpace(lines[i]), ":")
			width = max(width, ansi.StringWidth(key))
		}
		for i := start; i < end; i++ {
			keyWidth[i] = width
		}
		start = max(end, start+1)
	}
	body := make([]string, 0, len(lines))
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		indent := raw[:len(raw)-len(strings.TrimLeft(raw, " "))]
		switch {
		case trimmed != "" && indent == "" && !isKeyValue(trimmed) && statusGlyph(trimmed) < 0 && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "  "):
			body = append(body, l.bold(trimmed, l.palette.Primary))
		case keyWidth[i] > 0:
			key, value, _ := strings.Cut(trimmed, ":")
			pad := strings.Repeat(" ", keyWidth[i]-ansi.StringWidth(key))
			body = append(body, indent+l.Muted(key+":"+pad)+" "+strings.TrimSpace(value))
		case indent != "" && statusGlyph(trimmed) < 0:
			// Filas de datos sangradas: texto normal, sin el énfasis de
			// categoría que la ayuda da a sus grupos.
			body = append(body, glyphs.Replace(raw))
		default:
			body = append(body, l.styleText(raw))
		}
	}
	return l.Panel(title, meta, "", body)
}

var glyphs = strings.NewReplacer("✅ ", "✓ ", "⚠️  ", "⚠ ", "⚠️ ", "⚠ ", "❌ ", "✗ ", "➖ ", "− ", "ℹ️  ", "· ", "ℹ️ ", "· ")

// statusGlyph devuelve el estado de una línea que empieza con glifo, o -1.
func statusGlyph(trimmed string) int {
	for state, glyph := range map[StepState]string{StateDone: "✓ ", StateWarn: "⚠ ", StateFail: "✗ ", StateSkip: "− "} {
		if strings.HasPrefix(trimmed, glyph) {
			return int(state)
		}
	}
	return -1
}

// wrapHanging ajusta al ancho; las continuaciones se alinean bajo el texto
// (después de la sangría y del glifo) en vez de volver a la columna 0.
func (l Layout) wrapHanging(line, indent string) string {
	width := l.Width()
	wrap := func(t string, w int) []string {
		return strings.Split(ansi.Hardwrap(ansi.Wrap(t, w, ""), w, false), "\n")
	}
	lines := wrap(line, width)
	if len(lines) == 1 {
		return lines[0]
	}
	hang := indent + "  "
	if ansi.StringWidth(hang) >= width/2 {
		hang = ""
	}
	rest := strings.TrimSpace(strings.Join(lines[1:], " "))
	lines = lines[:1]
	for _, cont := range wrap(rest, width-ansi.StringWidth(hang)) {
		lines = append(lines, hang+cont)
	}
	return strings.Join(lines, "\n")
}

// isHeading reconoce los títulos de sección escritos en mayúsculas.
func isHeading(s string) bool {
	letters := 0
	for _, r := range s {
		switch {
		case unicode.IsLower(r):
			return false
		case unicode.IsLetter(r):
			letters++
		case r == ':' || r == '=' || r == '/':
			return false
		}
	}
	return letters >= 4
}

var headingAcronyms = map[string]bool{"MCP": true, "CLI": true, "TUI": true, "ADR": true, "API": true, "JSON": true, "NDJSON": true, "IA": true, "AI": true, "ACR": true, "AAR": true, "ID": true}

// headingCase pasa un título en mayúsculas a frase, conservando siglas.
func headingCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if headingAcronyms[w] {
			continue
		}
		lower := []rune(strings.ToLower(w))
		if i == 0 && len(lower) > 0 {
			lower[0] = unicode.ToUpper(lower[0])
		}
		words[i] = string(lower)
	}
	return strings.Join(words, " ")
}

// isKeyValue reconoce "Clave: valor" con una clave breve de texto.
func isKeyValue(s string) bool {
	key, value, ok := strings.Cut(s, ":")
	if !ok || strings.TrimSpace(value) == "" || len([]rune(key)) > 40 || key == "" {
		return false
	}
	if !unicode.IsLetter([]rune(key)[0]) || strings.ContainsAny(key, "/`→\\") || strings.HasPrefix(key, "mem ") || strings.HasPrefix(key, "http") {
		return false
	}
	return value[0] == ' '
}

// Printf y Println son los puntos de entrada para mensajes humanos; los
// serializadores y los documentos exportados siguen escribiendo directamente.
func Printf(format string, args ...any) { emit(fmt.Sprintf(format, args...)) }
func Println(args ...any)               { emit(fmt.Sprintln(args...)) }

// page acumula la salida humana de un reporte para darle forma de una sola
// vez: una sección solo puede ir en un panel si se ve completa.
var page *strings.Builder

func emit(text string) {
	if page != nil {
		page.WriteString(text)
		return
	}
	fmt.Print(NewLayout(DetectEnv()).Document(text))
}

// BeginPage empieza a acumular la salida humana de un reporte; EndPage la
// presenta completa. Solo para reportes sin prompts ni progreso en vivo.
func BeginPage() {
	if page == nil {
		page = &strings.Builder{}
	}
}

func EndPage() {
	if page == nil {
		return
	}
	text := page.String()
	page = nil
	emit(text)
}

// WithWidth devuelve el mismo layout acotado a width columnas, para
// componer tablas y textos dentro de un panel.
func (l Layout) WithWidth(width int) Layout {
	l.env.Width = width
	return l
}

// numericColumns marca las columnas cuyas celdas son todas números, que se
// alinean a la derecha para comparar magnitudes de un vistazo.
func numericColumns(n int, rows [][]string) []bool {
	numeric := make([]bool, n)
	for i := range numeric {
		numeric[i] = len(rows) > 0
		for _, row := range rows {
			if i >= len(row) {
				numeric[i] = false
				break
			}
			cell := strings.ReplaceAll(row[i], " ", "")
			if cell == "—" {
				continue
			}
			if cell == "" || strings.Trim(cell, "0123456789.,%-msk") != "" || strings.Trim(cell, ".,%-msk") == "" {
				numeric[i] = false
				break
			}
		}
	}
	return numeric
}

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
		// Ficha compacta: la primera columna titula y el resto va en una
		// línea "Campo valor · Campo valor" ajustada con sangría colgante.
		for _, row := range rows {
			if len(row) == 0 {
				continue
			}
			b.WriteString(l.wrapHanging(l.Section(row[0]), "") + "\n")
			fields := make([]string, 0, len(row)-1)
			for i := 1; i < len(row) && i < len(headers); i++ {
				// Espacio duro: el ajuste nunca separa un campo de su valor.
				value := row[i]
				if ansi.StringWidth(value) <= 12 {
					value = strings.ReplaceAll(value, " ", "\u00a0")
				}
				fields = append(fields, l.Muted(strings.ReplaceAll(headers[i], " ", "\u00a0"))+"\u00a0"+value)
			}
			if len(fields) > 0 {
				b.WriteString(l.wrapHanging("  "+strings.Join(fields, l.Muted(" · ")), "  ") + "\n")
			}
		}
		return b.String()
	} else {
		numeric := numericColumns(len(headers), rows)
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
					cell = l.Muted(cell)
				}
				switch {
				case numeric[i]:
					b.WriteString(pad + cell)
				case i < len(row)-1:
					b.WriteString(cell + pad)
				default:
					b.WriteString(cell)
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
