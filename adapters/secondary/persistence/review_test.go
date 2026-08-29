package persistence

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

func TestReviewSchemaMigrationIsAdditive(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	memoryID, err := InsertMemory(db, &domain.Memory{
		Project: "review-schema",
		Type:    domain.Learning,
		Title:   "dato previo",
		Content: "debe sobrevivir la migración",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dir)
	if err != nil {
		t.Fatalf("abrir una BD preexistente: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"reviews", "reviewer_results", "findings", "consensus_findings", "fix_rounds"} {
		var count int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("la migración no creó la tabla %q", table)
		}
	}

	var title string
	if err := db.QueryRow(`SELECT title FROM memories WHERE id = ?`, memoryID).Scan(&title); err != nil {
		t.Fatalf("la memoria preexistente se perdió: %v", err)
	}
	if title != "dato previo" {
		t.Fatalf("la memoria preexistente cambió: %q", title)
	}
	var sourceReviewID sql.NullInt64
	if err := db.QueryRow(`SELECT source_review_id FROM memories WHERE id = ?`, memoryID).Scan(&sourceReviewID); err != nil {
		t.Fatalf("source_review_id no fue añadida: %v", err)
	}
	if sourceReviewID.Valid {
		t.Fatalf("source_review_id debe ser nullable para datos previos: %d", sourceReviewID.Int64)
	}
}

func TestReviewRepositoryRoundTripAndIdempotentResubmission(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	target, err := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", []string{"domain/"})
	if err != nil {
		t.Fatal(err)
	}
	reviews := NewReviewRepository(db)
	consensus := NewConsensusRepository(db)
	review := &domain.Review{
		ID:                 "acr_test",
		Project:            "proj",
		Target:             target,
		MaxFixRounds:       2,
		AutoFixSeverities:  []domain.Severity{domain.SeverityCritical, domain.SeverityHigh},
		IndependenceLevel:  domain.IndependenceDegraded,
		IndependenceReason: "same-model",
		Status:             domain.ReviewFrozen,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	stored, err := reviews.GetReview("proj", "acr_test")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Target.Digest() != "sha256:frozen" {
		t.Fatalf("review round-trip inválido: %#v", stored)
	}

	result := &domain.ReviewerResult{
		Reviewer: domain.ReviewerA,
		Round:    0,
		Provider: "provider",
		Model:    "model",
		Status:   domain.ReviewerResultSuccess,
		Findings: []domain.Finding{{
			LocalID:       "A-001",
			Location:      "domain/review.go:1",
			Severity:      domain.SeverityHigh,
			Category:      "state",
			Claim:         "primera versión",
			EvidenceClass: domain.EvidenceDeterministic,
			Evidence:      []string{"reproducción"},
			Confidence:    "high",
		}},
	}
	if err := reviews.UpsertReviewerResult("proj", "acr_test", result); err != nil {
		t.Fatal(err)
	}
	result.Findings[0].Claim = "versión actualizada"
	if err := reviews.UpsertReviewerResult("proj", "acr_test", result); err != nil {
		t.Fatal(err)
	}
	results, err := reviews.ListReviewerResults("proj", "acr_test", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].Findings) != 1 {
		t.Fatalf("el reenvío duplicó entidades: results=%d findings=%d", len(results), len(results[0].Findings))
	}
	if results[0].Findings[0].Claim != "versión actualizada" {
		t.Fatalf("el reenvío no actualizó el hallazgo: %#v", results[0].Findings[0])
	}

	ledger := &domain.ConsensusFinding{
		Round:            0,
		ConsensusLocalID: "C-001",
		Status:           domain.ConsensusSuspect,
		Severity:         domain.SeverityHigh,
		Claim:            "versión actualizada",
		SourceFindingIDs: []int64{results[0].Findings[0].ID},
	}
	if err := consensus.UpsertConsensusFinding("proj", "acr_test", ledger); err != nil {
		t.Fatal(err)
	}
	storedLedger, err := consensus.GetConsensusFinding("proj", "acr_test", "C-001")
	if err != nil {
		t.Fatal(err)
	}
	if storedLedger == nil || storedLedger.Status != domain.ConsensusSuspect {
		t.Fatalf("consensus round-trip inválido: %#v", storedLedger)
	}

	delta := &domain.FixDelta{
		Round:                 1,
		BaseTargetDigest:      "sha256:frozen",
		FixedTargetDigest:     "sha256:fixed",
		AddressedConsensusIDs: []string{"C-001"},
		ModifiedPaths:         []string{"domain/review.go"},
		Verification:          []string{"go test ./domain"},
		DiffDigest:            "sha256:delta",
	}
	if err := consensus.UpsertFixDelta("proj", "acr_test", delta); err != nil {
		t.Fatal(err)
	}
	deltas, err := consensus.ListFixDeltas("proj", "acr_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 || deltas[0].FixedTargetDigest != "sha256:fixed" {
		t.Fatalf("fix delta round-trip inválido: %#v", deltas)
	}
}

// TestReviewRedactaSecretosEnTextoLibre cubre FR-041 y el §35 de la entrada.
//
// Un revisor cita el código que analiza: si esa línea trae una credencial, la
// cita entra en `claim`, en `evidence` o en `verification`. Sin redacción,
// gomemory convierte una revisión de seguridad en el sitio donde el secreto
// queda persistido en claro y luego se sirve por `mem review show` — con el
// agravante de que el ledger está pensado para durar.
func TestReviewRedactaSecretosEnTextoLibre(t *testing.T) {
	db := openTestDB(t)
	repo := NewReviewRepository(db)
	ledger := NewConsensusRepository(db)
	const project = "proj-redact"

	target, err := domain.NewTarget(domain.TargetDiff, "abc", "sha256:v0", nil)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	review := &domain.Review{ID: "acr_redact", Project: project, Target: target, MaxFixRounds: 2}
	if err := repo.CreateReview(review); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	const secreto = "ghp_0123456789abcdefghijklmnopqrstuvwxyzAB"

	result := &domain.ReviewerResult{
		ReviewID: review.ID, Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
		Findings: []domain.Finding{{
			LocalID: "A-001", Severity: domain.SeverityHigh, EvidenceClass: domain.EvidenceDeterministic,
			Claim:    "el token " + secreto + " viaja en la URL",
			Evidence: []string{"la petición incluye " + secreto},
		}},
	}
	if err := repo.UpsertReviewerResult(project, review.ID, result); err != nil {
		t.Fatalf("UpsertReviewerResult: %v", err)
	}
	if err := ledger.UpsertConsensusFinding(project, review.ID, &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-001", Status: domain.ConsensusConfirmed,
		Severity: domain.SeverityHigh, Claim: "credencial expuesta: " + secreto,
	}); err != nil {
		t.Fatalf("UpsertConsensusFinding: %v", err)
	}
	if err := ledger.UpsertFixDelta(project, review.ID, &domain.FixDelta{
		ReviewID: review.ID, Round: 1,
		BaseTargetDigest: "sha256:v0", FixedTargetDigest: "sha256:v1",
		Verification: []string{"curl -H 'Authorization: " + secreto + "'"},
	}); err != nil {
		t.Fatalf("UpsertFixDelta: %v", err)
	}

	// Se barre la base entera: da igual por qué columna entró el secreto.
	for _, consulta := range []struct{ nombre, sql string }{
		{"findings.claim", `SELECT COALESCE(claim,'') FROM findings`},
		{"findings.evidence", `SELECT COALESCE(evidence,'') FROM findings`},
		{"consensus.claim", `SELECT COALESCE(claim,'') FROM consensus_findings`},
		{"fix.verification", `SELECT COALESCE(verification,'') FROM fix_rounds`},
	} {
		rows, err := db.Query(consulta.sql)
		if err != nil {
			t.Fatalf("%s: %v", consulta.nombre, err)
		}
		for rows.Next() {
			var valor string
			if err := rows.Scan(&valor); err != nil {
				rows.Close()
				t.Fatalf("%s: %v", consulta.nombre, err)
			}
			if strings.Contains(valor, secreto) {
				rows.Close()
				t.Fatalf("%s guardó el secreto en claro: %s", consulta.nombre, valor)
			}
		}
		rows.Close()
	}
}
