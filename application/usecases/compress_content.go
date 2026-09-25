package usecases

import "mem/application/ports"

// CompressContent es la fachada de compresión para los puntos de entrega de
// gomemory (feature 033): fija el nivel y las garantías Preserve* y registra el
// uso. La privacidad (FR-024) y las estadísticas por compresor NO viven aquí,
// sino en el propio compresor (native.Engine), para que las cumpla cualquier
// llamador del puerto, incluidos BuildContextPack y el paquete de Octopus.
//
// Un error del compresor nunca deja al agente sin contenido: se entrega el
// original (FR-006/FR-011 de la feature 015).
func CompressContent(c ports.Compressor, level ports.CompressionLevel, text string, recorder ports.UsageRecorder, op string) ports.CompressionResult {
	res, err := c.Compress(text, ports.CompressionOptions{
		Level:          level,
		PreserveCode:   true,
		PreserveURLs:   true,
		PreservePaths:  true,
		PreserveErrors: true,
	})
	if err != nil {
		n := (len([]rune(text)) + 3) / 4
		return ports.CompressionResult{Content: text, RawTokens: n, Tokens: n, Compressor: "none", FallbackReason: "error"}
	}
	if recorder != nil && op != "" {
		recorder.Record(op, res.RawTokens, res.Tokens)
	}
	return res
}
