package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"mem/application/ports"
	"mem/domain"
	"os"
	"strings"
	"time"
)

func (h *HCD) ConfigureDeliveryCodec(parse func([]byte) (domain.Delivery, error), render func(string, domain.Delivery) []byte) {
	h.parseDelivery = parse
	h.renderDelivery = render
}
func (h *HCD) ReadDelivery(ctx context.Context, id string) (domain.Delivery, error) {
	u, e := h.repo.GetUnit(ctx, id)
	if e != nil {
		return domain.Delivery{}, e
	}
	return h.readDelivery(ctx, u)
}
func (h *HCD) readDelivery(ctx context.Context, u domain.Unit) (domain.Delivery, error) {
	if h.parseDelivery == nil {
		return domain.Delivery{}, domain.ErrNotAllowed
	}
	data, e := h.artifacts.Read(ctx, strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "delivery.md")
	if errors.Is(e, os.ErrNotExist) {
		return domain.Delivery{}, nil
	}
	if e != nil {
		return domain.Delivery{}, e
	}
	return h.parseDelivery(data)
}
func (h *HCD) Slice(ctx context.Context, id string, expected int64, op string, slices []domain.Slice) (domain.Unit, error) {
	// Un archivo sin encabezados «## VS-NN.» no debe registrarse como entrega vacía.
	if len(slices) == 0 {
		return domain.Unit{}, &domain.HCDValidationError{Checks: []domain.PendingCheck{{Code: "SLICE-EMPTY", Field: "delivery", Message: "el archivo no contiene slices con encabezado «## VS-NN. Título»"}}}
	}
	return h.updateDelivery(ctx, id, expected, op, "slice", func(d *domain.Delivery) error {
		for _, s := range slices {
			if s.ID == "" {
				return domain.ErrValidation
			}
			if v := domain.SliceViolations(s); len(v) > 0 {
				return &domain.HCDValidationError{Checks: v}
			}
			found := false
			for i := range d.Slices {
				if d.Slices[i].ID == s.ID {
					d.Slices[i] = s
					found = true
					break
				}
			}
			if !found {
				d.Slices = append(d.Slices, s)
			}
		}
		return nil
	}, slices)
}
func (h *HCD) Tasks(ctx context.Context, id string, expected int64, op, sliceID string, tasks []domain.Task) (domain.Unit, error) {
	// Igual que Slice: sin encabezados «### T-NNN.» no hay nada que registrar.
	if len(tasks) == 0 {
		return domain.Unit{}, &domain.HCDValidationError{Checks: []domain.PendingCheck{{Code: "TASK-EMPTY", Field: sliceID, Message: "el archivo no contiene tareas con encabezado «### T-NNN. Título»"}}}
	}
	return h.updateDelivery(ctx, id, expected, op, "tasks", func(d *domain.Delivery) error {
		exists := false
		for _, s := range d.Slices {
			exists = exists || s.ID == sliceID
		}
		if !exists {
			return domain.ErrNotFound
		}
		for _, task := range tasks {
			if task.SliceID != sliceID || task.ID == "" {
				return domain.ErrValidation
			}
			found := false
			for i := range d.Tasks {
				if d.Tasks[i].ID == task.ID {
					d.Tasks[i] = task
					found = true
					break
				}
			}
			if !found {
				d.Tasks = append(d.Tasks, task)
			}
		}
		if domain.DependencyCycle(d.Tasks) {
			return domain.ErrAtomicity
		}
		return nil
	}, tasks)
}
func (h *HCD) updateDelivery(ctx context.Context, id string, expected int64, op, command string, update func(*domain.Delivery) error, payload any) (domain.Unit, error) {
	u, e := h.repo.GetUnit(ctx, id)
	if e != nil {
		return u, e
	}
	dir := strings.TrimPrefix(u.ID, "HCD-") + "-" + u.Slug
	previous, readErr := h.artifacts.Read(ctx, dir, "delivery.md")
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return u, readErr
	}
	wrote := false
	bytes, _ := json.Marshal(payload)
	result, e := h.mutate(ctx, id, expected, op, hcdPayloadHash(bytes), command, func(repo ports.HCDRepository, current domain.Unit) (domain.Unit, any, error) {
		if current.Approach != "hcd" {
			return current, nil, domain.ErrApproachMismatch
		}
		if current.State != domain.StateReady && current.State != domain.StateInProgress {
			return current, nil, domain.ErrNotAllowed
		}
		if h.renderDelivery == nil {
			return current, nil, domain.ErrNotAllowed
		}
		if current.DeliveryRev != "" {
			actual, e := h.artifacts.Hash(ctx, dir, "delivery.md")
			if e != nil {
				return current, nil, e
			}
			if actual != current.DeliveryRev {
				return current, nil, domain.ErrRevisionConflict
			}
		}
		d, e := h.readDelivery(ctx, current)
		if e != nil {
			return current, nil, e
		}
		if e = update(&d); e != nil {
			return current, nil, e
		}
		content := []byte(domain.ProtectText(string(h.renderDelivery(id, d))))
		if e = h.artifacts.Write(ctx, dir, "delivery.md", content); e != nil {
			return current, nil, e
		}
		wrote = true
		current.DeliveryRev = domain.ContentRevision(content)
		for _, s := range d.Slices {
			for _, c := range s.Criteria {
				if e = repo.AddLink(ctx, domain.Link{UnitID: id, FromKind: "slice", FromID: s.ID, ToKind: "criterion", ToID: c, Relation: "covers"}); e != nil {
					return current, nil, e
				}
			}
		}
		for _, t := range d.Tasks {
			if e = repo.AddLink(ctx, domain.Link{UnitID: id, FromKind: "task", FromID: t.ID, ToKind: "slice", ToID: t.SliceID, Relation: "belongs_to"}); e != nil {
				return current, nil, e
			}
			for _, c := range t.Criteria {
				if e = repo.AddLink(ctx, domain.Link{UnitID: id, FromKind: "task", FromID: t.ID, ToKind: "criterion", ToID: c, Relation: "covers"}); e != nil {
					return current, nil, e
				}
			}
			for _, dep := range t.Dependencies {
				if e = repo.AddLink(ctx, domain.Link{UnitID: id, FromKind: "task", FromID: t.ID, ToKind: "task", ToID: dep, Relation: "depends_on"}); e != nil {
					return current, nil, e
				}
			}
		}
		return current, current, nil
	})
	if e != nil {
		if wrote {
			var rollback error
			cctx, cancel := compensationContext(ctx)
			if readErr == nil {
				rollback = h.artifacts.Write(cctx, dir, "delivery.md", previous)
			} else {
				rollback = h.artifacts.Delete(cctx, dir, "delivery.md")
			}
			cancel()
			if rollback != nil {
				return domain.Unit{}, errors.Join(e, rollback)
			}
		}
		return domain.Unit{}, e
	}
	e = json.Unmarshal([]byte(result), &u)
	return u, e
}

type HCDAdmission struct {
	Task                       domain.Task
	Checks                     []domain.PendingCheck
	DefinitionRev, DeliveryRev string
}

func (h *HCD) work(ctx context.Context, repo ports.HCDRepository, u domain.Unit) (domain.HCDWork, error) {
	d, e := h.readDelivery(ctx, u)
	if e != nil {
		return domain.HCDWork{}, e
	}
	data, e := h.artifacts.Read(ctx, strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "definition.md")
	if e != nil {
		return domain.HCDWork{}, e
	}
	def, e := h.ParseDefinition(data)
	if e != nil {
		return domain.HCDWork{}, e
	}
	decisions, e := repo.ListDecisions(ctx, u.ID)
	return domain.HCDWork{Unit: u, Definition: def, Slices: d.Slices, Tasks: domain.ActiveTasks(d.Tasks, decisions), Decisions: decisions}, e
}
func (h *HCD) check(ctx context.Context, repo ports.HCDRepository, u domain.Unit, taskID string) (HCDAdmission, error) {
	if u.DeliveryRev != "" {
		actual, e := h.artifacts.Hash(ctx, strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "delivery.md")
		if e != nil {
			return HCDAdmission{}, e
		}
		if actual != u.DeliveryRev {
			return HCDAdmission{}, domain.ErrRevisionConflict
		}
	}
	w, e := h.work(ctx, repo, u)
	if e != nil {
		return HCDAdmission{}, e
	}
	var task domain.Task
	for _, t := range w.Tasks {
		if t.ID == taskID {
			task = t
			break
		}
	}
	if task.ID == "" {
		return HCDAdmission{}, domain.ErrNotFound
	}
	clarifications, e := repo.ListOpenClarifications(ctx, u.ID)
	if e != nil {
		return HCDAdmission{}, e
	}
	checks := domain.AtomicityViolations(task, w, clarifications)
	blocks, e := repo.ListBlocks(ctx, u.ID)
	if e != nil {
		return HCDAdmission{}, e
	}
	for _, b := range blocks {
		if b.Status == "open" && (b.TargetRef == task.ID || b.TargetRef == task.SliceID || b.Critical) {
			checks = append(checks, domain.PendingCheck{Code: "BLOCK-OPEN", Field: b.TargetRef, Message: b.Reason})
		}
	}
	return HCDAdmission{Task: task, Checks: checks, DefinitionRev: u.DefinitionRev, DeliveryRev: u.DeliveryRev}, nil
}
func (h *HCD) Check(ctx context.Context, id, taskID string) (HCDAdmission, error) {
	u, e := h.repo.GetUnit(ctx, id)
	if e != nil {
		return HCDAdmission{}, e
	}
	return h.check(ctx, h.repo, u, taskID)
}

type HCDRunResult struct {
	Execution domain.Execution
	Admission HCDAdmission
}

func (h *HCD) Run(ctx context.Context, id string, expected int64, op, taskID, client string, timeoutHours ...int) (HCDRunResult, error) {
	if client != "claude" && client != "codex" && client != "opencode" {
		return HCDRunResult{}, domain.ErrNoExecutor
	}
	hours := 24
	if h.hcdSettings.GrantTimeoutHours != nil {
		hours = *h.hcdSettings.GrantTimeoutHours
	}
	if len(timeoutHours) > 0 {
		hours = timeoutHours[0]
	}
	if hours < 1 {
		return HCDRunResult{}, domain.ErrValidation
	}
	if e := h.ExpireExecutions(ctx, id); e != nil {
		return HCDRunResult{}, e
	}
	payload, _ := json.Marshal([]any{taskID, client, hours})
	encoded, e := h.mutate(ctx, id, expected, op, hcdPayloadHash(payload), "run", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if u.State != domain.StateReady && u.State != domain.StateInProgress {
			return u, nil, domain.ErrNotAllowed
		}
		if u.ApprovedRev == "" || u.ApprovedRev != u.DefinitionRev {
			return u, nil, domain.ErrNotAllowed
		}
		a, e := h.check(ctx, repo, u, taskID)
		if e != nil {
			return u, nil, e
		}
		if len(a.Checks) > 0 {
			return u, nil, domain.ErrAtomicity
		}
		executions, e := repo.ListExecutions(ctx, id)
		if e != nil {
			return u, nil, e
		}
		for _, x := range executions {
			if x.TaskID == taskID {
				if x.State == "uncertain" {
					return u, nil, domain.ErrUncertain
				}
				if x.State == "reported" {
					// Revalidar (FR-032): solo se concede otra ejecución cuando
					// toda la evidencia de la anterior caducó.
					fresh, e := h.reportStillFresh(ctx, repo, u, x.ID)
					if e != nil {
						return u, nil, e
					}
					if fresh {
						return u, nil, domain.ErrNotAllowed
					}
					continue
				}
				if x.State == "granted" || (x.State == "cancelled" && x.DefinitionRev == u.DefinitionRev && x.DeliveryRev == u.DeliveryRev) {
					return u, nil, domain.ErrNotAllowed
				}
			}
		}
		x := domain.Execution{ID: "X-" + h.newID(), UnitID: id, TaskID: taskID, DefinitionRev: u.DefinitionRev, DeliveryRev: u.DeliveryRev, State: "granted", GrantedAt: h.clock.Now().UTC().Format(time.RFC3339Nano), ExpiresAt: h.clock.Now().Add(time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339Nano)}
		if e = repo.SaveExecution(ctx, x); e != nil {
			return u, nil, e
		}
		u.State = domain.StateInProgress
		return u, HCDRunResult{Execution: x, Admission: a}, nil
	})
	var out HCDRunResult
	if e == nil {
		e = json.Unmarshal([]byte(encoded), &out)
	}
	return out, e
}
func (h *HCD) reportStillFresh(ctx context.Context, repo ports.HCDRepository, u domain.Unit, executionID string) (bool, error) {
	ev, e := repo.ListEvidence(ctx, u.ID)
	if e != nil {
		return false, e
	}
	c := h.subjectContext(ctx, u, ev)
	for _, v := range ev {
		if v.ExecutionID == executionID && domain.EvidenceFreshness(v, c) == "fresh" {
			return true, nil
		}
	}
	return false, nil
}
func (h *HCD) Report(ctx context.Context, id string, expected int64, op, executionID string, evidence domain.Evidence) (domain.Execution, error) {
	var missing []domain.PendingCheck
	for _, f := range []struct{ name, value string }{{"Procedimiento", evidence.Procedure}, {"Entorno", evidence.Environment}, {"Sujeto", strings.Join(evidence.SubjectRef, "")}, {"Resultado", evidence.Result}} {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, domain.PendingCheck{Code: "EVIDENCE-FIELDS", Field: f.name, Message: "falta " + f.name})
		}
	}
	if evidence.Result != "pass" && evidence.Result != "fail" && evidence.Result != "" {
		missing = append(missing, domain.PendingCheck{Code: "EVIDENCE-RESULT", Field: "Resultado", Message: "resultado debe ser pass o fail"})
	}
	if len(missing) > 0 {
		return domain.Execution{}, &domain.HCDValidationError{Checks: missing}
	}

	payload, _ := json.Marshal(evidence)
	hash := hcdPayloadHash(payload)
	encoded, e := h.mutate(ctx, id, expected, op, hash, "report", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		x, e := repo.GetExecution(ctx, id, executionID)
		if e != nil {
			return u, nil, e
		}
		if x.State == "reported" {
			if x.ResultHash != hash {
				return u, nil, domain.ErrOperationConflict
			}
			return u, x, nil
		}
		if x.State != "granted" && x.State != "uncertain" {
			return u, nil, domain.ErrNotAllowed
		}
		evidence.ID = "EV-" + h.newID()
		evidence.UnitID = id
		evidence.ExecutionID = x.ID
		evidence.DefinitionRev = x.DefinitionRev
		evidence.DeliveryRev = x.DeliveryRev
		evidence.PlaybookRev = u.PlaybookRev
		evidence.ConstitutionRev = u.ConstitutionRev
		evidence.RecordedAt = h.clock.Now().UTC().Format(time.RFC3339Nano)
		if e = repo.SaveEvidence(ctx, evidence); e != nil {
			return u, nil, e
		}
		for _, ref := range evidence.Covers {
			if e = repo.AddLink(ctx, domain.Link{UnitID: id, FromKind: "evidence", FromID: evidence.ID, ToKind: "reference", ToID: ref, Relation: "verifies"}); e != nil {
				return u, nil, e
			}
		}
		x.State = "reported"
		x.ReportedAt = evidence.RecordedAt
		x.ResultHash = hash
		if e = repo.SaveExecution(ctx, x); e != nil {
			return u, nil, e
		}
		return u, x, nil
	})
	var out domain.Execution
	if e == nil {
		e = json.Unmarshal([]byte(encoded), &out)
	}
	return out, e
}

func (h *HCD) ConfigureEvidenceParser(parse func([]byte) (domain.Evidence, error)) {
	h.parseEvidence = parse
}
func (h *HCD) ParseEvidence(data []byte) (domain.Evidence, error) {
	if h.parseEvidence == nil {
		return domain.Evidence{}, domain.ErrNotAllowed
	}
	return h.parseEvidence(data)
}
func (h *HCD) ParseDelivery(data []byte) (domain.Delivery, error) {
	if h.parseDelivery == nil {
		return domain.Delivery{}, domain.ErrNotAllowed
	}
	return h.parseDelivery(data)
}
func (h *HCD) List(ctx context.Context) ([]domain.Unit, error) { return h.listApproaches(ctx) }

func (h *HCD) ExpireExecutions(ctx context.Context, id string) error {
	return h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		xs, e := repo.ListExecutions(ctx, id)
		if e != nil {
			return e
		}
		for _, x := range xs {
			if x.State != "granted" {
				continue
			}
			deadline := x.ExpiresAt
			if deadline == "" {
				at, e := time.Parse(time.RFC3339Nano, x.GrantedAt)
				if e != nil {
					return e
				}
				deadline = at.Add(24 * time.Hour).Format(time.RFC3339Nano)
			}
			expires, e := time.Parse(time.RFC3339Nano, deadline)
			if e != nil {
				return e
			}
			if !h.clock.Now().Before(expires) {
				x.State = "uncertain"
				if e = repo.SaveExecution(ctx, x); e != nil {
					return e
				}
			}
		}
		return nil
	})
}
