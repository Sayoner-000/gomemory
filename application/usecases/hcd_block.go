package usecases

import (
	"context"
	"encoding/json"
	"mem/application/ports"
	"mem/domain"
	"strings"
)

type HCDBlockInput struct {
	UnitID                                                string
	ExpectedRevision                                      int64
	OperationID, ResolveID, TargetRef, Reason, Resolution string
	Critical                                              bool
}

func (h *HCD) Block(ctx context.Context, input HCDBlockInput) (domain.Block, error) {
	payload, _ := json.Marshal(input)
	encoded, e := h.mutate(ctx, input.UnitID, input.ExpectedRevision, input.OperationID, hcdPayloadHash(payload), "block", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if u.State == domain.StateCompleted || u.State == domain.StateCancelled || u.State == domain.StateSuperseded {
			return u, nil, domain.ErrNotAllowed
		}
		var b domain.Block
		kind := "block_open"
		blocks, e := repo.ListBlocks(ctx, u.ID)
		if e != nil {
			return u, nil, e
		}
		if input.ResolveID != "" {
			kind = "block_resolve"
			for _, v := range blocks {
				if v.ID == input.ResolveID {
					b = v
				}
			}
			if b.ID == "" {
				return u, nil, domain.ErrNotFound
			}
			if b.Status != "open" || strings.TrimSpace(input.Resolution) == "" {
				return u, nil, domain.ErrValidation
			}
			b.Status = "resolved"
			b.Resolution = input.Resolution
		} else {
			if strings.TrimSpace(input.Reason) == "" || input.TargetRef == "" {
				return u, nil, domain.ErrValidation
			}
			if input.Critical && input.TargetRef != u.ID {
				return u, nil, domain.ErrValidation
			}
			if input.TargetRef != u.ID {
				w, e := h.work(ctx, repo, u)
				if e != nil {
					return u, nil, e
				}
				found := false
				for _, t := range w.Tasks {
					found = found || t.ID == input.TargetRef
				}
				for _, s := range w.Slices {
					found = found || s.ID == input.TargetRef
				}
				if !found {
					return u, nil, domain.ErrNotFound
				}
			}
			id, e := repo.NextBlockID(ctx)
			if e != nil {
				return u, nil, e
			}
			b = domain.Block{ID: id, UnitID: u.ID, TargetRef: input.TargetRef, Reason: input.Reason, Critical: input.Critical, Status: "open"}
			if b.Critical {
				if u.State != domain.StateBlocked {
					u.PrevState = u.State
				}
				u.State = domain.StateBlocked
			}
		}
		if input.ResolveID != "" {
			xs, e := repo.ListExecutions(ctx, u.ID)
			if e != nil {
				return u, nil, e
			}
			for _, x := range xs {
				if x.TaskID == b.TargetRef && x.State == "uncertain" {
					x.State = "cancelled"
					if e = repo.SaveExecution(ctx, x); e != nil {
						return u, nil, e
					}
				}
			}
		}
		if e = repo.SaveBlock(ctx, b); e != nil {
			return u, nil, e
		}
		if input.ResolveID != "" && b.Critical && u.State == domain.StateBlocked {
			other := false
			for _, v := range blocks {
				if v.ID != b.ID && v.Critical && v.Status == "open" {
					other = true
				}
			}
			if !other {
				switch u.PrevState {
				case domain.StateDraft:
					u.State = domain.StateDraft
				case domain.StateReady, domain.StateInProgress, domain.StateVerification:
					if u.ApprovedRev == u.DefinitionRev && u.ApprovedRev != "" {
						u.State = u.PrevState
					} else {
						u.State = domain.StateDraft
					}
				default:
					u.State = domain.StateDraft
				}
				u.PrevState = ""
			}
		}
		decision := domain.HCDDecision{UnitID: u.ID, Kind: kind, Status: "accepted", Summary: b.Reason + " " + b.Resolution, Revision: u.DefinitionRev, Refs: []string{b.ID, b.TargetRef}, Source: "block", MemorySync: "pending"}
		if e = recordDecisionInRepo(ctx, repo, &decision); e != nil {
			return u, nil, e
		}
		return u, b, nil
	})
	var b domain.Block
	if e == nil {
		e = json.Unmarshal([]byte(encoded), &b)
	}
	return b, e
}
