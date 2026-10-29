package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mem/domain"
)

func (s *HCDStore) SaveExecution(ctx context.Context, x domain.Execution) error {
	_, e := s.writeContext(ctx, `INSERT INTO hcd_executions(project,id,unit_id,task_id,definition_rev,delivery_rev,state,granted_at,reported_at,result_hash) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(project,id) DO UPDATE SET state=excluded.state,reported_at=excluded.reported_at,result_hash=excluded.result_hash`, s.project, x.ID, x.UnitID, x.TaskID, x.DefinitionRev, x.DeliveryRev, x.State, x.GrantedAt, x.ReportedAt, x.ResultHash)
	return e
}
func (s *HCDStore) GetExecution(ctx context.Context, unit, id string) (domain.Execution, error) {
	var x domain.Execution
	e := s.q.QueryRowContext(ctx, `SELECT id,unit_id,task_id,definition_rev,delivery_rev,state,granted_at,reported_at,result_hash FROM hcd_executions WHERE project=? AND unit_id=? AND id=?`, s.project, unit, id).Scan(&x.ID, &x.UnitID, &x.TaskID, &x.DefinitionRev, &x.DeliveryRev, &x.State, &x.GrantedAt, &x.ReportedAt, &x.ResultHash)
	if errors.Is(e, sql.ErrNoRows) {
		e = domain.ErrNotFound
	}
	if e == nil {
		x.ExpiresAt, e = s.executionDeadline(ctx, x.UnitID, x.ID)
	}
	return x, e
}
func (s *HCDStore) ListExecutions(ctx context.Context, unit string) ([]domain.Execution, error) {
	rows, e := s.q.QueryContext(ctx, `SELECT id,unit_id,task_id,definition_rev,delivery_rev,state,granted_at,reported_at,result_hash FROM hcd_executions WHERE project=? AND unit_id=? ORDER BY id`, s.project, unit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Execution
	for rows.Next() {
		var x domain.Execution
		if e = rows.Scan(&x.ID, &x.UnitID, &x.TaskID, &x.DefinitionRev, &x.DeliveryRev, &x.State, &x.GrantedAt, &x.ReportedAt, &x.ResultHash); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for i := range out {
		out[i].ExpiresAt, e = s.executionDeadline(ctx, out[i].UnitID, out[i].ID)
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (s *HCDStore) SaveEvidence(ctx context.Context, v domain.Evidence) error {
	covers, e := json.Marshal(v.Covers)
	if e != nil {
		return e
	}
	subject, e := json.Marshal(v.SubjectRef)
	if e != nil {
		return e
	}
	inspect, e := json.Marshal(v.InspectRef)
	if e != nil {
		return e
	}
	_, e = s.writeContext(ctx, `INSERT INTO hcd_evidence(project,id,unit_id,execution_id,covers_json,procedure,environment,result,subject_ref,inspect_ref,definition_rev,delivery_rev,playbook_rev,constitution_rev,executor,recorded_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.project, v.ID, v.UnitID, v.ExecutionID, string(covers), domain.ProtectText(v.Procedure), domain.ProtectText(v.Environment), v.Result, domain.ProtectText(string(subject)), domain.ProtectText(string(inspect)), v.DefinitionRev, v.DeliveryRev, v.PlaybookRev, v.ConstitutionRev, v.Executor, v.RecordedAt)
	return e
}
func (s *HCDStore) ListEvidence(ctx context.Context, unit string) ([]domain.Evidence, error) {
	rows, e := s.q.QueryContext(ctx, `SELECT id,unit_id,execution_id,covers_json,procedure,environment,result,subject_ref,inspect_ref,definition_rev,delivery_rev,playbook_rev,constitution_rev,executor,recorded_at FROM hcd_evidence WHERE project=? AND unit_id=? ORDER BY id`, s.project, unit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Evidence
	for rows.Next() {
		var v domain.Evidence
		var c, s, i string
		if e = rows.Scan(&v.ID, &v.UnitID, &v.ExecutionID, &c, &v.Procedure, &v.Environment, &v.Result, &s, &i, &v.DefinitionRev, &v.DeliveryRev, &v.PlaybookRev, &v.ConstitutionRev, &v.Executor, &v.RecordedAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(c), &v.Covers); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(s), &v.SubjectRef); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(i), &v.InspectRef); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// The immutable run operation stores the grant's complete result, including its
// deadline. Keep that snapshot rather than recomputing with today's settings.
func (s *HCDStore) executionDeadline(ctx context.Context, unit, id string) (string, error) {
	var raw string
	e := s.q.QueryRowContext(ctx, `SELECT result_json FROM hcd_operations WHERE project=? AND unit_id=? AND command='run' AND json_extract(result_json,'$.Execution.ID')=? LIMIT 1`, s.project, unit, id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	var result struct{ Execution domain.Execution }
	e = json.Unmarshal([]byte(raw), &result)
	return result.Execution.ExpiresAt, e
}
