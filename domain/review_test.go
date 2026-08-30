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

// TestReviewTransitionToProtegeLosTerminales cubre el hueco que dejó la 027: la
// máquina de estados existía y era correcta, pero ningún llamador la usaba y los
// casos de uso asignaban Status a pelo. TransitionTo es el único punto de escritura.
func TestReviewTransitionToProtegeLosTerminales(t *testing.T) {
	viva := Review{Status: ReviewConsensusReady}
	if err := viva.TransitionTo(ReviewFixing); err != nil {
		t.Fatalf("transición válida rechazada: %v", err)
	}
	if viva.Status != ReviewFixing {
		t.Fatalf("TransitionTo no aplicó el estado: %s", viva.Status)
	}

	if err := viva.TransitionTo(ReviewApproved); err == nil {
		t.Error("fixing -> approved no es una transición permitida")
	}
	if viva.Status != ReviewFixing {
		t.Errorf("una transición rechazada no debe mover el estado: %s", viva.Status)
	}

	for _, terminal := range []ReviewStatus{ReviewApproved, ReviewEscalated, ReviewIncomplete} {
		cerrada := Review{Status: terminal}
		for _, destino := range []ReviewStatus{ReviewFixing, ReviewRejudging, ReviewConsensusReady, terminal} {
			if err := cerrada.TransitionTo(destino); err == nil {
				t.Errorf("el estado terminal %s aceptó pasar a %s", terminal, destino)
			}
		}
		if cerrada.Status != terminal {
			t.Errorf("el estado terminal %s cambió a %s", terminal, cerrada.Status)
		}
	}

	invalida := Review{Status: ReviewConsensusReady}
	if err := invalida.TransitionTo(ReviewStatus("inventado")); err == nil {
		t.Error("un estado desconocido no puede aceptarse como destino")
	}
}
