package domain

import "testing"

func TestStageFor(t *testing.T) {
	cases := []struct {
		name string
		unit Unit
		want Stage
	}{
		{"sin unidad", Unit{}, HCDStageContext},
		{"intención incompleta", Unit{State: StateDraft}, HCDStageIntent},
		{"definición", Unit{State: StateDraft, IntentComplete: true}, HCDStageDefinition},
		{"aclaración", Unit{State: StateDraft, IntentComplete: true, OpenClarification: true}, HCDStageClarification},
		{"aprobación", Unit{State: StateDraft, IntentComplete: true, ValidationPassed: true}, HCDStageApproval},
		{"slicing", Unit{State: StateReady}, HCDStageSlicing},
		{"descomposición", Unit{State: StateReady, ReadySlice: true, UnadmittedTask: true}, HCDStageDecomposition},
		{"ejecución", Unit{State: StateInProgress, AdmittedTask: true}, HCDStageExecution},
		{"verificación", Unit{State: StateVerification}, HCDStageVerification},
		{"continuidad", Unit{State: StateInProgress, AllSlicesVerified: true, WorkRemaining: true}, HCDStageContinuity},
		{"cierre", Unit{State: StateCompleted}, HCDStageClosure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StageFor(tc.unit); got != tc.want {
				t.Fatalf("StageFor(%+v) = %q, want %q", tc.unit, got, tc.want)
			}
		})
	}
}

func TestUnitTerminalStatesAreImmutable(t *testing.T) {
	for _, state := range []State{StateCompleted, StateCancelled, StateSuperseded} {
		for _, action := range AllowedActions(Unit{State: state}) {
			if action != "show" && action != "resume" {
				t.Errorf("terminal state %q allows mutation %q", state, action)
			}
		}
	}
}

func TestHCDEntityTransitions(t *testing.T) {
	cases := []struct {
		entity, from, to string
		valid            bool
	}{
		{"story", "draft", "ready", true}, {"story", "satisfied", "draft", false},
		{"criterion", "defined", "covered", true}, {"criterion", "verified", "failed", false},
		{"slice", "proposed", "ready", true}, {"slice", "verified", "in_progress", false},
		{"task", "pending", "in_progress", true}, {"task", "verified", "pending", false},
		{"exception", "proposed", "approved", true}, {"exception", "expired", "approved", false},
		{"migration", "proposed", "active", true}, {"migration", "active", "proposed", false},
	}
	for _, tc := range cases {
		if got := ValidHCDTransition(tc.entity, tc.from, tc.to); got != tc.valid {
			t.Errorf("ValidHCDTransition(%q, %q, %q) = %v, want %v", tc.entity, tc.from, tc.to, got, tc.valid)
		}
	}
}

func TestNextActionAfterVerificationPass(t *testing.T) {
	pass := NextAction(Unit{State: StateVerification}, nil)
	if pass.Skill != "hcd-close" {
		t.Fatalf("after PASS next=%+v, want hcd-close", pass)
	}
	pending := NextAction(Unit{State: StateInProgress, ReadySlice: true}, nil)
	if pending.Skill != "hcd-verify" {
		t.Fatalf("work reported next=%+v, want hcd-verify", pending)
	}
	if done := NextAction(Unit{State: StateCompleted}, nil); done.Skill != "hcd" || done.Command != "list" {
		t.Fatalf("completed next=%+v", done)
	}
}

func TestConstitutionAdmitsHCD(t *testing.T) {
	for content, want := range map[string]bool{
		"":                    true,
		"specs/{NNN}-{name}/": false,
		"specs/{NNN}-{name}/ hcd/features/{NNN}-{name}/": true,
	} {
		if got := ConstitutionAdmitsHCD(content); got != want {
			t.Errorf("ConstitutionAdmitsHCD(%q)=%v, want %v", content, got, want)
		}
	}
}
