package persistence

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"mem/application/ports"
	"mem/domain"
)

type ReviewRepository struct {
	db *sql.DB
}

func NewReviewRepository(db *sql.DB) ports.ReviewRepository {
	return &ReviewRepository{db: db}
}

func NewConsensusRepository(db *sql.DB) ports.ConsensusRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) CreateReview(review *domain.Review) error {
	scope, err := json.Marshal(review.Target.Scope)
	if err != nil {
		return fmt.Errorf("encode target scope: %w", err)
	}
	severities, err := json.Marshal(review.AutoFixSeverities)
	if err != nil {
		return fmt.Errorf("encode auto-fix severities: %w", err)
	}
	_, err = r.db.Exec(`
		INSERT INTO reviews (
			project, review_id, target_type, target_revision, target_digest, target_scope,
			max_fix_rounds, auto_fix_severities, independence_level, independence_reason,
			round, status, verdict, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), `+Now+`), COALESCE(NULLIF(?, ''), `+Now+`))`,
		review.Project, review.ID, string(review.Target.Type), review.Target.Revision, review.Target.Digest(), string(scope),
		review.MaxFixRounds, string(severities), string(review.IndependenceLevel), review.IndependenceReason,
		review.Round, string(review.Status), nullableVerdict(review.Verdict), formatReviewTime(review.CreatedAt),
		formatReviewTime(review.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) GetReview(project, reviewID string) (*domain.Review, error) {
	row := r.db.QueryRow(`
		SELECT target_type, target_revision, target_digest, target_scope, max_fix_rounds,
		       auto_fix_severities, independence_level, independence_reason, round, status,
		       verdict, created_at, updated_at
		FROM reviews WHERE project = ? AND review_id = ?`, project, reviewID)
	return scanReview(row, project, reviewID)
}

func (r *ReviewRepository) UpdateReview(review *domain.Review) error {
	severities, err := json.Marshal(review.AutoFixSeverities)
	if err != nil {
		return fmt.Errorf("encode auto-fix severities: %w", err)
	}
	result, err := r.db.Exec(`
		UPDATE reviews SET max_fix_rounds = ?, auto_fix_severities = ?, independence_level = ?,
			independence_reason = ?, round = ?, status = ?, verdict = ?, updated_at = `+Now+`
		WHERE project = ? AND review_id = ?`,
		review.MaxFixRounds, string(severities), string(review.IndependenceLevel), review.IndependenceReason,
		review.Round, string(review.Status), nullableVerdict(review.Verdict), review.Project, review.ID,
	)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return requireAffected(result, "review")
}

func (r *ReviewRepository) ListReviews(project string, limit int) ([]domain.Review, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(`
		SELECT review_id, target_type, target_revision, target_digest, target_scope, max_fix_rounds,
		       auto_fix_severities, independence_level, independence_reason, round, status,
		       verdict, created_at, updated_at
		FROM reviews WHERE project = ? ORDER BY id DESC LIMIT ?`, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()
	var out []domain.Review
	for rows.Next() {
		var reviewID string
		var targetType, revision, digest, scopeJSON, severitiesJSON, independence, reason, status string
		var maxRounds, round int
		var verdict sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&reviewID, &targetType, &revision, &digest, &scopeJSON, &maxRounds,
			&severitiesJSON, &independence, &reason, &round, &status, &verdict, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		review, err := reviewFromValues(project, reviewID, targetType, revision, digest, scopeJSON, maxRounds,
			severitiesJSON, independence, reason, round, status, verdict, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, *review)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) UpsertReviewerResult(project, reviewID string, result *domain.ReviewerResult) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	internalID, err := reviewInternalID(tx, project, reviewID)
	if err != nil {
		return err
	}
	var resultID int64
	err = tx.QueryRow(`
		INSERT INTO reviewer_results (review_id, reviewer, round, provider, model, status, submitted_at)
		VALUES (?, ?, ?, ?, ?, ?, `+Now+`)
		ON CONFLICT(review_id, reviewer, round) DO UPDATE SET
			provider = excluded.provider, model = excluded.model, status = excluded.status, submitted_at = excluded.submitted_at
		RETURNING id`,
		internalID, string(result.Reviewer), result.Round, result.Provider, result.Model, string(result.Status),
	).Scan(&resultID)
	if err != nil {
		return fmt.Errorf("upsert reviewer result: %w", err)
	}
	result.ID = resultID
	result.ReviewID = reviewID
	for i := range result.Findings {
		finding := &result.Findings[i]
		evidence, err := json.Marshal(redactarLista(finding.Evidence))
		if err != nil {
			return fmt.Errorf("encode finding evidence: %w", err)
		}
		err = tx.QueryRow(`
			INSERT INTO findings (reviewer_result_id, local_id, location, severity, category, claim, evidence_class, evidence, confidence)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(reviewer_result_id, local_id) DO UPDATE SET
				location = excluded.location, severity = excluded.severity, category = excluded.category,
				claim = excluded.claim, evidence_class = excluded.evidence_class,
				evidence = excluded.evidence, confidence = excluded.confidence
			RETURNING id`, resultID, finding.LocalID, finding.Location, string(finding.Severity), finding.Category,
			redactarTexto(finding.Claim), string(finding.EvidenceClass), string(evidence), finding.Confidence).Scan(&finding.ID)
		if err != nil {
			return fmt.Errorf("upsert finding %s: %w", finding.LocalID, err)
		}
		finding.ReviewerResultID = resultID
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reviewer result: %w", err)
	}
	return nil
}

func (r *ReviewRepository) ListReviewerResults(project, reviewID string, round int) ([]domain.ReviewerResult, error) {
	rows, err := r.db.Query(`
		SELECT rr.id, rr.reviewer, rr.round, rr.provider, rr.model, rr.status, rr.submitted_at
		FROM reviewer_results rr JOIN reviews rv ON rv.id = rr.review_id
		WHERE rv.project = ? AND rv.review_id = ? AND rr.round = ? ORDER BY rr.reviewer`, project, reviewID, round)
	if err != nil {
		return nil, fmt.Errorf("list reviewer results: %w", err)
	}
	defer rows.Close()
	var out []domain.ReviewerResult
	for rows.Next() {
		var result domain.ReviewerResult
		var reviewer, status, submittedAt string
		if err := rows.Scan(&result.ID, &reviewer, &result.Round, &result.Provider, &result.Model, &status, &submittedAt); err != nil {
			return nil, err
		}
		result.ReviewID = reviewID
		result.Reviewer = domain.Reviewer(reviewer)
		result.Status = domain.ReviewerResultStatus(status)
		result.SubmittedAt = parseReviewTime(submittedAt)
		result.Findings, err = r.findingsForResult(result.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) GetFinding(project, reviewID string, findingID int64) (*domain.Finding, error) {
	row := r.db.QueryRow(`
		SELECT f.id, f.reviewer_result_id, f.local_id, f.location, f.severity, f.category,
		       f.claim, f.evidence_class, f.evidence, f.confidence
		FROM findings f
		JOIN reviewer_results rr ON rr.id = f.reviewer_result_id
		JOIN reviews rv ON rv.id = rr.review_id
		WHERE rv.project = ? AND rv.review_id = ? AND f.id = ?`, project, reviewID, findingID)
	finding, err := scanFinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return finding, err
}

func (r *ReviewRepository) ListFindings(project, reviewID string, round int) ([]domain.Finding, error) {
	rows, err := r.db.Query(`
		SELECT f.id, f.reviewer_result_id, f.local_id, f.location, f.severity, f.category,
		       f.claim, f.evidence_class, f.evidence, f.confidence
		FROM findings f
		JOIN reviewer_results rr ON rr.id = f.reviewer_result_id
		JOIN reviews rv ON rv.id = rr.review_id
		WHERE rv.project = ? AND rv.review_id = ? AND rr.round = ? ORDER BY f.id`, project, reviewID, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Finding
	for rows.Next() {
		finding, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *finding)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) UpsertConsensusFinding(project, reviewID string, finding *domain.ConsensusFinding) error {
	internalID, err := r.lookupReviewID(project, reviewID)
	if err != nil {
		return err
	}
	sources, err := json.Marshal(finding.SourceFindingIDs)
	if err != nil {
		return err
	}
	err = r.db.QueryRow(`
		INSERT INTO consensus_findings
			(review_id, round, consensus_local_id, status, severity, claim, source_finding_ids, rejudgment_state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(review_id, consensus_local_id) DO UPDATE SET
			round = excluded.round, status = excluded.status, severity = excluded.severity,
			claim = excluded.claim, source_finding_ids = excluded.source_finding_ids,
			rejudgment_state = excluded.rejudgment_state
		RETURNING id`, internalID, finding.Round, finding.ConsensusLocalID, string(finding.Status),
		string(finding.Severity), redactarTexto(finding.Claim), string(sources), nullableRejudgment(finding.RejudgmentState)).Scan(&finding.ID)
	if err != nil {
		return fmt.Errorf("upsert consensus finding: %w", err)
	}
	finding.ReviewID = reviewID
	return nil
}

func (r *ReviewRepository) GetConsensusFinding(project, reviewID, localID string) (*domain.ConsensusFinding, error) {
	row := r.db.QueryRow(`
		SELECT cf.id, cf.round, cf.consensus_local_id, cf.status, cf.severity, cf.claim,
		       cf.source_finding_ids, cf.rejudgment_state
		FROM consensus_findings cf JOIN reviews rv ON rv.id = cf.review_id
		WHERE rv.project = ? AND rv.review_id = ? AND cf.consensus_local_id = ?`, project, reviewID, localID)
	finding, err := scanConsensus(row, reviewID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return finding, err
}

func (r *ReviewRepository) ListConsensusFindings(project, reviewID string, round int) ([]domain.ConsensusFinding, error) {
	rows, err := r.db.Query(`
		SELECT cf.id, cf.round, cf.consensus_local_id, cf.status, cf.severity, cf.claim,
		       cf.source_finding_ids, cf.rejudgment_state
		FROM consensus_findings cf JOIN reviews rv ON rv.id = cf.review_id
		WHERE rv.project = ? AND rv.review_id = ? AND cf.round = ? ORDER BY cf.id`, project, reviewID, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ConsensusFinding
	for rows.Next() {
		finding, err := scanConsensus(rows, reviewID)
		if err != nil {
			return nil, err
		}
		out = append(out, *finding)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) ListAllConsensusFindings(project, reviewID string) ([]domain.ConsensusFinding, error) {
	rows, err := r.db.Query(`
		SELECT cf.id, cf.round, cf.consensus_local_id, cf.status, cf.severity, cf.claim,
		       cf.source_finding_ids, cf.rejudgment_state
		FROM consensus_findings cf JOIN reviews rv ON rv.id = cf.review_id
		WHERE rv.project = ? AND rv.review_id = ? ORDER BY cf.id`, project, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ConsensusFinding
	for rows.Next() {
		finding, err := scanConsensus(rows, reviewID)
		if err != nil {
			return nil, err
		}
		out = append(out, *finding)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) UpsertFixDelta(project, reviewID string, delta *domain.FixDelta) error {
	internalID, err := r.lookupReviewID(project, reviewID)
	if err != nil {
		return err
	}
	addressed, _ := json.Marshal(delta.AddressedConsensusIDs)
	paths, _ := json.Marshal(delta.ModifiedPaths)
	verification, _ := json.Marshal(redactarLista(delta.Verification))
	err = r.db.QueryRow(`
		INSERT INTO fix_rounds
			(review_id, round, base_target_digest, fixed_target_digest, addressed_consensus_ids,
			 modified_paths, verification, diff_digest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(review_id, round) DO UPDATE SET
			base_target_digest = excluded.base_target_digest, fixed_target_digest = excluded.fixed_target_digest,
			addressed_consensus_ids = excluded.addressed_consensus_ids, modified_paths = excluded.modified_paths,
			verification = excluded.verification, diff_digest = excluded.diff_digest
		RETURNING id`, internalID, delta.Round, delta.BaseTargetDigest, delta.FixedTargetDigest, string(addressed),
		string(paths), string(verification), delta.DiffDigest).Scan(&delta.ID)
	if err != nil {
		return fmt.Errorf("upsert fix delta: %w", err)
	}
	delta.ReviewID = reviewID
	return nil
}

func (r *ReviewRepository) ListFixDeltas(project, reviewID string) ([]domain.FixDelta, error) {
	rows, err := r.db.Query(`
		SELECT fr.id, fr.round, fr.base_target_digest, fr.fixed_target_digest,
		       fr.addressed_consensus_ids, fr.modified_paths, fr.verification, fr.diff_digest, fr.created_at
		FROM fix_rounds fr JOIN reviews rv ON rv.id = fr.review_id
		WHERE rv.project = ? AND rv.review_id = ? ORDER BY fr.round`, project, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FixDelta
	for rows.Next() {
		var delta domain.FixDelta
		var addressed, paths, verification, createdAt string
		if err := rows.Scan(&delta.ID, &delta.Round, &delta.BaseTargetDigest, &delta.FixedTargetDigest,
			&addressed, &paths, &verification, &delta.DiffDigest, &createdAt); err != nil {
			return nil, err
		}
		delta.ReviewID = reviewID
		delta.CreatedAt = parseReviewTime(createdAt)
		if err := json.Unmarshal([]byte(addressed), &delta.AddressedConsensusIDs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(paths), &delta.ModifiedPaths); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(verification), &delta.Verification); err != nil {
			return nil, err
		}
		out = append(out, delta)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReview(row scanner, project, reviewID string) (*domain.Review, error) {
	var targetType, revision, digest, scopeJSON, severitiesJSON, independence, reason, status string
	var maxRounds, round int
	var verdict sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&targetType, &revision, &digest, &scopeJSON, &maxRounds, &severitiesJSON,
		&independence, &reason, &round, &status, &verdict, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return reviewFromValues(project, reviewID, targetType, revision, digest, scopeJSON, maxRounds,
		severitiesJSON, independence, reason, round, status, verdict, createdAt, updatedAt)
}

func reviewFromValues(project, reviewID, targetType, revision, digest, scopeJSON string, maxRounds int,
	severitiesJSON, independence, reason string, round int, status string, verdict sql.NullString,
	createdAt, updatedAt string,
) (*domain.Review, error) {
	var scope []string
	if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
		return nil, err
	}
	var severities []domain.Severity
	if err := json.Unmarshal([]byte(severitiesJSON), &severities); err != nil {
		return nil, err
	}
	target, err := domain.NewTarget(domain.TargetType(targetType), revision, digest, scope)
	if err != nil {
		return nil, err
	}
	target.CreatedAt = parseReviewTime(createdAt)
	return &domain.Review{
		ID: reviewID, Project: project, Target: target, MaxFixRounds: maxRounds, AutoFixSeverities: severities,
		IndependenceLevel: domain.IndependenceLevel(independence), IndependenceReason: reason, Round: round,
		Status: domain.ReviewStatus(status), Verdict: domain.Verdict(verdict.String),
		CreatedAt: parseReviewTime(createdAt), UpdatedAt: parseReviewTime(updatedAt),
	}, nil
}

func scanFinding(row scanner) (*domain.Finding, error) {
	var finding domain.Finding
	var severity, evidenceClass, evidence string
	if err := row.Scan(&finding.ID, &finding.ReviewerResultID, &finding.LocalID, &finding.Location, &severity,
		&finding.Category, &finding.Claim, &evidenceClass, &evidence, &finding.Confidence); err != nil {
		return nil, err
	}
	finding.Severity = domain.Severity(severity)
	finding.EvidenceClass = domain.EvidenceClass(evidenceClass)
	if err := json.Unmarshal([]byte(evidence), &finding.Evidence); err != nil {
		return nil, err
	}
	return &finding, nil
}

func (r *ReviewRepository) findingsForResult(resultID int64) ([]domain.Finding, error) {
	rows, err := r.db.Query(`
		SELECT id, reviewer_result_id, local_id, location, severity, category, claim, evidence_class, evidence, confidence
		FROM findings WHERE reviewer_result_id = ? ORDER BY id`, resultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Finding
	for rows.Next() {
		finding, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *finding)
	}
	return out, rows.Err()
}

func scanConsensus(row scanner, reviewID string) (*domain.ConsensusFinding, error) {
	var finding domain.ConsensusFinding
	var status, severity, sources string
	var rejudgment sql.NullString
	if err := row.Scan(&finding.ID, &finding.Round, &finding.ConsensusLocalID, &status, &severity,
		&finding.Claim, &sources, &rejudgment); err != nil {
		return nil, err
	}
	finding.ReviewID = reviewID
	finding.Status = domain.ConsensusStatus(status)
	finding.Severity = domain.Severity(severity)
	finding.RejudgmentState = domain.ReJudgmentState(rejudgment.String)
	if err := json.Unmarshal([]byte(sources), &finding.SourceFindingIDs); err != nil {
		return nil, err
	}
	return &finding, nil
}

func (r *ReviewRepository) lookupReviewID(project, reviewID string) (int64, error) {
	var id int64
	err := r.db.QueryRow(`SELECT id FROM reviews WHERE project = ? AND review_id = ?`, project, reviewID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("review %s not found", reviewID)
	}
	return id, err
}

func reviewInternalID(tx *sql.Tx, project, reviewID string) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM reviews WHERE project = ? AND review_id = ?`, project, reviewID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("review %s not found", reviewID)
	}
	return id, err
}

func requireAffected(result sql.Result, entity string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s not found", entity)
	}
	return nil
}

func nullableVerdict(verdict domain.Verdict) any {
	if verdict == "" {
		return nil
	}
	return string(verdict)
}

func nullableRejudgment(state domain.ReJudgmentState) any {
	if state == "" {
		return nil
	}
	return string(state)
}

func formatReviewTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func parseReviewTime(value string) time.Time {
	return parseStoredTime(sql.NullString{String: value, Valid: value != ""})
}

// redactarTexto aplica al ledger de revisión la misma depuración que ya protege
// a las memorias (InsertMemory).
//
// Un revisor CITA el código que analiza: si esa línea trae una credencial, la
// cita entra en `claim`, en `evidence` o en `verification`. Sin esto, una
// revisión de seguridad se convierte en el sitio donde el secreto queda
// persistido en claro y luego se sirve por `mem review show` — y el ledger está
// pensado para durar, así que el descuido no caduca.
func redactarTexto(s string) string {
	return domain.RedactSecrets(domain.RedactPrivate(s))
}

func redactarLista(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, redactarTexto(item))
	}
	return out
}
