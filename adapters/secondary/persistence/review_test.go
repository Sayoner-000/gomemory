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

func TestReviewerResultAtomicoRechazaTerminalSinMutarElLedger(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewReviewRepository(db)
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: "acr_resultado_terminal", Project: "proj", Target: target,
		Status: domain.ReviewAwaitingReviewers,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	original := &domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Round: 0, Status: domain.ReviewerResultSuccess,
	}
	if err := repo.UpsertReviewerResult("proj", review.ID, original); err != nil {
		t.Fatal(err)
	}
	review.Status = domain.ReviewApproved
	review.Verdict = domain.VerdictApproved
	if err := repo.UpdateReview(review); err != nil {
		t.Fatal(err)
	}

	tardio := &domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Round: 0, Status: domain.ReviewerResultFailure,
	}
	if err := repo.UpsertReviewerResultAtomically(
		"proj", review.ID, domain.ReviewAwaitingReviewers, 0, tardio,
	); err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("un resultado tardío debe rechazarse antes de escribir, got %v", err)
	}
	results, err := repo.ListReviewerResults("proj", review.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != domain.ReviewerResultSuccess {
		t.Fatalf("el resultado terminal fue mutado: %#v", results)
	}
}

func TestReviewerResultsMarkCambiaConElLedger(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewReviewRepository(db)
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{
		ID: "acr_marca_resultados", Project: "proj", Target: target,
		Status: domain.ReviewAwaitingReviewers,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	result := &domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Round: 0, Status: domain.ReviewerResultSuccess,
	}
	if err := repo.UpsertReviewerResult("proj", review.ID, result); err != nil {
		t.Fatal(err)
	}
	antes, err := repo.ReviewerResultsMark("proj", review.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	result.Status = domain.ReviewerResultFailure
	if err := repo.UpsertReviewerResult("proj", review.ID, result); err != nil {
		t.Fatal(err)
	}
	despues, err := repo.ReviewerResultsMark("proj", review.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if antes == despues {
		t.Fatal("la marca no cambió al cambiar el estado del resultado")
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

// TestMigracion028EsAditivaSobreEsquemaPrevio comprueba que las columnas y la tabla
// que introduce la funcionalidad 028 se añaden sobre una base que ya tiene el esquema
// de la 027, sin perder filas y sin volver NOT NULL nada preexistente.
//
// Se construye la base "vieja" con Open y luego se le retiran las novedades de 028,
// que es lo más cerca que se puede estar de una instalación anterior sin versionar un
// dump binario en el repositorio.
func TestMigracion028EsAditivaSobreEsquemaPrevio(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	target, err := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:previo", nil)
	if err != nil {
		t.Fatal(err)
	}
	reviews := NewReviewRepository(db)
	previa := &domain.Review{
		ID: "acr_previa", Project: "proj", Target: target,
		MaxFixRounds: 2, Status: domain.ReviewAwaitingReviewers,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := reviews.CreateReview(previa); err != nil {
		t.Fatal(err)
	}
	// Simula la base anterior a 028: sin la tabla nueva y con las columnas nuevas a
	// NULL, que es como quedan las filas escritas por una versión anterior. Ponerlas
	// a NULL a mano es imprescindible: si se deja que las escriba el código actual,
	// el test comprueba la ruta nueva y no la de compatibilidad.
	if _, err := db.Exec(`DROP TABLE IF EXISTS rejudgments`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE reviews SET current_target_digest = NULL, fix_authorized = NULL,
		reviewer_a_provider = NULL, reviewer_a_model = NULL,
		reviewer_b_provider = NULL, reviewer_b_model = NULL`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dir)
	if err != nil {
		t.Fatalf("reabrir una base previa a 028: %v", err)
	}
	defer db.Close()

	columnasNuevas := map[string][]string{
		"reviews": {
			"current_target_digest", "fix_authorized",
			"reviewer_a_provider", "reviewer_a_model",
			"reviewer_b_provider", "reviewer_b_model",
		},
		"consensus_findings": {"round_fingerprint"},
	}
	for tabla, columnas := range columnasNuevas {
		info := map[string]bool{}
		rows, err := db.Query(`SELECT name, "notnull" FROM pragma_table_info(?)`, tabla)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var nombre string
			var notNull int
			if err := rows.Scan(&nombre, &notNull); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			info[nombre] = notNull == 1
		}
		rows.Close()
		for _, columna := range columnas {
			obligatoria, existe := info[columna]
			if !existe {
				t.Errorf("la migración no añadió %s.%s", tabla, columna)
				continue
			}
			if obligatoria {
				t.Errorf("%s.%s se creó NOT NULL: rompería las filas previas", tabla, columna)
			}
		}
	}

	var tablas int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'rejudgments'`,
	).Scan(&tablas); err != nil {
		t.Fatal(err)
	}
	if tablas != 1 {
		t.Error("la migración no creó la tabla rejudgments")
	}

	recuperada, err := NewReviewRepository(db).GetReview("proj", "acr_previa")
	if err != nil {
		t.Fatalf("la revisión previa dejó de leerse: %v", err)
	}
	if recuperada == nil {
		t.Fatal("la revisión previa se perdió en la migración")
	}
	// Una revisión anterior a 028 no declara nada: el target vigente es el original
	// y la corrección se asume autorizada, para no cambiar su comportamiento.
	if recuperada.CurrentTargetDigest != "sha256:previo" {
		t.Errorf("CurrentTargetDigest = %q, se esperaba el digest original", recuperada.CurrentTargetDigest)
	}
	if !recuperada.FixAuthorized {
		t.Error("una revisión previa a 028 debe seguir autorizando corrección")
	}
}

// TestReJudgmentRedactaEvidencia cierra FR-027 sobre el campo nuevo. La evidencia de
// un re-juicio es texto libre que un revisor copia del código que acaba de verificar:
// si esa línea trae una credencial, sin redacción el ledger la persiste en claro y
// después la sirve por `mem review show`.
func TestReJudgmentRedactaEvidencia(t *testing.T) {
	db := openTestDB(t)
	repo := NewReviewRepository(db)
	ledger := NewConsensusRepository(db)
	const project = "proj-rejudge-redact"

	target, err := domain.NewTarget(domain.TargetDiff, "abc", "sha256:v0", nil)
	if err != nil {
		t.Fatal(err)
	}
	review := &domain.Review{
		ID: "acr_rejudge", Project: project, Target: target,
		CurrentTargetDigest: "sha256:v0", MaxFixRounds: 2, FixAuthorized: true,
		Status: domain.ReviewConsensusReady,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	confirmado := &domain.ConsensusFinding{
		ReviewID: review.ID, ConsensusLocalID: "C-001",
		Status: domain.ConsensusConfirmed, Severity: domain.SeverityHigh,
		SourceFindingIDs: []int64{1, 2},
	}
	if err := ledger.UpsertConsensusFinding(project, review.ID, confirmado); err != nil {
		t.Fatal(err)
	}

	const secreto = "ghp_0123456789abcdefghijklmnopqrstuvwxyzAB"
	judgment := &domain.ReJudgment{
		ReviewID: review.ID, Round: 1, ConsensusLocalID: "C-001",
		Reviewer: domain.ReviewerA, State: domain.ReJudgmentResolved,
		Evidence: []string{"verificado con token " + secreto},
	}
	if err := ledger.UpsertReJudgment(project, review.ID, judgment); err != nil {
		t.Fatal(err)
	}

	var crudo string
	if err := db.QueryRow(`SELECT evidence FROM rejudgments WHERE id = ?`, judgment.ID).Scan(&crudo); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(crudo, secreto) {
		t.Fatal("el secreto llegó en claro a la tabla rejudgments")
	}

	leidos, err := ledger.ListReJudgments(project, review.ID, "C-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(leidos) != 1 {
		t.Fatalf("se persistieron %d re-juicios, se esperaba 1", len(leidos))
	}
	if strings.Contains(strings.Join(leidos[0].Evidence, " "), secreto) {
		t.Error("el secreto vuelve a salir al leer el re-juicio")
	}

	// El estado agregado se recalcula en la misma transacción: con un solo
	// revisor no puede quedar RESOLVED.
	finding, err := ledger.GetConsensusFinding(project, review.ID, "C-001")
	if err != nil {
		t.Fatal(err)
	}
	if finding.RejudgmentState != domain.ReJudgmentUnresolved {
		t.Errorf("estado agregado = %s, con un solo revisor debe ser UNRESOLVED", finding.RejudgmentState)
	}
}

// TestListConsensusFindings_RevisionInexistenteDevuelveVacio fija el contrato del
// método: una revisión que no existe no es un error, es una lista vacía.
//
// Se rompió al extraer la consulta para poder releerla dentro de una transacción: el
// lookup que se añadió delante traduce "no hay filas" en error. No tenía llamadores en
// producción, que es justo lo que lo hacía fácil de dejar pasar.
func TestListConsensusFindings_RevisionInexistenteDevuelveVacio(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ledger := NewConsensusRepository(db)

	out, err := ledger.ListConsensusFindings("proj", "acr_no_existe", 0)
	if err != nil {
		t.Fatalf("una revisión inexistente no es un error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("se esperaba lista vacía, llegaron %d hallazgos", len(out))
	}
}
