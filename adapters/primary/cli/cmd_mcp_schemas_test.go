package cli

import (
	"testing"

	"mem/adapters/secondary/tokens"
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

	// 19 operaciones verificadas contra el código real: 14 en cmd_mcp.go + 5
	// en cmd_mcp_code_tools.go (research.md §3). Si mañana se añade una
	// operación, este número debe seguir cuadrando SIN tocar código de
	// medición — por eso el test no hardcodea "19" como mágico aislado, lo
	// documenta como expectativa verificada.
	if schemaOperations != 19 {
		t.Fatalf("schemaOperations = %d, want 19 (recuento verificado en research.md)", schemaOperations)
	}
	if schemaTokens <= 0 {
		t.Fatalf("schemaTokens = %d, se esperaba un costo mayor que cero", schemaTokens)
	}
}
