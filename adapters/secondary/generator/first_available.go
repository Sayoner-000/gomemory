package generator

import (
	"context"

	"mem/application/ports"
	"mem/domain"
)

// FirstAvailable delegates to the first available generator of a list the
// composition root has already ordered from socratica_generators. It knows no
// concrete adapter (constitución §4).
type FirstAvailable struct{ generators []ports.SocraticGenerator }

func NewFirstAvailable(generators []ports.SocraticGenerator) *FirstAvailable {
	return &FirstAvailable{generators: generators}
}

func (f *FirstAvailable) pick() (ports.SocraticGenerator, error) {
	for _, g := range f.generators {
		if g.Available() {
			return g, nil
		}
	}
	return nil, ports.ErrNoGenerator
}

// Name is the generator that would run now, or "" when none is available.
func (f *FirstAvailable) Name() string {
	if g, err := f.pick(); err == nil {
		return g.Name()
	}
	return ""
}

func (f *FirstAvailable) Available() bool {
	_, err := f.pick()
	return err == nil
}

func (f *FirstAvailable) GenerateQuestions(ctx context.Context, req ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	g, err := f.pick()
	if err != nil {
		return domain.GenerationResult{}, err
	}
	return g.GenerateQuestions(ctx, req)
}

func (f *FirstAvailable) Rewrite(ctx context.Context, req ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	g, err := f.pick()
	if err != nil {
		return domain.RewriteResult{}, err
	}
	return g.Rewrite(ctx, req)
}
