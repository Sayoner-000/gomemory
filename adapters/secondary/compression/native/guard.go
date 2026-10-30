package native

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// errLiteral indica que un compresor alteró texto que debía conservar literal.
var errLiteral = errors.New("guarda de literalidad: el compresor alteró el contenido")

// markerPattern reconoce todas las formas de marcador que emiten los
// compresores, incluido el envoltorio de comentario o de JSON que las rodea:
//
//	/* ⟦mem⟧ … */     // ⟦mem⟧ …     # ⟦mem⟧ …     -- ⟦mem⟧ …
//	{"⟦mem⟧": …}      "⟦mem⟧ …"      ⟦mem⟧ … · ref=… (o hasta fin de línea)
var markerPattern = regexp.MustCompile(
	`/\* ⟦mem⟧[^\n]*?\*/` +
		`|\{"⟦mem⟧":[^\n]*?\}(?:\})?` +
		`|"⟦mem⟧[^"\n]*"` +
		`|(?://|#|--)?[ \t]*⟦mem⟧(?:[^\n]*?· ref=[0-9a-f]+|[^\n]*)`)

// checkLiteral es la guarda común de literalidad (research.md R7, FR-011):
// quitados los marcadores, cada trozo restante de la salida tiene que aparecer
// en la entrada, en el mismo orden. Así un compresor puede omitir, pero nunca
// reescribir, parafrasear ni reordenar.
// checkKeep exige que cada literal de keep presente en el original siga en la
// salida (ports.CompressionOptions.Keep): un compresor que no conoce keep y lo
// omite deja el bloque literal en vez de esconderlo tras una marca.
func checkKeep(input, output string, keep []string) error {
	for _, k := range keep {
		if k != "" && strings.Contains(input, k) && !strings.Contains(output, k) {
			return fmt.Errorf("%w: %.80q debía conservarse", errLiteral, k)
		}
	}
	return nil
}

func checkLiteral(input, output string) error {
	cursor := 0
	for _, line := range strings.Split(output, "\n") {
		for _, piece := range markerPattern.Split(line, -1) {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			idx := strings.Index(input[cursor:], piece)
			if idx < 0 {
				return fmt.Errorf("%w: %.80q no aparece en el original tras la posición %d", errLiteral, piece, cursor)
			}
			cursor += idx + len(piece)
		}
	}
	return nil
}
