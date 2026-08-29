package domain

import "testing"

func TestNewTargetRejectsEmptyDigest(t *testing.T) {
	if _, err := NewTarget(TargetDiff, "working-tree", "", []string{"domain/"}); err == nil {
		t.Fatal("NewTarget() debe rechazar un digest vacío")
	}
}

func TestTargetDigestIsFrozen(t *testing.T) {
	target, err := NewTarget(TargetDiff, "working-tree", "sha256:original", []string{"domain/"})
	if err != nil {
		t.Fatal(err)
	}
	if got := target.Digest(); got != "sha256:original" {
		t.Fatalf("Digest() = %q, se esperaba el digest congelado", got)
	}
	if err := target.ValidateDigest("sha256:changed"); err == nil {
		t.Fatal("ValidateDigest() debe rechazar un target modificado")
	}
	if got := target.Digest(); got != "sha256:original" {
		t.Fatalf("el digest congelado cambió a %q", got)
	}
}

func TestReviewStatusTransitions(t *testing.T) {
	valid := map[ReviewStatus][]ReviewStatus{
		ReviewFrozen:            {ReviewAwaitingReviewers},
		ReviewAwaitingReviewers: {ReviewConsensusReady, ReviewIncomplete},
		ReviewConsensusReady:    {ReviewFixing, ReviewApproved, ReviewEscalated, ReviewIncomplete},
		ReviewFixing:            {ReviewRejudging, ReviewEscalated, ReviewIncomplete},
		ReviewRejudging:         {ReviewFixing, ReviewApproved, ReviewEscalated, ReviewIncomplete},
	}
	terminal := []ReviewStatus{ReviewApproved, ReviewEscalated, ReviewIncomplete}

	for from, destinations := range valid {
		if !from.Valid() {
			t.Fatalf("estado válido no reconocido: %q", from)
		}
		for _, to := range destinations {
			if !from.CanTransitionTo(to) {
				t.Errorf("transición válida rechazada: %s -> %s", from, to)
			}
		}
	}
	for _, status := range terminal {
		if !status.Terminal() {
			t.Errorf("%q debe ser terminal", status)
		}
		if status.CanTransitionTo(ReviewFrozen) {
			t.Errorf("el estado terminal %q no debe permitir transiciones", status)
		}
	}
	if ReviewStatus("invented").Valid() {
		t.Fatal("un estado desconocido no puede ser válido")
	}
}
