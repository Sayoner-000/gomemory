package native

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"mem/domain"
)

// Compresor de datos estructurados (research.md R3, práctica SmartCrusher).
//
// Trabaja sobre los BYTES originales: localiza los tramos de cada elemento de
// cada array y copia literalmente los que conserva. Así el formato, la
// sangría y el orden de claves no cambian, y la guarda de literalidad se
// cumple por construcción. Los elementos que se omiten se sustituyen por un
// único objeto marcador en la posición del hueco.

// jsonMarker es la forma de omisión dentro de un array de objetos. La clave
// ⟦mem⟧ va primero a propósito: la guarda reconoce el marcador por su inicio.
func jsonMarker(summary, ref string, campos map[string]string) string {
	var b strings.Builder
	b.WriteString(`{"` + domain.MarkerTag + `": `)
	b.WriteString(strconv.Quote(summary))
	if ref != "" {
		b.WriteString(`, "ref": ` + strconv.Quote(ref))
	}
	if len(campos) > 0 {
		keys := make([]string, 0, len(campos))
		for k := range campos {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(`, "campos": {`)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Quote(k) + ": " + strconv.Quote(campos[k]))
		}
		b.WriteString("}")
	}
	b.WriteString("}")
	return b.String()
}

// jsonStringMarker es la forma de omisión dentro de un array de escalares: un
// string más, para que el array siga siendo homogéneo y JSON válido.
func jsonStringMarker(summary, ref string) string {
	return strconv.Quote(domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: summary}))
}

// compressJSON comprime un documento JSON válido o un bloque de JSON Lines.
func compressJSON(content, lang, ref string, th domain.Thresholds) (string, int) {
	if lang == "jsonl" {
		return compressJSONLines(content, ref, th)
	}
	c := jsonCrusher{src: content, ref: ref, th: th}
	var out strings.Builder
	start := skipWS(content, 0)
	out.WriteString(content[:start])
	end := c.value(&out, start)
	out.WriteString(content[end:])
	return out.String(), c.omitted
}

type jsonCrusher struct {
	src     string
	ref     string
	th      domain.Thresholds
	omitted int
}

// value copia (comprimido) el valor que empieza en i y devuelve su final.
func (c *jsonCrusher) value(out *strings.Builder, i int) int {
	switch c.src[i] {
	case '[':
		return c.array(out, i)
	case '{':
		return c.object(out, i)
	default:
		end := valueEnd(c.src, i)
		out.WriteString(c.src[i:end])
		return end
	}
}

// object copia un objeto literal, comprimiendo recursivamente sus valores.
func (c *jsonCrusher) object(out *strings.Builder, i int) int {
	j := i + 1
	for {
		k := skipWS(c.src, j)
		if c.src[k] == '}' {
			out.WriteString(c.src[i : k+1])
			return k + 1
		}
		// clave
		kEnd := valueEnd(c.src, k)
		colon := skipWS(c.src, kEnd) // ':'
		v := skipWS(c.src, colon+1)
		out.WriteString(c.src[i:v])
		vEnd := c.value(out, v)
		next := skipWS(c.src, vEnd)
		if c.src[next] == '}' {
			out.WriteString(c.src[vEnd : next+1])
			return next + 1
		}
		// ','
		i = vEnd
		j = next + 1
		out.WriteString(c.src[vEnd:j])
		i = j
	}
}

type span struct{ start, end int }

// array decide qué elementos conservar y copia los tramos elegidos.
func (c *jsonCrusher) array(out *strings.Builder, i int) int {
	var elems []span
	j := skipWS(c.src, i+1)
	if c.src[j] == ']' {
		out.WriteString(c.src[i : j+1])
		return j + 1
	}
	for {
		e := valueEnd(c.src, j)
		elems = append(elems, span{j, e})
		k := skipWS(c.src, e)
		if c.src[k] == ']' {
			j = k
			break
		}
		j = skipWS(c.src, k+1)
	}
	closeIdx := j

	keep := c.selectKeep(elems)
	objects := c.src[elems[0].start] == '{'

	out.WriteString(c.src[i:elems[0].start])
	for idx := 0; idx < len(elems); {
		if keep[idx] {
			c.value(out, elems[idx].start)
			if idx+1 < len(elems) {
				out.WriteString(c.src[elems[idx].end:elems[idx+1].start])
			}
			idx++
			continue
		}
		run := idx
		for run < len(elems) && !keep[run] {
			run++
		}
		n := run - idx
		c.omitted += n
		summary := fmt.Sprintf("omitidos %d de %d elementos", n, len(elems))
		if objects {
			out.WriteString(jsonMarker(summary, c.ref, c.fieldSummary(elems[idx:run])))
		} else {
			out.WriteString(jsonStringMarker(summary, c.ref))
		}
		if run < len(elems) {
			out.WriteString(c.src[elems[run-1].end:elems[run].start])
		}
		idx = run
	}
	out.WriteString(c.src[elems[len(elems)-1].end : closeIdx+1])
	return closeIdx + 1
}

// selectKeep aplica la regla de SmartCrusher: extremos, elementos con señal de
// error, valores numéricos atípicos (más de 3σ) y valores categóricos raros de
// campos de baja cardinalidad. La selección principal es estadística; las
// señales de error son un suelo de seguridad.
func (c *jsonCrusher) selectKeep(elems []span) []bool {
	n := len(elems)
	keep := make([]bool, n)
	minItems := c.th.JSONMinItems
	if minItems <= 0 || n < minItems {
		for i := range keep {
			keep[i] = true
		}
		return keep
	}
	keep[0], keep[n-1] = true, true

	values := make([]any, n)
	for i, e := range elems {
		_ = json.Unmarshal([]byte(c.src[e.start:e.end]), &values[i])
		if hasErrorSignal(values[i]) {
			keep[i] = true
		}
	}

	// Estadística por campo (solo claves de primer nivel de objetos).
	numeric := map[string][]float64{}
	numericIdx := map[string][]int{}
	cats := map[string]map[string][]int{}
	for i, v := range values {
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for k, fv := range obj {
			switch x := fv.(type) {
			case float64:
				numeric[k] = append(numeric[k], x)
				numericIdx[k] = append(numericIdx[k], i)
			case string, bool:
				if cats[k] == nil {
					cats[k] = map[string][]int{}
				}
				key := fmt.Sprint(x)
				cats[k][key] = append(cats[k][key], i)
			}
		}
	}
	for k, xs := range numeric {
		if len(xs) < minItems {
			continue
		}
		mean, sd := meanStd(xs)
		if sd == 0 {
			continue
		}
		for j, x := range xs {
			if math.Abs(x-mean) > 3*sd {
				keep[numericIdx[k][j]] = true
			}
		}
	}
	maxCard := n / 10
	if maxCard < 5 {
		maxCard = 5
	}
	for _, byVal := range cats {
		if len(byVal) > maxCard || len(byVal) < 2 {
			continue
		}
		for _, idxs := range byVal {
			if len(idxs)*100 <= n { // frecuencia ≤ 1 %
				for _, i := range idxs {
					keep[i] = true
				}
			}
		}
	}
	return keep
}

// fieldSummary resume, para los campos de baja cardinalidad, la distribución
// de valores de los elementos omitidos (p. ej. "status": "200×185, 404×2").
func (c *jsonCrusher) fieldSummary(elems []span) map[string]string {
	counts := map[string]map[string]int{}
	for _, e := range elems {
		var v any
		if json.Unmarshal([]byte(c.src[e.start:e.end]), &v) != nil {
			continue
		}
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for k, fv := range obj {
			switch fv.(type) {
			case string, bool, float64:
			default:
				continue
			}
			s := sanitize(fmt.Sprint(fv))
			if len(s) > 24 {
				continue
			}
			if counts[k] == nil {
				counts[k] = map[string]int{}
			}
			counts[k][s]++
		}
	}
	out := map[string]string{}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys) // determinismo: el orden de un mapa de Go es aleatorio
	for _, k := range keys {
		byVal := counts[k]
		if len(byVal) > 3 || len(out) >= 3 {
			continue
		}
		vals := make([]string, 0, len(byVal))
		for v := range byVal {
			vals = append(vals, v)
		}
		sort.Slice(vals, func(a, b int) bool {
			if byVal[vals[a]] != byVal[vals[b]] {
				return byVal[vals[a]] > byVal[vals[b]]
			}
			return vals[a] < vals[b]
		})
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = fmt.Sprintf("%s×%d", v, byVal[v])
		}
		out[k] = strings.Join(parts, ", ")
	}
	return out
}

// sanitize quita del resumen los caracteres que romperían la forma del marcador.
func sanitize(s string) string {
	return strings.NewReplacer("{", "", "}", "", "\"", "", "\n", " ").Replace(s)
}

var errorKeys = map[string]bool{"error": true, "err": true, "exception": true, "failed": true, "failure": true, "panic": true}

// hasErrorSignal es el suelo de seguridad: nunca se omite un elemento que
// declara un error.
func hasErrorSignal(v any) bool {
	obj, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for k, fv := range obj {
		lk := strings.ToLower(k)
		if errorKeys[lk] {
			switch x := fv.(type) {
			case nil:
			case bool:
				if x {
					return true
				}
			case string:
				if x != "" {
					return true
				}
			default:
				return true
			}
		}
		switch x := fv.(type) {
		case string:
			lx := strings.ToLower(x)
			if (lk == "level" || lk == "severity" || lk == "action" || lk == "status" || lk == "state") &&
				(lx == "error" || lx == "fatal" || lx == "fail" || lx == "failed" || lx == "panic") {
				return true
			}
			if strings.Contains(x, "panic:") || strings.Contains(x, "--- FAIL") || strings.HasPrefix(x, "FAIL") {
				return true
			}
		case float64:
			if (lk == "status" || lk == "status_code" || lk == "code") && x >= 400 && x < 600 {
				return true
			}
		case bool:
			if (lk == "ok" || lk == "success") && !x {
				return true
			}
		}
	}
	return false
}

func meanStd(xs []float64) (float64, float64) {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var sq float64
	for _, x := range xs {
		sq += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(sq / float64(len(xs)))
}

// compressJSONLines trata cada línea como un elemento: se conservan las mismas
// líneas que conservaría un array y las series omitidas pasan a una línea
// marcador.
func compressJSONLines(content, ref string, th domain.Thresholds) (string, int) {
	lines := strings.SplitAfter(content, "\n")
	var elems []span
	var idxs []int
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			idxs = append(idxs, i)
		}
	}
	// Reutiliza la selección del array construyendo un documento virtual.
	var doc strings.Builder
	doc.WriteString("[")
	for n, i := range idxs {
		if n > 0 {
			doc.WriteString(",")
		}
		s := doc.Len()
		doc.WriteString(strings.TrimSpace(lines[i]))
		elems = append(elems, span{s, doc.Len()})
	}
	doc.WriteString("]")
	c := jsonCrusher{src: doc.String(), ref: ref, th: th}
	if len(elems) == 0 {
		return content, 0
	}
	keep := c.selectKeep(elems)
	keepLine := map[int]bool{}
	for n, i := range idxs {
		keepLine[i] = keep[n]
	}
	var out strings.Builder
	omitted := 0
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" || keepLine[i] {
			out.WriteString(lines[i])
			i++
			continue
		}
		j := i
		var run []span
		for j < len(lines) && strings.TrimSpace(lines[j]) != "" && !keepLine[j] {
			for n, k := range idxs {
				if k == j {
					run = append(run, elems[n])
				}
			}
			j++
		}
		omitted += j - i
		out.WriteString(jsonMarker(fmt.Sprintf("omitidas %d de %d líneas", j-i, len(idxs)), ref, c.fieldSummary(run)))
		out.WriteString("\n")
		i = j
	}
	return out.String(), omitted
}

func skipWS(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t' || s[i] == '\r') {
		i++
	}
	return i
}

// valueEnd devuelve el índice posterior al valor JSON que empieza en i. La
// entrada ya pasó json.Valid, así que el escáner puede ser mínimo.
func valueEnd(s string, i int) int {
	switch s[i] {
	case '"':
		for j := i + 1; j < len(s); j++ {
			switch s[j] {
			case '\\':
				j++
			case '"':
				return j + 1
			}
		}
		return len(s)
	case '{', '[':
		depth := 0
		for j := i; j < len(s); j++ {
			switch s[j] {
			case '"':
				j = valueEnd(s, j) - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return j + 1
				}
			}
		}
		return len(s)
	default:
		j := i
		for j < len(s) && !strings.ContainsRune(",}] \n\t\r", rune(s[j])) {
			j++
		}
		return j
	}
}
