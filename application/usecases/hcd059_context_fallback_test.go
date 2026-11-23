package usecases_test

import (
	"errors"
	"mem/application/ports"
	"mem/domain"
	"testing"
)

// Only List is exposed, deliberately hiding optional full/summary ports.
type hcd059ListOnly struct{ ports.MemoryLister }

var hcd059ReadError = errors.New("injected read failure")

type hcd059FailSummary struct {
	hcd059FullOnly
	failFull, failList bool
}

func (r hcd059FailSummary) ListContextSummaries(string) ([]domain.ContextMemorySummary, error) {
	return nil, hcd059ReadError
}
func (r hcd059FailSummary) ListAll(p string) ([]domain.Memory, error) {
	if r.failFull {
		return nil, hcd059ReadError
	}
	return r.MemoryFullLister.ListAll(p)
}
func (r hcd059FailSummary) List(p string, n int) ([]domain.Memory, error) {
	if r.failList {
		return nil, hcd059ReadError
	}
	return r.MemoryLister.List(p, n)
}

func TestHCD059ContextSummaryFallback(t *testing.T) {
	builder := hcd059ContextFixture(t, "SinRelaciones", 130)
	base := hcd059FullOnly{builder.Lister, builder.Lister.(ports.MemoryFullLister)}
	for _, fullError := range []bool{false, true} {
		current := *builder
		current.Lister = hcd059FailSummary{hcd059FullOnly: base, failFull: fullError}
		legacy := *builder
		if fullError {
			legacy.Lister = hcd059ListOnly{base.MemoryLister}
		} else {
			legacy.Lister = base
		}
		got, err := current.Build()
		if err != nil {
			t.Fatal(err)
		}
		want, err := legacy.Build()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("fallback differs fullError=%v", fullError)
		}
	}
	current := *builder
	current.Lister = hcd059FailSummary{hcd059FullOnly: base, failList: true}
	if _, err := current.Build(); !errors.Is(err, hcd059ReadError) {
		t.Fatalf("lost List error: %v", err)
	}
}
