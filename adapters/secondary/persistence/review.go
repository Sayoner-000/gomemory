package persistence

import (
	"context"
	"crypto/sha256"
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
			round, status, verdict, created_at, updated_at,
			current_target_digest, fix_authorized,
			reviewer_a_provider, reviewer_a_model, reviewer_b_provider, reviewer_b_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), `+Now+`), COALESCE(NULLIF(?, ''), `+Now+`),
			?, ?, ?, ?, ?, ?)`,
		review.Project, review.ID, string(review.Target.Type), review.Target.Revision, review.Target.Digest(), string(scope),
		review.MaxFixRounds, string(severities), string(review.IndependenceLevel), review.IndependenceReason,
		review.Round, string(review.Status), nullableVerdict(review.Verdict), formatReviewTime(review.CreatedAt),
		formatReviewTime(review.UpdatedAt),
		review.ActiveTargetDigest(), boolToInt(review.FixAuthorized),
		review.ReviewerA.Provider, review.ReviewerA.Model, review.ReviewerB.Provider, review.ReviewerB.Model,
	)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) GetReview(project, reviewID string) (*domain.Review, error) {
	row := r.db.QueryRow(`
		SELECT `+reviewColumns+`
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
			independence_reason = ?, round = ?, status = ?, verdict = ?, updated_at = `+Now+`,
			current_target_digest = ?, fix_authorized = ?
		WHERE project = ? AND review_id = ?`,
		review.MaxFixRounds, string(severities), string(review.IndependenceLevel), review.IndependenceReason,
		review.Round, string(review.Status), nullableVerdict(review.Verdict),
		review.ActiveTargetDigest(), boolToInt(review.FixAuthorized), review.Project, review.ID,
	)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return requireAffected(result, "review")
}

// RejudgmentMark resume en un valor comparable el conjunto de re-juicios de una
// revisión: cuántos hay y cuál es el más reciente por identificador y por estado.
//
// Se calcula en el motor y no se acumula en un contador para que no pueda
// desincronizarse de las filas de las que sale. Un alta cambia el recuento; una
// modificación por ON CONFLICT cambia el estado agregado sin cambiar el recuento, y
// por eso la marca lleva las dos cosas.
func (r *ReviewRepository) RejudgmentMark(project, reviewID string) (string, error) {
	internalID, err := r.lookupReviewID(project, reviewID)
	if err != nil {
		return "", err
	}
	return rejudgmentMark(r.db, internalID)
}

func rejudgmentMark(q querier, internalID int64) (string, error) {
	rows, err := q.Query(`
		SELECT COUNT(*), COALESCE(MAX(id), 0), COALESCE(GROUP_CONCAT(id || ':' || state, ','), '')
		FROM (SELECT id, state FROM rejudgments WHERE review_id = ? ORDER BY id)`, internalID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", rows.Err()
	}
	var total, maxID int64
	var estados string
	if err := rows.Scan(&total, &maxID, &estados); err != nil {
		return "", err
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%x", total, maxID, sha256.Sum256([]byte(estados))), nil
}

// SetReviewStatusAtomically escribe el estado de una revisión bajo comparación-y-cambio.
//
// La superficie de escritura es deliberadamente mínima: verdict, status y updated_at.
// La finalización y el envío de resultados usaban UpdateReview, que reescribe TODAS las columnas desde un objeto
// leído fuera de cualquier transacción; si una corrección se colaba en medio, la
// finalización devolvía `round` y `current_target_digest` a los valores obsoletos que
// había leído y encima cerraba la revisión. Al no existir aquí ninguna sentencia capaz
// de tocar esas columnas, ese daño deja de ser improbable y pasa a ser inexpresable.
//
// Lo que la guarda sí decide es si el veredicto sigue siendo válido: se derivó de un
// estado, una ronda y un target concretos, y si alguno cambió ya no es el mismo juicio.
func (r *ReviewRepository) SetReviewStatusAtomically(
	project, reviewID string, transition ports.StatusTransition,
) error {
	ctx := context.Background()
	// Conexión dedicada con BEGIN IMMEDIATE, por el motivo ya documentado en
	// RecordFixAtomically: el BEGIN diferido de database/sql toma el bloqueo en la
	// primera escritura y en WAL el perdedor recibe un SQLITE_BUSY que el
	// busy_timeout no reintenta.
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("tomar el bloqueo de escritura: %w", err)
	}
	comprometida := false
	defer func() {
		if !comprometida {
			conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()

	var internalID int64
	var estado string
	var ronda int
	var vigente, original sql.NullString
	err = conn.QueryRowContext(ctx,
		`SELECT id, status, round, current_target_digest, target_digest FROM reviews
		 WHERE project = ? AND review_id = ?`,
		project, reviewID).Scan(&internalID, &estado, &ronda, &vigente, &original)
	if err == sql.ErrNoRows {
		return fmt.Errorf("review %s not found", reviewID)
	}
	if err != nil {
		return err
	}

	if domain.ReviewStatus(estado) != transition.ExpectedStatus {
		return fmt.Errorf(
			"la revisión pasó a %s mientras se finalizaba: vuelve a derivar el veredicto",
			estado,
		)
	}
	if ronda != transition.ExpectedRound {
		return fmt.Errorf(
			"la revisión avanzó a la ronda %d mientras se finalizaba: vuelve a derivar el veredicto",
			ronda,
		)
	}
	actual := vigente.String
	if actual == "" {
		actual = original.String
	}
	if transition.ExpectedDigest != "" && actual != transition.ExpectedDigest {
		return fmt.Errorf(
			"el target vigente cambió mientras se finalizaba: el veredicto ya no corresponde a %s",
			transition.ExpectedDigest,
		)
	}
	// La marca de re-juicios se relee DENTRO de la transacción. Es la comprobación
	// que las otras tres no cubren: un re-juicio no toca status, round ni digest.
	if transition.ExpectedRejudgmentMark != "" {
		marca, err := rejudgmentMark(connQuerier{ctx, conn}, internalID)
		if err != nil {
			return err
		}
		if marca != transition.ExpectedRejudgmentMark {
			return fmt.Errorf(
				"los re-juicios cambiaron mientras se finalizaba: vuelve a derivar el veredicto",
			)
		}
	}

	result, err := conn.ExecContext(ctx,
		`UPDATE reviews SET status = ?, verdict = ?, updated_at = `+Now+` WHERE id = ?`,
		string(transition.NextStatus), nullableVerdict(transition.Verdict), internalID)
	if err != nil {
		return fmt.Errorf("escribir el estado de la revisión: %w", err)
	}
	if err := requireAffected(result, "review"); err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("confirmar el cambio de estado: %w", err)
	}
	comprometida = true
	return nil
}

func (r *ReviewRepository) ListReviews(project string, limit int) ([]domain.Review, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(`
		SELECT review_id, `+reviewColumns+`
		FROM reviews WHERE project = ? ORDER BY id DESC LIMIT ?`, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()
	var out []domain.Review
	for rows.Next() {
		var reviewID string
		var raw reviewRow
		if err := rows.Scan(append([]any{&reviewID}, raw.dest()...)...); err != nil {
			return nil, err
		}
		review, err := raw.toDomain(project, reviewID)
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

// upsertConsensusSQL escribe un hallazgo de consenso. Se comparte entre la escritura
// suelta y ReplaceConsensusRound para que la ronda completa y la fila individual no
// puedan divergir en columnas.
const upsertConsensusSQL = `
	INSERT INTO consensus_findings
		(review_id, round, consensus_local_id, status, severity, claim, source_finding_ids,
		 rejudgment_state, round_fingerprint, rejudgment_round)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(review_id, consensus_local_id) DO UPDATE SET
		round = excluded.round, status = excluded.status, severity = excluded.severity,
		claim = excluded.claim, source_finding_ids = excluded.source_finding_ids,
		rejudgment_state = excluded.rejudgment_state, round_fingerprint = excluded.round_fingerprint,
		rejudgment_round = excluded.rejudgment_round
	RETURNING id`

func consensusArgs(internalID int64, finding *domain.ConsensusFinding) ([]any, error) {
	sources, err := json.Marshal(finding.SourceFindingIDs)
	if err != nil {
		return nil, err
	}
	return []any{
		internalID, finding.Round, finding.ConsensusLocalID, string(finding.Status),
		string(finding.Severity), redactarTexto(finding.Claim), string(sources),
		nullableRejudgment(finding.RejudgmentState), finding.RoundFingerprint,
		finding.RejudgmentRound,
	}, nil
}

func (r *ReviewRepository) UpsertConsensusFinding(project, reviewID string, finding *domain.ConsensusFinding) error {
	internalID, err := r.lookupReviewID(project, reviewID)
	if err != nil {
		return err
	}
	args, err := consensusArgs(internalID, finding)
	if err != nil {
		return err
	}
	if err := r.db.QueryRow(upsertConsensusSQL, args...).Scan(&finding.ID); err != nil {
		return fmt.Errorf("upsert consensus finding: %w", err)
	}
	finding.ReviewID = reviewID
	return nil
}

// ReplaceConsensusRound persiste la clasificación completa de una ronda dentro de una
// única transacción, con la comprobación de "ya existe" hecha DENTRO de ella.
//
// Antes esto era un check-then-write en el caso de uso: listar los hallazgos, ver el
// ledger vacío y recorrer un bucle de inserciones, cada una en su propia transacción
// implícita. Dos llamadas simultáneas veían las dos el ledger vacío y escribían las
// dos, dejando la ronda mezclada; y un error a mitad del bucle dejaba media
// clasificación escrita, que es exactamente el estado que hayFuentesSinClasificar
// tiene que detectar más tarde. Ahora la ronda entra entera o no entra.
func (r *ReviewRepository) ReplaceConsensusRound(
	project, reviewID string, expectedRound int, fingerprint string,
	findings []domain.ConsensusFinding,
) ([]domain.ConsensusFinding, bool, error) {
	ctx := context.Background()
	// Conexión dedicada con BEGIN IMMEDIATE, por el mismo motivo que
	// RecordFixAtomically y UpsertReJudgment: el BEGIN diferido de database/sql toma
	// el bloqueo en la primera escritura y en WAL el perdedor recibe un SQLITE_BUSY
	// que el busy_timeout no reintenta para una transacción que ya leyó.
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return nil, false, fmt.Errorf("tomar el bloqueo de escritura: %w", err)
	}
	comprometida := false
	defer func() {
		if !comprometida {
			conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()

	var internalID int64
	var estado string
	var ronda int
	err = conn.QueryRowContext(ctx,
		`SELECT id, status, round FROM reviews WHERE project = ? AND review_id = ?`,
		project, reviewID).Scan(&internalID, &estado, &ronda)
	if err == sql.ErrNoRows {
		return nil, false, fmt.Errorf("review %s not found", reviewID)
	}
	if err != nil {
		return nil, false, err
	}
	// Estado y ronda se revalidan aquí dentro: el caso de uso los comprobó con una
	// lectura de fuera, y entre esa lectura y esta escritura cabe una corrección
	// entera o una finalización.
	if domain.ReviewStatus(estado).Terminal() {
		return nil, false, fmt.Errorf("la revisión está en estado terminal %s y no admite cambios", estado)
	}
	if ronda != expectedRound {
		return nil, false, fmt.Errorf(
			"la revisión avanzó a la ronda %d mientras se construía el consenso de la ronda %d",
			ronda, expectedRound,
		)
	}

	existentes, err := consensusDeRonda(connQuerier{ctx, conn}, internalID, reviewID, expectedRound)
	if err != nil {
		return nil, false, err
	}
	if len(existentes) > 0 {
		// Reenviar la ronda exacta es una lectura; reemplazarla por otra distinta
		// permitiría reclasificar un confirmado como informativo justo antes de
		// finalizar, que es una aprobación falsa por otra puerta (FR-005).
		//
		// La huella se recalcula de las filas y no se lee de round_fingerprint: una
		// ronda escrita antes de esa columna la tiene vacía, y compararla con la
		// columna dejaría pasar el reemplazo justo en las revisiones más antiguas.
		if domain.ClassificationFingerprint(existentes) != fingerprint {
			return nil, false, fmt.Errorf(
				"la ronda %d ya tiene un consenso registrado y no admite reemplazo", expectedRound,
			)
		}
		return existentes, true, nil
	}

	persistidos := make([]domain.ConsensusFinding, len(findings))
	copy(persistidos, findings)
	for i := range persistidos {
		args, err := consensusArgs(internalID, &persistidos[i])
		if err != nil {
			return nil, false, err
		}
		if err := conn.QueryRowContext(ctx, upsertConsensusSQL, args...).Scan(&persistidos[i].ID); err != nil {
			return nil, false, fmt.Errorf("persistir el consenso %s: %w", persistidos[i].ConsensusLocalID, err)
		}
		persistidos[i].ReviewID = reviewID
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return nil, false, fmt.Errorf("confirmar el consenso: %w", err)
	}
	comprometida = true
	return persistidos, false, nil
}

func (r *ReviewRepository) GetConsensusFinding(project, reviewID, localID string) (*domain.ConsensusFinding, error) {
	row := r.db.QueryRow(`
		SELECT cf.id, cf.round, cf.consensus_local_id, cf.status, cf.severity, cf.claim,
		       cf.source_finding_ids, cf.rejudgment_state, cf.round_fingerprint, cf.rejudgment_round
		FROM consensus_findings cf JOIN reviews rv ON rv.id = cf.review_id
		WHERE rv.project = ? AND rv.review_id = ? AND cf.consensus_local_id = ?`, project, reviewID, localID)
	finding, err := scanConsensus(row, reviewID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return finding, err
}

func (r *ReviewRepository) ListConsensusFindings(project, reviewID string, round int) ([]domain.ConsensusFinding, error) {
	// Una revisión inexistente devuelve lista vacía y ningún error, que es el
	// contrato que tenía la consulta original con su JOIN. Al extraer la consulta
	// para poder releerla dentro de una transacción se coló un lookup que traduce
	// "no hay filas" en error: un cambio de contrato de un método público del puerto
	// que nadie pidió. Hoy no tiene llamadores en producción, y eso no lo vuelve
	// inofensivo — lo vuelve una trampa para el primero que llegue.
	internalID, encontrada, err := r.lookupReviewIDOpcional(project, reviewID)
	if err != nil || !encontrada {
		return nil, err
	}
	return consensusDeRonda(r.db, internalID, reviewID, round)
}

// consensusDeRonda lee la clasificación de una ronda por el id interno de la revisión.
//
// Toma un querier en vez de usar r.db para que ReplaceConsensusRound pueda releer las
// filas DENTRO de su transacción. Comprobar si la ronda ya existe desde fuera es
// precisamente lo que hacía de esto un check-then-write.
func consensusDeRonda(q querier, internalID int64, reviewID string, round int) ([]domain.ConsensusFinding, error) {
	rows, err := q.Query(`
		SELECT cf.id, cf.round, cf.consensus_local_id, cf.status, cf.severity, cf.claim,
		       cf.source_finding_ids, cf.rejudgment_state, cf.round_fingerprint, cf.rejudgment_round
		FROM consensus_findings cf
		WHERE cf.review_id = ? AND cf.round = ? ORDER BY cf.id`, internalID, round)
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
		       cf.source_finding_ids, cf.rejudgment_state, cf.round_fingerprint, cf.rejudgment_round
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

// reviewColumns fija el orden de lectura de una revisión. Existe como constante
// porque tres consultas distintas deben coincidir columna a columna con reviewRow:
// listarlas a mano en cada una es la forma habitual de que una se quede atrás al
// añadir un campo.
const reviewColumns = `target_type, target_revision, target_digest, target_scope, max_fix_rounds,
	       auto_fix_severities, independence_level, independence_reason, round, status,
	       verdict, created_at, updated_at, current_target_digest, fix_authorized,
	       reviewer_a_provider, reviewer_a_model, reviewer_b_provider, reviewer_b_model`

// reviewRow son los valores crudos de una fila de reviews. Las columnas de la
// funcionalidad 028 se leen como nullable: una base anterior las tiene a NULL y debe
// seguir abriendo con el comportamiento de antes.
type reviewRow struct {
	targetType, revision, digest, scopeJSON string
	maxRounds                               int
	severitiesJSON, independence, reason    string
	round                                   int
	status                                  string
	verdict                                 sql.NullString
	createdAt, updatedAt                    string
	currentDigest                           sql.NullString
	fixAuthorized                           sql.NullInt64
	reviewerAProvider, reviewerAModel       sql.NullString
	reviewerBProvider, reviewerBModel       sql.NullString
}

func (raw *reviewRow) dest() []any {
	return []any{
		&raw.targetType, &raw.revision, &raw.digest, &raw.scopeJSON, &raw.maxRounds,
		&raw.severitiesJSON, &raw.independence, &raw.reason, &raw.round, &raw.status,
		&raw.verdict, &raw.createdAt, &raw.updatedAt, &raw.currentDigest, &raw.fixAuthorized,
		&raw.reviewerAProvider, &raw.reviewerAModel, &raw.reviewerBProvider, &raw.reviewerBModel,
	}
}

func (raw *reviewRow) toDomain(project, reviewID string) (*domain.Review, error) {
	var scope []string
	if err := json.Unmarshal([]byte(raw.scopeJSON), &scope); err != nil {
		return nil, err
	}
	var severities []domain.Severity
	if err := json.Unmarshal([]byte(raw.severitiesJSON), &severities); err != nil {
		return nil, err
	}
	target, err := domain.NewTarget(domain.TargetType(raw.targetType), raw.revision, raw.digest, scope)
	if err != nil {
		return nil, err
	}
	target.CreatedAt = parseReviewTime(raw.createdAt)
	review := &domain.Review{
		ID: reviewID, Project: project, Target: target, MaxFixRounds: raw.maxRounds, AutoFixSeverities: severities,
		IndependenceLevel: domain.IndependenceLevel(raw.independence), IndependenceReason: raw.reason, Round: raw.round,
		Status: domain.ReviewStatus(raw.status), Verdict: domain.Verdict(raw.verdict.String),
		CreatedAt: parseReviewTime(raw.createdAt), UpdatedAt: parseReviewTime(raw.updatedAt),
		CurrentTargetDigest: raw.currentDigest.String,
		// NULL = revisión anterior a 028: autorizaba corregir, porque no existía
		// ninguna que no lo hiciera. Interpretarla como solo lectura la escalaría
		// sin que nadie lo hubiera pedido.
		FixAuthorized: !raw.fixAuthorized.Valid || raw.fixAuthorized.Int64 != 0,
		ReviewerA: domain.ReviewerIdentity{
			Provider: raw.reviewerAProvider.String, Model: raw.reviewerAModel.String,
		},
		ReviewerB: domain.ReviewerIdentity{
			Provider: raw.reviewerBProvider.String, Model: raw.reviewerBModel.String,
		},
	}
	if review.CurrentTargetDigest == "" {
		review.CurrentTargetDigest = target.Digest()
	}
	return review, nil
}

func scanReview(row scanner, project, reviewID string) (*domain.Review, error) {
	var raw reviewRow
	err := row.Scan(raw.dest()...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return raw.toDomain(project, reviewID)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
	var rejudgment, fingerprint sql.NullString
	var rejudgmentRound sql.NullInt64
	if err := row.Scan(&finding.ID, &finding.Round, &finding.ConsensusLocalID, &status, &severity,
		&finding.Claim, &sources, &rejudgment, &fingerprint, &rejudgmentRound); err != nil {
		return nil, err
	}
	finding.ReviewID = reviewID
	finding.Status = domain.ConsensusStatus(status)
	finding.Severity = domain.Severity(severity)
	finding.RejudgmentState = domain.ReJudgmentState(rejudgment.String)
	finding.RoundFingerprint = fingerprint.String
	finding.RejudgmentRound = int(rejudgmentRound.Int64)
	if err := json.Unmarshal([]byte(sources), &finding.SourceFindingIDs); err != nil {
		return nil, err
	}
	return &finding, nil
}

// lookupReviewIDOpcional distingue "no existe" de "falló la consulta". Es lo que
// permite que una consulta de listado devuelva vacío en vez de error sin tener que
// inspeccionar el texto del error de lookupReviewID.
func (r *ReviewRepository) lookupReviewIDOpcional(project, reviewID string) (int64, bool, error) {
	var id int64
	err := r.db.QueryRow(`SELECT id FROM reviews WHERE project = ? AND review_id = ?`, project, reviewID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
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

// UpsertReJudgment persiste el re-juicio de un revisor y recalcula, en la MISMA
// transacción, el estado agregado del hallazgo.
//
// Que el recálculo viva aquí y no en el caso de uso es deliberado:
// consensus_findings.rejudgment_state es un valor derivado de la tabla rejudgments,
// y dos escrituras separadas dejarían una ventana en la que la columna afirma algo
// que sus filas de origen ya no respaldan. Esa ventana es justo la que aprovecharía
// una finalización concurrente para leer RESOLVED sin unanimidad.
func (r *ReviewRepository) UpsertReJudgment(project, reviewID string, judgment *domain.ReJudgment) error {
	if err := judgment.Validate(); err != nil {
		return err
	}
	// Conexión dedicada con BEGIN IMMEDIATE, igual que RecordFixAtomically y por el
	// mismo motivo: db.Begin() emite un BEGIN diferido que toma el bloqueo de
	// escritura en el primer INSERT, y en WAL el perdedor recibe SQLITE_BUSY, que el
	// busy_timeout no reintenta para una transacción que ya leyó.
	//
	// No es teórico: dos revisores re-juzgando en paralelo —el flujo normal de este
	// protocolo, no un caso raro— perdían la mayoría de las escrituras con
	// "database is locked". Reproducido con 16 re-juicios simultáneos: fallaban 11.
	ctx := context.Background()
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("tomar el bloqueo de escritura: %w", err)
	}
	comprometida := false
	defer func() {
		if !comprometida {
			conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()

	var internalID int64
	var estado string
	err = conn.QueryRowContext(ctx,
		`SELECT id, status FROM reviews WHERE project = ? AND review_id = ?`,
		project, reviewID).Scan(&internalID, &estado)
	if err == sql.ErrNoRows {
		return fmt.Errorf("review %s not found", reviewID)
	}
	if err != nil {
		return err
	}
	// El estado terminal se revalida DENTRO de la transacción. El caso de uso ya lo
	// comprobó, pero con una lectura de fuera: un revisor que se retracta lee la
	// revisión todavía abierta, pasa la comprobación, y escribe cuando la
	// finalización ya la cerró. El resultado era un ledger con APPROVED y el
	// hallazgo severo marcado como reaparecido — la aprobación falsa que el
	// protocolo existe para impedir.
	if domain.ReviewStatus(estado).Terminal() {
		return fmt.Errorf("la revisión está en estado terminal %s y no admite cambios", estado)
	}
	var findingID int64
	err = conn.QueryRowContext(ctx,
		`SELECT id FROM consensus_findings WHERE review_id = ? AND consensus_local_id = ?`,
		internalID, judgment.ConsensusLocalID).Scan(&findingID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("el hallazgo de consenso %s no existe en esta revisión", judgment.ConsensusLocalID)
	}
	if err != nil {
		return err
	}

	evidence, err := json.Marshal(redactarLista(judgment.Evidence))
	if err != nil {
		return err
	}
	err = conn.QueryRowContext(ctx, `
		INSERT INTO rejudgments (review_id, round, consensus_finding_id, reviewer, state, evidence)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(review_id, round, consensus_finding_id, reviewer) DO UPDATE SET
			state = excluded.state, evidence = excluded.evidence
		RETURNING id`,
		internalID, judgment.Round, findingID, string(judgment.Reviewer),
		string(judgment.State), string(evidence)).Scan(&judgment.ID)
	if err != nil {
		return fmt.Errorf("upsert rejudgment: %w", err)
	}

	judgments, err := reJudgmentsForFinding(connQuerier{ctx, conn}, internalID, findingID, reviewID, judgment.ConsensusLocalID)
	if err != nil {
		return err
	}
	// Solo la ronda que se acaba de escribir: agregar todas dejaría que un
	// RESOLVED de una corrección anterior complete la unanimidad de esta.
	agregado := domain.AggregateReJudgmentForRound(judgments, judgment.Round)
	// La ronda viaja con el estado, en la misma sentencia. Guardar el veredicto sin
	// su ronda lo convierte en un valor sin fecha, y RecordFix abría la corrección
	// siguiente dejándolo intacto: el veredicto leía como vigente un RESOLVED que
	// pertenecía a la ronda anterior y aprobaba sin haber re-verificado nada.
	if _, err := conn.ExecContext(ctx,
		`UPDATE consensus_findings SET rejudgment_state = ?, rejudgment_round = ? WHERE id = ?`,
		string(agregado), judgment.Round, findingID); err != nil {
		return fmt.Errorf("actualizar el estado agregado: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("confirmar el re-juicio: %w", err)
	}
	comprometida = true
	judgment.ReviewID = reviewID
	return nil
}

// connQuerier adapta una *sql.Conn a la interfaz querier, para poder releer los
// re-juicios dentro de la transacción abierta sobre esa conexión.
type connQuerier struct {
	ctx  context.Context
	conn *sql.Conn
}

func (q connQuerier) Query(query string, args ...any) (*sql.Rows, error) {
	return q.conn.QueryContext(q.ctx, query, args...)
}

func (r *ReviewRepository) ListReJudgments(project, reviewID, consensusLocalID string) ([]domain.ReJudgment, error) {
	internalID, err := r.lookupReviewID(project, reviewID)
	if err != nil {
		return nil, err
	}
	var findingID int64
	err = r.db.QueryRow(`SELECT id FROM consensus_findings WHERE review_id = ? AND consensus_local_id = ?`,
		internalID, consensusLocalID).Scan(&findingID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return reJudgmentsForFinding(r.db, internalID, findingID, reviewID, consensusLocalID)
}

// querier abstrae *sql.DB y *sql.Tx para poder leer los re-juicios dentro y fuera de
// una transacción sin duplicar la consulta.
type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func reJudgmentsForFinding(
	q querier, internalReviewID, findingID int64, reviewID, consensusLocalID string,
) ([]domain.ReJudgment, error) {
	rows, err := q.Query(`
		SELECT id, round, reviewer, state, evidence
		FROM rejudgments WHERE review_id = ? AND consensus_finding_id = ?
		ORDER BY round, reviewer`, internalReviewID, findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ReJudgment
	for rows.Next() {
		judgment := domain.ReJudgment{ReviewID: reviewID, ConsensusLocalID: consensusLocalID}
		var reviewer, state, evidence string
		if err := rows.Scan(&judgment.ID, &judgment.Round, &reviewer, &state, &evidence); err != nil {
			return nil, err
		}
		judgment.Reviewer = domain.Reviewer(reviewer)
		judgment.State = domain.ReJudgmentState(state)
		if err := json.Unmarshal([]byte(evidence), &judgment.Evidence); err != nil {
			return nil, err
		}
		out = append(out, judgment)
	}
	return out, rows.Err()
}

// RecordFixAtomically escribe la ronda de corrección completa en una transacción.
//
// Se abre con BEGIN IMMEDIATE, no con el BEGIN diferido por defecto: el bloqueo de
// escritura se toma al abrir y no en el primer INSERT, que es lo que cierra la
// ventana entre contar las rondas existentes y escribir la nueva. El UNIQUE
// (review_id, round) es la red de seguridad final, y el recuento contra
// ExpectedRounds convierte la carrera perdida en un error explícito en vez de un
// sobrescrito silencioso.
func (r *ReviewRepository) RecordFixAtomically(
	project, reviewID string, transition ports.FixTransition,
) error {
	ctx := context.Background()
	// Una conexión dedicada, no una transacción de database/sql: db.Begin() emite
	// un BEGIN diferido, que en WAL toma el bloqueo de escritura al primer INSERT y
	// no al abrir. Entre el COUNT y el INSERT cabe otra transacción, y el perdedor
	// recibe SQLITE_BUSY_SNAPSHOT, que el busy_timeout NO reintenta. BEGIN IMMEDIATE
	// toma el bloqueo desde el principio y el busy_timeout sí lo cubre.
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("tomar el bloqueo de escritura: %w", err)
	}
	comprometida := false
	defer func() {
		if !comprometida {
			conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()

	var internalID int64
	var estado string
	var vigente, original sql.NullString
	err = conn.QueryRowContext(ctx,
		`SELECT id, status, current_target_digest, target_digest FROM reviews
		 WHERE project = ? AND review_id = ?`,
		project, reviewID).Scan(&internalID, &estado, &vigente, &original)
	if err == sql.ErrNoRows {
		return fmt.Errorf("review %s not found", reviewID)
	}
	if err != nil {
		return err
	}

	// El estado se revalida DENTRO de la transacción, y no basta con el digest: una
	// finalización NO cambia el target, así que una corrección que llegaba tarde
	// pasaba la comprobación de abajo y reabría una revisión ya terminal.
	if transition.ExpectedStatus != "" && domain.ReviewStatus(estado) != transition.ExpectedStatus {
		return fmt.Errorf(
			"la revisión pasó a %s mientras se registraba la corrección de la ronda %d",
			estado, transition.NextRound,
		)
	}

	// El target vigente se revalida DENTRO de la transacción. El caso de uso ya lo
	// comprobó, pero con una lectura de fuera: dos correcciones simultáneas leían
	// el mismo target, derivaban rondas distintas y se registraban las dos.
	actual := vigente.String
	if actual == "" {
		actual = original.String
	}
	if transition.ExpectedBaseDigest != "" && actual != transition.ExpectedBaseDigest {
		return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
	}

	var rondas int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM fix_rounds WHERE review_id = ?`, internalID).Scan(&rondas); err != nil {
		return err
	}
	if rondas != transition.ExpectedRounds {
		return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
	}

	delta := transition.Delta
	addressed, _ := json.Marshal(delta.AddressedConsensusIDs)
	paths, _ := json.Marshal(delta.ModifiedPaths)
	verification, _ := json.Marshal(redactarLista(delta.Verification))
	err = conn.QueryRowContext(ctx, `
		INSERT INTO fix_rounds
			(review_id, round, base_target_digest, fixed_target_digest, addressed_consensus_ids,
			 modified_paths, verification, diff_digest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`, internalID, delta.Round, delta.BaseTargetDigest, delta.FixedTargetDigest,
		string(addressed), string(paths), string(verification), delta.DiffDigest).Scan(&delta.ID)
	if err != nil {
		return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
	}

	result, err := conn.ExecContext(ctx, `
		UPDATE reviews SET round = ?, status = ?, current_target_digest = ?, updated_at = `+Now+`
		WHERE id = ?`, transition.NextRound, string(transition.NextStatus),
		transition.CurrentTargetDigest, internalID)
	if err != nil {
		return fmt.Errorf("avanzar la revisión: %w", err)
	}
	if err := requireAffected(result, "review"); err != nil {
		return err
	}

	// Abrir una ronda invalida el veredicto de re-juicio de la anterior, en la misma
	// transacción que la abre. La columna es derivada y no debe sobrevivir al target
	// del que se dedujo: dejarla puesta hacía que review_status siguiera mostrando
	// RESOLVED sobre un código que ya nadie había vuelto a mirar.
	if _, err := conn.ExecContext(ctx, `
		UPDATE consensus_findings SET rejudgment_state = NULL, rejudgment_round = NULL
		WHERE review_id = ?`, internalID); err != nil {
		return fmt.Errorf("invalidar los re-juicios de la ronda anterior: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("confirmar la corrección: %w", err)
	}
	comprometida = true
	delta.ReviewID = reviewID
	return nil
}

// CountPromotedMemories cuenta lo que esta revisión aportó a la memoria del proyecto.
//
//   - promovidas: memorias cuyo source_review_id es esta revisión.
//   - deduplicadas: de esas, las que se escribieron más de una vez (updated_at por
//     encima de created_at), es decir, promociones posteriores que reforzaron la
//     misma memoria en vez de crear otra.
//
// Salvedad honesta sobre el contrato: `review_finalize` publica estas dos métricas,
// pero promover exige veredicto APPROVED (FR-021), que solo existe DESPUÉS de
// finalizar. En la primera finalización de una revisión aprobada valdrán cero por
// construcción, y reflejan promociones ya hechas —el caso de una revisión que se
// consulta más tarde—. Se derivan del ledger y no de un contador acumulado, así que
// nunca afirman más de lo que hay escrito.
func (r *ReviewRepository) CountPromotedMemories(project, reviewID string) (int, int, error) {
	var promovidas, deduplicadas int
	err := r.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN updated_at > created_at THEN 1 ELSE 0 END), 0)
		FROM memories WHERE project = ? AND source_review_id = ?`,
		project, reviewID).Scan(&promovidas, &deduplicadas)
	if err != nil {
		return 0, 0, fmt.Errorf("contar memorias promovidas: %w", err)
	}
	return promovidas, deduplicadas, nil
}
