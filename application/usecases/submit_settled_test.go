package usecases

import (
	"testing"

	"mem/domain"
)

// HCD-032 VS-03: un revisor cita un hallazgo heredado con settled_ref.

func hallazgo(localID, location, claim string, settledRef int64) domain.Finding {
	return domain.Finding{LocalID: localID, Location: location, Severity: domain.SeverityHigh, Category: "c", Claim: claim,
		EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"e"}, Confidence: "high", SettledRef: settledRef}
}

func enviar(t *testing.T, reviews *memoryReviewRepository, review *domain.Review, reviewer domain.Reviewer, fs ...domain.Finding) error {
	t.Helper()
	_, err := SubmitReviewerResult(reviews, SubmitReviewerResultInput{Project: "proj", ReviewID: review.ID,
		TargetDigest: review.ActiveTargetDigest(),
		Result:       domain.ReviewerResult{Reviewer: reviewer, Status: domain.ReviewerResultSuccess, Findings: fs}})
	return err
}

func TestSubmitReviewerResult_SettledRef(t *testing.T) {
	// El heredado C-1 (id 1) vive en domain/rules.go.
	start := func(t *testing.T, delta []string) (*memoryReviewRepository, *memoryConsensusRepository, *domain.Review, int64) {
		reviews, ledger := revisionAnteriorTerminada(t, domain.ReviewApproved)
		review, err := StartReviewSince(reviews, ledger, &fakeRevisionDiff{files: delta}, StartReviewInput{
			Project: "proj", TargetType: domain.TargetCommit, Revision: "def456", Digest: "sha256:new", SinceReviewID: "acr_prev"})
		if err != nil {
			t.Fatal(err)
		}
		inherited, _ := reviews.InheritedFindings("proj", review.ID)
		return reviews, ledger, review, inherited[0].ConsensusID
	}

	t.Run("una referencia que no es un heredado se rechaza", func(t *testing.T) {
		reviews, _, review, _ := start(t, []string{"adapters/x.go"})
		if err := enviar(t, reviews, review, domain.ReviewerA, hallazgo("A-1", "domain/rules.go:42", "x", 999)); err == nil {
			t.Fatal("settled_ref debe citar un hallazgo heredado de esta revisión")
		}
	})

	t.Run("archivo sin cambios: asentado y la revisión aprueba sin consenso", func(t *testing.T) {
		reviews, ledger, review, ref := start(t, []string{"adapters/x.go"})
		if err := enviar(t, reviews, review, domain.ReviewerA, hallazgo("A-1", "domain/rules.go:42", "resuelto", ref)); err != nil {
			t.Fatal(err)
		}
		if err := enviar(t, reviews, review, domain.ReviewerB, hallazgo("B-1", "domain/rules.go:42", "resuelto", ref)); err != nil {
			t.Fatal(err)
		}
		results, _ := reviews.ListReviewerResults("proj", review.ID, 0)
		for _, r := range results {
			if !r.Findings[0].Settled {
				t.Fatalf("el motor debe marcar asentado el hallazgo de %s: %+v", r.Reviewer, r.Findings[0])
			}
		}
		final, err := FinalizeReview(reviews, ledger, "proj", review.ID)
		if err != nil {
			t.Fatal(err)
		}
		if final.Verdict != domain.VerdictApproved {
			t.Fatalf("solo asentados: aprueba sin re-adjudicar: %s", final.Verdict)
		}
	})

	t.Run("archivo cambiado: se reabre y exige clasificarse; los asentados no", func(t *testing.T) {
		reviews, ledger, review, ref := start(t, []string{"domain/rules.go"})
		// A cita el heredado, cuyo archivo cambió; B reporta un hallazgo nuevo.
		if err := enviar(t, reviews, review, domain.ReviewerA, hallazgo("A-1", "domain/rules.go:42", "regresión", ref)); err != nil {
			t.Fatal(err)
		}
		if err := enviar(t, reviews, review, domain.ReviewerB, hallazgo("B-1", "docs/x.md:1", "nuevo", 0)); err != nil {
			t.Fatal(err)
		}
		results, _ := reviews.ListReviewerResults("proj", review.ID, 0)
		var ids []int64
		for _, r := range results {
			if r.Findings[0].Settled {
				t.Fatalf("con su archivo en el delta el heredado se reabre: %+v", r.Findings[0])
			}
			if r.Reviewer == domain.ReviewerA && r.Findings[0].SettledRef != ref {
				t.Fatalf("el reabierto conserva el enlace al heredado: %+v", r.Findings[0])
			}
			ids = append(ids, r.Findings[0].ID)
		}
		if _, err := FinalizeReview(reviews, ledger, "proj", review.ID); err == nil {
			t.Fatal("sin clasificar el reabierto no se puede finalizar")
		}
		if _, err := BuildConsensus(reviews, ledger, BuildConsensusInput{Project: "proj", ReviewID: review.ID,
			Unmatched: []ConsensusUnmatched{
				{FindingID: ids[0], Status: domain.ConsensusSuspect, Severity: domain.SeverityHigh},
				{FindingID: ids[1], Status: domain.ConsensusSuspect, Severity: domain.SeverityHigh},
			}}); err != nil {
			t.Fatalf("los reabiertos se clasifican como cualquier hallazgo: %v", err)
		}
	})

	t.Run("el consenso no exige clasificar los asentados", func(t *testing.T) {
		reviews, ledger, review, ref := start(t, []string{"adapters/x.go"})
		if err := enviar(t, reviews, review, domain.ReviewerA, hallazgo("A-1", "domain/rules.go:42", "resuelto", ref), hallazgo("A-2", "docs/y.md:1", "nuevo", 0)); err != nil {
			t.Fatal(err)
		}
		if err := enviar(t, reviews, review, domain.ReviewerB); err != nil {
			t.Fatal(err)
		}
		results, _ := reviews.ListReviewerResults("proj", review.ID, 0)
		var nuevo int64
		for _, f := range results[0].Findings {
			if !f.Settled {
				nuevo = f.ID
			}
		}
		if _, err := BuildConsensus(reviews, ledger, BuildConsensusInput{Project: "proj", ReviewID: review.ID,
			Unmatched: []ConsensusUnmatched{{FindingID: nuevo, Status: domain.ConsensusSuspect, Severity: domain.SeverityHigh}}}); err != nil {
			t.Fatalf("clasificar solo el nuevo debe bastar: %v", err)
		}
	})
}
