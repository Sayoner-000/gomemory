package usecases_test

import (
	"fmt"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

type hcd059FullOnly struct {
	ports.MemoryLister
	ports.MemoryFullLister
}
type hcd059Recording struct{ raw, final int }

func (r *hcd059Recording) Record(_ string, raw, final int) { r.raw = raw; r.final = final }

func hcd059LegacyBuilder(b *usecases.Builder) *usecases.Builder {
	cp := *b
	cp.Lister = hcd059FullOnly{b.Lister, b.Lister.(ports.MemoryFullLister)}
	return &cp
}

type hcd059SummarySpy struct {
	ports.MemoryRepository
	calls, full int
}

func (s *hcd059SummarySpy) ListContextSummaries(project string) ([]domain.ContextMemorySummary, error) {
	s.calls++
	return s.MemoryRepository.(ports.ContextMemorySummaryLister).ListContextSummaries(project)
}
func (s *hcd059SummarySpy) ListAll(project string) ([]domain.Memory, error) {
	s.full++
	return s.MemoryRepository.ListAll(project)
}

func TestHCD059ContextEquivalence(t *testing.T) {
	for _, kind := range []string{"SinRelaciones", "SinTitulo", "Relaciones", "Anclas"} {
		t.Run(kind, func(t *testing.T) {
			builder := hcd059ContextFixture(t, kind, 130)
			repo := builder.Lister.(ports.MemoryRepository)
			if err := repo.UpdateContent("perf", 1, "", strings.Repeat("日本語árbol", 20)); err != nil {
				t.Fatal(err)
			}
			if err := repo.UpdateContent("perf", 3, "\x1b[31mUnicode\x1b[0m", "contenido histórico"); err != nil {
				t.Fatal(err)
			}
			if _, err := builder.Relations.(ports.RelationRepository).Insert(&domain.Relation{Project: "perf", MemoryIDA: 1, MemoryIDB: 3, Relation: domain.ConflictsWith, Confidence: 1}); err != nil {
				t.Fatal(err)
			}
			builder.CodeProviders = []ports.CodeGraphProvider{&fakeCodeProvider{snap: domain.CodeProviderSnapshot{Provider: "fake-provider", Available: true}}}
			for _, budget := range []int{0, 12000} {
				for _, index := range []bool{false, true} {
					t.Run(fmt.Sprintf("budget%d/index%v", budget, index), func(t *testing.T) {
						current := *builder
						current.Budget = budget
						current.IndexMode = index
						legacy := hcd059LegacyBuilder(&current)
						spy := &hcd059SummarySpy{MemoryRepository: repo}
						current.Lister = spy
						a, b := &hcd059Recording{}, &hcd059Recording{}
						current.Counter = tokens.ApproximateTokenCounter{}
						current.Recorder = a
						legacy.Counter = tokens.ApproximateTokenCounter{}
						legacy.Recorder = b
						got, err := current.Build()
						if err != nil {
							t.Fatal(err)
						}
						want, err := legacy.Build()
						if err != nil {
							t.Fatal(err)
						}
						if spy.calls != 1 || spy.full != 0 {
							t.Fatalf("summary calls=%d full calls=%d", spy.calls, spy.full)
						}
						if got != want {
							t.Fatal("context differs from full-history path")
						}
						if *a != *b {
							t.Fatalf("metrics differ: %+v %+v", a, b)
						}
						if !strings.Contains(got, "日本語") || !strings.Contains(got, "Unicode") {
							t.Fatal("missing old conflict titles")
						}
					})
				}
			}
		})
	}
}
