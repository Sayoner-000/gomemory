package usecases

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"mem/application/ports"
	"mem/domain"
)

type ReviewerIdentity = domain.ReviewerIdentity

type StartReviewInput struct {
	Project    string
	TargetType domain.TargetType
	Revision   string
	Digest     string
	Scope      []string
	ReviewerA  ReviewerIdentity
	ReviewerB  ReviewerIdentity
	// Policy es la política del PROYECTO, que el llamador lee de Settings. Vacía
	// significa "sin configurar" y cae a los defectos del dominio.
	Policy domain.ReviewPolicy
	// MaxFixRounds y AutoFixSeverities son los valores explícitos de esta revisión
	// concreta y ganan a la política del proyecto.
	MaxFixRounds      int
	AutoFixSeverities []domain.Severity
	// FixAuthorized se pasa como puntero para distinguir "no lo declaró" de
	// "declaró false", que es justamente la distinción que --read-only necesita.
	FixAuthorized *bool
}

func StartReview(repo ports.ReviewRepository, input StartReviewInput) (*domain.Review, error) {
	target, err := domain.NewTarget(input.TargetType, input.Revision, input.Digest, input.Scope)
	if err != nil {
		return nil, err
	}

	politica := input.Policy
	if politica.MaxFixRounds <= 0 && len(politica.AutoFixSeverities) == 0 {
		politica = domain.DefaultReviewPolicy()
	}
	efectiva := politica.Resolve(domain.ReviewPolicy{
		MaxFixRounds:      input.MaxFixRounds,
		AutoFixSeverities: input.AutoFixSeverities,
	})
	autorizada := politica.FixAuthorized
	if input.FixAuthorized != nil {
		autorizada = *input.FixAuthorized
	}

	level, reason := reviewerIndependence(input.ReviewerA, input.ReviewerB)
	now := time.Now()
	review := &domain.Review{
		ID:      "acr_" + uuid.NewString(),
		Project: input.Project,
		Target:  target,
		// El target vigente arranca igual que el original y solo lo mueve una
		// corrección registrada (FR-008).
		CurrentTargetDigest: target.Digest(),
		MaxFixRounds:        efectiva.MaxFixRounds,
		AutoFixSeverities:   efectiva.AutoFixSeverities,
		FixAuthorized:       autorizada,
		ReviewerA:           input.ReviewerA,
		ReviewerB:           input.ReviewerB,
		IndependenceLevel:   level,
		IndependenceReason:  reason,
		Status:              domain.ReviewAwaitingReviewers,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := repo.CreateReview(review); err != nil {
		return nil, err
	}
	return review, nil
}

func reviewerIndependence(a, b ReviewerIdentity) (domain.IndependenceLevel, string) {
	if strings.EqualFold(strings.TrimSpace(a.Provider), strings.TrimSpace(b.Provider)) &&
		strings.EqualFold(strings.TrimSpace(a.Model), strings.TrimSpace(b.Model)) {
		return domain.IndependenceDegraded, "same-model"
	}
	return domain.IndependenceFull, ""
}
