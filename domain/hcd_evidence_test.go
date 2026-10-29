package domain

import (
	"testing"
	"time"
)

func evidenceFixture() (Evidence, HCDVerificationContext) {
	u := Unit{ID: "HCD-001", Approach: "hcd", DefinitionRev: "def", ApprovedRev: "def", DeliveryRev: "delivery", PlaybookRev: "playbook", ConstitutionRev: "constitution"}
	v := Evidence{ID: "EV-01", Result: "pass", Covers: []string{"C-01", "T-001"}, SubjectRef: []string{"tree:abc"}, DefinitionRev: u.DefinitionRev, DeliveryRev: u.DeliveryRev, PlaybookRev: u.PlaybookRev, ConstitutionRev: u.ConstitutionRev}
	return v, HCDVerificationContext{Unit: u, Subjects: map[string]bool{"tree:abc": true}}
}
func TestHCDEvidenceFreshness(t *testing.T) {
	v, c := evidenceFixture()
	if EvidenceFreshness(v, c) != "fresh" {
		t.Fatal("fresh evidence rejected")
	}
	for _, field := range []string{"definition", "delivery", "playbook", "constitution", "subject"} {
		t.Run(field, func(t *testing.T) {
			v, c := evidenceFixture()
			switch field {
			case "definition":
				v.DefinitionRev = "old"
			case "delivery":
				v.DeliveryRev = "old"
			case "playbook":
				v.PlaybookRev = "old"
			case "constitution":
				v.ConstitutionRev = "old"
			case "subject":
				c.Subjects["tree:abc"] = false
			}
			if EvidenceFreshness(v, c) != "stale" {
				t.Fatal("stale evidence accepted")
			}
		})
	}
	v.InspectRef = []string{"https://inaccessible.invalid"}
	if EvidenceFreshness(v, c) != "stale" {
		t.Fatal("inaccessible external evidence accepted")
	}
}
func TestHCDCriterionAndSliceStatus(t *testing.T) {
	v, c := evidenceFixture()
	criterion := Criterion{ID: "C-01", Verification: VerificationAutomated}
	if CriterionStatus(criterion, nil, c, nil) != CriterionCovered {
		t.Fatal("strategy alone became verified")
	}
	if CriterionStatus(criterion, []Evidence{v}, c, nil) != CriterionVerified {
		t.Fatal("pass not verified")
	}
	fail := v
	fail.Result = "fail"
	if CriterionStatus(criterion, []Evidence{v, fail}, c, nil) != CriterionFailed {
		t.Fatal("fail hidden by pass")
	}
	criterion.State = CriterionNotApplicable
	if CriterionStatus(criterion, nil, c, nil) == CriterionNotApplicable {
		t.Fatal("NA lacks decision")
	}
	decisions := []HCDDecision{{Status: "accepted", Kind: "not_applicable", Refs: []string{"C-01"}}}
	if CriterionStatus(criterion, nil, c, decisions) != CriterionNotApplicable {
		t.Fatal("NA with decision rejected")
	}
	slice := Slice{ID: "VS-01", Criteria: []string{"C-01"}}
	tasks := []Task{{ID: "T-001", SliceID: slice.ID}}
	if SliceStatus(slice, tasks, []Criterion{{ID: "C-01", Verification: VerificationAutomated}}, []Evidence{v}, c, nil) != "verified" {
		t.Fatal("slice not derived verified")
	}
}
func TestHCDClosureBlockers(t *testing.T) {
	v, c := evidenceFixture()
	w := HCDWork{Unit: c.Unit, Definition: Definition{Criteria: []Criterion{{ID: "C-01", Verification: VerificationAutomated}}}, Slices: []Slice{{ID: "VS-01", Criteria: []string{"C-01"}}}, Tasks: []Task{{ID: "T-001", SliceID: "VS-01"}}}
	now := time.Now()
	for _, tc := range []struct {
		name       string
		blocks     []Block
		exceptions []Exception
		evidence   []Evidence
	}{{"critical", []Block{{Critical: true, Status: "open"}}, nil, []Evidence{v}}, {"fail", nil, nil, []Evidence{{Result: "fail", Covers: v.Covers, SubjectRef: v.SubjectRef, DefinitionRev: v.DefinitionRev, DeliveryRev: v.DeliveryRev, PlaybookRev: v.PlaybookRev, ConstitutionRev: v.ConstitutionRev}}}, {"expired", nil, []Exception{{ID: "E-01", Status: "approved", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)}}, []Evidence{v}}} {
		t.Run(tc.name, func(t *testing.T) {
			r := ClosureVerdict(w, tc.evidence, c, tc.blocks, tc.exceptions, now)
			if r.CanClose || len(r.Causes) == 0 {
				t.Fatalf("verdict=%+v", r)
			}
		})
	}
	if r := ClosureVerdict(w, []Evidence{v}, c, nil, nil, now); !r.CanClose {
		t.Fatalf("valid closure=%+v", r)
	}
}
