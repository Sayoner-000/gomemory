package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// TestMemoryResource_ResuelveMemoriaFueraDeLaVentanaReciente: el recurso
// mem://memory/{id} buscaba dentro de List(project, 200), así que una memoria
// más antigua que las 200 más recientes respondía "no encontrada" aunque
// existiera.
func TestMemoryResource_ResuelveMemoriaFueraDeLaVentanaReciente(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	memRepo := persistence.NewMemoryRepository(db)
	var primera int64
	for i := 0; i < 205; i++ {
		id, err := memRepo.Insert(&domain.Memory{
			Project: "proj", Type: domain.Learning,
			Title: fmt.Sprintf("memoria %d", i), Content: fmt.Sprintf("contenido %d", i),
		})
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
		if i == 0 {
			primera = id
		}
	}
	// Todas se insertan en el mismo segundo y el empate en created_at deja el
	// orden al azar; se envejece la primera para que quede fuera de las 200
	// más recientes, como ocurre en una base real.
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now', '-30 days') WHERE id = ?`, primera); err != nil {
		t.Fatalf("envejecer memoria: %v", err)
	}

	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:  memRepo,
		SessionRepo: persistence.NewSessionRepository(db),
	}
	cs := connectMCPTestSession(t, deps, root, "proj")

	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: fmt.Sprintf("mem://memory/%d", primera),
	})
	if err != nil {
		t.Fatalf("ReadResource de la memoria %d: %v", primera, err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "contenido 0") {
		t.Errorf("contenido inesperado: %+v", res.Contents)
	}

	if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: "mem://memory/999999",
	}); err == nil {
		t.Error("una memoria inexistente debe devolver error")
	}
}
