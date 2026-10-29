package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"path/filepath"
	"strings"
	"time"
)

func (h *HCD) RecordDecision(ctx context.Context, unitID, kind, status, summary, revision string, refs []string, source string) (domain.HCDDecision, error) {
	decision := domain.HCDDecision{UnitID: unitID, Kind: kind, Status: status, Summary: domain.ProtectText(summary), Revision: revision, Refs: append([]string(nil), refs...), Source: source, MemorySync: "pending"}
	err := h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		if _, err := repo.GetUnit(ctx, unitID); err != nil {
			return err
		}
		return recordDecisionInRepo(ctx, repo, &decision, h.clock.Now())
	})
	if err == nil {
		h.syncDecisionsAfterCommit(ctx, unitID)
	}
	return decision, err
}

func recordDecisionInRepo(ctx context.Context, repo ports.HCDRepository, decision *domain.HCDDecision, at ...time.Time) error {
	if decision.Status == "accepted" {
		var now time.Time
		if len(at) > 0 {
			now = at[0]
		}
		if err := validateDecisionOverrides(ctx, repo, *decision, now); err != nil {
			return err
		}
	}
	id, err := repo.NextDecisionID(ctx, decision.UnitID)
	if err != nil {
		return err
	}
	decision.ID = id
	if err := repo.SaveDecision(ctx, *decision); err != nil {
		return err
	}
	for _, ref := range decision.Refs {
		if err := repo.AddLink(ctx, domain.Link{UnitID: decision.UnitID, FromKind: "decision", FromID: id, ToKind: "reference", ToID: ref, Relation: "justifies"}); err != nil {
			return err
		}
	}
	return ensureDecisionClarifications(ctx, repo, *decision)
}

func (h *HCD) ConfigureMemory(mem ports.MemoryRepository, relations ports.RelationRepository) {
	h.memoryRepo = mem
	h.relationRepo = relations
}

// Memory projection runs only after the control transaction commits. Failures
// leave the decision pending and visible to status/close; retries use topic_key.
func (h *HCD) syncDecisionsAfterCommit(ctx context.Context, id string) {
	if h.memoryRepo != nil {
		_ = h.MemorySync(ctx, id)
	}
}
func (h *HCD) MemorySync(ctx context.Context, id string) error {
	if h.memoryRepo == nil {
		return domain.ErrNotAllowed
	}
	u, e := h.repo.GetUnit(ctx, id)
	if e != nil {
		return e
	}
	decisions, e := h.repo.ListDecisions(ctx, id)
	if e != nil {
		return e
	}
	for _, d := range decisions {
		if (d.Status != "accepted" && d.Status != "superseded") || d.MemorySync == "synced" {
			continue
		}
		m := domain.Memory{Project: h.project, Type: domain.Decision, Title: fmt.Sprintf("%s %s: %s", u.ID, d.ID, d.Kind), TopicKey: "hcd:" + u.ID + ":" + d.ID, Filepath: filepath.Join("hcd", "features", strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "definition.md"), Content: domain.ProtectText(fmt.Sprintf("Unidad: %s (%s)\nDecisión: %s\nRevisión: %s\nElementos: %s\nOrigen: %s\nFecha: %s\n%s", u.ID, u.Kind, d.ID, d.Revision, strings.Join(d.Refs, ", "), d.Source, h.clock.Now().UTC().Format(time.RFC3339Nano), d.Summary))}
		memoryID, e := h.memoryRepo.Insert(&m)
		if e != nil {
			return e
		}
		if strings.HasPrefix(d.Source, "supersedes:") && h.relationRepo != nil {
			oldID := strings.TrimPrefix(d.Source, "supersedes:")
			fresh, e := h.repo.ListDecisions(ctx, id)
			if e != nil {
				return e
			}
			var oldMemory int64
			for _, old := range fresh {
				if old.ID == oldID {
					oldMemory = old.MemoryID
				}
			}
			if oldMemory == 0 {
				return domain.ErrBlocked
			}
			relation, e := h.relationRepo.GetByPair(h.project, memoryID, oldMemory)
			if e != nil {
				return e
			}
			if relation == nil {
				_, e = h.relationRepo.Insert(&domain.Relation{Project: h.project, MemoryIDA: memoryID, MemoryIDB: oldMemory, Relation: domain.Supersedes, Confidence: 1, Reasoning: "HCD decisión citada que sustituye a " + oldID})
			} else if relation.Relation != domain.Supersedes {
				e = h.relationRepo.Update(relation.ID, domain.Supersedes, 1, "HCD sustitución citada")
			}
			if e != nil {
				return e
			}
		}
		if e = h.repo.MarkDecisionSynced(ctx, id, d.ID, memoryID); e != nil {
			return e
		}
	}
	return nil
}

type HCDDecisionInput struct {
	UnitID                                                    string
	ExpectedRevision                                          int64
	OperationID, Action, ID, By, Kind, Summary, Quote, Client string
	Refs                                                      []string
}

func (h *HCD) Decision(ctx context.Context, input HCDDecisionInput) (domain.HCDDecision, error) {
	payload, _ := json.Marshal(input)
	encoded, e := h.mutate(ctx, input.UnitID, input.ExpectedRevision, input.OperationID, hcdPayloadHash(payload), "decision", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if input.Action == "propose" {
			d := domain.HCDDecision{UnitID: u.ID, Kind: input.Kind, Status: "proposed", Summary: domain.ProtectText(input.Summary), Revision: u.DefinitionRev, Refs: input.Refs, Source: "decision", MemorySync: "pending"}
			if strings.TrimSpace(d.Kind) == "" || strings.TrimSpace(d.Summary) == "" {
				return u, nil, domain.ErrValidation
			}
			e := recordDecisionInRepo(ctx, repo, &d)
			return u, d, e
		}
		if input.Client == "tui" {
			return u, nil, domain.ErrNotAllowed
		}
		if !domain.ValidQuote(input.Quote, u.DefinitionRev) {
			return u, nil, domain.ErrInvalidQuote
		}
		all, e := repo.ListDecisions(ctx, u.ID)
		if e != nil {
			return u, nil, e
		}
		var old, current domain.HCDDecision
		for _, d := range all {
			if d.ID == input.ID {
				old = d
			}
			if d.ID == input.By {
				current = d
			}
		}
		if old.ID == "" {
			return u, nil, domain.ErrNotFound
		}
		action := "decision:" + old.ID
		switch input.Action {
		case "accept":
			if old.Status != "proposed" {
				return u, nil, domain.ErrNotAllowed
			}
			current = old
			current.Status = "accepted"
			current.Source = input.Client
		case "supersede":
			if old.Status != "accepted" || current.ID == "" || current.Status != "proposed" {
				return u, nil, domain.ErrNotAllowed
			}
			old.Status = "superseded"
			if e = repo.UpdateDecision(ctx, old); e != nil {
				return u, nil, e
			}
			current.Status = "accepted"
			current.Source = "supersedes:" + old.ID
			action = "decision:" + current.ID
		default:
			return u, nil, domain.ErrValidation
		}
		current.Revision = u.DefinitionRev
		current.MemorySync = "pending"
		if e = validateDecisionOverrides(ctx, repo, current, h.clock.Now()); e != nil {
			return u, nil, e
		}
		if e = repo.UpdateDecision(ctx, current); e != nil {
			return u, nil, e
		}
		if e = ensureDecisionClarifications(ctx, repo, current); e != nil {
			return u, nil, e
		}
		id, e := repo.NextApprovalID(ctx, u.ID)
		if e != nil {
			return u, nil, e
		}
		origin := "conversation"
		if input.Client == "terminal" {
			origin = "terminal"
		}
		if e = repo.SaveApproval(ctx, domain.Approval{ID: id, UnitID: u.ID, Object: action, Revision: u.DefinitionRev, Origin: origin, Quote: input.Quote, Client: input.Client, ApprovedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			return u, nil, e
		}
		if e = repo.AddLink(ctx, domain.Link{UnitID: u.ID, FromKind: "decision", FromID: current.ID, ToKind: "approval", ToID: id, Relation: "justifies"}); e != nil {
			return u, nil, e
		}
		return u, current, nil
	})
	var d domain.HCDDecision
	if e == nil {
		e = json.Unmarshal([]byte(encoded), &d)
	}
	return d, e
}
