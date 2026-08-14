package main

import (
	"strings"
	"testing"

	"mem/adapters/primary/cli"
	"mem/adapters/secondary/compression"
)

func TestCompressText_CollapsesDuplicateParagraphsAndReportsTokens(t *testing.T) {
	input := "Repetido.\n\nRepetido.\n\nAlgo distinto."
	result, err := cli.CompressText(compression.StructuralCompressor{}, input)
	if err != nil {
		t.Fatalf("CompressText: %v", err)
	}
	if strings.Count(result.Content, "Repetido.") != 1 {
		t.Fatalf("el párrafo duplicado debería colapsar a una sola ocurrencia: %q", result.Content)
	}
	if result.RawTokens <= result.Tokens {
		t.Errorf("RawTokens (%d) debería ser > Tokens (%d) tras comprimir contenido con duplicados", result.RawTokens, result.Tokens)
	}
	if !result.Compressed {
		t.Error("se esperaba Compressed=true")
	}
}
