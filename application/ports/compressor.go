package ports

import "mem/domain"

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
	// CompressionMax activa el motor nativo completo (feature 033): router por
	// tipo de contenido, compresores con pérdida recuperable y marcadores ⟦mem⟧.
	// Va al final del iota a propósito: Structural sigue siendo el valor cero y
	// None conserva su valor.
	CompressionMax
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

	// Campos opcionales del motor nativo (feature 033, data-model.md). Los
	// llamadores anteriores solo leen los cuatro de arriba; un compresor que no
	// los rellena los deja en su valor cero.
	//
	// Compressor nombra la etapa que produjo Content: "none", "structural",
	// "json", "code", "log", "diff", "table", "prose" o "mixed".
	Compressor string
	// ContentType es el tipo detectado por el router (domain.ContentType).
	ContentType string
	// StructuralTokens son los tokens que habría dado la compresión estructural
	// sola: permite informar del ahorro por etapa.
	StructuralTokens int
	// Refs son las referencias de los originales omitidos, recuperables con
	// pack_retrieve / mem pack retrieve.
	Refs []string
	// FallbackReason explica por qué se entregó la compresión estructural en vez
	// de la del motor: "", "error", "literal_guard", "no_gain", "too_large",
	// "store_unavailable" o "private".
	FallbackReason string
	LatencyMicros  int64
	// Omissions cuenta los elementos, líneas o frases omitidos (con marcador).
	Omissions int
	// BlockOmissions desglosa Omissions por el compresor y el tipo de cada
	// bloque omitido: es la clave con la que se guarda su original y con la que
	// se cuentan sus recuperaciones, así la tasa de FR-027 compara lo mismo.
	BlockOmissions []BlockOmission
}

// BlockOmission son las omisiones de un bloque con su compresor y su tipo.
type BlockOmission struct {
	Compressor  string
	ContentType string
	Omissions   int
}

// Compressor acorta contenido de forma reversible: nunca sobreescribe el
// original (el llamador decide si lo descarta), y ante un error el llamador
// debe seguir con el contenido original en vez de abortar (FR-010/FR-011).
type Compressor interface {
	Compress(input string, opts CompressionOptions) (CompressionResult, error)
}

// CompressionLevelFromSetting traduce el nivel efectivo de los ajustes
// ("none" | "structural" | "max", ya resuelto con domain.ParseCompressionLevel)
// al nivel del puerto. Cualquier otro valor es structural, el comportamiento
// de la v2.25.0.
func CompressionLevelFromSetting(level string) CompressionLevel {
	switch level {
	case domain.CompressionLevelNone:
		return CompressionNone
	case domain.CompressionLevelMax:
		return CompressionMax
	default:
		return CompressionStructural
	}
}
