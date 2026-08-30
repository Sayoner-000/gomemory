package main

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// resultadosDeLaRonda reenvía el "success" de ambos revisores en la ronda vigente.
// El veredicto los exige por ronda: sin ellos sale INCOMPLETE y el test pasaría sin
// llegar a ejercitar la regla que quiere comprobar.
func resultadosDeLaRonda(t *testing.T, reviews ports.ReviewRepository, proyecto, reviewID string, ronda int) {
	t.Helper()
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		r := domain.ReviewerResult{Reviewer: revisor, Round: ronda, Status: domain.ReviewerResultSuccess}
		if err := reviews.UpsertReviewerResult(proyecto, reviewID, &r); err != nil {
			t.Fatal(err)
		}
	}
}

// TestUnResueltoNoSobreviveALaRondaSiguiente cubre el defecto CRITICAL C-001 de
// acr_83428b4c: un RESOLVED de la primera corrección aprobaba la segunda.
//
// consensus_findings.rejudgment_state es un valor derivado, y RecordFix abría la ronda
// siguiente sin invalidarlo. El escenario no es rebuscado, es el normal cuando hay dos
// defectos confirmados: la ronda 1 arregla C-001 y no C-002, la ronda 2 aborda C-002,
// y rejudge solo admite los hallazgos que la corrección VIGENTE aborda — así que nadie
// puede volver a juzgar C-001 sobre el código de la ronda 2. Con el estado heredado, el
// veredicto lo leía como resuelto y aprobaba una revisión en la que un CRITICAL nunca
// se verificó contra el target que de verdad estaba en evaluación.
func TestUnResueltoNoSobreviveALaRondaSiguiente(t *testing.T) {
	autorizado := true
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	const proyecto = "vigencia"

	review, err := usecases.StartReview(reviews, usecases.StartReviewInput{
		Project: proyecto, TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: &autorizado,
		AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		ReviewerA:         usecases.ReviewerIdentity{Provider: "one", Model: "a"},
		ReviewerB:         usecases.ReviewerIdentity{Provider: "two", Model: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Dos defectos, confirmados los dos por ambos revisores.
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
		Provider: "one", Model: "a", Findings: []domain.Finding{
			{LocalID: "A-001", Location: "x.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "primer defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
			{LocalID: "A-002", Location: "y.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "segundo defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
		}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
		Provider: "two", Model: "b", Findings: []domain.Finding{
			{LocalID: "B-001", Location: "x.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "mismo primer defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
			{LocalID: "B-002", Location: "y.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "mismo segundo defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
		}}
	for _, result := range []*domain.ReviewerResult{&a, &b} {
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0", Result: *result,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := usecases.BuildConsensus(reviews, ledger, usecases.BuildConsensusInput{
		Project: proyecto, ReviewID: review.ID,
		Matches: []usecases.ConsensusMatch{
			{Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
				Severity: domain.SeverityCritical, Claim: "primer defecto"},
			{Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[1].ID, FindingIDB: b.Findings[1].ID,
				Severity: domain.SeverityCritical, Claim: "segundo defecto"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Ronda 1: se corrige C-001 y los dos revisores lo dan por resuelto.
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

	// Ronda 2: se corrige C-002. Nadie vuelve a mirar C-001 sobre este código.
	if _, err := usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
		Project: proyecto, ReviewID: review.ID, AddressedConsensusIDs: []string{"C-002"},
		BaseTargetDigest: "sha256:v1", FixedTargetDigest: "sha256:v2",
	}); err != nil {
		t.Fatal(err)
	}

	// El RESOLVED de la ronda 1 no debe seguir en pie sobre el target de la ronda 2.
	hallazgos, err := ledger.ListAllConsensusFindings(proyecto, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, hallazgo := range hallazgos {
		if hallazgo.RejudgmentState != "" {
			t.Fatalf("%s conserva rejudgment_state=%s tras abrir la ronda 2",
				hallazgo.ConsensusLocalID, hallazgo.RejudgmentState)
		}
	}

	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		if _, err := usecases.RejudgeReview(reviews, ledger, usecases.RejudgeReviewInput{
			Project: proyecto, ReviewID: review.ID, Reviewer: revisor,
			Judgments: map[string]usecases.ReJudgeEntry{
				"C-002": {State: domain.ReJudgmentResolved, Evidence: []string{"ya no reproduce"}},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}

	// C-002 está resuelto en la ronda vigente, C-001 no se ha vuelto a verificar.
	// Aprobar aquí es aprobar un CRITICAL a ciegas.
	resultadosDeLaRonda(t, reviews, proyecto, review.ID, 2)
	finalizada, err := usecases.FinalizeReview(reviews, ledger, proyecto, review.ID)
	if err == nil && finalizada.Verdict == domain.VerdictApproved {
		t.Fatal("APPROVED con un CRITICAL cuyo último re-juicio pertenece a una ronda anterior")
	}
}

// TestElResueltoDeLaRondaVigenteSiAprueba es el control del test de arriba: sin él,
// exigir la vigencia de la ronda podría estar bloqueando cualquier aprobación y los
// dos tests seguirían en verde.
func TestElResueltoDeLaRondaVigenteSiAprueba(t *testing.T) {
	autorizado := true
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	const proyecto = "vigencia-ok"

	review, err := usecases.StartReview(reviews, usecases.StartReviewInput{
		Project: proyecto, TargetType: domain.TargetDiff, Revision: "working-tree",
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
			LocalID: "A-001", Location: "x.go:1", Severity: domain.SeverityCritical, Category: "correctness",
			Claim: "defecto", Confidence: "high",
			EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}}}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
		Provider: "two", Model: "b", Findings: []domain.Finding{{
			LocalID: "B-001", Location: "x.go:1", Severity: domain.SeverityCritical, Category: "correctness",
			Claim: "mismo defecto", Confidence: "high",
			EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}}}}
	for _, result := range []*domain.ReviewerResult{&a, &b} {
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0", Result: *result,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := usecases.BuildConsensus(reviews, ledger, usecases.BuildConsensusInput{
		Project: proyecto, ReviewID: review.ID,
		Matches: []usecases.ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
			Severity: domain.SeverityCritical, Claim: "defecto"}},
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
	finalizada, err := usecases.FinalizeReview(reviews, ledger, proyecto, review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finalizada.Verdict != domain.VerdictApproved {
		t.Fatalf("verdict=%s, want APPROVED: el re-juicio pertenece a la ronda vigente", finalizada.Verdict)
	}
}

// TestDosDefectosEnRondasDistintasSiPuedenAprobar es la mitad que faltaba, y la que
// destapó la revisión adversarial de v2.16.3.
//
// El test de arriba comprueba que un RESOLVED de la ronda 1 NO aprueba la ronda 2. Es
// correcto, pero por sí solo no distingue "se cerró la puerta de más". Al exigir la
// vigencia de ronda sin ampliar qué se puede re-juzgar, el protocolo pasó a exigir una
// verificación que él mismo prohibía aportar: con dos defectos corregidos en rondas
// distintas no había NINGUNA forma de llegar a APPROVED, ni con más presupuesto.
//
// Un test que solo verifica que una puerta se cierra no detecta que se cerraron dos.
func TestDosDefectosEnRondasDistintasSiPuedenAprobar(t *testing.T) {
	autorizado := true
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	const proyecto = "dos-rondas"

	review, err := usecases.StartReview(reviews, usecases.StartReviewInput{
		Project: proyecto, TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: &autorizado,
		AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		ReviewerA:         usecases.ReviewerIdentity{Provider: "one", Model: "a"},
		ReviewerB:         usecases.ReviewerIdentity{Provider: "two", Model: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
		Provider: "one", Model: "a", Findings: []domain.Finding{
			{LocalID: "A-001", Location: "x.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "primer defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
			{LocalID: "A-002", Location: "y.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "segundo defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
		}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
		Provider: "two", Model: "b", Findings: []domain.Finding{
			{LocalID: "B-001", Location: "x.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "mismo primer defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
			{LocalID: "B-002", Location: "y.go:1", Severity: domain.SeverityCritical, Category: "correctness",
				Claim: "mismo segundo defecto", Confidence: "high",
				EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"reproducido"}},
		}}
	for _, result := range []*domain.ReviewerResult{&a, &b} {
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: proyecto, ReviewID: review.ID, TargetDigest: "sha256:v0", Result: *result,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := usecases.BuildConsensus(reviews, ledger, usecases.BuildConsensusInput{
		Project: proyecto, ReviewID: review.ID,
		Matches: []usecases.ConsensusMatch{
			{Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
				Severity: domain.SeverityCritical, Claim: "primer defecto"},
			{Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[1].ID, FindingIDB: b.Findings[1].ID,
				Severity: domain.SeverityCritical, Claim: "segundo defecto"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Ronda 1: se corrige C-001 y ambos revisores lo dan por resuelto.
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

	// Ronda 2: se corrige C-002. Ahora ambos revisores verifican LOS DOS contra el
	// target de esta ronda, que es justo lo que antes era imposible.
	if _, err := usecases.RecordFix(reviews, ledger, usecases.RecordFixInput{
		Project: proyecto, ReviewID: review.ID, AddressedConsensusIDs: []string{"C-002"},
		BaseTargetDigest: "sha256:v1", FixedTargetDigest: "sha256:v2",
	}); err != nil {
		t.Fatal(err)
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		if _, err := usecases.RejudgeReview(reviews, ledger, usecases.RejudgeReviewInput{
			Project: proyecto, ReviewID: review.ID, Reviewer: revisor,
			Judgments: map[string]usecases.ReJudgeEntry{
				"C-001": {State: domain.ReJudgmentResolved, Evidence: []string{"sigue sin reproducir sobre v2"}},
				"C-002": {State: domain.ReJudgmentResolved, Evidence: []string{"ya no reproduce"}},
			},
		}); err != nil {
			t.Fatalf("no se pudo re-verificar en la ronda vigente: %v", err)
		}
	}

	resultadosDeLaRonda(t, reviews, proyecto, review.ID, 2)
	finalizada, err := usecases.FinalizeReview(reviews, ledger, proyecto, review.ID)
	if err != nil {
		t.Fatalf("la revisión no pudo cerrarse con los dos defectos verificados: %v", err)
	}
	if finalizada.Verdict != domain.VerdictApproved {
		t.Fatalf("verdict=%s, want APPROVED: los dos defectos están corregidos y verificados por ambos revisores en la ronda vigente",
			finalizada.Verdict)
	}
}
