package domain

import "testing"

func TestCoverageReport_ProblemsCountsOnlyBrokenStates(t *testing.T) {
	r := CoverageReport{Channels: []ActivationChannel{
		{State: StateOK},
		{State: StateOutdated},
		{State: StateDuplicated},
		{State: StateMissing},
		{State: StateNotApplicable},
	}}
	if got := r.Problems(); got != 3 {
		t.Errorf("Problems() = %d, se esperaban 3 (outdated+duplicated+missing)", got)
	}
}

func TestCoverageReport_NotApplicableAndOKAreNotProblems(t *testing.T) {
	r := CoverageReport{Channels: []ActivationChannel{
		{State: StateOK},
		{State: StateNotApplicable},
	}}
	if got := r.Problems(); got != 0 {
		t.Errorf("Problems() = %d, se esperaba 0", got)
	}
}

func TestSortChannels_OrdenDeterminista(t *testing.T) {
	channels := []ActivationChannel{
		{Arm: ArmCodegraph, Agent: "claude", Scope: ScopeProject, Kind: KindInstructions},
		{Arm: ArmGomemory, Agent: "opencode", Scope: ScopeProject, Kind: KindTurnReminder},
		{Arm: ArmGomemory, Agent: "claude", Scope: ScopeUser, Kind: KindPlanGuard},
		{Arm: ArmGomemory, Agent: "claude", Scope: ScopeProject, Kind: KindPlanEntry},
	}
	SortChannels(channels)

	for i := 1; i < len(channels); i++ {
		a, b := channels[i-1], channels[i]
		if a.Arm > b.Arm {
			t.Fatalf("orden roto en Arm: %v antes de %v", a, b)
		}
	}

	// Repetir el orden dos veces debe dar exactamente el mismo resultado.
	again := append([]ActivationChannel{}, channels...)
	SortChannels(again)
	for i := range channels {
		if channels[i] != again[i] {
			t.Fatalf("el orden no es estable entre ejecuciones en la posición %d", i)
		}
	}
}

func TestDuplicatedIsDistinctFromOK(t *testing.T) {
	if StateDuplicated == StateOK {
		t.Fatal("duplicated debe ser un estado propio, no un caso de ok")
	}
}
