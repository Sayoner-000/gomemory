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
