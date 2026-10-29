package usecases

import (
	"context"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"strings"
	"time"
)

func (h *HCD) ConfigureSpecKitReader(reader ports.SpecKitReader) { h.specKitReader = reader }
func (h *HCD) listApproaches(ctx context.Context) ([]domain.Unit, error) {
	units, e := h.repo.ListHCDUnits(ctx)
	if e != nil {
		return nil, e
	}
	reader, ok := h.artifacts.(ports.HCDApproachReader)
	if !ok {
		return units, nil
	}
	candidates, e := reader.ApproachCandidates(ctx)
	if e != nil {
		return nil, e
	}
	migrations, e := h.repo.ListMigrations(ctx)
	if e != nil {
		return nil, e
	}
	migrated := map[string]bool{}
	for _, m := range migrations {
		migrated[m.SourceDir] = migrated[m.SourceDir] || m.Status == domain.MigrationActivated
	}
	byFolder := map[string]int{}
	for i, u := range units {
		folder := "hcd/features/" + strings.TrimPrefix(u.ID, "HCD-") + "-" + u.Slug
		units[i].SourceDir = folder
		byFolder[folder] = i
	}
	for _, candidate := range candidates {
		if candidate.Origin == "speckit" {
			if migrated[candidate.Folder] {
				continue // una unidad migrada conserva un único enfoque operativo (C-21)
			}
			if h.specKitReader != nil {
				if _, e := h.specKitReader.Read(h.projectRoot(), candidate.Name, ""); e != nil {
					return nil, e
				}
			}
			units = append(units, domain.Unit{ID: candidate.Name, Title: candidate.Name, Approach: "speckit", SourceDir: candidate.Folder})
			continue
		}
		if i, found := byFolder[candidate.Folder]; found {
			if candidate.HasDefinition && candidate.HasSpec {
				decisions, e := h.repo.ListDecisions(ctx, units[i].ID)
				if e != nil {
					return nil, e
				}
				classified := false
				for _, d := range decisions {
					classified = classified || d.Kind == "classification" && d.Status == "accepted"
				}
				if !classified {
					units[i].Approach = "unclassified"
				}
			}
			continue
		}
		id := strings.SplitN(candidate.Name, "-", 2)[0]
		units = append(units, domain.Unit{ID: "HCD-" + id, Title: candidate.Name, Approach: "unclassified", SourceDir: candidate.Folder})
	}
	return units, nil
}
func (h *HCD) projectRoot() string {
	return strings.TrimSuffix(strings.TrimSuffix(h.artifacts.Root(), "/"), "/hcd")
}
func (h *HCD) Classify(ctx context.Context, folder, approach, quote, client string) (domain.Unit, error) {
	if client == "tui" {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	if approach != "hcd" && approach != "speckit" {
		return domain.Unit{}, domain.ErrValidation
	}
	all, e := h.List(ctx)
	if e != nil {
		return domain.Unit{}, e
	}
	var unit domain.Unit
	for _, u := range all {
		if u.SourceDir == folder {
			unit = u
		}
	}
	if unit.ID == "" {
		return unit, domain.ErrNotFound
	}
	if unit.Approach != "unclassified" {
		return unit, domain.ErrNotAllowed
	}
	if !domain.ValidQuote(quote, "") {
		return unit, domain.ErrInvalidQuote
	}
	name := strings.TrimPrefix(folder, "hcd/features/")
	if name == folder {
		return unit, domain.ErrApproachMismatch
	}
	content, e := h.artifacts.Read(ctx, name, "definition.md")
	if e != nil {
		return unit, e
	}
	d, e := h.ParseDefinition(content)
	if e != nil {
		return unit, e
	}
	slug := strings.SplitN(name, "-", 2)
	if len(slug) != 2 {
		return unit, domain.ErrValidation
	}
	if approach == "hcd" && d.ID != unit.ID {
		return unit, domain.ErrValidation
	}
	unit.Slug = slug[1]
	unit.Title = d.Title
	unit.Kind = d.Kind
	unit.Implementer = d.Implementer
	unit.DefinitionRev = domain.ContentRevision(content)
	unit.Approach = approach
	unit.State = domain.StateDraft
	unit.StateRevision = 1
	unit.PlaybookRev = d.Playbook
	unit.ConstitutionRev = domain.ContentRevision([]byte(h.constitutionTemplate))
	e = h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		existing, e := repo.GetUnit(ctx, unit.ID)
		if e == nil {
			if existing.State == domain.StateCompleted || existing.State == domain.StateCancelled || existing.State == domain.StateSuperseded {
				return domain.ErrNotAllowed
			}
			unit.StateRevision = existing.StateRevision + 1
			if e = repo.UpdateUnit(ctx, unit, existing.StateRevision); e != nil {
				return e
			}
		} else if !errors.Is(e, domain.ErrNotFound) {
			return e
		} else if e = repo.CreateUnit(ctx, unit); e != nil {
			return e
		}
		approvalID, e := repo.NextApprovalID(ctx, unit.ID)
		if e != nil {
			return e
		}
		origin := "conversation"
		if client == "terminal" {
			origin = "terminal"
		}
		if e = repo.SaveApproval(ctx, domain.Approval{ID: approvalID, UnitID: unit.ID, Object: "classify:" + folder, Revision: unit.DefinitionRev, Origin: origin, Quote: quote, Client: client, ApprovedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			return e
		}
		decision := domain.HCDDecision{UnitID: unit.ID, Kind: "classification", Status: "accepted", Summary: fmt.Sprintf("Clasificar %s como %s", folder, approach), Revision: unit.DefinitionRev, Refs: []string{folder, approvalID}, Source: "classify", MemorySync: "pending"}
		return recordDecisionInRepo(ctx, repo, &decision)
	})
	if e == nil {
		h.syncDecisionsAfterCommit(ctx, unit.ID)
	}
	return unit, e
}
