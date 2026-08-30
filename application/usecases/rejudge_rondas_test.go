package usecases

import (
	"testing"

	"mem/domain"
)

// TestRejudgeReview_LaUnanimidadNoCruzaRondas es la regresión de un defecto
// encontrado revisando la propia funcionalidad 028, y producía un APPROVED falso.
//
// Los re-juicios se acumulan ronda tras ronda. Al agregar el conjunto entero, un
// RESOLVED emitido sobre una corrección anterior completaba la unanimidad de una
// posterior: el revisor que lo emitió había juzgado otro target. Reproducido: A da
// RESOLVED en la ronda 1 y B da UNRESOLVED; llega una corrección nueva, solo B la
// revalida, y el hallazgo quedaba RESOLVED y la revisión APPROVED sin que A hubiera
// visto jamás ese arreglo (FR-013: la corrección VIGENTE).
func TestRejudgeReview_LaUnanimidadNoCruzaRondas(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 3)

	// Ronda 1: A da por resuelto, B no. Agregado correcto: UNRESOLVED.
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:r1", "C-001")); err != nil {
		t.Fatal(err)
	}
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})); err != nil {
		t.Fatal(err)
	}
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerB,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentUnresolved})); err != nil {
		t.Fatal(err)
	}
	f, _ := ledger.GetConsensusFinding("proj", "acr_test", "C-001")
	if f.RejudgmentState != domain.ReJudgmentUnresolved {
		t.Fatalf("ronda 1 debe quedar UNRESOLVED, quedó %s", f.RejudgmentState)
	}

	// Ronda 2: corrección NUEVA. Solo B la revalida y la da por resuelta.
	// A nunca vio este target corregido.
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:r1", "sha256:r2", "C-001")); err != nil {
		t.Fatal(err)
	}
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerB,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})); err != nil {
		t.Fatal(err)
	}
	f, _ = ledger.GetConsensusFinding("proj", "acr_test", "C-001")

	if f.RejudgmentState == domain.ReJudgmentResolved {
		t.Error("el RESOLVED de A en la ronda 1 completó la unanimidad de la ronda 2")
	}

	// ¿Y llega a aprobar?
	ronda, _ := reviews.GetReview("proj", "acr_test")
	for _, rev := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		r := domain.ReviewerResult{Reviewer: rev, Round: ronda.Round, Status: domain.ReviewerResultSuccess}
		if err := reviews.UpsertReviewerResult("proj", "acr_test", &r); err != nil {
			t.Fatal(err)
		}
	}
	review, err := FinalizeReview(reviews, ledger, "proj", "acr_test")
	if err != nil {
		// Correcto: con el defecto sin resolver y presupuesto disponible, la
		// revisión sigue abierta en vez de aprobarse.
		return
	}
	if review.Verdict == domain.VerdictApproved {
		t.Error("aprobó con un revisor que nunca validó la corrección vigente")
	}
}
