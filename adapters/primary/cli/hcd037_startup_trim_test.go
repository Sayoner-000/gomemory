package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// HCD-037 — de punta a punta, con la base real: tras un arranque que el canal
// recorta, get_context sustituye por ⟦ya entregado⟧ solo las entradas que
// llegaron completas y entrega entera la que quedó fuera. get_plan_context
// tampoco cree que el documento entero ya se entregó.
func TestArranqueRecortado_GetContextSoloSuprimeLoVisto(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	sessions := persistence.NewSessionRepository(db)
	if _, err := sessions.Start("own"); err != nil {
		t.Fatal(err)
	}
	historial := historialConColaLarga()
	deps := &Deps{
		Root: root, Project: "own", SessionRepo: sessions,
		MemoryRepo:       persistence.NewMemoryRepository(db),
		ContextBuilder:   contextBuilderFalso{salida: historial},
		CompressionLevel: ports.CompressionMax,
		DeliveryLog:      persistence.NewDeliveryLogRepository(db, "own"),
		DeliveredBlocks:  persistence.NewDeliveredBlocksRepository(db, "own", clock.SystemClock{}),
	}

	if got := entregaContextoDeArranque(deps); got != historial {
		t.Fatal("el arranque debe emitir el contexto")
	}
	if _, ok := deps.DeliveryLog.Last(ports.DeliveryContext); ok {
		t.Error("un arranque recortado no debe registrar el canal context")
	}

	cs := connectMCPTestSession(t, deps, root, "own")
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_context", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("get_context: %v %q", err, toolText(res))
	}
	got := toolText(res)
	if !strings.Contains(got, "2 entradas ya entregadas") {
		t.Errorf("A y B llegaron en el arranque y deben sustituirse:\n%.400s", got)
	}
	if !strings.Contains(got, "- **C**: tres "+strings.Repeat("x", 100)) {
		t.Errorf("C quedó fuera del recorte y debe llegar entera:\n%.400s", got)
	}
	if strings.Contains(got, domain.HookTrimNotice) {
		t.Error("get_context no debe recortarse como el canal de arranque")
	}
}
