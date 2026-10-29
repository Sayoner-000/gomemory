package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"strings"
	"time"
)

type HCDPlaybookView struct {
	Doc, FromRevision, ToRevision, FromContent, Content string
	Checks                                              []domain.PendingCheck
	Warnings                                            []string
	Urgent                                              bool
}

func (h *HCD) PlaybookShow(ctx context.Context, id, doc string) (HCDPlaybookView, error) {
	if doc == "" {
		doc = "playbook"
	}
	u, err := h.repo.GetUnit(ctx, id)
	if err != nil {
		return HCDPlaybookView{}, err
	}
	topic, fallback, rev := domain.TopicHCDPlaybook, h.playbookTemplate, u.PlaybookRev
	if doc == "constitution" {
		topic, fallback, rev = domain.TopicConstitution, h.constitutionTemplate, u.ConstitutionRev
	} else if doc != "playbook" {
		return HCDPlaybookView{}, domain.ErrValidation
	}
	current, err := ResolvePinnedDoc(h.topics, h.project, topic, fallback)
	if err != nil {
		return HCDPlaybookView{}, err
	}
	old, err := h.repo.GetNormativeRevision(ctx, doc, rev)
	if err != nil {
		return HCDPlaybookView{}, err
	}
	r := HCDPlaybookView{Doc: doc, FromRevision: rev, ToRevision: domain.ContentRevision([]byte(current.Content)), FromContent: old.Content, Content: current.Content, Urgent: strings.Contains(current.Content, "Urgente: seguridad"), Warnings: []string{"Revisar manualmente posibles conflictos de sentido con la Constitución"}}
	if doc == "playbook" {
		constitution, err := ResolvePinnedDoc(h.topics, h.project, domain.TopicConstitution, h.constitutionTemplate)
		if err != nil {
			return r, err
		}
		r.Checks = domain.ValidatePlaybook(current.Content, constitution.Content)
	}
	return r, nil
}
func (h *HCD) PlaybookValidate(ctx context.Context, id string) (HCDPlaybookView, error) {
	return h.PlaybookShow(ctx, id, "playbook")
}

type HCDAdoptInput struct {
	UnitID, Doc, Revision, Quote, Client, Reason, Actor, OperationID string
	ExpectedRevision                                                 int64
}

func (h *HCD) PlaybookAdopt(ctx context.Context, in HCDAdoptInput) (domain.Unit, error) {
	if in.Client == "tui" {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	if !domain.ValidQuote(in.Quote, in.Revision) {
		return domain.Unit{}, domain.ErrInvalidQuote
	}
	if strings.TrimSpace(in.Reason) == "" || strings.TrimSpace(in.Actor) == "" {
		return domain.Unit{}, domain.ErrValidation
	}
	payload, _ := json.Marshal(in)
	encoded, err := h.mutate(ctx, in.UnitID, in.ExpectedRevision, in.OperationID, hcdPayloadHash(payload), "playbook-adopt", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		// Resolve and compare inside the transaction: approval targets the shown revision.
		topic, fallback, from := domain.TopicHCDPlaybook, h.playbookTemplate, u.PlaybookRev
		if in.Doc == "constitution" {
			topic, fallback, from = domain.TopicConstitution, h.constitutionTemplate, u.ConstitutionRev
		} else if in.Doc != "playbook" {
			return u, nil, domain.ErrValidation
		}
		current, e := ResolvePinnedDoc(h.topics, h.project, topic, fallback)
		if e != nil {
			return u, nil, e
		}
		if domain.ContentRevision([]byte(current.Content)) != in.Revision {
			return u, nil, domain.ErrRevisionConflict
		}
		if from == in.Revision {
			return u, nil, domain.ErrAlreadyExists
		}
		if in.Doc == "playbook" {
			constitution, e := ResolvePinnedDoc(h.topics, h.project, domain.TopicConstitution, h.constitutionTemplate)
			if e != nil {
				return u, nil, e
			}
			if checks := domain.ValidatePlaybook(current.Content, constitution.Content); len(checks) > 0 {
				return u, nil, &domain.HCDValidationError{Checks: checks}
			}
		}
		old, e := repo.GetNormativeRevision(ctx, in.Doc, from)
		if e != nil {
			return u, nil, e
		}
		impact, _ := json.Marshal(map[string]string{"from": old.Content, "to": current.Content})
		approval, e := h.governanceApproval(ctx, repo, u, "adopt:"+in.Doc, in.Revision, in.Quote, in.Client)
		if e != nil {
			return u, nil, e
		}
		id, e := repo.NextGovernanceID(ctx, "adoption")
		if e != nil {
			return u, nil, e
		}
		if e = repo.SaveNormativeRevision(ctx, in.Doc, in.Revision, current.Content, docSource(current.FromMemory), in.Reason, strings.Contains(current.Content, "Urgente: seguridad")); e != nil {
			return u, nil, e
		}
		if e = repo.SaveAdoption(ctx, domain.PlaybookAdoption{ID: id, UnitID: u.ID, Doc: in.Doc, FromRev: from, ToRev: in.Revision, ImpactJSON: string(impact), Reason: in.Reason, Actor: in.Actor, ApprovalID: approval, AdoptedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			return u, nil, e
		}
		if in.Doc == "playbook" {
			u.PlaybookRev = in.Revision
		} else {
			u.ConstitutionRev = in.Revision
		}
		if u.State == domain.StateVerification {
			u.State = domain.StateInProgress
		}
		d := domain.HCDDecision{UnitID: u.ID, Kind: "adopt", Status: "accepted", Revision: u.DefinitionRev, Summary: "Adoptar " + in.Doc + ": " + domain.ProtectText(in.Reason), Refs: []string{id, approval}, Source: "playbook-adopt", MemorySync: "pending"}
		if e = recordDecisionInRepo(ctx, repo, &d); e != nil {
			return u, nil, e
		}
		return u, u, nil
	})
	var u domain.Unit
	if err == nil {
		err = json.Unmarshal([]byte(encoded), &u)
	}
	return u, err
}
func (h *HCD) governanceApproval(ctx context.Context, repo ports.HCDRepository, u domain.Unit, object, revision, quote, client string) (string, error) {
	id, err := repo.NextApprovalID(ctx, u.ID)
	if err != nil {
		return "", err
	}
	origin := "conversation"
	if client == "terminal" {
		origin = "terminal"
	}
	err = repo.SaveApproval(ctx, domain.Approval{ID: id, UnitID: u.ID, Object: object, Revision: revision, Quote: domain.ProtectText(quote), Client: client, Origin: origin, ApprovedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)})
	return id, err
}

type HCDExceptionInput struct {
	UnitID, Quote, Client, OperationID string
	ExpectedRevision                   int64
	Exception                          domain.Exception
}

func (h *HCD) Exception(ctx context.Context, in HCDExceptionInput) (domain.Exception, error) {
	if in.Client == "tui" {
		return domain.Exception{}, domain.ErrNotAllowed
	}
	payload, _ := json.Marshal(in)
	encoded, err := h.mutate(ctx, in.UnitID, in.ExpectedRevision, in.OperationID, hcdPayloadHash(payload), "exception", func(repo ports.HCDRepository, u domain.Unit) (domain.Unit, any, error) {
		if !domain.ValidQuote(in.Quote, u.DefinitionRev) {
			return u, nil, domain.ErrInvalidQuote
		}
		x := in.Exception
		x.UnitID = u.ID
		for _, value := range []string{x.RuleID, x.Scope, x.Reason, x.Alternatives, x.Risks, x.Compensating, x.Owner} {
			if strings.TrimSpace(value) == "" {
				return u, nil, domain.ErrValidation
			}
		}
		if x.ExpiresAt == "" && strings.TrimSpace(x.ReviewCondition) == "" {
			return u, nil, domain.ErrValidation
		}
		if x.ExpiresAt != "" {
			expires, e := time.Parse(time.RFC3339, x.ExpiresAt)
			if e != nil || !h.clock.Now().Before(expires) {
				return u, nil, domain.ErrValidation
			}
		}
		p, e := repo.GetNormativeRevision(ctx, "playbook", u.PlaybookRev)
		if e != nil {
			return u, nil, e
		}
		var rule domain.NormativeRule
		for _, r := range domain.PlaybookRules(p.Content) {
			if r.ID == x.RuleID {
				rule = r
				break
			}
		}
		if rule.ID == "" {
			return u, nil, domain.ErrNotFound
		}
		if rule.Origin != "playbook" || rule.NonExceptionable || strings.Contains(strings.ToLower(rule.ExceptionPolicy), "no admite") || rule.ExceptionPolicy == "ninguna." {
			return u, nil, domain.ErrBlocked
		}
		x.ID, e = repo.NextGovernanceID(ctx, "exception")
		if e != nil {
			return u, nil, e
		}
		x.ApprovalID, e = h.governanceApproval(ctx, repo, u, "exception:"+x.ID, u.DefinitionRev, in.Quote, in.Client)
		if e != nil {
			return u, nil, e
		}
		x.Status = "approved"
		if !domain.CanOverride(domain.NormativeDecision{Level: domain.NormativeUnit, UnitID: u.ID, Scope: x.Scope}, rule, []domain.Exception{x}, h.clock.Now()) {
			return u, nil, domain.ErrBlocked
		}
		x.Reason = domain.ProtectText(x.Reason)
		x.Risks = domain.ProtectText(x.Risks)
		x.Compensating = domain.ProtectText(x.Compensating)
		x.Alternatives = domain.ProtectText(x.Alternatives)
		x.Owner = domain.ProtectText(x.Owner)
		x.Scope = domain.ProtectText(x.Scope)
		x.ReviewCondition = domain.ProtectText(x.ReviewCondition)
		if e = repo.SaveException(ctx, x); e != nil {
			return u, nil, e
		}
		d := domain.HCDDecision{UnitID: u.ID, Kind: "exception", Status: "accepted", Revision: u.DefinitionRev, Summary: x.Reason, Refs: []string{x.ID, x.ApprovalID, x.RuleID}, Source: "exception", MemorySync: "pending"}
		if e = recordDecisionInRepo(ctx, repo, &d); e != nil {
			return u, nil, e
		}
		return u, x, nil
	})
	var x domain.Exception
	if err == nil {
		err = json.Unmarshal([]byte(encoded), &x)
	}
	return x, err
}

// Explicit override references are durable decision references, not inferred
// interpretations of prose. Semantic conflicts still require human review.
// Format: override:<rule>:<scope> or override:<§N>:<scope>:<§25.N>.
func validateDecisionOverrides(ctx context.Context, repo ports.HCDRepository, d domain.HCDDecision, now time.Time) error {
	var overrides []string
	for _, ref := range d.Refs {
		if strings.HasPrefix(ref, "override:") {
			overrides = append(overrides, ref)
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	if now.IsZero() {
		return domain.ErrNotAllowed
	}
	u, e := repo.GetUnit(ctx, d.UnitID)
	if e != nil {
		return e
	}
	p, e := repo.GetNormativeRevision(ctx, "playbook", u.PlaybookRev)
	if e != nil {
		return e
	}
	x, e := repo.ListExceptions(ctx, u.ID)
	if e != nil {
		return e
	}
	for _, ref := range overrides {
		parts := strings.Split(ref, ":")
		if len(parts) < 3 || len(parts) > 4 || parts[2] == "" {
			return domain.ErrValidation
		}
		var rule domain.NormativeRule
		for _, r := range domain.PlaybookRules(p.Content) {
			if r.ID == parts[1] {
				rule = r
				break
			}
		}
		decision := domain.NormativeDecision{UnitID: u.ID, Level: domain.NormativeUnit, Scope: parts[2]}
		if strings.HasPrefix(parts[1], "§") {
			rule = domain.NormativeRule{ID: parts[1], Origin: parts[1], Level: domain.NormativeConstitution}
			if len(parts) == 4 {
				constitution, e := repo.GetNormativeRevision(ctx, "constitution", u.ConstitutionRev)
				if e != nil {
					return e
				}
				// Match an actual exception heading and its affected rule in the pinned document.
				heading := "### " + strings.TrimPrefix(parts[3], "§") + " "
				_, section, found := strings.Cut(constitution.Content, heading)
				if found {
					section = strings.SplitN(section, "\n### ", 2)[0]
					if strings.Contains(section, parts[1]) {
						decision.ConstitutionException = parts[3]
					}
				}
			}
		}
		if rule.ID == "" {
			return fmt.Errorf("regla inexistente: %w", domain.ErrNotFound)
		}
		if !domain.CanOverride(decision, rule, x, now) {
			return domain.ErrBlocked
		}
	}
	return nil
}

func (h *HCD) memoryException(ctx context.Context, repo ports.HCDRepository, u domain.Unit) (bool, error) {
	x, err := repo.ListExceptions(ctx, u.ID)
	if err != nil {
		return false, err
	}
	return domain.CanOverride(domain.NormativeDecision{Level: domain.NormativeUnit, UnitID: u.ID, Scope: "close"}, domain.NormativeRule{ID: "HCD-MEM-01", Origin: "playbook", Level: domain.NormativePlaybook}, x, h.clock.Now()), nil
}
