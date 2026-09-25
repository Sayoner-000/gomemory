package domain

import "time"

// Valores de fábrica del motor nativo de compresión (feature 033,
// data-model.md). Es la ÚNICA fuente de estas cifras: los ajustes del proyecto
// solo las sobrescriben cuando traen un valor positivo.
const (
	// CompressionMinTokens: por debajo, el bloque sale intacto (FR-004). 64
	// tokens ≈ 50 palabras, el mismo orden que min_input_words de Headroom.
	CompressionMinTokens = 64
	// CompressionOriginalsTTLDays: días sin uso tras los que caduca un original.
	CompressionOriginalsTTLDays = 7
	// CompressionOriginalsMaxBytes: tope del almacén de originales (LRU).
	CompressionOriginalsMaxBytes = 256 << 20
	// CompressionAdaptiveThreshold: tasa de recuperación a partir de la cual se
	// baja un escalón la agresividad de un tipo de contenido (FR-027).
	CompressionAdaptiveThreshold = 0.20
	// CompressionAdaptiveMinOmissions: muestra mínima para decidir un ajuste.
	CompressionAdaptiveMinOmissions = 20
	// CompressionAdaptiveWindowDays: ventana de la tasa de recuperación.
	CompressionAdaptiveWindowDays = 7
	// ToolOutputMinTokens: por debajo, el hook de salidas de herramientas no
	// actúa (no compensa su coste).
	ToolOutputMinTokens = 400
	// CompressionMaxInputBytes: por encima, solo compresión estructural (R14).
	CompressionMaxInputBytes = 2 << 20
	// HookBudget: presupuesto total del hook de salidas de herramientas.
	HookBudget = 150 * time.Millisecond
	// StoreTimeout: plazo del puente entre puertos sin ctx y los puertos nuevos
	// con ctx (excepción E-002 de la constitución): nunca se llama a un
	// repositorio nuevo con context.Background() sin plazo.
	StoreTimeout = 500 * time.Millisecond
)

// Niveles de compresión tal como se guardan en los ajustes (FR-030).
const (
	CompressionLevelNone       = "none"
	CompressionLevelStructural = "structural"
	CompressionLevelMax        = "max"
)

// ParseCompressionLevel resuelve el nivel efectivo a partir del ajuste
// context_compression_level y del heredado context_compression_disabled
// (research.md R13):
//
//   - valor válido            → ese valor
//   - ausente y legacy=true   → "none" (el ajuste heredado sigue mandando)
//   - ausente o inválido      → "structural" (comportamiento de la v2.25.0)
func ParseCompressionLevel(level string, legacyDisabled bool) string {
	switch level {
	case CompressionLevelNone, CompressionLevelStructural, CompressionLevelMax:
		return level
	}
	if level == "" && legacyDisabled {
		return CompressionLevelNone
	}
	return CompressionLevelStructural
}

// ValidCompressionLevel dice si s es uno de los tres niveles admitidos.
func ValidCompressionLevel(s string) bool {
	switch s {
	case CompressionLevelNone, CompressionLevelStructural, CompressionLevelMax:
		return true
	}
	return false
}

// Aggressiveness es el escalón de agresividad de un tipo de contenido en un
// proyecto: 3 es el valor inicial (máxima) y 0 equivale a solo estructural. El
// ajuste adaptativo solo la baja; nunca la sube sola (R12).
type Aggressiveness int

const (
	AggressivenessMax Aggressiveness = 3
	AggressivenessOff Aggressiveness = 0
)

// Thresholds son los umbrales que cada compresor recibe según el escalón.
type Thresholds struct {
	// JSONMinItems: arrays con menos elementos no se recortan.
	JSONMinItems int
	// CodeMinBodyLines: cuerpos más cortos se conservan enteros.
	CodeMinBodyLines int
	// LogMinRun: series de líneas similares más cortas no se colapsan.
	LogMinRun int
	// ProseMaxSentences: párrafos más cortos se conservan enteros.
	ProseMaxSentences int
	// Enabled=false significa escalón 0: solo compresión estructural.
	Enabled bool
}

// ThresholdsFor devuelve los umbrales de un escalón (data-model.md). Un valor
// fuera de rango se trata como el más cercano.
func ThresholdsFor(a Aggressiveness) Thresholds {
	switch {
	case a >= 3:
		return Thresholds{JSONMinItems: 8, CodeMinBodyLines: 6, LogMinRun: 3, ProseMaxSentences: 8, Enabled: true}
	case a == 2:
		return Thresholds{JSONMinItems: 16, CodeMinBodyLines: 12, LogMinRun: 5, ProseMaxSentences: 12, Enabled: true}
	case a == 1:
		return Thresholds{JSONMinItems: 32, CodeMinBodyLines: 25, LogMinRun: 10, ProseMaxSentences: 20, Enabled: true}
	default:
		return Thresholds{}
	}
}
