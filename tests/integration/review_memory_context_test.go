package main

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestReviewLearningLlegaAlContextoNormal cierra la Historia 3 (FR-035, AC-008)
// contra SQLite real.
//
// Es el test que decide si la feature entera sirve para algo: una revisión
// puede detectar, confirmar y corregir impecablemente, y si su aprendizaje no
// reaparece solo en la siguiente sesión, el ciclo no se cierra y cada agente
// vuelve a tropezar con el mismo patrón.
//
// Verifica además la decisión de diseño de no crear un almacén paralelo: la
// memoria promovida sale por `get_context` SIN que nada de la ruta de contexto
// sepa que existen las revisiones.
func TestReviewLearningLlegaAlContextoNormal(t *testing.T) {
	root := t.TempDir()
	if err := persistence.EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	const project = "proj-aprendizaje"
	repo := persistence.NewReviewRepository(db)
	ledger := persistence.NewConsensusRepository(db)
	memorias := persistence.NewMemoryRepository(db)

	target, err := domain.NewTarget(domain.TargetDiff, "abc123", "sha256:v0", []string{"internal/"})
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	review := &domain.Review{
		ID: "acr_aprendizaje", Project: project, Target: target,
		CurrentTargetDigest: "sha256:v0",
		MaxFixRounds:        2, FixAuthorized: true, Status: domain.ReviewConsensusReady,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	confirmado := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-001",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		Claim: "la limpieza por expiración borra una memoria refrescada", SourceFindingIDs: []int64{1, 2},
	}
	if err := ledger.UpsertConsensusFinding(project, review.ID, confirmado); err != nil {
		t.Fatalf("UpsertConsensusFinding: %v", err)
	}
	if _, err := usecases.RecordFix(repo, ledger, usecases.RecordFixInput{
		Project: project, ReviewID: review.ID,
		AddressedConsensusIDs: []string{"C-001"},
		BaseTargetDigest:      "sha256:v0", FixedTargetDigest: "sha256:v1",
	}); err != nil {
		t.Fatalf("RecordFix: %v", err)
	}
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		if _, err := usecases.RejudgeReview(repo, ledger, usecases.RejudgeReviewInput{
			Project: project, ReviewID: review.ID, Reviewer: revisor,
			Judgments: map[string]usecases.ReJudgeEntry{
				"C-001": {
					State:    domain.ReJudgmentResolved,
					Evidence: []string{"la expiración ya no borra la memoria refrescada"},
				},
			},
		}); err != nil {
			t.Fatalf("RejudgeReview revisor %s: %v", revisor, err)
		}
	}

	// La promoción exige veredicto APPROVED (FR-021): el aprendizaje de una revisión
	// que todavía puede escalar es una hipótesis, no conocimiento del proyecto.
	for _, revisor := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		resultado := domain.ReviewerResult{
			Reviewer: revisor, Round: 1, Status: domain.ReviewerResultSuccess,
		}
		if err := repo.UpsertReviewerResult(project, review.ID, &resultado); err != nil {
			t.Fatalf("UpsertReviewerResult: %v", err)
		}
	}
	finalizada, err := usecases.FinalizeReview(repo, ledger, project, review.ID)
	if err != nil {
		t.Fatalf("FinalizeReview: %v", err)
	}
	if finalizada.Verdict != domain.VerdictApproved {
		t.Fatalf("la revisión debe quedar APPROVED antes de promover, quedó %s", finalizada.Verdict)
	}

	promovidas, err := usecases.PromoteReviewMemory(repo, ledger, memorias, usecases.PromoteReviewMemoryInput{
		Project: project, ReviewID: review.ID,
		Learnings: map[string]domain.ReviewLearning{
			"C-001": {
				Category: "expiration", Component: "memory",
				Problem:    "la limpieza por expiración borraba una memoria recién refrescada",
				RootCause:  "se usaba la marca de expiración obsoleta tras el refresco",
				Resolution: "se verifica el estado de expiración vigente antes de borrar",
				Verification: []string{
					"TestExpirationDoesNotDeleteRefreshedMemory",
				},
				Confidence: "high",
			},
		},
	})
	if err != nil {
		t.Fatalf("PromoteReviewMemory: %v", err)
	}
	if len(promovidas) != 1 {
		t.Fatalf("se promovieron %d memorias, se esperaba 1", len(promovidas))
	}

	// El linaje debe haber llegado a la columna, no solo al struct.
	var origen string
	if err := db.QueryRow(
		`SELECT COALESCE(source_review_id,'') FROM memories WHERE id = ?`, promovidas[0].ID,
	).Scan(&origen); err != nil {
		t.Fatalf("leer source_review_id: %v", err)
	}
	if origen != review.ID {
		t.Errorf("source_review_id = %q, se esperaba %q", origen, review.ID)
	}

	// Y ahora lo que de verdad importa: aparece en el contexto normal.
	builder := usecases.New(
		persistence.NewMemoryRepository(db),
		persistence.NewSessionRepository(db),
		persistence.NewRelationRepository(db),
		root, project,
	)
	contexto, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, esperado := range []string{"expiration", "memory"} {
		if !strings.Contains(contexto, esperado) {
			t.Errorf("el contexto no menciona %q; el aprendizaje no se recupera solo:\n%s", esperado, contexto)
		}
	}
}
