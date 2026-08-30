package main

import (
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestConsensoConcurrenteEscribeUnaSolaClasificacion cubre el defecto HIGH C-003 de
// acr_83428b4c.
//
// BuildConsensus hacía check-then-write: listaba los hallazgos ya registrados y después
// recorría un bucle de upserts, cada uno en su propia transacción implícita. Varias
// llamadas simultáneas veían todas el ledger vacío y escribían todas; el resultado
// quedaba mezclado y un fallo a mitad del bucle dejaba media clasificación persistida.
//
// La afirmación fuerte no es "no hay error": es que la ronda persistida sea COMPLETA y
// tenga una sola huella. Una ronda a medias es justo lo que el veredicto tiene que
// atrapar después, y llegar ahí ya es haber perdido la propiedad.
func TestConsensoConcurrenteEscribeUnaSolaClasificacion(t *testing.T) {
	const proyecto = "consenso"
	const simultaneas = 8
	for intento := range 20 {
		db, err := persistence.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reviews := persistence.NewReviewRepository(db)
		ledger := persistence.NewConsensusRepository(db)

		review, err := usecases.StartReview(reviews, usecases.StartReviewInput{
			Project: proyecto, TargetType: domain.TargetDiff, Revision: "wt",
			Digest:    "sha256:v0",
			ReviewerA: usecases.ReviewerIdentity{Provider: "one", Model: "a"},
			ReviewerB: usecases.ReviewerIdentity{Provider: "two", Model: "b"},
		})
		if err != nil {
			t.Fatal(err)
		}

		a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
			Provider: "one", Model: "a"}
		b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
			Provider: "two", Model: "b"}
		for i := range 3 {
			a.Findings = append(a.Findings, domain.Finding{
				LocalID: "A-00" + string(rune('1'+i)), Location: "x.go:1", Severity: domain.SeverityHigh,
				Category: "correctness", Claim: "defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}})
			b.Findings = append(b.Findings, domain.Finding{
				LocalID: "B-00" + string(rune('1'+i)), Location: "x.go:1", Severity: domain.SeverityHigh,
				Category: "correctness", Claim: "mismo defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}})
		}
		for _, result := range []*domain.ReviewerResult{&a, &b} {
			if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
				Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0", Result: *result,
			}); err != nil {
				t.Fatal(err)
			}
		}

		entrada := usecases.BuildConsensusInput{Project: proyecto, ReviewID: review.ID}
		for i := range 3 {
			entrada.Matches = append(entrada.Matches, usecases.ConsensusMatch{
				Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[i].ID,
				FindingIDB: b.Findings[i].ID, Severity: domain.SeverityHigh, Claim: "defecto",
			})
		}

		var wg sync.WaitGroup
		errores := make([]error, simultaneas)
		wg.Add(simultaneas)
		for i := range simultaneas {
			go func() {
				defer wg.Done()
				_, errores[i] = usecases.BuildConsensus(reviews, ledger, entrada)
			}()
		}
		wg.Wait()

		// Reenviar la MISMA clasificación es idempotente: ninguna de las llamadas
		// simultáneas debe fallar, ni siquiera la que llega segunda.
		for i, err := range errores {
			if err != nil {
				t.Fatalf("intento %d: la llamada %d falló con %v", intento, i, err)
			}
		}

		persistidos, err := ledger.ListAllConsensusFindings(proyecto, review.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persistidos) != 3 {
			t.Fatalf("intento %d: quedaron %d hallazgos de consenso, la ronda tiene 3",
				intento, len(persistidos))
		}
		// Una sola huella: filas de dos escrituras distintas mezcladas en la misma
		// ronda es exactamente el daño que esto tenía que impedir.
		huella := persistidos[0].RoundFingerprint
		vistos := map[string]bool{}
		for _, hallazgo := range persistidos {
			if hallazgo.RoundFingerprint != huella {
				t.Fatalf("intento %d: la ronda mezcla huellas %s y %s",
					intento, huella, hallazgo.RoundFingerprint)
			}
			if vistos[hallazgo.ConsensusLocalID] {
				t.Fatalf("intento %d: %s persistido dos veces", intento, hallazgo.ConsensusLocalID)
			}
			vistos[hallazgo.ConsensusLocalID] = true
		}
		db.Close()
	}
}
