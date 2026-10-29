package usecases

import (
	"context"
	"mem/application/ports"
	"mem/domain"
	"strings"
)

type HCDDecisionConflict struct {
	A, B            domain.HCDDecision
	SuggestedAction string
}

func functionalDecision(d domain.HCDDecision) bool {
	switch d.Kind {
	case "approach", "approval", "block_open", "block_resolve", "close", "reject", "cancel", "clarification_answer":
		return false
	}
	return d.Status == "accepted"
}
func sameDecisionScope(a, b domain.HCDDecision) bool {
	if a.Kind != b.Kind || a.Summary == b.Summary {
		return false
	}
	for _, x := range a.Refs {
		for _, y := range b.Refs {
			if x == y {
				return true
			}
		}
	}
	return false
}
func ensureDecisionClarifications(ctx context.Context, repo ports.HCDRepository, d domain.HCDDecision) error {
	if !functionalDecision(d) {
		return nil
	}
	all, e := repo.ListDecisions(ctx, d.UnitID)
	if e != nil {
		return e
	}
	open, e := repo.ListOpenClarifications(ctx, d.UnitID)
	if e != nil {
		return e
	}
	for _, other := range all {
		if other.ID == d.ID || !functionalDecision(other) || !sameDecisionScope(other, d) {
			continue
		}
		a, b := other.ID, d.ID
		if a > b {
			a, b = b, a
		}
		target := "decision:" + a + ":" + b
		found := false
		for _, c := range open {
			found = found || c.TargetRef == target
		}
		if found {
			continue
		}
		id, e := repo.NextClarificationID(ctx, d.UnitID)
		if e != nil {
			return e
		}
		if e = repo.SaveClarification(ctx, domain.Clarification{ID: id, UnitID: d.UnitID, TargetRef: target, Question: "Resolver decisiones incompatibles " + a + " y " + b, Options: []string{"Conservar " + a, "Conservar " + b, "Reformular ambas"}, Recommendation: "Revisar las fuentes y sustituir explícitamente la decisión descartada", Status: "open", Blocking: true}); e != nil {
			return e
		}
	}
	return nil
}
func (h *HCD) decisionConflicts(decisions []domain.HCDDecision) ([]HCDDecisionConflict, error) {
	var out []HCDDecisionConflict
	seen := map[string]bool{}
	for i, a := range decisions {
		if !functionalDecision(a) {
			continue
		}
		for _, b := range decisions[i+1:] {
			if functionalDecision(b) && sameDecisionScope(a, b) {
				out = append(out, HCDDecisionConflict{A: a, B: b, SuggestedAction: "hcd-clarify"})
				seen[a.ID+":"+b.ID] = true
			}
		}
	}
	if h.relationRepo != nil {
		relations, e := h.relationRepo.ListAll(h.project)
		if e != nil {
			return nil, e
		}
		byMemory := map[int64]domain.HCDDecision{}
		for _, d := range decisions {
			if d.MemoryID != 0 {
				byMemory[d.MemoryID] = d
			}
		}
		for _, relation := range relations {
			if relation.Relation != domain.ConflictsWith {
				continue
			}
			a, okA := byMemory[relation.MemoryIDA]
			b, okB := byMemory[relation.MemoryIDB]
			if !okA || !okB {
				continue
			}
			if strings.Compare(a.ID, b.ID) > 0 {
				a, b = b, a
			}
			if seen[a.ID+":"+b.ID] {
				for i := range out {
					if out[i].A.ID == a.ID && out[i].B.ID == b.ID {
						out[i].SuggestedAction = "judge_memories; hcd-clarify"
					}
				}
			}
			if !seen[a.ID+":"+b.ID] {
				out = append(out, HCDDecisionConflict{A: a, B: b, SuggestedAction: "judge_memories"})
			}
		}
	}
	return out, nil
}
