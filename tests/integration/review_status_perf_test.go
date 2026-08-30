package main

import (
	"fmt"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestReviewStatusYFinalizeConMilHallazgos cubre SC-008: las consultas de estado y
// finalización deben responder en menos de 2 s con revisiones de hasta 1.000
// hallazgos.
//
// Importa porque review_status pasó de devolver cuatro campos a recorrer el linaje
// completo, con una consulta de re-juicios por hallazgo. Ese cambio es justo el que
// podría convertir una consulta barata en una lenta sin que nadie lo note hasta
// tener una revisión grande.
func TestReviewStatusYFinalizeConMilHallazgos(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)

	const project = "perf"
	const total = 1000
	target, err := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:perf", nil)
	if err != nil {
		t.Fatal(err)
	}
	review := &domain.Review{
		ID: "acr_perf", Project: project, Target: target,
		CurrentTargetDigest: "sha256:perf", MaxFixRounds: 2, FixAuthorized: true,
		Status:    domain.ReviewConsensusReady,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		resultado := &domain.ReviewerResult{
			Reviewer: revisor, Round: 0, Status: domain.ReviewerResultSuccess,
		}
		if err := reviews.UpsertReviewerResult(project, review.ID, resultado); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= total; i++ {
		finding := &domain.ConsensusFinding{
			ReviewID: review.ID, ConsensusLocalID: fmt.Sprintf("C-%04d", i),
			Status: domain.ConsensusSuspect, Severity: domain.SeverityLow,
			Claim: fmt.Sprintf("hallazgo %d", i), SourceFindingIDs: []int64{int64(i)},
		}
		if err := ledger.UpsertConsensusFinding(project, review.ID, finding); err != nil {
			t.Fatal(err)
		}
	}

	// Lectura equivalente a la que arma review_status: todos los hallazgos más el
	// linaje de re-juicios de cada uno.
	inicio := time.Now()
	findings, err := ledger.ListAllConsensusFindings(project, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != total {
		t.Fatalf("se leyeron %d hallazgos, se esperaban %d", len(findings), total)
	}
	for _, finding := range findings {
		if _, err := usecases.ReJudgmentsByReviewer(ledger, project, review.ID, finding.ConsensusLocalID, review.Round); err != nil {
			t.Fatal(err)
		}
	}
	if transcurrido := time.Since(inicio); transcurrido > 2*time.Second {
		t.Errorf("el estado con %d hallazgos tardó %v, el límite es 2s", total, transcurrido)
	}

	inicio = time.Now()
	if _, _, err := usecases.FinalizeReviewWithMetrics(reviews, ledger, project, review.ID); err != nil {
		t.Fatal(err)
	}
	if transcurrido := time.Since(inicio); transcurrido > 2*time.Second {
		t.Errorf("la finalización con %d hallazgos tardó %v, el límite es 2s", total, transcurrido)
	}
}
