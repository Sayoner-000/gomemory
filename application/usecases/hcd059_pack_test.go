package usecases_test

import (
	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

// Only Search is supplied: any unexpected repository access fails loudly via
// the nil embedded port rather than silently returning invented data.
type hcd059CandidateRepo struct {
	ports.MemoryRepository
	candidates []domain.Memory
}

func (r hcd059CandidateRepo) Search(string, string, int) ([]domain.Memory, error) {
	return r.candidates, nil
}
func hcd059PackCandidates() ports.MemoryRepository {
	return hcd059CandidateRepo{candidates: []domain.Memory{
		{ID: 1, Type: domain.Learning, Content: "alpha beta gamma"},
		{ID: 2, Type: domain.Learning, Content: "alpha beta gamma delta"},
		{ID: 3, Type: domain.Decision, Content: "alpha beta gamma delta"},
		{ID: 4, Type: domain.Checkpoint, Content: "alpha beta gamma delta"},
	}}
}

func TestHCD059PackDuplicateCompatibility(t *testing.T) {
	repo := hcd059PackCandidates()
	pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{Project: "perf", Task: "alpha", MaxTokens: 10000})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, item := range pack.Items {
		ids[item.ID] = true
	}
	if len(ids) != 2 || !ids["memory:2"] || !ids["memory:3"] {
		t.Fatalf("wrong survivors: %v", ids)
	}
	if pack.Stats.ItemsDuplicate != 1 || pack.Stats.ItemsRetrieved != 3 {
		t.Fatalf("wrong dedup stats: %+v", pack.Stats)
	}
}
