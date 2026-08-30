package main

import (
	"fmt"
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestReJuiciosConcurrentesNoSePierden cubre el flujo NORMAL de este protocolo: dos
// revisores independientes trabajando en paralelo.
//
// UpsertReJudgment abría la transacción con el BEGIN diferido de database/sql, que en
// WAL toma el bloqueo de escritura en el primer INSERT; el perdedor recibía
// SQLITE_BUSY y el busy_timeout no lo reintenta para una transacción que ya leyó. Con
// 16 re-juicios simultáneos fallaban 11 con "database is locked". Se corrigió usando
// una conexión dedicada con BEGIN IMMEDIATE, igual que RecordFixAtomically.
func TestReJuiciosConcurrentesNoSePierden(t *testing.T) {
	for intento := range 20 {
		db, err := persistence.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reviews := persistence.NewReviewRepository(db)
		ledger := persistence.NewConsensusRepository(db)
		const project = "qa"

		target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
		review := &domain.Review{
			ID: "acr_qa", Project: project, Target: target,
			CurrentTargetDigest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: true,
			AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
			Status:            domain.ReviewConsensusReady,
		}
		if err := reviews.CreateReview(review); err != nil {
			t.Fatal(err)
		}
		if err := ledger.UpsertConsensusFinding(project, review.ID, &domain.ConsensusFinding{
			ReviewID: review.ID, ConsensusLocalID: "C-001",
			Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
			SourceFindingIDs: []int64{1, 2},
		}); err != nil {
			t.Fatal(err)
		}
		for n := 2; n <= 8; n++ {
			if err := ledger.UpsertConsensusFinding(project, review.ID, &domain.ConsensusFinding{
				ReviewID: review.ID, ConsensusLocalID: fmt.Sprintf("C-%03d", n),
				Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
				SourceFindingIDs: []int64{int64(n * 10)},
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
			Project: project, ReviewID: review.ID,
			AddressedConsensusIDs: []string{"C-001", "C-002", "C-003", "C-004", "C-005", "C-006", "C-007", "C-008"},
			BaseTargetDigest:      "sha256:v0", FixedTargetDigest: "sha256:v1",
		}); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		var fallos []error
		for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
			for n := 1; n <= 8; n++ {
				wg.Add(1)
				go func(rev domain.Reviewer, id string) {
					defer wg.Done()
					_, err := usecases.RejudgeReview(reviews, ledger, usecases.RejudgeReviewInput{
						Project: project, ReviewID: review.ID, Reviewer: rev,
						Judgments: map[string]usecases.ReJudgeEntry{
							id: {State: domain.ReJudgmentResolved, Evidence: []string{"verificado"}},
						},
					})
					if err != nil {
						mu.Lock()
						fallos = append(fallos, err)
						mu.Unlock()
					}
				}(revisor, fmt.Sprintf("C-%03d", n))
			}
		}
		wg.Wait()
		errs := []error{nil, nil}
		if len(fallos) > 0 {
			t.Errorf("intento %d: %d re-juicios fallaron, primero: %v", intento, len(fallos), fallos[0])
			db.Close()
			return
		}

		for i, err := range errs {
			if err != nil {
				t.Logf("intento %d: revisor %d falló: %v", intento, i, err)
			}
		}
		f, err := ledger.GetConsensusFinding(project, review.ID, "C-001")
		if err != nil {
			t.Fatal(err)
		}
		if errs[0] == nil && errs[1] == nil && f.RejudgmentState != domain.ReJudgmentResolved {
			t.Errorf("intento %d: los dos revisores dieron RESOLVED pero el agregado quedó %s",
				intento, f.RejudgmentState)
			db.Close()
			return
		}
		db.Close()
	}

}
