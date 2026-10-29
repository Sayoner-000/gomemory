package domain

import (
	"regexp"
	"strings"
	"time"
)

// Larger values have precedence over smaller values.
type NormativeLevel int

const (
	NormativeDelivery NormativeLevel = iota + 1
	NormativeDefinition
	NormativeUnit
	NormativePlaybook
	NormativeConstitution
	NormativeSecurity
)

type NormativeRule struct {
	ID, Origin                                                     string
	Description, Scope, Check, Evidence, ExceptionPolicy, Citation string
	Level                                                          NormativeLevel
	NonExceptionable                                               bool
}

func PlaybookRules(content string) []NormativeRule {
	var rules []NormativeRule
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "### HCD-") {
			id := strings.SplitN(strings.TrimPrefix(line, "### "), ".", 2)[0]
			rules = append(rules, NormativeRule{ID: id, Origin: "playbook", Level: NormativePlaybook})
			continue
		}
		if len(rules) == 0 {
			continue
		}
		r := &rules[len(rules)-1]
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Descripción":
			r.Description = value
		case "Ámbito":
			r.Scope = value
		case "Comprobación":
			r.Check = value
		case "Evidencia":
			r.Evidence = value
		case "Excepción":
			r.ExceptionPolicy = value
		case "Origen":
			r.Citation = value
			if strings.HasPrefix(value, "§") {
				r.Origin = value
				r.Level = NormativeConstitution
			}
		case "non_exceptionable":
			r.NonExceptionable = strings.EqualFold(value, "true")
		}
	}
	return rules
}

func ValidatePlaybook(content, constitution string) []PendingCheck {
	var checks []PendingCheck
	add := func(code, field, msg string) {
		checks = append(checks, PendingCheck{Code: code, Field: field, Message: msg})
	}
	seen := map[string]bool{}
	clean, solid := false, false
	refs := regexp.MustCompile(`§([0-9]+(?:\.[0-9]+)*)`)
	for _, r := range PlaybookRules(content) {
		if seen[r.ID] {
			add("PLAYBOOK-ID", r.ID, "identificador duplicado")
		}
		seen[r.ID] = true
		clean = clean || strings.HasPrefix(r.ID, "HCD-CLEAN-")
		solid = solid || strings.HasPrefix(r.ID, "HCD-SOLID-")
		if r.Description == "" || r.Scope == "" || r.Evidence == "" || r.ExceptionPolicy == "" || (r.Check != "structural" && r.Check != "review") {
			add("PLAYBOOK-FIELDS", r.ID, "completar los campos y la comprobación de la regla")
		}
		for _, ref := range refs.FindAllStringSubmatch(r.Description+" "+r.ExceptionPolicy+" "+r.Citation, -1) {
			if !regexp.MustCompile(`(?m)^#{1,6}\s+` + regexp.QuoteMeta(ref[1]) + `(?:[.\s]|$)`).MatchString(constitution) {
				add("CONSTITUTION-SECTION", r.ID, "sección constitucional inexistente: "+ref[0])
			}
		}
		if refs.MatchString(r.Description) && regexp.MustCompile(`(?i)(relaja|sustituye|exceptúa|relajar|sustituir|exceptuar)`).MatchString(r.Description) {
			add("CONSTITUTION-OVERRIDE", r.ID, "el Playbook no puede sustituir reglas constitucionales")
		}
	}
	if !clean || !solid {
		add("PLAYBOOK-ARCHITECTURE", "playbook", "incluir reglas HCD-CLEAN y HCD-SOLID con comprobación")
	}
	for _, line := range strings.Split(constitution, "\n") {
		line = strings.TrimSpace(line)
		if len(strings.Fields(line)) >= 12 && strings.Contains(content, line) {
			add("CONSTITUTION-COPY", "playbook", "referenciar la Constitución sin copiar sus bloques")
			break
		}
	}
	return checks
}

type NormativeRevision struct {
	Doc, Revision, Content, Source, ChangeKind string
	Urgent                                     bool
}
type PlaybookAdoption struct{ ID, UnitID, Doc, FromRev, ToRev, ImpactJSON, Reason, Actor, ApprovalID, AdoptedAt string }
type NormativeDecision struct {
	Level         NormativeLevel
	UnitID, Scope string
	// A caller must resolve this reference against the pinned Constitution's §25.
	ConstitutionException string
}

func CanOverride(decision NormativeDecision, rule NormativeRule, exceptions []Exception, now time.Time) bool {
	if decision.Level < NormativeDelivery || decision.Level > NormativeSecurity || rule.Level < NormativeDelivery || rule.Level > NormativeSecurity {
		return false
	}
	if decision.Level >= rule.Level {
		return true
	}
	if rule.NonExceptionable {
		return false
	}
	if strings.HasPrefix(rule.Origin, "§") || rule.Level == NormativeConstitution {
		return strings.HasPrefix(decision.ConstitutionException, "§25.")
	}
	if rule.Origin != "playbook" {
		return false
	}
	for _, x := range exceptions {
		if x.UnitID != decision.UnitID || x.RuleID != rule.ID || x.Scope != decision.Scope || x.Status != "approved" || x.ApprovalID == "" {
			continue
		}
		if x.ExpiresAt != "" {
			expires, err := time.Parse(time.RFC3339, x.ExpiresAt)
			if err != nil || !now.Before(expires) {
				continue
			}
		} else if strings.TrimSpace(x.ReviewCondition) == "" {
			continue
		}
		return true
	}
	return false
}
