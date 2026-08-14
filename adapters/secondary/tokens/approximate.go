// Package tokens contiene adaptadores del puerto ports.TokenCounter.
package tokens

// charsPerToken aproxima ~4 caracteres por token, la misma heurística de
// orden de magnitud que usan los proveedores de LLM más comunes para textos
// en prosa. Determinista, sin dependencias externas (feature 015,
// research.md §4) — un contador específico de proveedor es un adaptador
// futuro detrás del mismo puerto ports.TokenCounter, no un reemplazo de
// este.
const charsPerToken = 4

// ApproximateTokenCounter implementa ports.TokenCounter aproximando el
// costo en tokens por la cantidad de runas del texto. Mismo input siempre
// produce el mismo output (SC-006).
type ApproximateTokenCounter struct{}

func (ApproximateTokenCounter) Count(text string) int {
	n := len([]rune(text))
	if n == 0 {
		return 0
	}
	tokens := (n + charsPerToken - 1) / charsPerToken // división entera redondeada hacia arriba
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}
