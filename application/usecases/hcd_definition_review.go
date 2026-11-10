package usecases

import (
	"context"
	"fmt"
	"mem/application/ports"
	"mem/domain"
)

type HCDDefinitionReviewInput struct {
	ID                   string
	Revision             domain.HCDReviewRevision
	OperationID          domain.HCDReviewOperationID
	Kind, Action, Client string
	Payload              domain.HCDDefinitionReviewPayload
}

func (h *HCD) ConfigureDefinitionReviews(repo ports.HCDDefinitionReviewRepository) { h.reviews = repo }

func (h *HCD) RecordDefinitionReview(ctx context.Context, in HCDDefinitionReviewInput) (HCDView, domain.HCDDefinitionReviewEvent, error) {
	var event domain.HCDDefinitionReviewEvent
	if h.reviews == nil {
		return HCDView{}, event, fmt.Errorf("%w: definition review repository is unavailable", domain.ErrNotAllowed)
	}
	view, err := h.Show(ctx, in.ID)
	if err != nil {
		return view, event, err
	}
	if view.Unit.State != domain.UnitActive {
		return view, event, fmt.Errorf("%w: unit is not active", domain.ErrNotAllowed)
	}
	if string(in.Revision) != view.DefinitionRevision {
		return view, event, fmt.Errorf("%w: definition revision changed; read the unit again", domain.ErrValidation)
	}
	if in.Kind == "splash" && !view.State.HasSocraticExam() {
		return view, event, fmt.Errorf("%w: socratic examination required before splash review", domain.ErrNotAllowed)
	}
	event = domain.HCDDefinitionReviewEvent{OperationID: in.OperationID, UnitID: view.Unit.ID, Revision: in.Revision, Kind: in.Kind, Action: in.Action, Client: in.Client, CreatedAt: h.clock.Now(), Payload: in.Payload}.Protected()
	if err := event.Validate(); err != nil {
		return view, event, err
	}
	for _, question := range event.Payload.Questions {
		if question.ClarificationID == "" {
			continue
		}
		found := false
		for _, clarification := range view.State.Doc.Clarifications {
			if clarification.ID == question.ClarificationID {
				found = true
				break
			}
		}
		if !found {
			return view, event, fmt.Errorf("%w: unknown clarification reference", domain.ErrValidation)
		}
	}
	events, err := h.reviews.DefinitionReviews(ctx, view.Unit.ID)
	if err != nil {
		return view, event, err
	}
	if err := event.ValidateTransition(events); err != nil {
		return view, event, err
	}
	var last domain.HCDReviewOperationID
	for _, previous := range events {
		if previous.Revision != in.Revision {
			continue
		}
		last = previous.OperationID
		// A completed examination is resumed by replaying its operation, not by
		// opening another round of questions for the same definition.
		if event.Kind == "socratic" && previous.Kind == "socratic" && previous.Action == "complete" && previous.OperationID != event.OperationID {
			return view, event, fmt.Errorf("%w: examination already recorded for this revision", domain.ErrNotAllowed)
		}
	}
	event, err = h.reviews.AppendReview(ctx, event, last)
	if err != nil {
		return view, event, err
	}
	next, err := h.Show(ctx, view.Unit.ID)
	return next, event, err
}
