package usecases_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type hcd001JSONRenderer struct{}

func (hcd001JSONRenderer) Render(p domain.ContextPack) string {
	b, _ := json.MarshalIndent(p, "", "  ")
	return string(b) + "\n"
}

func TestHCD001MandatoryOverflow(t *testing.T) {
	repo := newContextPackTestDeps(t)
	m := domain.Memory{Project: "mandatory", Type: domain.Decision, Title: "regla", Content: "regla obligatoria sin truncar"}
	if _, err := repo.Insert(&m); err != nil {
		t.Fatal(err)
	}
	pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{Project: "mandatory", Task: "regla", MaxTokens: 100, Renderer: hcd001JSONRenderer{}})
	if !errors.Is(err, domain.ErrCriticalContextOverflow) || len(pack.Items) != 0 {
		t.Fatalf("desbordamiento obligatorio silencioso: %v %+v", err, pack)
	}
}

func TestHCD001QuerySelection(t *testing.T) {
	repo := newContextPackTestDeps(t)
	for _, m := range []domain.Memory{
		{Project: "queries", Type: domain.Learning, Title: "autenticacion", Content: "autenticacion requiere RS256"},
		{Project: "queries", Type: domain.Learning, Title: "compactacion", Content: "compactacion reinicia entregas"},
	} {
		if _, err := repo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"autenticacion", "compactacion"} {
		pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{Project: "queries", Task: query, MaxTokens: 4000, Renderer: hcd001JSONRenderer{}})
		if err != nil {
			t.Fatal(err)
		}
		if len(pack.Items) != 1 || !strings.Contains(pack.Items[0].Content, query) {
			t.Fatalf("selección no condicionada: %s %+v", query, pack.Items)
		}
	}
}

func TestHCD001SerializedBudget(t *testing.T) {
	repo := newContextPackTestDeps(t)
	for _, m := range []domain.Memory{
		{Project: "budget", Type: domain.Learning, Title: "limite", Content: strings.Repeat("limite á漢字 verificable ", 70)},
		{Project: "budget", Type: domain.Discovery, Title: "limite secundario", Content: strings.Repeat("limite resultado diferente ", 60)},
	} {
		if _, err := repo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	counter := tokens.ApproximateTokenCounter{}
	pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, counter, nil, usecases.ContextRequest{Project: "budget", Task: "limite", MaxTokens: 1100, Compression: ports.CompressionNone, Renderer: hcd001JSONRenderer{}})
	if err != nil {
		t.Fatal(err)
	}
	if got := counter.Count(hcd001JSONRenderer{}.Render(pack)); got > pack.Budget {
		t.Fatalf("respuesta completa=%d > presupuesto=%d", got, pack.Budget)
	}
	if pack.Stats.MetadataTokens <= 0 {
		t.Fatal("metadatos no contados")
	}
}
