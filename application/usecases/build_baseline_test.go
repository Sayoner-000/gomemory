package usecases_test

import (
	"context"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func baselineBuilder(t *testing.T) (*usecases.Builder, ports.MemoryRepository) {
	t.Helper()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	memRepo := persistence.NewMemoryRepository(db)
	b := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), t.TempDir(), "p")
	b.Topics = memRepo.(ports.MemoryTopicQuerier)
	b.BaselineDefault = "baseline embebido\n"
	return b, memRepo
}

func TestBuilder_BaselineSinSemillaUsaPlantilla(t *testing.T) {
	b, _ := baselineBuilder(t)
	out, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Baseline universal (memoria fijada)\n\nbaseline embebido") {
		t.Fatalf("el contexto no trae el baseline:\n%s", out)
	}
}

func TestBuilder_BaselineFijadoPrevalece(t *testing.T) {
	b, memRepo := baselineBuilder(t)
	if _, err := memRepo.ImportMemory(&domain.Memory{Project: "p", Type: domain.Preference, Title: "b", Content: "baseline editado", TopicKey: domain.TopicUniversalBaseline}); err != nil {
		t.Fatal(err)
	}
	out, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "baseline editado") || strings.Contains(out, "baseline embebido") {
		t.Fatalf("el baseline fijado no prevalece:\n%s", out)
	}
}

func TestBuilder_BaselineFijadoNoSeRepiteEnPreferencias(t *testing.T) {
	b, memRepo := baselineBuilder(t)
	if _, err := memRepo.ImportMemory(&domain.Memory{Project: "p", Type: domain.Preference, Title: "Baseline fijado", Content: "baseline editado", TopicKey: domain.TopicUniversalBaseline}); err != nil {
		t.Fatal(err)
	}
	out, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "baseline editado") != 1 || strings.Contains(out, "**Baseline fijado**") {
		t.Fatalf("el baseline fijado se repite en el contexto:\n%s", out)
	}
}

type bloquesVistos struct{}

func (bloquesVistos) Seen(_ context.Context, hashes []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, h := range hashes {
		out[h] = true
	}
	return out, nil
}
func (bloquesVistos) Mark(context.Context, []string) error { return nil }
func (bloquesVistos) Reset(context.Context) error          { return nil }

func TestSessionDelta_ConservaBaseline(t *testing.T) {
	doc := "## Baseline universal (memoria fijada)\n\ncuerpo\n<!-- mem:baseline-end -->\n"
	got := usecases.SessionDelta{Blocks: bloquesVistos{}, KeepBaseline: true}.ApplyContext(context.Background(), doc)
	if !strings.Contains(got, "cuerpo") {
		t.Fatalf("baseline elidido:\n%s", got)
	}
	got = usecases.SessionDelta{Blocks: bloquesVistos{}}.ApplyContext(context.Background(), doc)
	if strings.Contains(got, "cuerpo") {
		t.Fatalf("sin KeepBaseline debe elidirse:\n%s", got)
	}
}

func TestBuilder_BaselineRespetaElPresupuesto(t *testing.T) {
	b, _ := baselineBuilder(t)
	b.BaselineDefault = strings.Repeat("baseline largo ", 400)
	b.Budget = 1500
	out, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "baseline largo") || !strings.Contains(out, "mem docs") {
		t.Fatalf("debe omitirse con su puntero: %d bytes\n%.200s", len(out), out)
	}
}
