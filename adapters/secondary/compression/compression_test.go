package compression

import (
	"strings"
	"testing"

	"mem/application/ports"
)

func TestNoopCompressor_Passthrough(t *testing.T) {
	c := NoopCompressor{}
	input := "Texto  con   espacios raros.\n\n\nY líneas de sobra."
	res, err := c.Compress(input, ports.CompressionOptions{Level: ports.CompressionNone})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if res.Content != input {
		t.Fatalf("NoopCompressor cambió el contenido:\nquiero: %q\nrecibí: %q", input, res.Content)
	}
	if res.Compressed {
		t.Fatalf("NoopCompressor no debería marcar Compressed=true")
	}
	if res.Tokens != res.RawTokens {
		t.Fatalf("NoopCompressor: Tokens (%d) debería == RawTokens (%d)", res.Tokens, res.RawTokens)
	}
}

func TestStructuralCompressor_CollapsesRepeatedWhitespaceAndDuplicateParagraphs(t *testing.T) {
	c := StructuralCompressor{}
	input := "## Architecture\n\nThe architecture is based on Kubernetes.\n\nThe architecture is based on Kubernetes.\n\n\n\nMás texto."
	res, err := c.Compress(input, ports.CompressionOptions{Level: ports.CompressionStructural})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if strings.Count(res.Content, "The architecture is based on Kubernetes.") != 1 {
		t.Fatalf("el párrafo duplicado debería colapsar a una sola ocurrencia, resultado: %q", res.Content)
	}
	if strings.Contains(res.Content, "\n\n\n\n") {
		t.Fatalf("no debería sobrevivir whitespace repetido: %q", res.Content)
	}
	if len(res.Content) >= len(input) {
		t.Fatalf("se esperaba que el resultado fuera más corto que el input (%d) pero midió %d", len(input), len(res.Content))
	}
	if !res.Compressed {
		t.Fatalf("se esperaba Compressed=true cuando el contenido se acorta")
	}
	if res.Tokens >= res.RawTokens {
		t.Fatalf("Tokens (%d) debería ser < RawTokens (%d) cuando el contenido se acorta", res.Tokens, res.RawTokens)
	}
}

func TestStructuralCompressor_PreservesCriticalInformation(t *testing.T) {
	c := StructuralCompressor{}
	input := "API endpoint: POST /v1/auth/refresh\n" +
		"API endpoint: POST /v1/auth/refresh\n" +
		"Timeout: 30 seconds\n" +
		"JWT expiration: 15 minutes\n" +
		"Redis key: auth:refresh\n" +
		"```go\nfunc Refresh() error { return nil }\n```\n" +
		"Ver https://example.com/docs y también /etc/gomemory/config.yaml\n" +
		"Versión: v2.3.3"

	res, err := c.Compress(input, ports.CompressionOptions{
		Level:          ports.CompressionStructural,
		PreserveCode:   true,
		PreserveURLs:   true,
		PreservePaths:  true,
		PreserveErrors: true,
	})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	mustContain := []string{
		"POST /v1/auth/refresh",
		"30 seconds",
		"15 minutes",
		"auth:refresh",
		"func Refresh() error { return nil }",
		"https://example.com/docs",
		"/etc/gomemory/config.yaml",
		"v2.3.3",
	}
	for _, want := range mustContain {
		if !strings.Contains(res.Content, want) {
			t.Errorf("la compresión perdió información crítica %q; resultado: %q", want, res.Content)
		}
	}
}

func TestStructuralCompressor_Deterministic(t *testing.T) {
	c := StructuralCompressor{}
	input := "Repetido.\n\nRepetido.\n\nAlgo distinto aquí."
	first, err := c.Compress(input, ports.CompressionOptions{Level: ports.CompressionStructural})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	second, err := c.Compress(input, ports.CompressionOptions{Level: ports.CompressionStructural})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if first.Content != second.Content {
		t.Fatalf("StructuralCompressor no es determinista:\n1: %q\n2: %q", first.Content, second.Content)
	}
}
