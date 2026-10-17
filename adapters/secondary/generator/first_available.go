package generator

import (
	"context"
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

func (f *FirstAvailable) Harnesses() []ports.SocraticHarness {
	choices := []ports.SocraticHarness{{ID: "claude", Label: "Claude Code"}, {ID: "codex", Label: "Codex"}, {ID: "opencode", Label: "OpenCode"}}
	for i := range choices {
		choices[i].Reason = "deshabilitado en la configuración"
		for _, g := range f.generators {
			if g.Name() != choices[i].ID {
				continue
			}
			choices[i].Available = g.Available()
			choices[i].Reason = ""
			if !choices[i].Available {
				choices[i].Reason = "no instalado en PATH o versión no compatible"
			}
			if status, ok := g.(interface{ UnavailableReason() string }); ok && !choices[i].Available {
				choices[i].Reason = status.UnavailableReason()
			}
			break
		}
	}
	return choices
}

func (f *FirstAvailable) forClient(client string) (ports.SocraticGenerator, error) {
	for _, g := range f.generators {
		if g.Name() == client && g.Available() {
			return g, nil
		}
	}
	return nil, fmt.Errorf("%w: %s; elige un harness disponible", ports.ErrNoGenerator, client)
}

// FirstAvailable retains its historical name for source compatibility. Actual
// generation always routes by Flow.Client, never by installation/order.
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

// Name is retained for legacy callers; UI choices come from Harnesses.
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
	g, err := f.forClient(req.Flow.Client)
	if err != nil {
		return domain.GenerationResult{}, err
	}
	return g.GenerateQuestions(ctx, req)
}

func (f *FirstAvailable) Rewrite(ctx context.Context, req ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	g, err := f.forClient(req.Flow.Client)
	if err != nil {
		return domain.RewriteResult{}, err
	}
	return g.Rewrite(ctx, req)
}
