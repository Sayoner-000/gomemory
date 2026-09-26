package native

import (
	"fmt"
	"regexp"
	"strings"

	"mem/domain"
)

// Compresor de prosa (research.md R6): extractivo y sin reescritura. Sustituye
// a Kompress, que es un modelo, por reglas deterministas: elimina frases
// repetidas y, en párrafos largos, conserva las primeras y las últimas frases
// y las que llevan elementos protegidos.

var (
	sentenceEnd = regexp.MustCompile(`[.!?…](\s+|$)`)
	// protectedToken: código en línea, rutas, URLs, versiones, números con
	// unidades e identificadores CamelCase.
	protectedToken = regexp.MustCompile("`[^`]+`|https?://\\S+|(\\w+/)+\\w+|\\bv?\\d+\\.\\d+(\\.\\d+)?\\b|\\b[a-z]+[A-Z]\\w*\\b|\\b[A-Z][a-z]+[A-Z]\\w*\\b|\\d+\\s?(ms|s|MB|KB|GB|%)")
	normSentence   = regexp.MustCompile(`\s+`)
)

const proseKeepHead, proseKeepTail = 3, 2

// splitSentences parte una línea en frases conservando el separador pegado a
// cada frase, para que unirlas reproduzca el texto literal.
func splitSentences(line string) []string {
	var out []string
	prev := 0
	for _, loc := range sentenceEnd.FindAllStringIndex(line, -1) {
		out = append(out, line[prev:loc[1]])
		prev = loc[1]
	}
	if prev < len(line) {
		out = append(out, line[prev:])
	}
	return out
}

// compressProse trabaja línea a línea: una frase ya vista (normalizada) se
// quita; una línea con más de ProseMaxSentences frases conserva cabeza, cola y
// frases protegidas. Líneas de lista, títulos y tablas nunca se recortan por
// dentro: solo pueden eliminarse enteras si son duplicadas.
func compressProse(content, ref string, th domain.Thresholds) (string, int) {
	lines := strings.Split(content, "\n")
	seen := map[string]bool{}
	var out []string
	omitted := 0
	// dupRun cuenta las líneas duplicadas seguidas que comparten una marca.
	dupRun := 0
	for _, line := range lines {
		t := strings.TrimSpace(line)
		// Las líneas en blanco y las marcas de gomemory (de omisión o de
		// entrega) se conservan tal cual: una marca ocupa el lugar de un
		// contenido concreto y nunca es un duplicado (C-002 de acr_6793454b).
		if t == "" || domain.IsMarkerLine(t) {
			dupRun = 0
			out = append(out, line)
			continue
		}
		key := strings.ToLower(normSentence.ReplaceAllString(t, " "))
		structural := strings.HasPrefix(t, "#") || strings.HasPrefix(t, "-") || strings.HasPrefix(t, "*") ||
			strings.HasPrefix(t, "|") || strings.HasPrefix(t, ">") || (len(t) > 1 && t[0] >= '0' && t[0] <= '9' && strings.Contains(t[:min(4, len(t))], "."))
		if structural || len(t) < 40 {
			if len(t) >= 40 && seen[key] {
				// Toda omisión deja su marca (FR-012); las seguidas comparten una.
				omitted++
				dupRun++
				marker := domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("%d líneas duplicadas omitidas", dupRun)})
				if dupRun > 1 {
					out[len(out)-1] = marker
				} else {
					out = append(out, marker)
				}
				continue
			}
			dupRun = 0
			seen[key] = true
			out = append(out, line)
			continue
		}
		dupRun = 0

		sentences := splitSentences(line)
		var kept []string
		dropped := 0
		for i, s := range sentences {
			sk := strings.ToLower(normSentence.ReplaceAllString(strings.TrimSpace(s), " "))
			dup := sk != "" && seen[sk]
			seen[sk] = true
			long := th.ProseMaxSentences > 0 && len(sentences) > th.ProseMaxSentences
			middle := i >= proseKeepHead && i < len(sentences)-proseKeepTail
			if dup || (long && middle && !protectedToken.MatchString(s)) {
				dropped++
				continue
			}
			if dropped > 0 && len(kept) > 0 {
				kept = append(kept, domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("%d frases omitidas", dropped)})+" ")
				omitted += dropped
				dropped = 0
			}
			kept = append(kept, s)
		}
		if dropped > 0 {
			kept = append(kept, " "+domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: fmt.Sprintf("%d frases omitidas", dropped)}))
			omitted += dropped
		}
		if len(kept) == 0 {
			continue
		}
		out = append(out, strings.Join(kept, ""))
	}
	return strings.Join(out, "\n"), omitted
}
