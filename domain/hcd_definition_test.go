package domain

import "testing"

func validDefinition(kind Kind) Definition {
	d := Definition{ID: "HCD-001", Kind: kind, Title: "Título", Implementer: "dev", Approach: "hcd", Playbook: "abc123", Problem: "problema", Actor: "usuario", ExpectedResult: "resultado", Included: "incluido", Excluded: "excluido", Restrictions: "restricciones", Stories: []Story{{ID: "US1", Title: "Historia", Actor: "usuario", Text: "Como usuario quiero hacerlo"}}, Criteria: []Criterion{{ID: "C-01", Title: "Criterio", Stories: []string{"US1"}, Condition: "observable", Verification: VerificationAutomated}}}
	switch kind {
	case KindFeature:
	case KindEvolution:
		d.CurrentBehavior = "antes"
		d.Change = "cambio"
		d.Compatibility = "compatible"
	case KindFix:
		d.ObservedFailure = "falla"
		d.Reproduction = "reproducible"
		d.RootCause = "pendiente"
	case KindTechnical:
		d.TechnicalProblem = "problema técnico"
		d.Beneficiary = "equipo"
		d.ImprovementCondition = "medible"
	}
	return d
}

func TestValidateDefinition_AllKindsAndSpecificFields(t *testing.T) {
	for _, kind := range []Kind{KindFeature, KindEvolution, KindFix, KindTechnical} {
		t.Run(string(kind), func(t *testing.T) {
			if got := ValidateDefinition(validDefinition(kind), kind); len(got) != 0 {
				t.Fatalf("unexpected checks: %+v", got)
			}
		})
	}
}

func TestValidateDefinition_StoryAndCriterionRequirements(t *testing.T) {
	d := validDefinition(KindFeature)
	d.Criteria = nil
	if got := ValidateDefinition(d, KindFeature); !containsPending(got, "DEF-CRITERION") {
		t.Fatalf("story without criterion must be flagged: %+v", got)
	}
	d = validDefinition(KindFeature)
	d.Criteria[0].Verification = ""
	if got := ValidateDefinition(d, KindFeature); !containsPending(got, "DEF-CRITERION") {
		t.Fatalf("criterion without verification must be flagged: %+v", got)
	}
}

func TestValidateDefinition_PendingMarkers(t *testing.T) {
	for _, value := range []string{"pendiente", "[pendiente]", "TODO", "[NEEDS CLARIFICATION: falta dato]"} {
		d := validDefinition(KindFeature)
		d.Included = value
		if got := ValidateDefinition(d, KindFeature); !containsPending(got, "DEF-PENDING") {
			t.Errorf("%q no produjo DEF-PENDING: %+v", value, got)
		}
	}
	d := validDefinition(KindFix)
	d.RootCause = "pendiente"
	if got := ValidateDefinition(d, KindFix); len(got) != 0 {
		t.Fatalf("causa raíz pendiente en fix debe admitirse temporalmente: %+v", got)
	}
}

func containsPending(checks []PendingCheck, code string) bool {
	for _, check := range checks {
		if check.Code == code {
			return true
		}
	}
	return false
}

// La plantilla sin editar no es una definición: sus marcadores «…» y «{…}» son pendientes (FR-016).
func TestValidateDefinition_TemplatePlaceholdersArePending(t *testing.T) {
	for _, value := range []string{"…", "Como …, quiero …, para …", "Dado …, cuando …, entonces …", "pendiente | …", "{Título de la historia}"} {
		if !isHCDPending(value) {
			t.Errorf("%q debería ser pendiente", value)
		}
	}
	for _, value := range []string{"Esperar… y reintentar", "usuario", "Dado un dato, cuando se guarda, entonces persiste"} {
		if isHCDPending(value) {
			t.Errorf("%q no debería ser pendiente", value)
		}
	}
	d := validDefinition(KindFix)
	d.Stories[0].Text = "Como …, quiero …, para …"
	d.Criteria[0].Condition = "Dado …, cuando …, entonces …"
	d.Excluded = "…"
	if got := ValidateDefinition(d, KindFix); len(got) != 3 {
		t.Fatalf("want 3 pending checks, got %+v", got)
	}
}
