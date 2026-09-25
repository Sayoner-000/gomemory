package native

import (
	"fmt"
	"regexp"
	"strings"

	"mem/domain"
)

// Compresores de código sin AST (research.md R4): llaves para Java, TS y JS,
// sangría para Python e INSERT repetidos para SQL. Emiten siempre líneas
// literales del original más líneas marcador, así que la guarda de
// literalidad los valida igual que al resto. ok=false indica que la
// estructura no cuadra (llaves desbalanceadas): el motor degrada a la
// compresión estructural.

var (
	// braceHeader reconoce cabeceras de función o método que abren bloque en la
	// misma línea. Las sentencias de control se excluyen.
	braceHeader = regexp.MustCompile(`\)\s*(:\s*[\w<>\[\]|., ?]+\s*)?(throws [\w., ]+)?\s*\{\s*$|=>\s*\{\s*$`)
	controlLine = regexp.MustCompile(`^\s*(\}\s*)?(if|for|while|switch|catch|else|do|try|finally|synchronized|return|with)\b`)
	pyDef       = regexp.MustCompile(`^(\s*)(async\s+)?def\s+\w+.*:\s*(#.*)?$`)
	sqlInsert   = regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\s+(\S+)`)
)

func codeMarker(indent, ref string, n int, comment string) string {
	return indent + comment + " " + domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("cuerpo omitido: %d líneas", n)})
}

// compressBraces omite el cuerpo de cada función o método con llaves de al
// menos CodeMinBodyLines líneas y conserva cabecera y cierre.
func compressBraces(content, ref string, th domain.Thresholds) (string, int, bool) {
	lines := strings.Split(content, "\n")
	depths, ok := braceDepths(lines)
	if !ok {
		return "", 0, false
	}
	var out []string
	omitted := 0
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if braceHeader.MatchString(l) && !controlLine.MatchString(l) {
			// Busca la línea de cierre: la primera posterior que vuelve a la
			// profundidad anterior a la cabecera.
			start := depths[i]
			end := -1
			for j := i + 1; j < len(lines); j++ {
				if depths[j+1] <= start {
					end = j
					break
				}
			}
			body := end - i - 1
			if end > 0 && body >= th.CodeMinBodyLines {
				indent := leadingWS(lines[i+1])
				out = append(out, l, codeMarker(indent, ref, body, "//"), lines[end])
				omitted += body
				i = end
				continue
			}
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n"), omitted, true
}

// braceDepths devuelve la profundidad de llaves al INICIO de cada línea (con
// una entrada extra para el final). Ignora llaves dentro de strings,
// caracteres, plantillas y comentarios. ok=false si no cuadran.
func braceDepths(lines []string) ([]int, bool) {
	depths := make([]int, len(lines)+1)
	depth := 0
	inBlock := false
	for i, l := range lines {
		depths[i] = depth
		var quote byte
		for j := 0; j < len(l); j++ {
			ch := l[j]
			if inBlock {
				if ch == '*' && j+1 < len(l) && l[j+1] == '/' {
					inBlock = false
					j++
				}
				continue
			}
			if quote != 0 {
				if ch == '\\' {
					j++
				} else if ch == quote {
					quote = 0
				}
				continue
			}
			switch {
			case ch == '/' && j+1 < len(l) && l[j+1] == '/':
				j = len(l)
			case ch == '/' && j+1 < len(l) && l[j+1] == '*':
				inBlock = true
				j++
			case ch == '"' || ch == '\'' || ch == '`':
				quote = ch
			case ch == '{':
				depth++
			case ch == '}':
				depth--
				if depth < 0 {
					return nil, false
				}
			}
		}
	}
	depths[len(lines)] = depth
	return depths, depth == 0
}

// compressPython omite los cuerpos de def de al menos CodeMinBodyLines líneas,
// conservando la cabecera y el docstring.
func compressPython(content, ref string, th domain.Thresholds) (string, int, bool) {
	lines := strings.Split(content, "\n")
	var out []string
	omitted := 0
	for i := 0; i < len(lines); i++ {
		m := pyDef.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			continue
		}
		defIndent := len(m[1])
		out = append(out, lines[i])
		j := i + 1
		// Docstring: se conserva entero.
		if j < len(lines) {
			t := strings.TrimSpace(lines[j])
			if strings.HasPrefix(t, `"""`) || strings.HasPrefix(t, `'''`) {
				q := t[:3]
				out = append(out, lines[j])
				closed := len(t) >= 6 && strings.HasSuffix(t, q)
				j++
				for !closed && j < len(lines) {
					out = append(out, lines[j])
					closed = strings.Contains(lines[j], q)
					j++
				}
			}
		}
		bodyStart := j
		end := j
		for end < len(lines) {
			l := lines[end]
			if strings.TrimSpace(l) != "" && len(leadingWS(l)) <= defIndent {
				break
			}
			end++
		}
		// No arrastrar las líneas en blanco finales al cuerpo omitido.
		for end > bodyStart && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		body := end - bodyStart
		if body >= th.CodeMinBodyLines {
			out = append(out, codeMarker(leadingWS(lines[bodyStart]), ref, body, "#"))
			omitted += body
		} else {
			out = append(out, lines[bodyStart:end]...)
		}
		i = end - 1
	}
	return strings.Join(out, "\n"), omitted, true
}

// compressSQL colapsa series de INSERT a la misma tabla: conserva el primero y
// el último.
func compressSQL(content, ref string, th domain.Thresholds) (string, int, bool) {
	lines := strings.Split(content, "\n")
	var out []string
	omitted := 0
	minRun := th.LogMinRun
	for i := 0; i < len(lines); {
		m := sqlInsert.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			i++
			continue
		}
		j := i + 1
		for j < len(lines) {
			m2 := sqlInsert.FindStringSubmatch(lines[j])
			if m2 == nil || !strings.EqualFold(m2[1], m[1]) {
				break
			}
			j++
		}
		if n := j - i; n >= minRun+2 {
			out = append(out, lines[i], "-- "+domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("×%d INSERT en %s", n-2, m[1])}), lines[j-1])
			omitted += n - 2
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return strings.Join(out, "\n"), omitted, true
}

func leadingWS(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}
