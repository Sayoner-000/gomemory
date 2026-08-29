package cli

import (
	"testing"

	"mem/adapters/secondary/tokens"
	"mem/domain"
)

// TestMeasurePublishedSchemas_CountsRealServer levanta el servidor MCP REAL
// sobre un transporte en memoria (sin stdio, sin proceso aparte) y mide lo
// que efectivamente publica — no una réplica escrita a mano de las 19
// llamadas mcp.AddTool, que el SDK no permite extraer de forma homogénea
// (research.md §3: cada una usa una struct anónima distinta como parámetro
// de tipo genérico).
func TestMeasurePublishedSchemas_CountsRealServer(t *testing.T) {
	deps := &Deps{Project: "proj"}

	schemaTokens, schemaOperations, err := measurePublishedSchemas(deps, "", "proj", tokens.ApproximateTokenCounter{})
	if err != nil {
		t.Fatalf("measurePublishedSchemas: %v", err)
	}

	// DERIVADO de domain.MCPAllTools(), no un número escrito a mano.
	//
	// Este total se subió a mano tres veces en una sola sesión (19 → 24 → 26 →
	// 27) según se añadían tools. Un número que hay que recordar actualizar es
	// un número que algún día no se actualiza, y entonces el test se relaja
	// «para que pase» en vez de avisar. Derivarlo convierte el recuento en la
	// invariante que de verdad importa: TODA tool declarada en el dominio está
	// efectivamente publicada por el servidor. Registrar una y olvidar
	// declararla —o al revés— ahora falla solo.
	esperadas := len(domain.MCPAllTools())
	if schemaOperations != esperadas {
		t.Fatalf("schemaOperations = %d, se esperaban %d (len(domain.MCPAllTools())): "+
			"una tool registrada sin declarar en el dominio, o declarada sin registrar",
			schemaOperations, esperadas)
	}
	if schemaTokens <= 0 {
		t.Fatalf("schemaTokens = %d, se esperaba un costo mayor que cero", schemaTokens)
	}
}
