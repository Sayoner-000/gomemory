package ports

// CompressionLevel es la agresividad de la compresión pedida. Solo None y
// Structural están respaldados por un adaptador en la primera implementación
// (feature 015, research.md §5) — Semantic/Aggressive quedan reservados para
// adaptadores futuros (p. ej. un compresor basado en LLM) sin cambiar esta
// interfaz.
//
// CompressionStructural es el valor cero a propósito: FR-009 exige que la
// compresión determinista sea el comportamiento por defecto, así que un
// ContextRequest construido sin fijar Compression explícitamente (Go zero
// value) debe comprimir, no dejar de comprimir. CompressionNone exige una
// decisión explícita (p. ej. --no-compress en el CLI).
type CompressionLevel int

const (
	CompressionStructural CompressionLevel = iota
	CompressionNone
)

// CompressionOptions controla qué preserva el compresor. Preserve* default a
// true en los llamadores de producción (FR-009): la compresión determinista
// nunca debe tocar código, URLs, rutas ni mensajes de error salvo que se pida
// explícitamente lo contrario.
type CompressionOptions struct {
	Level          CompressionLevel
	PreserveCode   bool
	PreserveURLs   bool
	PreservePaths  bool
	PreserveErrors bool
}

// CompressionResult es la salida de Compress: el contenido final (igual al
// original si no se comprimió) más el costo en tokens antes/después.
type CompressionResult struct {
	Content    string
	RawTokens  int
	Tokens     int
	Compressed bool
}

// Compressor acorta contenido de forma reversible: nunca sobreescribe el
// original (el llamador decide si lo descarta), y ante un error el llamador
// debe seguir con el contenido original en vez de abortar (FR-010/FR-011).
type Compressor interface {
	Compress(input string, opts CompressionOptions) (CompressionResult, error)
}
