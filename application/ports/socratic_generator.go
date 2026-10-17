package ports

import (
	"context"
	"errors"
	"mem/domain"
)

// ErrNoGenerator: no generator in the configured list is available locally.
var ErrNoGenerator = errors.New("no socratica generator available")

type SocraticGenerationRequest struct {
	Flow    domain.RefinementFlow
	Context string
}
type SocraticGenerator interface {
	Name() string
	Available() bool
	GenerateQuestions(context.Context, SocraticGenerationRequest) (domain.GenerationResult, error)
	Rewrite(context.Context, SocraticGenerationRequest) (domain.RewriteResult, error)
}

// SocraticHarness describes a choice without making a provider recommendation.
type SocraticHarness struct {
	ID, Label, Reason string
	Available         bool
}

// SocraticHarnessCatalog is implemented by a router of local generators.
type SocraticHarnessCatalog interface {
	Harnesses() []SocraticHarness
}

type SocraticFailure struct {
	Message   string
	Retryable bool
}

// SocraticFailureExplainer translates provider diagnostics, including errors
// persisted by older versions, without putting provider rules in the UI.
type SocraticFailureExplainer interface {
	ExplainFailure(string) SocraticFailure
}
