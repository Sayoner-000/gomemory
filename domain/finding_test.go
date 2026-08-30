package domain

import "testing"

func TestSeverityValuesAreClosed(t *testing.T) {
	valid := []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
	for _, severity := range valid {
		if !severity.Valid() {
			t.Errorf("severidad válida no reconocida: %q", severity)
		}
	}
	if Severity("BLOCKER").Valid() {
		t.Fatal("una severidad fuera del contrato no puede ser válida")
	}
}

func TestEvidenceClassValuesAreClosed(t *testing.T) {
	valid := []EvidenceClass{
		EvidenceDeterministic,
		EvidenceReproduced,
		EvidenceContract,
		EvidenceStaticAnalysis,
		EvidenceTestFailure,
		EvidenceRuntimeObservation,
		EvidenceProbabilistic,
	}
	for _, evidenceClass := range valid {
		if !evidenceClass.Valid() {
			t.Errorf("clase de evidencia válida no reconocida: %q", evidenceClass)
		}
	}
	if EvidenceClass("opinion").Valid() {
		t.Fatal("una clase de evidencia fuera del contrato no puede ser válida")
	}
}

func TestFindingWithoutEvidenceIsNotConfirmable(t *testing.T) {
	finding := Finding{
		LocalID:       "A-001",
		Severity:      SeverityHigh,
		Claim:         "el estado se pierde",
		EvidenceClass: EvidenceDeterministic,
	}
	if finding.Confirmable() {
		t.Fatal("un hallazgo sin evidencia no puede ser confirmable")
	}
	finding.Evidence = []string{"la fila desaparece tras reiniciar"}
	if !finding.Confirmable() {
		t.Fatal("un hallazgo válido con evidencia concreta debe ser confirmable")
	}
}

// TestSeverityRankYMaxSeverity fija el orden que usa la derivación conservadora de
// severidad del consenso. Sin un orden explícito, "la severidad más alta aplicable"
// de FR-003 no es computable.
func TestSeverityRankYMaxSeverity(t *testing.T) {
	orden := []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
	for i := 1; i < len(orden); i++ {
		if orden[i-1].Rank() >= orden[i].Rank() {
			t.Errorf("%s debe ordenarse por debajo de %s", orden[i-1], orden[i])
		}
	}
	if Severity("inventada").Rank() >= SeverityInfo.Rank() {
		t.Error("una severidad desconocida no puede ordenarse por encima de INFO")
	}

	casos := []struct {
		entrada  []Severity
		esperado Severity
	}{
		{[]Severity{SeverityHigh, SeverityLow}, SeverityHigh},
		{[]Severity{SeverityLow, SeverityHigh}, SeverityHigh},
		{[]Severity{SeverityCritical, SeverityHigh}, SeverityCritical},
		{[]Severity{SeverityMedium, SeverityMedium}, SeverityMedium},
		{[]Severity{SeverityInfo}, SeverityInfo},
		{[]Severity{"inventada", SeverityLow}, SeverityLow},
	}
	for _, caso := range casos {
		if got := MaxSeverity(caso.entrada...); got != caso.esperado {
			t.Errorf("MaxSeverity(%v) = %s, se esperaba %s", caso.entrada, got, caso.esperado)
		}
	}
	if MaxSeverity() != SeverityInfo {
		t.Error("sin fuentes, la severidad derivada debe ser la mínima, no una vacía")
	}
}
