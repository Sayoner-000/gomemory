package native

import (
	"fmt"
	"regexp"
	"strings"

	"mem/domain"
)

// Compresor de logs y trazas (research.md R5). La normalización (fechas,
// números, hex, rutas…) solo sirve para AGRUPAR líneas de la misma forma: lo
// que se emite es siempre una línea original literal.

var (
	normTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}([.,]\d+)?(Z|[+-]\d{2}:?\d{2})?`)
	normUUID      = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	normHex       = regexp.MustCompile(`(?i)\b(0x)?[0-9a-f]{7,}\b`)
	normPath      = regexp.MustCompile(`(/[\w.@-]+){2,}/?|[\w.-]+(/[\w.@-]+)+`)
	normNumber    = regexp.MustCompile(`\d+([.,]\d+)?`)
	normBars      = regexp.MustCompile(`[+-]{2,}`)
	normSpace     = regexp.MustCompile(`\s+`)

	// protectedLine: nunca se colapsa (errores, pánicos, fallos de prueba).
	protectedLine = regexp.MustCompile(`(?i)(\b(ERROR|FATAL|PANIC|CRITICAL)\b|panic:|^\s*--- FAIL|^FAIL\b|Exception\b|Error:)`)
	warnLine      = regexp.MustCompile(`(?i)\bWARN(ING)?\b`)
	// traceStart abre una traza de pila; traceFrame es una línea de marco.
	traceStart = regexp.MustCompile(`^(goroutine \d+ \[|Traceback \(most recent call last\):|Exception in thread |panic: )`)
	traceFrame = regexp.MustCompile(`^(\s+\S|[\w./*()]+\(.*\)$|created by )`)
)

// traceKeepLines: cabecera + unos 5 marcos (en Go cada marco son 2 líneas).
const traceKeepLines = 10

func shapeOf(line string) string {
	s := normTimestamp.ReplaceAllString(line, "<ts>")
	s = normUUID.ReplaceAllString(s, "<uuid>")
	s = normHex.ReplaceAllString(s, "<hex>")
	s = normPath.ReplaceAllString(s, "<path>")
	s = normNumber.ReplaceAllString(s, "<n>")
	s = normBars.ReplaceAllString(s, "<bar>")
	return normSpace.ReplaceAllString(strings.TrimSpace(s), " ")
}

func logMarker(indent, ref string, n int, what string) string {
	return indent + domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("×%d %s", n, what)})
}

// compressLog colapsa series de líneas de la misma forma (primera y última
// literales + marcador), recorta las trazas largas tras sus primeros marcos y
// nunca toca una línea protegida.
func compressLog(content, ref string, th domain.Thresholds) (string, int) {
	lines := strings.Split(content, "\n")
	var out []string
	omitted := 0
	minRun := th.LogMinRun
	if minRun < 2 {
		minRun = 2
	}
	for i := 0; i < len(lines); {
		l := lines[i]

		// Traza de pila: cabecera y primeros marcos literales; el resto de
		// marcos consecutivos se omite.
		if traceStart.MatchString(l) {
			j := i + 1
			for j < len(lines) && (traceFrame.MatchString(lines[j]) || strings.TrimSpace(lines[j]) == "") && !traceStart.MatchString(lines[j]) {
				j++
			}
			// Las líneas en blanco finales no forman parte de la traza.
			for j > i+1 && strings.TrimSpace(lines[j-1]) == "" {
				j--
			}
			frames := j - i - 1
			if frames > traceKeepLines+minRun {
				out = append(out, lines[i:i+1+traceKeepLines]...)
				n := frames - traceKeepLines
				out = append(out, logMarker(leadingWS(lines[i+1]), ref, n, "líneas de traza"))
				omitted += n
			} else {
				out = append(out, lines[i:j]...)
			}
			i = j
			continue
		}

		if protectedLine.MatchString(l) || strings.TrimSpace(l) == "" {
			out = append(out, l)
			i++
			continue
		}

		// Serie de la misma forma (WARN solo si es idéntica).
		warn := warnLine.MatchString(l)
		shape := shapeOf(l)
		j := i + 1
		for j < len(lines) {
			lj := lines[j]
			if protectedLine.MatchString(lj) || traceStart.MatchString(lj) || strings.TrimSpace(lj) == "" {
				break
			}
			if warn {
				if lj != l {
					break
				}
			} else if warnLine.MatchString(lj) || shapeOf(lj) != shape {
				break
			}
			j++
		}
		if n := j - i; n >= minRun+2 {
			out = append(out, lines[i], logMarker(leadingWS(lines[i]), ref, n-2, "líneas similares"), lines[j-1])
			omitted += n - 2
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return strings.Join(out, "\n"), omitted
}

// compressDiff conserva cabeceras, líneas cambiadas y una línea de contexto a
// cada lado de cada cambio; las series de contexto más largas se omiten. Los
// recuentos de las cabeceras @@ se conservan literales: la salida es para
// leer, no para aplicar como parche.
func compressDiff(content, ref string, th domain.Thresholds) (string, int) {
	lines := strings.Split(content, "\n")
	isChange := func(l string) bool {
		return (strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++")) ||
			(strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"))
	}
	isHeader := func(l string) bool {
		return strings.HasPrefix(l, "diff ") || strings.HasPrefix(l, "index ") || strings.HasPrefix(l, "@@") ||
			strings.HasPrefix(l, "+++") || strings.HasPrefix(l, "---") || strings.HasPrefix(l, "new file") ||
			strings.HasPrefix(l, "deleted file") || strings.HasPrefix(l, "rename ") || strings.HasPrefix(l, "similarity ") ||
			strings.HasPrefix(l, "Binary files")
	}
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if isHeader(l) || isChange(l) || !strings.HasPrefix(l, " ") {
			keep[i] = true
			if isChange(l) {
				if i > 0 {
					keep[i-1] = true
				}
				if i+1 < len(lines) {
					keep[i+1] = true
				}
			}
		}
	}
	var out []string
	omitted := 0
	for i := 0; i < len(lines); {
		if keep[i] {
			out = append(out, lines[i])
			i++
			continue
		}
		j := i
		for j < len(lines) && !keep[j] {
			j++
		}
		if n := j - i; n >= th.LogMinRun {
			out = append(out, " "+logMarker("", ref, n, "líneas de contexto"))
			omitted += n
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return strings.Join(out, "\n"), omitted
}

// compressTable conserva la cabecera (y su separador), las primeras y las
// últimas filas y las filas protegidas; las filas intermedias se omiten.
func compressTable(content, ref string, th domain.Thresholds) (string, int) {
	lines := strings.Split(content, "\n")
	head := 1
	if len(lines) > 1 && strings.Trim(strings.TrimSpace(lines[1]), "-|:+ \t") == "" {
		head = 2
	}
	const first, last = 3, 2
	rows := len(lines) - head
	if rows <= first+last+th.LogMinRun {
		return content, 0
	}
	var out []string
	out = append(out, lines[:head+first]...)
	omitted := 0
	var run int
	flush := func() {
		if run > 0 {
			out = append(out, logMarker("", ref, run, "filas"))
			omitted += run
			run = 0
		}
	}
	tailStart := len(lines) - last
	// Una línea vacía final (salida con \n) no cuenta como fila.
	if strings.TrimSpace(lines[len(lines)-1]) == "" {
		tailStart--
	}
	for i := head + first; i < tailStart; i++ {
		if protectedLine.MatchString(lines[i]) {
			flush()
			out = append(out, lines[i])
			continue
		}
		run++
	}
	flush()
	out = append(out, lines[tailStart:]...)
	return strings.Join(out, "\n"), omitted
}
