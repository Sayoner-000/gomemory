package domain

import (
	"strings"
	"time"
)

type HCDVerificationContext struct {
	Unit                  Unit
	Subjects, Inspections map[string]bool
}
type Exception struct{ ID, UnitID, RuleID, Scope, Reason, Alternatives, Risks, Compensating, Owner, ApprovalID, Status, ExpiresAt, ReviewCondition string }

func EvidenceFreshness(e Evidence, c HCDVerificationContext) string {
	u := c.Unit
	if e.DefinitionRev != u.DefinitionRev || e.DeliveryRev != u.DeliveryRev || e.PlaybookRev != u.PlaybookRev || e.ConstitutionRev != u.ConstitutionRev || len(e.SubjectRef) == 0 {
		return "stale"
	}
	for _, ref := range e.SubjectRef {
		if !c.Subjects[ref] {
			return "stale"
		}
	}
	for _, ref := range e.InspectRef {
		if (strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://")) && !c.Inspections[ref] {
			return "stale"
		}
	}
	return "fresh"
}
func evidenceStatus(ref string, ev []Evidence, c HCDVerificationContext) string {
	pass := false
	for _, v := range ev {
		if EvidenceFreshness(v, c) != "fresh" {
			continue
		}
		covers := false
		for _, r := range v.Covers {
			covers = covers || r == ref
		}
		if !covers {
			continue
		}
		if v.Result == "fail" {
			return "failed"
		}
		pass = pass || v.Result == "pass"
	}
	if pass {
		return "verified"
	}
	return "covered"
}
func CriterionStatus(criterion Criterion, ev []Evidence, c HCDVerificationContext, decisions []HCDDecision) CriterionState {
	status := evidenceStatus(criterion.ID, ev, c)
	if status == "failed" || status == "verified" {
		return CriterionState(status)
	}
	{
		for _, d := range decisions {
			if d.Status == "accepted" && d.Kind == "not_applicable" {
				for _, ref := range d.Refs {
					if ref == criterion.ID {
						return CriterionNotApplicable
					}
				}
			}
		}
	}
	if criterion.Verification != "" {
		return CriterionCovered
	}
	return CriterionDefined
}
func SliceStatus(slice Slice, tasks []Task, criteria []Criterion, ev []Evidence, c HCDVerificationContext, decisions []HCDDecision) SliceState {
	count := 0
	for _, t := range tasks {
		if t.SliceID == slice.ID {
			count++
			if evidenceStatus(t.ID, ev, c) != "verified" {
				return "proposed"
			}
		}
	}
	if count == 0 || len(slice.Criteria) == 0 {
		return "proposed"
	}
	for _, id := range slice.Criteria {
		found := false
		for _, criterion := range criteria {
			if criterion.ID == id {
				state := CriterionStatus(criterion, ev, c, decisions)
				found = state == CriterionVerified || state == CriterionNotApplicable
			}
		}
		if !found {
			return "implemented"
		}
	}
	return "verified"
}

type HCDClosureVerdict struct {
	CanClose bool
	Causes   []PendingCheck
}

func ClosureVerdict(w HCDWork, ev []Evidence, c HCDVerificationContext, blocks []Block, exceptions []Exception, now time.Time) HCDClosureVerdict {
	r := HCDClosureVerdict{CanClose: true}
	add := func(code, ref, msg string) {
		r.CanClose = false
		r.Causes = append(r.Causes, PendingCheck{Code: code, Field: ref, Message: msg})
	}
	if w.Unit.Approach != "hcd" {
		add("APPROACH", w.Unit.ID, "la unidad no usa HCD")
	}
	if len(w.Slices) == 0 {
		add("SLICE-MISSING", w.Unit.ID, "no hay slices")
	}
	for _, s := range w.Slices {
		if SliceStatus(s, w.Tasks, w.Definition.Criteria, ev, c, w.Decisions) != "verified" {
			add("SLICE-UNVERIFIED", s.ID, "slice sin verificar")
		}
	}
	for _, criterion := range w.Definition.Criteria {
		status := CriterionStatus(criterion, ev, c, w.Decisions)
		if status != CriterionVerified && status != CriterionNotApplicable {
			add("CRITERION-UNVERIFIED", criterion.ID, "criterio sin verificar")
		}
	}
	for _, b := range blocks {
		if b.Status == "open" && b.Critical {
			add("BLOCK-CRITICAL", b.ID, b.Reason)
		}
	}
	for _, x := range exceptions {
		if x.Status != "approved" || x.ExpiresAt == "" {
			continue
		}
		expires, e := time.Parse(time.RFC3339, x.ExpiresAt)
		if e != nil || !now.Before(expires) {
			add("EXCEPTION-EXPIRED", x.ID, "excepción vencida")
		}
	}
	return r
}
