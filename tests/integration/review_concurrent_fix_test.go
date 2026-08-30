package main

import (
	"fmt"
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestRecordFixConcurrenteConservaUnaSolaRonda cubre SC-003 y FR-010 contra SQLite
// real, que es donde vive la garantía.
//
// El defecto original no era teórico: RecordFix hacía cuatro operaciones sueltas
// —contar rondas, derivar el número, insertar el delta, actualizar la revisión— y
// dos procesos que leían el mismo recuento derivaban la misma ronda; el segundo
// sobrescribía al primero por el UPSERT, sin error y sin dejar rastro.
//
// La corrección se apoya en tres cosas a la vez: BEGIN IMMEDIATE (toma el bloqueo de
// escritura al abrir, no al primer INSERT), el recuento contra ExpectedRounds, y el
// UNIQUE(review_id, round) como red final.
func TestRecordFixConcurrenteConservaUnaSolaRonda(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)

	const project = "concurrencia"
	target, err := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:v0", nil)
	if err != nil {
		t.Fatal(err)
	}
	review := &domain.Review{
		ID: "acr_carrera", Project: project, Target: target,
		CurrentTargetDigest: "sha256:v0",
		MaxFixRounds:        5,
		AutoFixSeverities:   []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		FixAuthorized:       true,
		Status:              domain.ReviewConsensusReady,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	confirmado := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-001",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		SourceFindingIDs: []int64{1, 2},
	}
	if err := ledger.UpsertConsensusFinding(project, review.ID, confirmado); err != nil {
		t.Fatal(err)
	}

	const intentos = 100
	var wg sync.WaitGroup
	exitosos := make([]string, 0, intentos)
	var mu sync.Mutex
	wg.Add(intentos)
	for i := range intentos {
		go func(n int) {
			defer wg.Done()
			delta, err := usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
				Project: project, ReviewID: review.ID,
				AddressedConsensusIDs: []string{"C-001"},
				BaseTargetDigest:      "sha256:v0",
				FixedTargetDigest:     fmt.Sprintf("sha256:fix-%03d", n),
				ModifiedPaths:         []string{"domain/verdict.go"},
				Verification:          []string{"go test ./domain/..."},
			})
			if err != nil {
				return
			}
			mu.Lock()
			exitosos = append(exitosos, delta.FixedTargetDigest)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(exitosos) != 1 {
		t.Fatalf("ganaron %d correcciones, debe ganar exactamente una: %v", len(exitosos), exitosos)
	}

	deltas, err := ledger.ListFixDeltas(project, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 {
		t.Fatalf("el ledger conserva %d rondas, se esperaba 1", len(deltas))
	}
	if deltas[0].Round != 1 {
		t.Errorf("la ronda registrada = %d, se esperaba 1", deltas[0].Round)
	}
	if deltas[0].FixedTargetDigest != exitosos[0] {
		t.Errorf("el ledger guardó %q pero la corrección ganadora fue %q",
			deltas[0].FixedTargetDigest, exitosos[0])
	}

	// La revisión debe reflejar exactamente a la ganadora: si avanzara con el
	// target de una corrección que no se persistió, la cadena quedaría rota.
	actualizada, err := reviews.GetReview(project, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actualizada.Round != 1 {
		t.Errorf("Round = %d, se esperaba 1", actualizada.Round)
	}
	if actualizada.CurrentTargetDigest != exitosos[0] {
		t.Errorf("el target vigente = %q, se esperaba el de la ganadora %q",
			actualizada.CurrentTargetDigest, exitosos[0])
	}
	if actualizada.Status != domain.ReviewRejudging {
		t.Errorf("estado = %s, se esperaba rejudging", actualizada.Status)
	}
}
