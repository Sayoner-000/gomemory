package main

import (
	"database/sql"
	"sync"
	"testing"

	"mem/application/ports"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// revisionConHallazgo deja una revisión lista para competir: dos resultados de revisor
// enviados, un CRITICAL confirmado y una CONTRADICTION severa que hace el veredicto
// siempre derivable, para que la finalización llegue a escribir de verdad.
func revisionConHallazgo(t *testing.T, db *sql.DB, proyecto, reviewID string) (
	ports.ReviewRepository, ports.ConsensusRepository,
) {
	t.Helper()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: reviewID, Project: proyecto, Target: target,
		CurrentTargetDigest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: true,
		AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		Status:            domain.ReviewConsensusReady,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		r := domain.ReviewerResult{Reviewer: revisor, Round: 0, Status: domain.ReviewerResultSuccess}
		if err := reviews.UpsertReviewerResult(proyecto, reviewID, &r); err != nil {
			t.Fatal(err)
		}
	}
	for _, hallazgo := range []domain.ConsensusFinding{
		{ReviewID: reviewID, ConsensusLocalID: "C-001", Status: domain.ConsensusConfirmed,
			Severity: domain.SeverityCritical, SourceFindingIDs: []int64{1, 2}},
		{ReviewID: reviewID, ConsensusLocalID: "C-002", Status: domain.ConsensusContradiction,
			Severity: domain.SeverityHigh, SourceFindingIDs: []int64{3, 4}},
	} {
		copia := hallazgo
		if err := ledger.UpsertConsensusFinding(proyecto, reviewID, &copia); err != nil {
			t.Fatal(err)
		}
	}
	return reviews, ledger
}

// TestEnviarUnResultadoNoReabreLoTerminal cubre el defecto que la corrección de la
// carrera de finalización dejó a medias.
//
// Se quitó el UPDATE ciego de la finalización, pero el envío de resultados —el único
// llamador que quedaba de ese método— seguía asignando el estado directamente,
// saltándose la máquina de estados, y reescribiendo TODAS las columnas desde un objeto
// leído fuera de transacción: reabría revisiones terminales, borraba veredictos ya
// escritos y devolvía la ronda y el target a los valores de antes con la corrección ya
// registrada en el ledger.
func TestEnviarUnResultadoNoReabreLoTerminal(t *testing.T) {
	const proyecto = "ciegas"
	for intento := range 40 {
		db, err := persistence.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reviews, ledger := revisionConHallazgo(t, db, proyecto, "acr_ciegas")

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			usecases.FinalizeReview(reviews, ledger, proyecto, "acr_ciegas")
		}()
		go func() {
			defer wg.Done()
			usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
				Project: proyecto, ReviewID: "acr_ciegas", TargetDigest: "sha256:v0",
				Result: domain.ReviewerResult{
					Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
				},
			})
		}()
		wg.Wait()

		final, err := reviews.GetReview(proyecto, "acr_ciegas")
		if err != nil {
			t.Fatal(err)
		}
		// Un veredicto escrito implica estado terminal. Que el envío lo borre o
		// devuelva la revisión a consensus_ready es la reapertura que se corrige.
		if final.Verdict != "" && !final.Status.Terminal() {
			t.Fatalf("intento %d: veredicto %s con la revisión reabierta en %s",
				intento, final.Verdict, final.Status)
		}
		if final.Status.Terminal() && final.Verdict == "" {
			t.Fatalf("intento %d: estado terminal %s sin veredicto: se borró por una escritura tardía",
				intento, final.Status)
		}
		db.Close()
	}
}

// TestUnResultadoTardioNoRevierteUnaCorreccion fija de forma determinista el tercer
// daño: la ronda y el target vigente no pueden retroceder por un envío de resultados.
func TestUnResultadoTardioNoRevierteUnaCorreccion(t *testing.T) {
	const proyecto = "ciegas-fix"
	for intento := range 40 {
		db, err := persistence.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reviews, ledger := revisionConHallazgo(t, db, proyecto, "acr_cf")

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
				Project: proyecto, ReviewID: "acr_cf", AddressedConsensusIDs: []string{"C-001"},
				BaseTargetDigest: "sha256:v0", FixedTargetDigest: "sha256:v1",
			})
		}()
		go func() {
			defer wg.Done()
			usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
				Project: proyecto, ReviewID: "acr_cf", TargetDigest: "sha256:v0",
				Result: domain.ReviewerResult{
					Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
				},
			})
		}()
		wg.Wait()

		final, err := reviews.GetReview(proyecto, "acr_cf")
		if err != nil {
			t.Fatal(err)
		}
		correcciones, err := ledger.ListFixDeltas(proyecto, "acr_cf")
		if err != nil {
			t.Fatal(err)
		}
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
		db.Close()
	}
}

// TestUnaRetractacionEnLaVentanaImpideAprobar cubre la parte de la comparación-y-cambio
// que faltaba: el veredicto se deriva también de los re-juicios, y un re-juicio no toca
// el estado, ni la ronda, ni el target.
//
// Un revisor que se retracta —cambia su RESOLVED por REGRESSED en la ronda vigente—
// entre la lectura y la escritura pasaba las tres guardas y dejaba registrado APPROVED
// sobre un hallazgo severo que el ledger marca como reaparecido.
func TestUnaRetractacionEnLaVentanaImpideAprobar(t *testing.T) {
	autorizado := true
	const proyecto = "retractacion"
	for intento := range 60 {
		db, err := persistence.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reviews := persistence.NewReviewRepository(db)
		ledger := persistence.NewConsensusRepository(db)

		review, err := usecases.StartReview(reviews, usecases.StartReviewInput{
			Project: proyecto, TargetType: domain.TargetDiff, Revision: "wt",
			Digest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: &autorizado,
			AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
			ReviewerA:         usecases.ReviewerIdentity{Provider: "one", Model: "a"},
			ReviewerB:         usecases.ReviewerIdentity{Provider: "two", Model: "b"},
		})
		if err != nil {
			t.Fatal(err)
		}
		a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
			Provider: "one", Model: "a", Findings: []domain.Finding{{
				LocalID: "A-001", Location: "x.go:1", Severity: domain.SeverityCritical,
				Category: "correctness", Claim: "defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}}}}
		bb := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
			Provider: "two", Model: "b", Findings: []domain.Finding{{
				LocalID: "B-001", Location: "x.go:1", Severity: domain.SeverityCritical,
				Category: "correctness", Claim: "mismo defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}}}}
		for _, result := range []*domain.ReviewerResult{&a, &bb} {
			if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
				Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0", Result: *result,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := usecases.BuildConsensus(reviews, ledger, usecases.BuildConsensusInput{
			Project: proyecto, ReviewID: review.ID,
			Matches: []usecases.ConsensusMatch{{
				Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID,
				FindingIDB: bb.Findings[0].ID, Severity: domain.SeverityCritical, Claim: "defecto"}},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
			Project: proyecto, ReviewID: review.ID, AddressedConsensusIDs: []string{"C-001"},
			BaseTargetDigest: "sha256:v0", FixedTargetDigest: "sha256:v1",
		}); err != nil {
			t.Fatal(err)
		}
		for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
			if _, err := usecases.RejudgeReview(reviews, ledger, usecases.RejudgeReviewInput{
				Project: proyecto, ReviewID: review.ID, Reviewer: revisor,
				Judgments: map[string]usecases.ReJudgeEntry{
					"C-001": {State: domain.ReJudgmentResolved, Evidence: []string{"ya no reproduce"}},
				},
			}); err != nil {
				t.Fatal(err)
			}
		}
		resultadosDeLaRonda(t, reviews, proyecto, review.ID, 1)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			usecases.FinalizeReview(reviews, ledger, proyecto, review.ID)
		}()
		go func() {
			defer wg.Done()
			usecases.RejudgeReview(reviews, ledger, usecases.RejudgeReviewInput{
				Project: proyecto, ReviewID: review.ID, Reviewer: domain.ReviewerA,
				Judgments: map[string]usecases.ReJudgeEntry{
					"C-001": {State: domain.ReJudgmentRegressed, Evidence: []string{"vuelve a reproducir"}},
				},
			})
		}()
		wg.Wait()

		final, err := reviews.GetReview(proyecto, review.ID)
		if err != nil {
			t.Fatal(err)
		}
		hallazgos, err := ledger.ListAllConsensusFindings(proyecto, review.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, hallazgo := range hallazgos {
			if !hallazgo.Severity.Severe() || hallazgo.Status != domain.ConsensusConfirmed {
				continue
			}
			if final.Verdict == domain.VerdictApproved && !hallazgo.ResueltoEn(final.Round) {
				t.Fatalf("intento %d: APPROVED con %s en estado %q de la ronda %d (vigente %d)",
					intento, hallazgo.ConsensusLocalID, hallazgo.RejudgmentState,
					hallazgo.RejudgmentRound, final.Round)
			}
		}
		db.Close()
	}
}

// TestUnaMarcaDeReJuiciosObsoletaRechazaElCierre prueba de forma determinista la
// ventana que el test concurrente de arriba NO alcanza de forma fiable: la
// retractación que CONFIRMA antes de que la finalización escriba.
//
// Ahí la revisión todavía no es terminal, así que la guarda de estado no aplica; y el
// estado, la ronda y el target no cambian, así que las otras tres comparaciones pasan.
// Lo único que se mueve es el conjunto de re-juicios, y por eso el cierre lleva su
// marca: sin ella se persiste APPROVED sobre un hallazgo que el ledger ya registra
// como reaparecido.
func TestUnaMarcaDeReJuiciosObsoletaRechazaElCierre(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const proyecto = "marca"
	reviews, ledger := revisionConHallazgo(t, db, proyecto, "acr_marca")

	if err := ledger.RecordFixAtomically(proyecto, "acr_marca", ports.FixTransition{
		Delta: &domain.FixDelta{
			ReviewID: "acr_marca", Round: 1, BaseTargetDigest: "sha256:v0",
			FixedTargetDigest: "sha256:v1", AddressedConsensusIDs: []string{"C-001"},
		},
		ExpectedRounds: 0, ExpectedBaseDigest: "sha256:v0",
		ExpectedStatus: domain.ReviewConsensusReady,
		NextRound:      1, NextStatus: domain.ReviewRejudging, CurrentTargetDigest: "sha256:v1",
	}); err != nil {
		t.Fatal(err)
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		j := domain.ReJudgment{
			ReviewID: "acr_marca", Round: 1, ConsensusLocalID: "C-001",
			Reviewer: revisor, State: domain.ReJudgmentResolved,
			Evidence: []string{"ya no reproduce"},
		}
		if err := ledger.UpsertReJudgment(proyecto, "acr_marca", &j); err != nil {
			t.Fatal(err)
		}
	}

	// Esta es la marca que la finalización habría leído antes de derivar.
	marca, err := reviews.RejudgmentMark(proyecto, "acr_marca")
	if err != nil {
		t.Fatal(err)
	}

	// Y aquí el revisor A se retracta, antes de que la finalización escriba.
	retractacion := domain.ReJudgment{
		ReviewID: "acr_marca", Round: 1, ConsensusLocalID: "C-001",
		Reviewer: domain.ReviewerA, State: domain.ReJudgmentRegressed,
		Evidence: []string{"vuelve a reproducir"},
	}
	if err := ledger.UpsertReJudgment(proyecto, "acr_marca", &retractacion); err != nil {
		t.Fatal(err)
	}

	previa, err := reviews.GetReview(proyecto, "acr_marca")
	if err != nil {
		t.Fatal(err)
	}
	err = reviews.SetReviewStatusAtomically(proyecto, "acr_marca", ports.StatusTransition{
		ExpectedStatus:         domain.ReviewRejudging,
		ExpectedRound:          1,
		ExpectedDigest:         "sha256:v1",
		ExpectedRejudgmentMark: marca,
		Verdict:                domain.VerdictApproved,
		NextStatus:             domain.ReviewApproved,
	})
	if err == nil {
		t.Fatal("se aprobó con una marca de re-juicios obsoleta: la retractación quedó invisible")
	}

	final, err := reviews.GetReview(proyecto, "acr_marca")
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != previa.Status || final.Verdict != previa.Verdict {
		t.Fatalf("un cierre rechazado no debe dejar rastro: %s/%q pasó a %s/%q",
			previa.Status, previa.Verdict, final.Status, final.Verdict)
	}
}

func TestUnaMarcaDeResultadosObsoletaRechazaElCierre(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const proyecto = "marca-resultados"
	reviews, _ := revisionConHallazgo(t, db, proyecto, "acr_marca_resultados")

	marca, err := reviews.ReviewerResultsMark(proyecto, "acr_marca_resultados", 0)
	if err != nil {
		t.Fatal(err)
	}
	falloTardio := domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Round: 0, Status: domain.ReviewerResultFailure,
	}
	// Escritura de bajo nivel para simular exactamente el cambio que puede caer
	// entre la lectura y el CAS de finalización.
	if err := reviews.UpsertReviewerResult(proyecto, "acr_marca_resultados", &falloTardio); err != nil {
		t.Fatal(err)
	}

	err = reviews.SetReviewStatusAtomically(proyecto, "acr_marca_resultados", ports.StatusTransition{
		ExpectedStatus:              domain.ReviewConsensusReady,
		ExpectedRound:               0,
		ExpectedDigest:              "sha256:v0",
		ExpectedReviewerResultsMark: marca,
		Verdict:                     domain.VerdictApproved,
		NextStatus:                  domain.ReviewApproved,
	})
	if err == nil {
		t.Fatal("se aprobó con una marca de resultados obsoleta: el fallo tardío quedó invisible")
	}
	final, err := reviews.GetReview(proyecto, "acr_marca_resultados")
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != domain.ReviewConsensusReady || final.Verdict != "" {
		t.Fatalf("un cierre rechazado dejó rastro: status=%s verdict=%q", final.Status, final.Verdict)
	}
}

// TestUnFailureTrasConsensusReadyAlcanzaIncomplete cubre la regresión que la guarda
// de fase introdujo: cerró la única puerta que quedaba al fail-closed.
//
// consensus_ready lo escribe el PROPIO envío del segundo revisor, así que la ventana
// se cerraba en el mismo instante en que ambos terminaban. A partir de ahí ningún
// revisor podía declarar failure y la transición consensus_ready -> incomplete
// quedaba inalcanzable: el dominio seguía declarándola legal y ya no había quien la
// ejecutara. Una revisión cuya ejecución fue inválida terminaba APPROVED.
func TestUnFailureTrasConsensusReadyAlcanzaIncomplete(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const proyecto = "failure-tardio"
	reviews := persistence.NewReviewRepository(db)
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: "acr_failure_tardio", Project: proyecto, Target: target,
		Status: domain.ReviewAwaitingReviewers,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: proyecto, ReviewID: "acr_failure_tardio", TargetDigest: "sha256:v0",
			Result: domain.ReviewerResult{Reviewer: revisor, Status: domain.ReviewerResultSuccess},
		}); err != nil {
			t.Fatalf("envío de %s: %v", revisor, err)
		}
	}
	previa, err := reviews.GetReview(proyecto, "acr_failure_tardio")
	if err != nil {
		t.Fatal(err)
	}
	if previa.Status != domain.ReviewConsensusReady {
		t.Fatalf("el escenario exige consensus_ready, está en %s", previa.Status)
	}

	// A descubre que su propia ejecución fue inválida.
	if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
		Project: proyecto, ReviewID: "acr_failure_tardio", TargetDigest: "sha256:v0",
		Result: domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultFailure},
	}); err != nil {
		t.Fatalf("un failure debe poder declararse siempre: %v", err)
	}

	final, err := reviews.GetReview(proyecto, "acr_failure_tardio")
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != domain.ReviewIncomplete || final.Verdict != domain.VerdictIncomplete {
		t.Fatalf("el fail-closed no se alcanzó: status=%s verdict=%q", final.Status, final.Verdict)
	}
}

// TestUnReenvioIdenticoTrasConsensusReadySigueSiendoNoOp cubre FR-039: la guarda de
// fase debe impedir que un reenvío MUTE las fuentes del consenso, no convertir un
// reintento de transporte en un error duro.
func TestUnReenvioIdenticoTrasConsensusReadySigueSiendoNoOp(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const proyecto = "reenvio-identico"
	reviews := persistence.NewReviewRepository(db)
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: "acr_reenvio", Project: proyecto, Target: target,
		Status: domain.ReviewAwaitingReviewers,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	resultadoDeA := func() domain.ReviewerResult {
		return domain.ReviewerResult{
			Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
			Findings: []domain.Finding{{
				LocalID: "A-001", Location: "domain/verdict.go:42", Severity: domain.SeverityHigh,
				Category: "correctness", Claim: "defecto", EvidenceClass: "deterministic",
				Evidence: []string{"e1"}, Confidence: "high",
			}},
		}
	}
	for _, entrada := range []domain.ReviewerResult{
		resultadoDeA(),
		{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess},
	} {
		copia := entrada
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: proyecto, ReviewID: "acr_reenvio", TargetDigest: "sha256:v0", Result: copia,
		}); err != nil {
			t.Fatalf("envío de %s: %v", entrada.Reviewer, err)
		}
	}

	// Reintento del cliente MCP tras un timeout cuya escritura sí se confirmó.
	if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
		Project: proyecto, ReviewID: "acr_reenvio", TargetDigest: "sha256:v0", Result: resultadoDeA(),
	}); err != nil {
		t.Fatalf("un reenvío idéntico debe seguir siendo un no-op: %v", err)
	}

	// Y lo que la guarda sí debe seguir impidiendo: mutar la fuente del consenso.
	mutado := resultadoDeA()
	mutado.Findings[0].Claim = "consenso distinto"
	if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
		Project: proyecto, ReviewID: "acr_reenvio", TargetDigest: "sha256:v0", Result: mutado,
	}); err == nil {
		t.Fatal("un reenvío que altera el contenido debe rechazarse")
	}
	stored, err := reviews.ListReviewerResults(proyecto, "acr_reenvio", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range stored {
		if r.Reviewer == domain.ReviewerA && r.Findings[0].Claim != "defecto" {
			t.Fatalf("la fuente del consenso fue alterada: %q", r.Findings[0].Claim)
		}
	}
}

// TestUnFailureFueraDeFaseNoPuedeTraerHallazgos cierra el agujero que abrió la propia
// corrección del fail-closed.
//
// Dejar pasar TODO resultado failure sin mirar su contenido convirtió la declaración
// de fallo en una vía de escritura libre: persistReviewerResult escribe los hallazgos
// con ON CONFLICT DO UPDATE, así que un failure con un hallazgo del mismo local_id
// reescribía el claim, la severidad o la evidencia de la fuente sobre la que YA se
// había construido el consenso. La guarda existía justo para impedir eso.
//
// Declarar failure sigue siendo siempre posible —es lo que sostiene INV-010—, pero
// fuera de la fase de recogida no puede traer hallazgos, por el mismo motivo que una
// ronda de revalidación tampoco los admite: no hay consenso que pueda clasificarlos.
func TestUnFailureFueraDeFaseNoPuedeTraerHallazgos(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const proyecto = "failure-con-hallazgos"
	reviews := persistence.NewReviewRepository(db)
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: "acr_failure_hallazgos", Project: proyecto, Target: target,
		Status: domain.ReviewAwaitingReviewers,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	hallazgo := func(claim string) domain.Finding {
		return domain.Finding{
			LocalID: "A-001", Location: "domain/verdict.go:42", Severity: domain.SeverityHigh,
			Category: "correctness", Claim: claim, EvidenceClass: "deterministic",
			Evidence: []string{"e1"}, Confidence: "high",
		}
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		resultado := domain.ReviewerResult{Reviewer: revisor, Status: domain.ReviewerResultSuccess}
		if revisor == domain.ReviewerA {
			resultado.Findings = []domain.Finding{hallazgo("defecto original")}
		}
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0", Result: resultado,
		}); err != nil {
			t.Fatalf("envío de %s: %v", revisor, err)
		}
	}

	// El failure que intenta reescribir la fuente del consenso al pasar.
	_, err = usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
		Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0",
		Result: domain.ReviewerResult{
			Reviewer: domain.ReviewerA, Status: domain.ReviewerResultFailure,
			Findings: []domain.Finding{hallazgo("CLAIM REESCRITO")},
		},
	})
	if err == nil {
		t.Fatal("un failure fuera de fase no puede traer hallazgos: sería una vía de escritura libre")
	}

	stored, err := reviews.ListReviewerResults(proyecto, review.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, resultado := range stored {
		for _, f := range resultado.Findings {
			if f.Claim != "defecto original" {
				t.Fatalf("la fuente del consenso fue reescrita: %q", f.Claim)
			}
		}
	}

	// Y el fail-closed sigue intacto: sin hallazgos, el failure pasa y llega a INCOMPLETE.
	if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
		Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0",
		Result: domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultFailure},
	}); err != nil {
		t.Fatalf("declarar failure sin hallazgos debe seguir siendo posible: %v", err)
	}
	final, err := reviews.GetReview(proyecto, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != domain.ReviewIncomplete {
		t.Fatalf("el fail-closed dejó de alcanzarse: %s", final.Status)
	}
}
