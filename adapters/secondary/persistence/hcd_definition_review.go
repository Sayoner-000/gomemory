package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"time"
)

var _ ports.HCDDefinitionReviewRepository = (*HCDStore)(nil)

const reviewEventColumns = "operation_id, unit_id, revision, kind, action, client, created_at, payload"

func scanDefinitionReview(row interface{ Scan(...any) error }) (domain.HCDDefinitionReviewEvent, error) {
	var e domain.HCDDefinitionReviewEvent
	var timestamp, payload string
	if err := row.Scan(&e.OperationID, &e.UnitID, &e.Revision, &e.Kind, &e.Action, &e.Client, &timestamp, &payload); err != nil {
		return e, err
	}
	var err error
	if e.CreatedAt, err = time.Parse(hcdTime, timestamp); err != nil {
		return e, fmt.Errorf("invalid review timestamp: %w", err)
	}
	if err = json.Unmarshal([]byte(payload), &e.Payload); err != nil {
		return e, fmt.Errorf("invalid review payload: %w", err)
	}
	return e, e.Validate()
}

func (s *HCDStore) DefinitionReviews(ctx context.Context, unitID string) ([]domain.HCDDefinitionReviewEvent, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+reviewEventColumns+" FROM hcd_definition_reviews WHERE project = ? AND unit_id = ? ORDER BY sequence", s.project, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.HCDDefinitionReviewEvent
	for rows.Next() {
		e, err := scanDefinitionReview(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func lastReviewOperation(ctx context.Context, tx *sql.Tx, project, unitID, revision string) (domain.HCDReviewOperationID, error) {
	var id domain.HCDReviewOperationID
	err := tx.QueryRowContext(ctx, "SELECT operation_id FROM hcd_definition_reviews WHERE project = ? AND unit_id = ? AND revision = ? ORDER BY sequence DESC LIMIT 1", project, unitID, revision).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func requireActiveReviewUnit(ctx context.Context, tx *sql.Tx, project, unitID string) error {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM hcd_units WHERE project = ? AND id = ?", project, unitID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if state != string(domain.UnitActive) {
		return fmt.Errorf("%w: unit is not active", domain.ErrNotAllowed)
	}
	return nil
}

func reviewConflict() error {
	return fmt.Errorf("%w: definition review changed; read the unit again", domain.ErrValidation)
}

func (s *HCDStore) AppendReview(ctx context.Context, event domain.HCDDefinitionReviewEvent, expectedLast domain.HCDReviewOperationID) (domain.HCDDefinitionReviewEvent, error) {
	event = event.Protected()
	if err := event.Validate(); err != nil {
		return event, err
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return event, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return event, err
	}
	defer tx.Rollback()
	previous, err := scanDefinitionReview(tx.QueryRowContext(ctx, "SELECT "+reviewEventColumns+" FROM hcd_definition_reviews WHERE project = ? AND operation_id = ?", s.project, event.OperationID))
	if err == nil {
		oldPayload, err := json.Marshal(previous.Payload)
		if err != nil {
			return event, err
		}
		if previous.UnitID != event.UnitID || previous.Revision != event.Revision || previous.Kind != event.Kind || previous.Action != event.Action || previous.Client != event.Client || string(oldPayload) != string(payload) {
			return event, reviewConflict()
		}
		return previous, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return event, err
	}
	if err := requireActiveReviewUnit(ctx, tx, s.project, event.UnitID); err != nil {
		return event, err
	}
	last, err := lastReviewOperation(ctx, tx, s.project, event.UnitID, string(event.Revision))
	if err != nil {
		return event, err
	}
	if last != expectedLast {
		return event, reviewConflict()
	}
	var approvedRevision string
	err = tx.QueryRowContext(ctx, "SELECT revision FROM hcd_approvals WHERE project = ? AND unit_id = ? AND action = ? ORDER BY created_at DESC, id DESC LIMIT 1", s.project, event.UnitID, domain.ApprovalApprove).Scan(&approvedRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return event, err
	}
	if approvedRevision == string(event.Revision) {
		return event, fmt.Errorf("%w: definition is already approved", domain.ErrNotAllowed)
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO hcd_definition_reviews(project, operation_id, unit_id, revision, kind, action, client, created_at, payload) VALUES(?,?,?,?,?,?,?,?,?)", s.project, event.OperationID, event.UnitID, event.Revision, event.Kind, event.Action, event.Client, event.CreatedAt.UTC().Format(hcdTime), string(payload))
	if err != nil {
		return event, err
	}
	return event, tx.Commit()
}

func (s *HCDStore) AddReviewedApproval(ctx context.Context, a domain.Approval, expectedLast domain.HCDReviewOperationID) error {
	if a.Action != domain.ApprovalApprove || a.ID == "" || a.CreatedAt.IsZero() || !domain.ValidQuote(a.Quote, a.Revision) {
		return domain.ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireActiveReviewUnit(ctx, tx, s.project, a.UnitID); err != nil {
		return err
	}
	last, err := lastReviewOperation(ctx, tx, s.project, a.UnitID, a.Revision)
	if err != nil {
		return err
	}
	if expectedLast == "" || last != expectedLast {
		return reviewConflict()
	}
	var revision, constitution, playbook string
	err = tx.QueryRowContext(ctx, "SELECT revision, constitution_rev, playbook_rev FROM hcd_approvals WHERE project = ? AND unit_id = ? AND action = ? ORDER BY created_at DESC, id DESC LIMIT 1", s.project, a.UnitID, domain.ApprovalApprove).Scan(&revision, &constitution, &playbook)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if revision == a.Revision && constitution == a.ConstitutionRev && playbook == a.PlaybookRev {
		return fmt.Errorf("%w: definition is already approved", domain.ErrNotAllowed)
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO hcd_approvals(id, project, unit_id, action, revision, fingerprint, origin, client, quote, constitution_rev, playbook_rev, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", a.ID, s.project, a.UnitID, a.Action, a.Revision, a.Fingerprint, a.Origin, a.Client, domain.ProtectText(a.Quote), a.ConstitutionRev, a.PlaybookRev, a.CreatedAt.UTC().Format(hcdTime))
	if err != nil {
		return err
	}
	return tx.Commit()
}
