package usecases

import (
	"testing"

	"mem/domain"
)

// escenarioReRevisable deja una revisión con una corrección ya registrada, que
// es la precondición de la re-revisión (INV-008).
func escenarioReRevisable(t *testing.T) (*memoryReviewRepository, *memoryConsensusRepository) {
	t.Helper()
	reviews, ledger, _ := escenarioCorregible(t, 2)
	if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001")); err != nil {
		t.Fatalf("RecordFix: %v", err)
	}
	return reviews, ledger
}

func entradaDeReRevision(estados map[string]domain.ReJudgmentState) RejudgeReviewInput {
	return RejudgeReviewInput{Project: "proj", ReviewID: "acr_test", States: estados}
}

// TestRejudgeReview_RegistraLosTresEstados: cada hallazgo confirmado termina la
// ronda en RESOLVED, UNRESOLVED o REGRESSED, y el estado queda persistido.
func TestRejudgeReview_RegistraLosTresEstados(t *testing.T) {
	for _, estado := range []domain.ReJudgmentState{
		domain.ReJudgmentResolved, domain.ReJudgmentUnresolved, domain.ReJudgmentRegressed,
	} {
		t.Run(string(estado), func(t *testing.T) {
			reviews, ledger := escenarioReRevisable(t)

			out, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
				map[string]domain.ReJudgmentState{"C-001": estado},
			))
			if err != nil {
				t.Fatalf("RejudgeReview: %v", err)
			}
			if len(out) != 1 || out[0].RejudgmentState != estado {
				t.Fatalf("estado devuelto = %v, se esperaba %s", out, estado)
			}

			persistido, _ := ledger.GetConsensusFinding("proj", "acr_test", "C-001")
			if persistido.RejudgmentState != estado {
				t.Errorf("estado persistido = %s, se esperaba %s", persistido.RejudgmentState, estado)
			}
		})
	}
}

// TestRejudgeReview_ExigeUnaCorreccionPrevia cubre INV-008: la re-revisión
// evalúa un fix delta concreto. Sin corrección registrada no hay nada que
// revalidar, y aceptarlo permitiría marcar RESOLVED sin que nadie arreglara nada.
func TestRejudgeReview_ExigeUnaCorreccionPrevia(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved},
	))
	if err == nil {
		t.Fatal("se aceptó una re-revisión sin corrección previa")
	}
}

// TestRejudgeReview_SoloConfirmados: un SUSPECT no pasó por corrección, así que
// no tiene resolución que declarar.
func TestRejudgeReview_SoloConfirmados(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"S-001": domain.ReJudgmentResolved},
	))
	if err == nil {
		t.Fatal("se aceptó re-juzgar un SUSPECT")
	}
}

// TestRejudgeReview_RechazaEstadoInvalido: el conjunto de estados es cerrado.
func TestRejudgeReview_RechazaEstadoInvalido(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentState("CASI")},
	))
	if err == nil {
		t.Fatal("se aceptó un estado de re-revisión inventado")
	}
}

// TestRejudgeReview_ResueltoPermiteAprobar cierra el ciclo de la Historia 2:
// tras marcar RESOLVED, la finalización deja la revisión aprobada.
func TestRejudgeReview_ResueltoPermiteAprobar(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)

	ronda, _ := reviews.GetReview("proj", "acr_test")
	for _, reviewer := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		resultado := domain.ReviewerResult{
			Reviewer: reviewer, Round: ronda.Round, Status: domain.ReviewerResultSuccess,
		}
		if err := reviews.UpsertReviewerResult("proj", "acr_test", &resultado); err != nil {
			t.Fatalf("UpsertReviewerResult: %v", err)
		}
	}
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved},
	)); err != nil {
		t.Fatalf("RejudgeReview: %v", err)
	}

	review, err := FinalizeReview(reviews, ledger, "proj", "acr_test")
	if err != nil {
		t.Fatalf("FinalizeReview: %v", err)
	}
	if review.Verdict != domain.VerdictApproved {
		t.Errorf("veredicto = %s, se esperaba APPROVED", review.Verdict)
	}
}

// TestRejudgeReview_SinResolverNoAprueba es el contraste que hace válido al
// test de arriba: si UNRESOLVED también aprobara, aquel verde no probaría nada.
func TestRejudgeReview_SinResolverNoAprueba(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)

	ronda, _ := reviews.GetReview("proj", "acr_test")
	for _, reviewer := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		resultado := domain.ReviewerResult{
			Reviewer: reviewer, Round: ronda.Round, Status: domain.ReviewerResultSuccess,
		}
		if err := reviews.UpsertReviewerResult("proj", "acr_test", &resultado); err != nil {
			t.Fatalf("UpsertReviewerResult: %v", err)
		}
	}
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentUnresolved},
	)); err != nil {
		t.Fatalf("RejudgeReview: %v", err)
	}

	review, err := FinalizeReview(reviews, ledger, "proj", "acr_test")
	if err == nil && review.Verdict == domain.VerdictApproved {
		t.Fatal("un defecto severo SIN RESOLVER quedó aprobado: la finalización no está viendo el hallazgo")
	}
}
