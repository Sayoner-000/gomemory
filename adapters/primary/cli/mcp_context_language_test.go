package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

type mcpDummyMemoryLister struct{}

func (d *mcpDummyMemoryLister) List(project string, limit int) ([]domain.Memory, error) {
	return []domain.Memory{}, nil
}

type mcpDummySessionQuerier struct{}

func (d *mcpDummySessionQuerier) Active(project string) (*domain.Session, error) {
	return nil, nil
}
func (d *mcpDummySessionQuerier) Recent(project string, limit int) ([]domain.Session, error) {
	return nil, nil
}

type mcpDummyRelationLister struct{}

func (d *mcpDummyRelationLister) List(project string, limit int) ([]domain.Relation, error) {
	return []domain.Relation{}, nil
}

func TestMCPContextLanguage(t *testing.T) {
	tmp := t.TempDir()
	memDir := filepath.Join(tmp, persistence.MemDir)
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsRepo := persistence.NewSettingsRepository()
	s := settingsRepo.Read(tmp)
	s.Language = "es"
	_ = settingsRepo.Write(tmp, s)

	builder := usecases.New(&mcpDummyMemoryLister{}, &mcpDummySessionQuerier{}, &mcpDummyRelationLister{}, tmp, "test")
	deps := &Deps{
		Root:           tmp,
		Project:        "test",
		ProjectRepo:    &fakeProjectRepo{root: tmp},
		SettingsRepo:   settingsRepo,
		ContextBuilder: builder,
	}

	server := newMCPServer(deps, tmp, "test")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()

	call := func(name string) (string, error) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{"full": true}})
		if err != nil {
			return "", err
		}
		for _, content := range res.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				return text.Text, nil
			}
		}
		return "", nil
	}

	// 1. Invocar get_context en español
	textES, err := call("get_context")
	if err != nil {
		t.Fatalf("get_context falló: %v", err)
	}
	if !strings.Contains(textES, "Memoria persistente activa (gomemory)") || !strings.Contains(textES, "# Memoria del Proyecto") {
		t.Errorf("se esperaba protocolo y encabezado en español, se obtuvo:\n%s", textES)
	}

	// 2. Invocar get_plan_context en español
	textPlanES, err := call("get_plan_context")
	if err != nil {
		t.Fatalf("get_plan_context falló: %v", err)
	}
	if !strings.Contains(textPlanES, "Descomposición Atómica") {
		t.Errorf("se esperaba método en español, se obtuvo:\n%s", textPlanES)
	}

	// 3. Cambiar idioma en caliente a inglés
	CmdLang(deps, []string{"en"})

	// 4. Invocar get_context en inglés inmediatamente sin reiniciar
	textEN, err := call("get_context")
	if err != nil {
		t.Fatalf("get_context en inglés falló: %v", err)
	}
	if !strings.Contains(textEN, "Persistent Memory") || !strings.Contains(textEN, "# Project Memory") {
		t.Errorf("se esperaba protocolo y encabezado en inglés tras switch en caliente, se obtuvo:\n%s", textEN)
	}

	// 5. Invocar get_plan_context en inglés inmediatamente
	textPlanEN, err := call("get_plan_context")
	if err != nil {
		t.Fatalf("get_plan_context en inglés falló: %v", err)
	}
	if !strings.Contains(textPlanEN, "Atomic Decomposition") {
		t.Errorf("se esperaba método en inglés tras switch en caliente, se obtuvo:\n%s", textPlanEN)
	}
}
