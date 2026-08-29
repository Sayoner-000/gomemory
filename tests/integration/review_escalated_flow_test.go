package main

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// TestReviewEscalatedFlow recorre el Escenario 2 de quickstart.md contra SQLite
// real, no contra dobles: revisión → consenso → fix #1 → re-revisión →
// fix #2 → re-revisión → ESCALATED.
//
// Lo que este test protege y ningún unitario puede: que el ledger persistido
// baste para derivar el veredicto. Los dobles en memoria devuelven lo que se
// les guardó; una columna que no se escribe o un filtro por ronda equivocado
// solo se ven contra la base de datos.
func TestReviewEscalatedFlow(t *testing.T) {
	root := t.TempDir()
	if err := persistence.EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	repo := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	const project = "proj-escalado"

	target, err := domain.NewTarget(domain.TargetDiff, "abc123", "sha256:v0", []string{"internal/"})
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	review := &domain.Review{
		ID: "acr_escalado", Project: project, Target: target,
		MaxFixRounds:      2,
		AutoFixSeverities: []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		Status:            domain.ReviewConsensusReady,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	// Ambos revisores entregan y el consenso confirma un defecto severo.
	enviarAmbosRevisores(t, repo, project, review.ID, 0)
	confirmado := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-001",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		Claim: "escritura concurrente pisa el estado", SourceFindingIDs: []int64{1, 2},
	}
	if err := ledger.UpsertConsensusFinding(project, review.ID, confirmado); err != nil {
		t.Fatalf("UpsertConsensusFinding: %v", err)
	}

	// Dos rondas de corrección insuficiente.
	for ronda := 1; ronda <= 2; ronda++ {
		delta, err := usecases.RecordFix(repo, ledger, usecases.RecordFixInput{
			Project: project, ReviewID: review.ID,
			AddressedConsensusIDs: []string{"C-001"},
			BaseTargetDigest:      "sha256:v" + itoa(ronda-1),
			FixedTargetDigest:     "sha256:v" + itoa(ronda),
			ModifiedPaths:         []string{"internal/memory/store.go"},
		})
		if err != nil {
			t.Fatalf("RecordFix ronda %d: %v", ronda, err)
		}
		if delta.Round != ronda {
			t.Fatalf("ronda registrada = %d, se esperaba %d", delta.Round, ronda)
		}
		enviarAmbosRevisores(t, repo, project, review.ID, ronda)
		if _, err := usecases.RejudgeReview(repo, ledger, usecases.RejudgeReviewInput{
			Project: project, ReviewID: review.ID,
			States: map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentUnresolved},
		}); err != nil {
			t.Fatalf("RejudgeReview ronda %d: %v", ronda, err)
		}
	}

	// La tercera ronda no existe: el presupuesto es la barrera, no una sugerencia.
	if _, err := usecases.RecordFix(repo, ledger, usecases.RecordFixInput{
		Project: project, ReviewID: review.ID,
		AddressedConsensusIDs: []string{"C-001"},
		BaseTargetDigest:      "sha256:v2", FixedTargetDigest: "sha256:v3",
	}); err == nil {
		t.Fatal("se registró una tercera ronda con max_fix_rounds=2")
	}

	final, err := usecases.FinalizeReview(repo, ledger, project, review.ID)
	if err != nil {
		t.Fatalf("FinalizeReview: %v", err)
	}
	if final.Verdict != domain.VerdictEscalated {
		t.Fatalf("veredicto = %s, se esperaba ESCALATED", final.Verdict)
	}
	if final.Status != domain.ReviewEscalated {
		t.Errorf("estado = %s, se esperaba %s", final.Status, domain.ReviewEscalated)
	}
}

func enviarAmbosRevisores(t *testing.T, repo ports.ReviewRepository, project, reviewID string, round int) {
	t.Helper()
	for _, reviewer := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		result := &domain.ReviewerResult{
			ReviewID: reviewID, Reviewer: reviewer, Round: round,
			Status: domain.ReviewerResultSuccess,
		}
		if err := repo.UpsertReviewerResult(project, reviewID, result); err != nil {
			t.Fatalf("UpsertReviewerResult(%s, ronda %d): %v", reviewer, round, err)
		}
	}
}

func itoa(n int) string { return string(rune('0' + n)) }
