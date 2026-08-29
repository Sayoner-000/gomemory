package usecases

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"mem/application/ports"
	"mem/domain"
)

type ReviewerIdentity struct {
	Provider string
	Model    string
}

type StartReviewInput struct {
	Project           string
	TargetType        domain.TargetType
	Revision          string
	Digest            string
	Scope             []string
	ReviewerA         ReviewerIdentity
	ReviewerB         ReviewerIdentity
	MaxFixRounds      int
	AutoFixSeverities []domain.Severity
}

func StartReview(repo ports.ReviewRepository, input StartReviewInput) (*domain.Review, error) {
	target, err := domain.NewTarget(input.TargetType, input.Revision, input.Digest, input.Scope)
	if err != nil {
		return nil, err
	}
	maxRounds := input.MaxFixRounds
	if maxRounds <= 0 {
		maxRounds = 2
	}
	severities := append([]domain.Severity(nil), input.AutoFixSeverities...)
	if len(severities) == 0 {
		severities = []domain.Severity{domain.SeverityCritical, domain.SeverityHigh}
	}
	level, reason := reviewerIndependence(input.ReviewerA, input.ReviewerB)
	now := time.Now()
	review := &domain.Review{
		ID:                 "acr_" + uuid.NewString(),
		Project:            input.Project,
		Target:             target,
		MaxFixRounds:       maxRounds,
		AutoFixSeverities:  severities,
		IndependenceLevel:  level,
		IndependenceReason: reason,
		Status:             domain.ReviewAwaitingReviewers,
		CreatedAt:          now,
		UpdatedAt:          now,
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
