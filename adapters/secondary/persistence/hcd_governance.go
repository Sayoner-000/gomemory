package persistence

import (
	"context"
	"fmt"
	"mem/domain"
)

func (s *HCDStore) NextGovernanceID(ctx context.Context, kind string) (string, error) {
	var query, prefix string
	switch kind {
	case "exception":
		query = "SELECT COALESCE(MAX(CAST(substr(id,3) AS INTEGER)),0)+1 FROM hcd_exceptions WHERE project=?"
		prefix = "E-"
	case "adoption":
		query = "SELECT COALESCE(MAX(CAST(substr(id,4) AS INTEGER)),0)+1 FROM hcd_playbook_adoptions WHERE project=?"
		prefix = "AD-"
	default:
		return "", domain.ErrValidation
	}
	var n int
	if err := s.q.QueryRowContext(ctx, query, s.project).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d", prefix, n), nil
}
func (s *HCDStore) SaveException(ctx context.Context, x domain.Exception) error {
	_, err := s.writeContext(ctx, `INSERT INTO hcd_exceptions(project,id,unit_id,rule_id,scope,reason,alternatives,risks,compensating,owner,approval_id,status,expires_at,review_condition) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.project, x.ID, x.UnitID, x.RuleID, x.Scope, x.Reason, x.Alternatives, x.Risks, x.Compensating, x.Owner, x.ApprovalID, x.Status, x.ExpiresAt, x.ReviewCondition)
	return err
}
func (s *HCDStore) ListExceptions(ctx context.Context, id string) ([]domain.Exception, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT id,unit_id,rule_id,scope,reason,alternatives,risks,compensating,owner,approval_id,status,expires_at,review_condition FROM hcd_exceptions WHERE project=? AND unit_id=? ORDER BY id`, s.project, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Exception
	for rows.Next() {
		var x domain.Exception
		if err = rows.Scan(&x.ID, &x.UnitID, &x.RuleID, &x.Scope, &x.Reason, &x.Alternatives, &x.Risks, &x.Compensating, &x.Owner, &x.ApprovalID, &x.Status, &x.ExpiresAt, &x.ReviewCondition); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *HCDStore) GetNormativeRevision(ctx context.Context, doc, rev string) (domain.NormativeRevision, error) {
	x := domain.NormativeRevision{Doc: doc, Revision: rev}
	err := s.q.QueryRowContext(ctx, `SELECT content,source,change_kind,urgent FROM hcd_normative_revisions WHERE project=? AND doc=? AND revision=?`, s.project, doc, rev).Scan(&x.Content, &x.Source, &x.ChangeKind, &x.Urgent)
	return x, err
}
func (s *HCDStore) ListUrgentRevisions(ctx context.Context) ([]domain.NormativeRevision, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT doc,revision,content,source,change_kind,urgent FROM hcd_normative_revisions WHERE project=? AND urgent=1 ORDER BY doc,revision`, s.project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.NormativeRevision
	for rows.Next() {
		var x domain.NormativeRevision
		if err = rows.Scan(&x.Doc, &x.Revision, &x.Content, &x.Source, &x.ChangeKind, &x.Urgent); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *HCDStore) SaveAdoption(ctx context.Context, x domain.PlaybookAdoption) error {
	_, err := s.writeContext(ctx, `INSERT INTO hcd_playbook_adoptions(project,id,unit_id,doc,from_rev,to_rev,impact_json,reason,actor,approval_id,adopted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, s.project, x.ID, x.UnitID, x.Doc, x.FromRev, x.ToRev, x.ImpactJSON, domain.ProtectText(x.Reason), domain.ProtectText(x.Actor), x.ApprovalID, x.AdoptedAt)
	return err
}
