// Package compression contiene los adaptadores del puerto ports.Compressor.
package compression

import "mem/application/ports"

// NoopCompressor implementa ports.Compressor sin tocar el contenido. Es el
// compresor por defecto cuando CompressionOptions.Level == CompressionNone
// (feature 015, research.md §5).
type NoopCompressor struct{}

func (NoopCompressor) Compress(input string, _ ports.CompressionOptions) (ports.CompressionResult, error) {
	t := approxTokens(input)
	return ports.CompressionResult{
		Content:    input,
		RawTokens:  t,
		Tokens:     t,
		Compressed: false,
	}, nil
}
