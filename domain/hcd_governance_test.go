package domain

import (
	"testing"
	"time"
)

func TestHCDNormativePrecedence(t *testing.T) {
	levels := []NormativeLevel{NormativeSecurity, NormativeConstitution, NormativePlaybook, NormativeUnit, NormativeDefinition, NormativeDelivery}
	for i, upper := range levels {
		for j, lower := range levels {
			rule := NormativeRule{ID: "R-01", Level: upper, Origin: "playbook"}
			decision := NormativeDecision{Level: lower}
			if got := CanOverride(decision, rule, nil, time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)); got != (j <= i) {
				t.Fatalf("level %v override %v = %v", lower, upper, got)
			}
		}
	}
}
func TestHCDOverrideExceptions(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	valid := Exception{ID: "E-01", UnitID: "HCD-001", RuleID: "HCD-MEM-01", Scope: "close", Status: "approved", ApprovalID: "AP-01", Owner: "dev", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
	for _, tc := range []struct {
		name       string
		rule       NormativeRule
		decision   NormativeDecision
		exceptions []Exception
		want       bool
	}{
		{"missing", NormativeRule{ID: valid.RuleID, Level: NormativePlaybook, Origin: "playbook"}, NormativeDecision{Level: NormativeUnit, UnitID: valid.UnitID, Scope: "close"}, nil, false},
		{"approved current", NormativeRule{ID: valid.RuleID, Level: NormativePlaybook, Origin: "playbook"}, NormativeDecision{Level: NormativeUnit, UnitID: valid.UnitID, Scope: "close"}, []Exception{valid}, true},
		{"wrong scope", NormativeRule{ID: valid.RuleID, Level: NormativePlaybook, Origin: "playbook"}, NormativeDecision{Level: NormativeUnit, UnitID: valid.UnitID, Scope: "run"}, []Exception{valid}, false},
		{"wrong unit", NormativeRule{ID: valid.RuleID, Level: NormativePlaybook, Origin: "playbook"}, NormativeDecision{Level: NormativeUnit, UnitID: "HCD-002", Scope: "close"}, []Exception{valid}, false},
		{"non exceptionable", NormativeRule{ID: valid.RuleID, Level: NormativeSecurity, Origin: "playbook", NonExceptionable: true}, NormativeDecision{Level: NormativeUnit, UnitID: valid.UnitID, Scope: "close"}, []Exception{valid}, false},
		{"constitutional hcd row insufficient", NormativeRule{ID: valid.RuleID, Level: NormativeConstitution, Origin: "§10"}, NormativeDecision{Level: NormativeUnit, UnitID: valid.UnitID, Scope: "close"}, []Exception{valid}, false},
		{"constitutional section25", NormativeRule{ID: valid.RuleID, Level: NormativeConstitution, Origin: "§10"}, NormativeDecision{Level: NormativeUnit, ConstitutionException: "§25.1"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanOverride(tc.decision, tc.rule, tc.exceptions, now); got != tc.want {
				t.Fatalf("got=%t want=%t", got, tc.want)
			}
		})
	}
	for _, mutate := range []func(*Exception){func(e *Exception) { e.Status = "proposed" }, func(e *Exception) { e.ApprovalID = "" }, func(e *Exception) { e.ExpiresAt = now.Format(time.RFC3339) }, func(e *Exception) { e.RuleID = "other" }} {
		x := valid
		mutate(&x)
		if CanOverride(NormativeDecision{Level: NormativeUnit, UnitID: valid.UnitID, Scope: "close"}, NormativeRule{ID: valid.RuleID, Level: NormativePlaybook, Origin: "playbook"}, []Exception{x}, now) {
			t.Fatal("invalid exception authorized override")
		}
	}
}
