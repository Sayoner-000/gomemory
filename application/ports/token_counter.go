package ports

// TokenCounter mide el costo en tokens de un texto. La implementación v1
// (adapters/secondary/tokens.ApproximateTokenCounter) es una aproximación
// determinista sin dependencias externas; contadores específicos de
// proveedor son adaptadores futuros intercambiables detrás del mismo puerto
// (feature 015, research.md §4).
type TokenCounter interface {
	Count(text string) int
}
