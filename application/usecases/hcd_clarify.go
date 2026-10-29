package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"mem/application/ports"
	"mem/domain"
)

type HCDClarifyInput struct {
	UnitID                                                                            string
	ExpectedRevision                                                                  int64
	OperationID, Action, TargetRef, Question, Recommendation, ClarificationID, Choice string
	Options                                                                           []string
	Blocking                                                                          bool
}

func (h *HCD) Clarify(ctx context.Context, input HCDClarifyInput) (domain.Clarification, error) {
	var result domain.Clarification
	payload, _ := json.Marshal(input)
	encoded, err := h.mutate(ctx, input.UnitID, input.ExpectedRevision, input.OperationID, hcdPayloadHash(payload), "clarify", func(repo ports.HCDRepository, currentUnit domain.Unit) (domain.Unit, any, error) {
		switch input.Action {
		case "open":
			if input.TargetRef == "" || input.Question == "" {
				return domain.Unit{}, nil, domain.ErrValidation
			}
			id, err := repo.NextClarificationID(ctx, input.UnitID)
			if err != nil {
				return domain.Unit{}, nil, err
			}
			result = domain.Clarification{ID: id, UnitID: input.UnitID, TargetRef: input.TargetRef, Question: input.Question, Options: input.Options, Recommendation: input.Recommendation, Status: "open", Blocking: input.Blocking}
			if err := repo.SaveClarification(ctx, result); err != nil {
				return domain.Unit{}, nil, err
			}
		case "answer":
			if input.ClarificationID == "" || input.Choice == "" {
				return domain.Unit{}, nil, domain.ErrValidation
			}
			current, err := repo.GetClarification(ctx, input.UnitID, input.ClarificationID)
			if err != nil {
				return domain.Unit{}, nil, err
			}
			if current.Status != "open" {
				return domain.Unit{}, nil, domain.ErrNotAllowed
			}
			current.Answer = input.Choice
			current.Status = "answered"
			result = current
			if err := repo.SaveClarification(ctx, current); err != nil {
				return domain.Unit{}, nil, err
			}
			decision := domain.HCDDecision{UnitID: input.UnitID, Kind: "clarification_answer", Status: "accepted", Revision: currentUnit.DefinitionRev, Summary: fmt.Sprintf("%s → %s", current.Question, input.Choice), Refs: []string{current.ID, current.TargetRef}, Source: "clarify", MemorySync: "pending"}
			if err := recordDecisionInRepo(ctx, repo, &decision); err != nil {
				return domain.Unit{}, nil, err
			}
		default:
			return domain.Unit{}, nil, domain.ErrValidation
		}
		unit, err := repo.GetUnit(ctx, input.UnitID)
		if err != nil {
			return domain.Unit{}, nil, err
		}
		return unit, result, nil
	})
	if err != nil {
		return domain.Clarification{}, err
	}
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		return domain.Clarification{}, err
	}
	return result, nil
}

func (h *HCD) ValidateUnit(ctx context.Context, unitID string, definition domain.Definition) ([]domain.PendingCheck, error) {
	unit, err := h.repo.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	checks := domain.ValidateDefinition(definition, unit.Kind)
	clarifications, err := h.repo.ListOpenClarifications(ctx, unitID)
	if err != nil {
		return nil, err
	}
	for _, clarification := range clarifications {
		if clarification.Blocking {
			checks = append(checks, domain.PendingCheck{Code: "CLARIFICATION-OPEN", Field: clarification.TargetRef, Message: "aclaración bloqueante abierta: " + clarification.ID})
		}
	}
	return checks, nil
}
