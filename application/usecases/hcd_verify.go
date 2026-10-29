package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"os"
	"strings"
	"time"
)

type HCDVerification struct {
	RequiredAdoptions            []string
	SliceID                      string `json:",omitempty"`
	PendingMemories              int
	EvidenceRecords              []domain.Evidence
	Approvals                    []domain.Approval
	Blocks                       []domain.Block
	Drift, Warnings              []string
	ReportRevision               string
	Verdict                      domain.HCDClosureVerdict
	Criteria                     map[string]domain.CriterionState
	Evidence                     map[string]string
	Implemented, Verified, Total int
	DefinitionRev, DeliveryRev   string
}

func (h *HCD) ConfigureSubjectInspector(inspect func(context.Context, string) (bool, error)) {
	h.inspectSubject = inspect
}

// ConfigureSubjectRef conecta el cálculo del sujeto `tree:` que el inspector acepta.
func (h *HCD) ConfigureSubjectRef(ref func(context.Context, []string) (string, error)) {
	h.subjectRef = ref
}

// SubjectRef devuelve la referencia `tree:` de los archivos dados. Es una
// consulta: no cambia la revisión ni registra nada.
func (h *HCD) SubjectRef(ctx context.Context, files []string) (string, error) {
	if h.subjectRef == nil {
		return "", domain.ErrNotAllowed
	}
	if len(files) == 0 {
		return "", domain.ErrValidation
	}
	return h.subjectRef(ctx, files)
}

// subjectContext inspecciona una vez cada sujeto citado por las evidencias.
func (h *HCD) subjectContext(ctx context.Context, u domain.Unit, ev []domain.Evidence) domain.HCDVerificationContext {
	c := domain.HCDVerificationContext{Unit: u, Subjects: map[string]bool{}}
	for _, v := range ev {
		for _, ref := range v.SubjectRef {
			if _, seen := c.Subjects[ref]; seen {
				continue
			}
			current := false
			if h.inspectSubject != nil {
				if ok, e := h.inspectSubject(ctx, ref); e == nil {
					current = ok
				}
			}
			c.Subjects[ref] = current
		}
	}
	return c
}
func (h *HCD) verification(ctx context.Context, repo ports.HCDRepository, u domain.Unit, scope ...string) (HCDVerification, error) {
	w, e := h.work(ctx, repo, u)
	if e != nil {
		return HCDVerification{}, e
	}
	fullWork := w
	sliceID := ""
	if len(scope) > 0 {
		sliceID = scope[0]
	}
	if sliceID != "" {
		found := false
		for _, s := range w.Slices {
			if s.ID != sliceID {
				continue
			}
			found = true
			w.Slices = []domain.Slice{s}
			w.Tasks = nil
			for _, task := range fullWork.Tasks {
				if task.SliceID == sliceID {
					w.Tasks = append(w.Tasks, task)
				}
			}
			w.Definition.Criteria = nil
			for _, criterion := range fullWork.Definition.Criteria {
				for _, ref := range s.Criteria {
					if criterion.ID == ref {
						w.Definition.Criteria = append(w.Definition.Criteria, criterion)
						break
					}
				}
			}
			break
		}
		if !found {
			return HCDVerification{}, domain.ErrNotFound
		}
	}
	ev, e := repo.ListEvidence(ctx, u.ID)
	if e != nil {
		return HCDVerification{}, e
	}
	blocks, e := repo.ListBlocks(ctx, u.ID)
	if e != nil {
		return HCDVerification{}, e
	}
	c := h.subjectContext(ctx, u, ev)
	exceptions, e := repo.ListExceptions(ctx, u.ID)
	if e != nil {
		return HCDVerification{}, e
	}
	r := HCDVerification{Verdict: domain.ClosureVerdict(w, ev, c, blocks, exceptions, h.clock.Now()), Criteria: map[string]domain.CriterionState{}, Evidence: map[string]string{}, Total: len(w.Tasks), DefinitionRev: u.DefinitionRev, DeliveryRev: u.DeliveryRev}
	for _, doc := range []struct{ name, topic, fallback, revision string }{{"playbook", domain.TopicHCDPlaybook, h.playbookTemplate, u.PlaybookRev}, {"constitution", domain.TopicConstitution, h.constitutionTemplate, u.ConstitutionRev}} {
		current, err := ResolvePinnedDoc(h.topics, h.project, doc.topic, doc.fallback)
		if err != nil {
			return r, err
		}
		if strings.Contains(current.Content, "Urgente: seguridad") && domain.ContentRevision([]byte(current.Content)) != doc.revision {
			r.RequiredAdoptions = append(r.RequiredAdoptions, doc.name)
			r.Verdict.CanClose = false
			r.Verdict.Causes = append(r.Verdict.Causes, domain.PendingCheck{Code: "NORMATIVE-URGENT", Field: doc.name, Message: "adoptar la revisión urgente de seguridad"})
		}
	}
	r.SliceID = sliceID
	if u.DefinitionRev == "" || u.ApprovedRev != u.DefinitionRev {
		r.Verdict.CanClose = false
		r.Verdict.Causes = append(r.Verdict.Causes, domain.PendingCheck{Code: "DEFINITION-UNAPPROVED", Field: "definition.md", Message: "aprobar la revisión vigente de la definición"})
	}
	actual, e := h.artifacts.Hash(ctx, strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "delivery.md")
	if e != nil || actual != u.DeliveryRev {
		r.Verdict.CanClose = false
		r.Verdict.Causes = append(r.Verdict.Causes, domain.PendingCheck{Code: "DELIVERY-STALE", Field: "delivery.md", Message: "la entrega cambió fuera del motor"})
	}
	conflicts, e := h.decisionConflicts(w.Decisions)
	if e != nil {
		return r, e
	}
	for _, conflict := range conflicts {
		r.Verdict.CanClose = false
		r.Verdict.Causes = append(r.Verdict.Causes, domain.PendingCheck{Code: "DECISION-CONFLICT", Field: conflict.A.ID + ":" + conflict.B.ID, Message: conflict.SuggestedAction})
	}
	for _, check := range domain.ValidateDefinition(fullWork.Definition, u.Kind) {
		r.Verdict.CanClose = false
		r.Verdict.Causes = append(r.Verdict.Causes, check)
	}
	for _, task := range w.Tasks {
		for _, check := range domain.AtomicityViolations(task, fullWork, nil) {
			r.Verdict.CanClose = false
			r.Verdict.Causes = append(r.Verdict.Causes, check)
		}
	}
	if domain.DependencyCycle(w.Tasks) {
		r.Verdict.CanClose = false
		r.Verdict.Causes = append(r.Verdict.Causes, domain.PendingCheck{Code: "DEPENDENCY-CYCLE", Message: "ciclo de dependencias"})
	}
	for _, criterion := range w.Definition.Criteria {
		r.Criteria[criterion.ID] = domain.CriterionStatus(criterion, ev, c, w.Decisions)
	}
	for _, v := range ev {
		r.Evidence[v.ID] = domain.EvidenceFreshness(v, c)
	}
	r.Drift, e = h.drift(ctx, repo, u.ID)
	if e != nil {
		return r, e
	}
	for _, d := range w.Decisions {
		if d.Status == "accepted" && d.MemorySync != "synced" {
			r.PendingMemories++
		}
	}
	r.EvidenceRecords = ev
	r.Approvals, e = repo.ListApprovals(ctx, u.ID)
	if e != nil {
		return r, e
	}
	r.Blocks = blocks
	xs, e := repo.ListExecutions(ctx, u.ID)
	if e != nil {
		return r, e
	}
	for _, task := range w.Tasks {
		implemented, verified := false, false
		for _, x := range xs {
			if x.TaskID == task.ID && x.State == "reported" {
				implemented = true
			}
		}
		for _, v := range ev {
			if domain.EvidenceFreshness(v, c) != "fresh" {
				continue
			}
			for _, ref := range v.Covers {
				if ref == task.ID {
					if v.Result == "fail" {
						verified = false
						goto verdictTask
					}
					verified = verified || v.Result == "pass"
				}
			}
		}
	verdictTask:
		if implemented {
			r.Implemented++
		}
		if verified {
			r.Verified++
		}
	}
	return r, nil
}
func (h *HCD) Status(ctx context.Context, id string) (HCDVerification, error) {
	u, e := h.repo.GetUnit(ctx, id)
	if e != nil {
		return HCDVerification{}, e
	}
	return h.verification(ctx, h.repo, u)
}
func (h *HCD) Verify(ctx context.Context, id string, expected int64, op string, scope ...string) (HCDVerification, error) {
	original, e := h.repo.GetUnit(ctx, id)
	if e != nil {
		return HCDVerification{}, e
	}
	if original.Approach != "hcd" {
		return HCDVerification{Verdict: domain.HCDClosureVerdict{CanClose: false, Causes: []domain.PendingCheck{{Code: "APPROACH", Field: id, Message: "la unidad no usa HCD"}}}}, nil
	}
	dir := strings.TrimPrefix(original.ID, "HCD-") + "-" + original.Slug
	previous, readErr := h.artifacts.Read(ctx, dir, "verification.md")
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return HCDVerification{}, readErr
	}
	wrote := false
	sliceID := ""
	if len(scope) > 0 {
		sliceID = scope[0]
	}
	payload, _ := json.Marshal([]string{id, sliceID})
	encoded, e := h.mutate(ctx, id, expected, op, hcdPayloadHash(payload), "verify", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if u.Approach != "hcd" {
			return u, nil, domain.ErrApproachMismatch
		}
		if u.State != domain.StateInProgress && u.State != domain.StateReady && u.State != domain.StateVerification {
			return u, nil, domain.ErrNotAllowed
		}
		r, e := h.verification(ctx, repo, u)
		if e != nil {
			return u, nil, e
		}
		response := r
		if sliceID != "" {
			response, e = h.verification(ctx, repo, u, sliceID)
			if e != nil {
				return u, nil, e
			}
		}
		last, e := repo.LastVerificationRevision(ctx, id)
		if e != nil {
			return u, nil, e
		}
		if readErr == nil && (last == "" || last != domain.ContentRevision(previous)) {
			r.Warnings = append(r.Warnings, "verification.md fue editado; se regeneró")
		}
		content := RenderHCDVerification(u, r)
		r.ReportRevision = domain.ContentRevision(content)
		if e = h.artifacts.Write(ctx, dir, "verification.md", content); e != nil {
			return u, nil, e
		}
		wrote = true
		if r.Verdict.CanClose {
			u.State = domain.StateVerification
		} else if u.State == domain.StateVerification {
			u.State = domain.StateInProgress
		}
		response.ReportRevision = r.ReportRevision
		response.Warnings = r.Warnings
		return u, response, nil
	})
	if e != nil && wrote {
		var rollback error
		cctx, cancel := compensationContext(ctx)
		if readErr == nil {
			rollback = h.artifacts.Write(cctx, dir, "verification.md", previous)
		} else {
			rollback = h.artifacts.Delete(cctx, dir, "verification.md")
		}
		cancel()
		e = errors.Join(e, rollback)
	}
	var r HCDVerification
	if e == nil {
		e = json.Unmarshal([]byte(encoded), &r)
	}
	return r, e
}

type HCDCloseInput struct {
	UnitID                                                     string
	ExpectedRevision                                           int64
	OperationID, Revision, Quote, Client, Reason, SupersededBy string
	Reject                                                     bool
}

func (h *HCD) Close(ctx context.Context, input HCDCloseInput) (domain.Unit, error) {
	return h.finish(ctx, input, false)
}
func (h *HCD) Cancel(ctx context.Context, input HCDCloseInput) (domain.Unit, error) {
	return h.finish(ctx, input, true)
}
func (h *HCD) finish(ctx context.Context, input HCDCloseInput, cancel bool) (domain.Unit, error) {
	if input.Client == "tui" {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	if !domain.ValidQuote(input.Quote, input.Revision) {
		return domain.Unit{}, domain.ErrInvalidQuote
	}
	payload, _ := json.Marshal(input)
	command := "close"
	if cancel {
		command = "cancel"
	}
	if input.OperationID == "" {
		input.OperationID = h.newID()
		payload, _ = json.Marshal(input)
	}
	memoryException := false
	if !cancel && !input.Reject && h.memoryRepo != nil {
		current, err := h.repo.GetUnit(ctx, input.UnitID)
		if err != nil {
			return domain.Unit{}, err
		}
		memoryException, err = h.memoryException(ctx, h.repo, current)
		if err != nil {
			return domain.Unit{}, err
		}
	}
	prepared := !cancel && !input.Reject && h.memoryRepo != nil && !memoryException
	preparedSource := "close_prepare:" + input.OperationID + ":" + hcdPayloadHash(payload)
	if prepared {
		if e := h.prepareClose(ctx, input, hcdPayloadHash(payload)); e != nil {
			return domain.Unit{}, e
		}
	}
	stale := false
	encoded, e := h.mutate(ctx, input.UnitID, input.ExpectedRevision, input.OperationID, hcdPayloadHash(payload), command, func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if u.State == domain.StateCompleted || u.State == domain.StateCancelled || u.State == domain.StateSuperseded {
			return u, nil, domain.ErrNotAllowed
		}
		kind := command
		if cancel {
			u.State = domain.StateCancelled
			if input.SupersededBy != "" {
				if input.SupersededBy == u.ID {
					return u, nil, domain.ErrValidation
				}
				if _, e := repo.GetUnit(ctx, input.SupersededBy); e != nil {
					return u, nil, e
				}
				u.State = domain.StateSuperseded
			}
		} else {
			if u.State != domain.StateVerification {
				return u, nil, domain.ErrNotAllowed
			}
			if input.Revision != u.DefinitionRev || input.Revision != u.ApprovedRev {
				return u, nil, domain.ErrRevisionConflict
			}
			if input.Reject {
				if strings.TrimSpace(input.Reason) == "" {
					return u, nil, domain.ErrValidation
				}
				u.State = domain.StateInProgress
				kind = "reject"
			} else {
				r, e := h.verification(ctx, repo, u)
				if e != nil {
					return u, nil, e
				}
				if !r.Verdict.CanClose {
					u.State = domain.StateInProgress
					stale = true
					// ACR C-002: prepareClose ya registró la aceptación citada; si
					// el cierre no ocurre, la decisión no puede quedar accepted.
					if prepared {
						if e := withdrawPreparedClose(ctx, repo, u.ID, preparedSource, "la verificación final no permitió cerrar"); e != nil {
							return u, nil, e
						}
					}
					return u, u, nil
				}
				u.State = domain.StateCompleted
			}
		}
		if !cancel && !input.Reject && h.memoryRepo != nil {
			allowed, err := h.memoryException(ctx, repo, u)
			if err != nil {
				return u, nil, err
			}
			decisions, e := repo.ListDecisions(ctx, u.ID)
			if e != nil {
				return u, nil, e
			}
			for _, d := range decisions {
				if d.Status == "accepted" && d.MemorySync != "synced" && !allowed {
					return u, nil, domain.ErrBlocked
				}
			}
		}
		if prepared {
			f, e := repo.GetFocus(ctx)
			if e != nil && !errors.Is(e, domain.ErrNotFound) {
				return u, nil, e
			}
			if e == nil && f.UnitID == u.ID {
				if e = repo.ClearFocus(ctx); e != nil {
					return u, nil, e
				}
			}
			return u, u, nil
		}
		id, e := repo.NextApprovalID(ctx, u.ID)
		if e != nil {
			return u, nil, e
		}
		origin := "conversation"
		if input.Client == "terminal" {
			origin = "terminal"
		}
		if e = repo.SaveApproval(ctx, domain.Approval{ID: id, UnitID: u.ID, Object: kind, Revision: input.Revision, Origin: origin, Quote: input.Quote, Client: input.Client, ApprovedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			return u, nil, e
		}
		decision := domain.HCDDecision{UnitID: u.ID, Kind: kind, Status: "accepted", Summary: fmt.Sprintf("%s %s", kind, input.Reason), Revision: u.DefinitionRev, Refs: []string{u.ID, id}, Source: command, MemorySync: "pending"}
		if e = recordDecisionInRepo(ctx, repo, &decision); e != nil {
			return u, nil, e
		}
		if !input.Reject {
			f, e := repo.GetFocus(ctx)
			if e != nil && !errors.Is(e, domain.ErrNotFound) {
				return u, nil, e
			}
			if e == nil && f.UnitID == u.ID {
				if e = repo.ClearFocus(ctx); e != nil {
					return u, nil, e
				}
			}
		}
		return u, u, nil
	})
	if e != nil && prepared {
		// La transacción final no ocurrió: la aceptación que prepareClose ya
		// registró y proyectó no puede quedar accepted para un cierre que no
		// existe. Contexto propio: el fallo pudo ser una cancelación.
		cctx, done := compensationContext(ctx)
		withdrawErr := h.uow.WithTransaction(cctx, func(repo ports.HCDRepository) error {
			return withdrawPreparedClose(cctx, repo, input.UnitID, preparedSource, "el cierre falló")
		})
		if withdrawErr == nil {
			// Si la proyección falla, queda pending y se reintenta.
			_ = h.MemorySync(cctx, input.UnitID)
		}
		done()
		if withdrawErr != nil {
			e = errors.Join(e, fmt.Errorf("withdraw prepared close: %w", withdrawErr))
		}
	}
	var u domain.Unit
	if e == nil {
		e = json.Unmarshal([]byte(encoded), &u)
	}
	if e == nil && stale {
		if prepared {
			// Proyecta la retirada a memoria; si falla queda pending y se reintenta.
			_ = h.MemorySync(ctx, input.UnitID)
		}
		e = domain.ErrNotAllowed
	}
	return u, e
}

// withdrawPreparedClose marca como superseded la aceptación de cierre que
// prepareClose guardó, cuando la verificación final impide cerrar.
func withdrawPreparedClose(ctx context.Context, repo ports.HCDRepository, unitID, source, reason string) error {
	decisions, e := repo.ListDecisions(ctx, unitID)
	if e != nil {
		return e
	}
	for _, d := range decisions {
		if d.Source == source && d.Status == "accepted" {
			d.Status = "superseded"
			d.Summary += " · retirada: " + reason
			d.MemorySync = "pending"
			return repo.UpdateDecision(ctx, d)
		}
	}
	return nil
}

// Project acceptance before the final state transition. A failed projection
// leaves the unit in verification and can be retried with the same operation.
func (h *HCD) prepareClose(ctx context.Context, input HCDCloseInput, hash string) error {
	prefix := "close_prepare:" + input.OperationID + ":"
	source := prefix + hash
	e := h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		u, e := repo.GetUnit(ctx, input.UnitID)
		if e != nil {
			return e
		}
		cached, replayed, e := repo.RecordOperation(ctx, input.OperationID, u.ID, "close", hash, "")
		_ = cached
		if e != nil {
			return e
		}
		if replayed {
			return nil
		}
		if u.State != domain.StateVerification || u.StateRevision != input.ExpectedRevision {
			return domain.ErrNotAllowed
		}
		if u.DefinitionRev != input.Revision || u.ApprovedRev != input.Revision {
			return domain.ErrRevisionConflict
		}
		r, e := h.verification(ctx, repo, u)
		if e != nil {
			return e
		}
		if !r.Verdict.CanClose {
			return domain.ErrNotAllowed
		}
		decisions, e := repo.ListDecisions(ctx, u.ID)
		if e != nil {
			return e
		}
		for _, d := range decisions {
			if strings.HasPrefix(d.Source, prefix) {
				// Una aceptación retirada no se reutiliza: cerrar con ella
				// dejaría la unidad completada sin aceptación vigente.
				if d.Source != source || d.Status != "accepted" {
					return domain.ErrOperationConflict
				}
				return nil
			}
		}
		id, e := repo.NextApprovalID(ctx, u.ID)
		if e != nil {
			return e
		}
		origin := "conversation"
		if input.Client == "terminal" {
			origin = "terminal"
		}
		if e = repo.SaveApproval(ctx, domain.Approval{ID: id, UnitID: u.ID, Object: "close", Revision: u.DefinitionRev, Origin: origin, Quote: input.Quote, Client: input.Client, ApprovedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			return e
		}
		decision := domain.HCDDecision{UnitID: u.ID, Kind: "close", Status: "accepted", Summary: "Aceptación citada del cierre", Revision: u.DefinitionRev, Refs: []string{u.ID, id}, Source: source, MemorySync: "pending"}
		return recordDecisionInRepo(ctx, repo, &decision)
	})
	if e != nil {
		return e
	}
	if e = h.MemorySync(ctx, input.UnitID); e != nil {
		return domain.ErrBlocked
	}
	return nil
}
