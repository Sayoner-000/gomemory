package usecases

import (
	"fmt"
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
	// SinceReviewID enlaza con una revisión anterior terminada (HCD-032, Q1).
	// Solo lo atiende StartReviewSince, que tiene lo necesario para heredar.
	SinceReviewID string
}

func StartReview(repo ports.ReviewRepository, input StartReviewInput) (*domain.Review, error) {
	if strings.TrimSpace(input.SinceReviewID) != "" {
		// Ignorarlo crearía una revisión sin herencia que el llamador cree
		// enlazada: re-adjudicaría todo sin avisar.
		return nil, fmt.Errorf("una revisión con --since se inicia con StartReviewSince")
	}
	review, err := newReview(input)
	if err != nil {
		return nil, err
	}
	if err := repo.CreateReview(review); err != nil {
		return nil, err
	}
	return review, nil
}

// StartReviewSince inicia una revisión enlazada a otra anterior del mismo
// proyecto (HCD-032): hereda lo asentado (Q2) y fija el delta (Q3). La anterior
// debe estar terminada: heredar de una abierta copiaría un estado que aún cambia.
func StartReviewSince(repo ports.ReviewRepository, consensus ports.ConsensusRepository, diff ports.RevisionDiff, input StartReviewInput) (*domain.Review, error) {
	sinceID := strings.TrimSpace(input.SinceReviewID)
	if sinceID == "" {
		return StartReview(repo, input)
	}
	prev, err := repo.GetReview(input.Project, sinceID)
	if err != nil {
		return nil, err
	}
	if prev == nil {
		return nil, fmt.Errorf("la revisión anterior %s no existe en este proyecto", sinceID)
	}
	if !prev.Status.Terminal() {
		return nil, fmt.Errorf("la revisión anterior %s sigue abierta (%s): solo se hereda de una terminada", sinceID, prev.Status)
	}
	findings, err := consensus.ListAllConsensusFindings(input.Project, sinceID)
	if err != nil {
		return nil, err
	}
	sources := map[int64]domain.Finding{}
	for round := 0; round <= prev.Round; round++ {
		fs, err := repo.ListFindings(input.Project, sinceID, round)
		if err != nil {
			return nil, err
		}
		for _, f := range fs {
			sources[f.ID] = f
		}
	}
	review, err := newReview(input)
	if err != nil {
		return nil, err
	}
	review.SinceReviewID = sinceID
	review.Inherited = domain.InheritableFindings(*prev, findings, sources)
	review.DeltaSource = domain.DeltaSourceScope
	review.DeltaScope = append([]string(nil), input.Scope...)
	if diff != nil {
		if files, err := diff.ChangedFiles(prev.Target.Revision, input.Revision); err == nil {
			review.DeltaSource, review.DeltaScope = domain.DeltaSourceGit, files
		}
	}
	if err := repo.CreateReview(review); err != nil {
		return nil, err
	}
	return review, nil
}

func newReview(input StartReviewInput) (*domain.Review, error) {
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
	return review, nil
}

func reviewerIndependence(a, b ReviewerIdentity) (domain.IndependenceLevel, string) {
	if strings.EqualFold(strings.TrimSpace(a.Provider), strings.TrimSpace(b.Provider)) &&
		strings.EqualFold(strings.TrimSpace(a.Model), strings.TrimSpace(b.Model)) {
		return domain.IndependenceDegraded, "same-model"
	}
	return domain.IndependenceFull, ""
}
