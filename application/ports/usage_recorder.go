package ports

// UsageRecorder registra una emisión de contexto ya medida. Fire-and-forget:
// nunca devuelve error (feature 020, FR-006) — medir jamás puede impedir
// emitir. La etiqueta de canal NO viaja en esta firma: se fija al construir el
// grabador, en el composition root, porque cada proceso de gomemory es
// exactamente un canal (mcp/cli/tui). Esto es lo que permite que añadir un
// canal emisor nuevo no toque ni este puerto ni ningún caso de uso (FR-017).
//
// nil es un valor válido en cualquier dependencia que lo reciba: sin
// grabador, el emisor funciona exactamente igual, sin medición.
type UsageRecorder interface {
	Record(operation string, baselineTokens, emittedTokens int)
}
