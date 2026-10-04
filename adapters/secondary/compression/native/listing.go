package native

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"mem/domain"
)

// Compresor de listados (feature 035, research.md R6): búsquedas por texto
// (grep, rg) y listados de rutas (find, ls). Conserva literales el principio,
// el final y toda línea con señal de error; el medio se sustituye por marcas
// con la ref del original y un conteo por archivo o directorio, para que el
// agente sepa dónde mirar sin leer cientos de líneas.

var listingErrorSignal = regexp.MustCompile(`(?i)\b(error|fail|failed|panic|fatal)\b`)

func compressListing(content, ref string, _ domain.Thresholds) (string, int) {
	trailingNL := strings.HasSuffix(content, "\n")
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	head, tail := domain.ListingKeepHead, domain.ListingKeepTail
	if len(lines) <= head+tail+1 {
		return content, 0
	}
	out := append([]string{}, lines[:head]...)
	omitted := 0
	var run []string
	flush := func() {
		if len(run) == 0 {
			return
		}
		omitted += len(run)
		out = append(out, domain.RenderTextMarker(domain.Omission{Ref: ref, Summary: listingSummary(run)}))
		run = nil
	}
	for _, l := range lines[head : len(lines)-tail] {
		if listingErrorSignal.MatchString(l) {
			flush()
			out = append(out, l)
			continue
		}
		run = append(run, l)
	}
	flush()
	out = append(out, lines[len(lines)-tail:]...)
	result := strings.Join(out, "\n")
	if trailingNL {
		result += "\n"
	}
	return result, omitted
}

// listingSummary resume un tramo omitido: cuántas líneas y de qué archivos
// (los cinco más frecuentes y «+N otros»). Para rutas sueltas agrupa por
// directorio.
func listingSummary(run []string) string {
	counts := map[string]int{}
	for _, l := range run {
		t := strings.TrimSpace(l)
		key := t
		if m := listingLine.FindString(t); m != "" {
			key = m[:strings.IndexByte(m, ':')]
		} else {
			key = filepath.Dir(t)
		}
		counts[key]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var parts []string
	for i, k := range keys {
		if i == 5 {
			parts = append(parts, fmt.Sprintf("+%d otros", len(keys)-5))
			break
		}
		parts = append(parts, fmt.Sprintf("%s×%d", k, counts[k]))
	}
	return fmt.Sprintf("%d líneas omitidas: %s", len(run), strings.Join(parts, ", "))
}
