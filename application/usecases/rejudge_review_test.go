package usecases

import (
	"strings"
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

// entradaDeReRevision construye el re-juicio de UN revisor. La firma cambió con la
// funcionalidad 028: el mapa plano no podía expresar quién emitía el juicio, así que
// un solo revisor bastaba para marcar RESOLVED y con ello aprobar la revisión.
func entradaDeReRevision(revisor domain.Reviewer, estados map[string]domain.ReJudgmentState) RejudgeReviewInput {
	judgments := make(map[string]ReJudgeEntry, len(estados))
	for localID, estado := range estados {
		judgments[localID] = ReJudgeEntry{
			State:    estado,
			Evidence: []string{"verificado por el revisor " + string(revisor)},
		}
	}
	return RejudgeReviewInput{
		Project: "proj", ReviewID: "acr_test", Reviewer: revisor, Judgments: judgments,
	}
}

// reRevisionUnanime aplica el mismo estado desde los dos revisores, que es lo que
// FR-013 exige para poder declarar un hallazgo resuelto.
func reRevisionUnanime(
	t *testing.T, reviews *memoryReviewRepository, ledger *memoryConsensusRepository,
	estados map[string]domain.ReJudgmentState,
) []domain.ConsensusFinding {
	t.Helper()
	var out []domain.ConsensusFinding
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		findings, err := RejudgeReview(reviews, ledger, entradaDeReRevision(revisor, estados))
		if err != nil {
			t.Fatalf("RejudgeReview(%s): %v", revisor, err)
		}
		out = findings
	}
	return out
}

// TestRejudgeReview_RegistraLosTresEstados: cada hallazgo confirmado termina la
// ronda en RESOLVED, UNRESOLVED o REGRESSED, y el estado queda persistido.
func TestRejudgeReview_RegistraLosTresEstados(t *testing.T) {
	for _, estado := range []domain.ReJudgmentState{
		domain.ReJudgmentResolved, domain.ReJudgmentUnresolved, domain.ReJudgmentRegressed,
	} {
		t.Run(string(estado), func(t *testing.T) {
			reviews, ledger := escenarioReRevisable(t)

			out := reRevisionUnanime(t, reviews, ledger,
				map[string]domain.ReJudgmentState{"C-001": estado})
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

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
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

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
		map[string]domain.ReJudgmentState{"S-001": domain.ReJudgmentResolved},
	))
	if err == nil {
		t.Fatal("se aceptó re-juzgar un SUSPECT")
	}
}

// TestRejudgeReview_RechazaEstadoInvalido: el conjunto de estados es cerrado.
func TestRejudgeReview_RechazaEstadoInvalido(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
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
	reRevisionUnanime(t, reviews, ledger,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})

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
	reRevisionUnanime(t, reviews, ledger,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentUnresolved})

	review, err := FinalizeReview(reviews, ledger, "proj", "acr_test")
	if err == nil && review.Verdict == domain.VerdictApproved {
		t.Fatal("un defecto severo SIN RESOLVER quedó aprobado: la finalización no está viendo el hallazgo")
	}
}

// TestRejudgeReview_UnSoloRevisorNoResuelve es la regresión central de FR-013: con
// el mapa plano anterior, un único juicio bastaba para marcar RESOLVED, y como el
// veredicto se deriva de ese estado, bastaba también para aprobar la revisión.
func TestRejudgeReview_UnSoloRevisorNoResuelve(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)

	out, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved}))
	if err != nil {
		t.Fatal(err)
	}
	if out[0].RejudgmentState != domain.ReJudgmentUnresolved {
		t.Fatalf("con un solo RESOLVED el agregado debe ser UNRESOLVED, fue %s", out[0].RejudgmentState)
	}

	out, err = RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerB,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved}))
	if err != nil {
		t.Fatal(err)
	}
	if out[0].RejudgmentState != domain.ReJudgmentResolved {
		t.Fatalf("con los dos RESOLVED el agregado debe ser RESOLVED, fue %s", out[0].RejudgmentState)
	}
}

// TestRejudgeReview_RegressedManda cubre FR-014: la discrepancia se resuelve del
// lado conservador, no del lado del revisor que mira menos.
func TestRejudgeReview_RegressedManda(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)
	if _, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})); err != nil {
		t.Fatal(err)
	}
	out, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerB,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentRegressed}))
	if err != nil {
		t.Fatal(err)
	}
	if out[0].RejudgmentState != domain.ReJudgmentRegressed {
		t.Fatalf("un REGRESSED debe mandar sobre el RESOLVED del otro, fue %s", out[0].RejudgmentState)
	}
}

// TestRejudgeReview_ExigePertenecerALaCorreccion cubre FR-013: un hallazgo que NINGUNA
// corrección abordó no tiene nada que revalidar, y declararlo resuelto afirmaría un
// arreglo que no existe. Es la frontera que no se mueve.
func TestRejudgeReview_ExigePertenecerALaCorreccion(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)
	otro := &domain.ConsensusFinding{
		ReviewID: "acr_test", ConsensusLocalID: "C-002",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		SourceFindingIDs: []int64{4, 5},
	}
	if err := ledger.UpsertConsensusFinding("proj", "acr_test", otro); err != nil {
		t.Fatal(err)
	}
	// La corrección solo aborda C-001.
	if _, err := RecordFix(reviews, ledger, entradaDeCorreccion("C-001")); err != nil {
		t.Fatal(err)
	}

	_, err := RejudgeReview(reviews, ledger, entradaDeReRevision(domain.ReviewerA,
		map[string]domain.ReJudgmentState{"C-002": domain.ReJudgmentResolved}))
	if err == nil {
		t.Fatal("se aceptó resolver un hallazgo que la corrección no incluye")
	}
	if !strings.Contains(err.Error(), "no lo abordó ninguna corrección") {
		t.Errorf("el error debe explicar que el hallazgo quedó fuera: %v", err)
	}
}

// TestRejudgeReview_AdmiteLoAbordadoEnUnaRondaAnterior es la contracara del test de
// arriba, y la razón por la que la frontera se define por "alguna corrección" y no por
// "la corrección vigente".
//
// Al abrir una ronda se invalida el re-juicio de TODOS los hallazgos, porque la
// corrección nueva pudo regresar cualquiera. Si solo se pudiera re-juzgar lo que esa
// ronda aborda, el hallazgo corregido en la ronda anterior se quedaba sin forma de
// volver a verificarse: el protocolo exigía una verificación que él mismo prohibía
// aportar, y la revisión terminaba escalada o bloqueada con los dos defectos
// corregidos y ambos revisores conformes.
func TestRejudgeReview_AdmiteLoAbordadoEnUnaRondaAnterior(t *testing.T) {
	reviews, ledger, _ := escenarioCorregible(t, 2)
	otro := &domain.ConsensusFinding{
		ReviewID: "acr_test", ConsensusLocalID: "C-002",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		SourceFindingIDs: []int64{4, 5},
	}
	if err := ledger.UpsertConsensusFinding("proj", "acr_test", otro); err != nil {
		t.Fatal(err)
	}
	// Ronda 1 aborda C-001; ronda 2 aborda C-002.
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:base", "sha256:r1", "C-001")); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordFix(reviews, ledger, correccionEncadenada("sha256:r1", "sha256:r2", "C-002")); err != nil {
		t.Fatal(err)
	}

	// C-001 lo abordó la ronda 1, no la vigente: debe poder re-verificarse contra el
	// target de la ronda 2, que es el que está en evaluación.
	out, err := RejudgeReview(reviews, ledger, RejudgeReviewInput{
		Project: "proj", ReviewID: "acr_test", Reviewer: domain.ReviewerA,
		Judgments: map[string]ReJudgeEntry{
			"C-001": {State: domain.ReJudgmentResolved, Evidence: []string{"sigue sin reproducir"}},
		},
	})
	if err != nil {
		t.Fatalf("no se pudo re-verificar un hallazgo corregido en una ronda anterior: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("se esperaba un hallazgo actualizado, se obtuvieron %d", len(out))
	}
	if out[0].RejudgmentRound != 2 {
		t.Fatalf("el re-juicio debe fecharse en la ronda vigente 2, se fechó en la %d", out[0].RejudgmentRound)
	}
}

// TestRejudgeReview_ExigeEvidencia: un re-juicio sin respaldo es una afirmación
// desnuda, y es la afirmación que decide si la revisión puede aprobarse.
func TestRejudgeReview_ExigeEvidencia(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)
	_, err := RejudgeReview(reviews, ledger, RejudgeReviewInput{
		Project: "proj", ReviewID: "acr_test", Reviewer: domain.ReviewerA,
		Judgments: map[string]ReJudgeEntry{"C-001": {State: domain.ReJudgmentResolved}},
	})
	if err == nil {
		t.Fatal("un re-juicio sin evidencia debe rechazarse")
	}
}

func TestRejudgeReview_ExigeDeclararElRevisor(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)
	_, err := RejudgeReview(reviews, ledger, RejudgeReviewInput{
		Project: "proj", ReviewID: "acr_test",
		Judgments: map[string]ReJudgeEntry{
			"C-001": {State: domain.ReJudgmentResolved, Evidence: []string{"x"}},
		},
	})
	if err == nil {
		t.Fatal("un re-juicio sin revisor no es corroboración independiente")
	}
}
