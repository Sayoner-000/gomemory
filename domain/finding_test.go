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
