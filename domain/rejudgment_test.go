package domain

import "testing"

// TestAggregateReJudgment recorre la tabla completa de FR-014. Con un único campo
// agregado en consensus_findings esta distinción era inexpresable: un revisor que
// decía RESOLVED bastaba para dar por resuelto el hallazgo.
func TestAggregateReJudgment(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  []ReJudgment
		esperado ReJudgmentState
	}{
		{"sin re-juicios", nil, ReJudgmentUnresolved},
		{
			"un solo RESOLVED no basta",
			[]ReJudgment{{Reviewer: ReviewerA, State: ReJudgmentResolved}},
			ReJudgmentUnresolved,
		},
		{
			"unanimidad RESOLVED",
			[]ReJudgment{
				{Reviewer: ReviewerA, State: ReJudgmentResolved},
				{Reviewer: ReviewerB, State: ReJudgmentResolved},
			},
			ReJudgmentResolved,
		},
		{
			"un REGRESSED manda sobre el RESOLVED del otro",
			[]ReJudgment{
				{Reviewer: ReviewerA, State: ReJudgmentResolved},
				{Reviewer: ReviewerB, State: ReJudgmentRegressed},
			},
			ReJudgmentRegressed,
		},
		{
			"un REGRESSED solo también manda",
			[]ReJudgment{{Reviewer: ReviewerA, State: ReJudgmentRegressed}},
			ReJudgmentRegressed,
		},
		{
			"discrepancia sin regresión queda sin resolver",
			[]ReJudgment{
				{Reviewer: ReviewerA, State: ReJudgmentResolved},
				{Reviewer: ReviewerB, State: ReJudgmentUnresolved},
			},
			ReJudgmentUnresolved,
		},
		{
			"dos juicios del mismo revisor no son unanimidad",
			[]ReJudgment{
				{Reviewer: ReviewerA, State: ReJudgmentResolved},
				{Reviewer: ReviewerA, State: ReJudgmentResolved},
			},
			ReJudgmentUnresolved,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if got := AggregateReJudgment(caso.entrada); got != caso.esperado {
				t.Errorf("AggregateReJudgment = %s, se esperaba %s", got, caso.esperado)
			}
		})
	}
}

func TestReJudgmentValida(t *testing.T) {
	valido := ReJudgment{
		ConsensusLocalID: "C-001", Reviewer: ReviewerA,
		State: ReJudgmentResolved, Evidence: []string{"go test ./domain"},
	}
	if err := valido.Validate(); err != nil {
		t.Fatalf("re-juicio válido rechazado: %v", err)
	}
	casos := map[string]ReJudgment{
		"sin hallazgo":    {Reviewer: ReviewerA, State: ReJudgmentResolved, Evidence: []string{"x"}},
		"sin revisor":     {ConsensusLocalID: "C-001", State: ReJudgmentResolved, Evidence: []string{"x"}},
		"estado inválido": {ConsensusLocalID: "C-001", Reviewer: ReviewerA, State: "QUIZA", Evidence: []string{"x"}},
		"sin evidencia":   {ConsensusLocalID: "C-001", Reviewer: ReviewerA, State: ReJudgmentResolved},
		"evidencia vacía": {
			ConsensusLocalID: "C-001", Reviewer: ReviewerA,
			State: ReJudgmentResolved, Evidence: []string{"  "},
		},
	}
	for nombre, caso := range casos {
		if err := caso.Validate(); err == nil {
			t.Errorf("%s: se aceptó un re-juicio inválido", nombre)
		}
	}
}
