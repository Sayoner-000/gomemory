package cli

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
)

// connectMCPTestSession arranca el servidor MCP real sobre un transporte en
// memoria y devuelve una sesión de cliente conectada, lista para CallTool.
// Mismo patrón que TestMCPServer_SearchAndList_RecordUsage.
func connectMCPTestSession(t *testing.T, deps *Deps, root, project string) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(deps, root, project)
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func toolText(res *mcp.CallToolResult) string {
	var out string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			out += tc.Text
		}
	}
	return out
}

// TestSaveSessionSummary_ActualizaSinCerrarSesionActiva cubre US2: la
// herramienta debe existir, y con una sesión activa debe persistir el
// resumen SIN cerrarla (a diferencia de end_session).
func TestSaveSessionSummary_ActualizaSinCerrarSesionActiva(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	sessRepo := persistence.NewSessionRepository(db)
	sess, err := sessRepo.Start("proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:       persistence.NewMemoryRepository(db),
		SessionRepo:      sessRepo,
		SessionSummaries: sessRepo.(ports.SessionSummaryUpdater),
	}

	cs := connectMCPTestSession(t, deps, root, "proj")
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "save_session_summary",
		Arguments: map[string]any{"summary": "Trabajo en curso: feature 030"},
	})
	if err != nil {
		t.Fatalf("CallTool save_session_summary: %v", err)
	}
	if res.IsError {
		t.Fatalf("save_session_summary con sesión activa no debía marcar error: %s", toolText(res))
	}

	active, err := sessRepo.Active("proj")
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil {
		t.Fatal("la sesión debía seguir activa tras save_session_summary")
	}
	if active.ID != sess.ID {
		t.Fatalf("save_session_summary no debía abrir una sesión nueva: activa=%s original=%s", active.ID, sess.ID)
	}
	if active.Summary != "Trabajo en curso: feature 030" {
		t.Errorf("summary tras la tool = %q", active.Summary)
	}
}

// TestSaveSessionSummary_SinSesionActivaAbreUna cubre el cierre de R4: si
// save_session_summary se llama sin sesión activa (p. ej. tras un
// end_session accidental), debe abrir una y guardar el resumen ahí, en vez
// de fallar o perder el resumen.
func TestSaveSessionSummary_SinSesionActivaAbreUna(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	sessRepo := persistence.NewSessionRepository(db)
	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:       persistence.NewMemoryRepository(db),
		SessionRepo:      sessRepo,
		SessionSummaries: sessRepo.(ports.SessionSummaryUpdater),
	}

	cs := connectMCPTestSession(t, deps, root, "proj")
	ctx := context.Background()

	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "save_session_summary",
		Arguments: map[string]any{"summary": "resumen sin sesión previa"},
	}); err != nil {
		t.Fatalf("CallTool save_session_summary: %v", err)
	}

	active, err := sessRepo.Active("proj")
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil {
		t.Fatal("save_session_summary debía abrir una sesión cuando no había ninguna activa")
	}
	if active.Summary != "resumen sin sesión previa" {
		t.Errorf("summary = %q", active.Summary)
	}
}

// TestSaveSessionSummary_VacioEsError cubre la validación: un resumen vacío
// no debe persistirse silenciosamente.
func TestSaveSessionSummary_VacioEsError(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	sessRepo := persistence.NewSessionRepository(db)
	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:       persistence.NewMemoryRepository(db),
		SessionRepo:      sessRepo,
		SessionSummaries: sessRepo.(ports.SessionSummaryUpdater),
	}

	cs := connectMCPTestSession(t, deps, root, "proj")
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "save_session_summary",
		Arguments: map[string]any{"summary": ""},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("save_session_summary con summary vacío debía responder un error")
	}
}

// TestSaveSessionSummary_DescripcionAcotada protege la huella (feature 008):
// la descripción de la tool no debe engordar el bootstrap de ToolSearch sin
// límite.
func TestSaveSessionSummary_DescripcionAcotada(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	sessRepo := persistence.NewSessionRepository(db)
	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:       persistence.NewMemoryRepository(db),
		SessionRepo:      sessRepo,
		SessionSummaries: sessRepo.(ports.SessionSummaryUpdater),
	}

	server := newMCPServer(deps, root, "proj")
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer func() { _ = serverSession.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	tools, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "save_session_summary" {
			found = true
			if len(tool.Description) > 250 {
				t.Errorf("descripción de save_session_summary tiene %d caracteres, tope 250: %q",
					len(tool.Description), tool.Description)
			}
		}
	}
	if !found {
		t.Fatal("save_session_summary debe estar registrada en el servidor")
	}
}
