package main

import (
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// TestFinalizarYCorregirNoSePisan cubre el defecto CRITICAL C-002 de acr_83428b4c.
//
// FinalizeReview leía la revisión FUERA de toda transacción y escribía con
// UpdateReview, un UPDATE ciego de todas las columnas. Entre la lectura y la escritura
// cabe una ronda de corrección entera: la finalización llegaba después y devolvía
// `round` y `current_target_digest` a los valores obsoletos que había leído, además de
// cerrar la revisión. En el sentido inverso, RecordFix revalidaba el digest pero no el
// estado, y como finalizar NO cambia el target, una corrección tardía reabría una
// revisión ya terminal.
//
// El escenario fuerza un veredicto siempre derivable —una CONTRADICTION severa hace
// ESCALATED de inmediato— para que la finalización escriba de verdad y compita.
func TestFinalizarYCorregirNoSePisan(t *testing.T) {
	const proyecto = "carrera"
	for intento := range 40 {
		db, err := persistence.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reviews := persistence.NewReviewRepository(db)
		ledger := persistence.NewConsensusRepository(db)

		target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
		review := &domain.Review{
			ID: "acr_carrera", Project: proyecto, Target: target,
			CurrentTargetDigest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: true,
			AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
			Status:            domain.ReviewConsensusReady,
		}
		if err := reviews.CreateReview(review); err != nil {
			t.Fatal(err)
		}
		for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
			r := domain.ReviewerResult{Reviewer: revisor, Round: 0, Status: domain.ReviewerResultSuccess}
			if err := reviews.UpsertReviewerResult(proyecto, review.ID, &r); err != nil {
				t.Fatal(err)
			}
		}
		// C-001 confirmado y corregible; C-002 en contradicción, que es lo que hace
		// que el veredicto sea siempre derivable y la finalización llegue a escribir.
		for _, hallazgo := range []domain.ConsensusFinding{
			{ReviewID: review.ID, ConsensusLocalID: "C-001", Status: domain.ConsensusConfirmed,
				Severity: domain.SeverityCritical, SourceFindingIDs: []int64{1, 2}},
			{ReviewID: review.ID, ConsensusLocalID: "C-002", Status: domain.ConsensusContradiction,
				Severity: domain.SeverityHigh, SourceFindingIDs: []int64{3, 4}},
		} {
			copia := hallazgo
			if err := ledger.UpsertConsensusFinding(proyecto, review.ID, &copia); err != nil {
				t.Fatal(err)
			}
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			usecases.FinalizeReview(reviews, ledger, proyecto, review.ID)
		}()
		go func() {
			defer wg.Done()
			usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
				Project: proyecto, ReviewID: review.ID, AddressedConsensusIDs: []string{"C-001"},
				BaseTargetDigest: "sha256:v0", FixedTargetDigest: "sha256:v1",
			})
		}()
		wg.Wait()

		final, err := reviews.GetReview(proyecto, review.ID)
		if err != nil {
			t.Fatal(err)
		}
		correcciones, err := ledger.ListFixDeltas(proyecto, review.ID)
		if err != nil {
			t.Fatal(err)
		}

		// Una corrección registrada obliga a que la ronda y el target vigente sean los
		// suyos. Es la afirmación que atrapa la finalización tardía: restauraba la
		// ronda 0 y el digest v0 con la corrección ya escrita en el ledger.
		if len(correcciones) > 0 {
			ultima := correcciones[len(correcciones)-1]
			if final.Round != ultima.Round {
				t.Fatalf("intento %d: ronda=%d con la corrección %d registrada",
					intento, final.Round, ultima.Round)
			}
			if final.ActiveTargetDigest() != ultima.FixedTargetDigest {
				t.Fatalf("intento %d: target vigente=%s, la corrección dejó %s",
					intento, final.ActiveTargetDigest(), ultima.FixedTargetDigest)
			}
		}

		// Un estado terminal no se reabre, y un veredicto escrito implica estado
		// terminal: es la dirección que la comprobación de digest no cubría.
		if final.Verdict != "" && !final.Status.Terminal() {
			t.Fatalf("intento %d: veredicto %s con la revisión reabierta en %s",
				intento, final.Verdict, final.Status)
		}
		// Terminal CON una corrección registrada es legítimo —la corrección ganó y la
		// finalización se derivó después sobre ese estado—; lo que no puede pasar es
		// que la revisión quede terminal con la ronda o el target de antes, y eso ya
		// lo afirma la comprobación de arriba.
		db.Close()
	}
}

// TestUnaCorreccionTardiaNoReabreLoTerminal fija de forma determinista la dirección
// que la comprobación de digest NO cubría.
//
// RecordFixAtomically revalidaba dentro de la transacción el target vigente y el
// recuento de rondas, pero no el estado. Finalizar no cambia el target, así que una
// corrección cuyo caso de uso leyó la revisión antes de la finalización pasaba ambas
// comprobaciones y reabría una revisión ya terminal.
func TestUnaCorreccionTardiaNoReabreLoTerminal(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	const proyecto = "tardia"

	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: "acr_tardia", Project: proyecto, Target: target,
		CurrentTargetDigest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: true,
		Status: domain.ReviewEscalated, Verdict: domain.VerdictEscalated,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}

	// La transición que traía en la mano una corrección que leyó la revisión cuando
	// todavía estaba en consensus_ready. El digest coincide, porque finalizar no lo
	// toca: solo el estado delata que llega tarde.
	err = ledger.RecordFixAtomically(proyecto, review.ID, ports.FixTransition{
		Delta: &domain.FixDelta{
			ReviewID: review.ID, Round: 1, BaseTargetDigest: "sha256:v0",
			FixedTargetDigest: "sha256:v1", AddressedConsensusIDs: []string{"C-001"},
		},
		ExpectedRounds:      0,
		ExpectedBaseDigest:  "sha256:v0",
		ExpectedStatus:      domain.ReviewConsensusReady,
		NextRound:           1,
		NextStatus:          domain.ReviewRejudging,
		CurrentTargetDigest: "sha256:v1",
	})
	if err == nil {
		t.Fatal("la corrección tardía reabrió una revisión terminal")
	}

	final, err := reviews.GetReview(proyecto, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != domain.ReviewEscalated || final.Round != 0 {
		t.Fatalf("la revisión quedó en %s ronda %d, debía seguir terminal en la ronda 0",
			final.Status, final.Round)
	}
}
