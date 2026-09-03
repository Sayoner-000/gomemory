package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
)

// C-002 (ACR 027, reintento): antes de esta prueba, ningún test ejercitaba la
// tool MCP octopus_report como la invoca un cliente real — HandleFailure se
// probaba llamándolo directo con un domain.DelegatedResult construido a mano
// (mismo patrón de fixture-sin-llamador-real que L005 pide vigilar). Esta
// prueba levanta el servidor MCP real, activa el módulo y llama la tool con
// evidence/artifacts como lo haría un agente, comprobando que el resultado
// parcial llega hasta el TEXTO de respuesta — no solo hasta FailureDecision.
func TestOctopusReport_FALLBACK_INLINE_DevuelveResultadoParcial(t *testing.T) {
	root := t.TempDir()
	if err := persistence.WriteSettings(root, persistence.Settings{OctopusEnabled: true}); err != nil {
		t.Fatalf("WriteSettings: %v", err)
	}
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	deps := &Deps{
		Root: root, Project: "proj",
		SettingsRepo: persistence.NewSettingsRepository(),
		OctopusRepo:  persistence.NewOctopusRepository(db),
	}

	server := newMCPServer(deps, root, "proj")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	// Reintentos ya agotados (retries=1 >= DefaultMaxDelegationRetries=1) y
	// parent_can_do_it=true: la política recomienda FALLBACK_INLINE.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "octopus_report",
		Arguments: map[string]any{
			"task_id":          "T004",
			"status":           "failed",
			"retries":          1,
			"parent_can_do_it": true,
			"evidence":         []string{"store.go:88 toma el lock antes de leer"},
			"artifacts":        []string{"informe-parcial.md"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool octopus_report: %v", err)
	}

	var texto string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texto += tc.Text
		}
	}
	if !strings.Contains(texto, "FALLBACK_INLINE") {
		t.Fatalf("esperaba la recomendación FALLBACK_INLINE en la respuesta, got: %q", texto)
	}
	if !strings.Contains(texto, "store.go:88 toma el lock antes de leer") {
		t.Errorf("la evidencia recogida por el subagente debe llegar a la respuesta: %q", texto)
	}
	if !strings.Contains(texto, "informe-parcial.md") {
		t.Errorf("los artefactos producidos deben llegar a la respuesta: %q", texto)
	}
}

// A-R001 (re-juicio ACR 027): "missing" es justo el campo pensado para
// insufficient_context, pero quedó fuera de ConservaResultadoParcial y de
// renderPartialResult en el primer cierre de C-002 — se detectó en el
// re-juicio y se cerró en el mismo cambio. Prueba la superficie real con
// SOLO missing poblado (sin evidence/artifacts): debe seguir entregando el
// parcial.
func TestOctopusReport_InsufficientContext_DevuelveLoFaltante(t *testing.T) {
	root := t.TempDir()
	if err := persistence.WriteSettings(root, persistence.Settings{OctopusEnabled: true}); err != nil {
		t.Fatalf("WriteSettings: %v", err)
	}
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	deps := &Deps{
		Root: root, Project: "proj",
		SettingsRepo: persistence.NewSettingsRepository(),
		OctopusRepo:  persistence.NewOctopusRepository(db),
	}

	server := newMCPServer(deps, root, "proj")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	// Ampliación ya agotada (expansions=1 >= DefaultMaxContextExpansions=1) y
	// parent_can_do_it=true: la política recomienda FALLBACK_INLINE.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "octopus_report",
		Arguments: map[string]any{
			"task_id":          "T004",
			"status":           "insufficient_context",
			"expansions":       1,
			"parent_can_do_it": true,
			"missing":          []string{"el contenido de config/rutas.yaml"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool octopus_report: %v", err)
	}

	var texto string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texto += tc.Text
		}
	}
	if !strings.Contains(texto, "FALLBACK_INLINE") {
		t.Fatalf("esperaba la recomendación FALLBACK_INLINE en la respuesta, got: %q", texto)
	}
	if !strings.Contains(texto, "el contenido de config/rutas.yaml") {
		t.Errorf("lo que le faltó al subagente debe llegar a la respuesta, aunque no haya evidence/artifacts: %q", texto)
	}
}
