package hcdfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseDefinition_Valid(t *testing.T) {
	got, warnings, err := ParseDefinition(fixture(t, "definition-valid.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if got.ID != "HCD-001" || got.Kind != domain.KindFix || len(got.Stories) != 1 || len(got.Criteria) != 1 {
		t.Fatalf("definition = %+v", got)
	}
}

func TestParseDefinition_MissingAndUnknownKeysProduceTypedWarnings(t *testing.T) {
	data := strings.ReplaceAll(string(fixture(t, "definition-missing.md")), "Problema: TODO", "Problema: TODO\nClave nueva: valor")
	_, warnings, err := ParseDefinition([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	missing, unknown := false, false
	for _, warning := range warnings {
		missing = missing || warning.Code == "DEF-PENDING"
		unknown = unknown || warning.Code == "ART-UNKNOWN-KEY"
	}
	if !missing || !unknown {
		t.Fatalf("warnings must include DEF-PENDING and ART-UNKNOWN-KEY: %+v", warnings)
	}
}

func TestParseDefinition_DuplicateIDIsError(t *testing.T) {
	if _, _, err := ParseDefinition(fixture(t, "definition-duplicate.md")); err == nil {
		t.Fatal("duplicate story ID must fail")
	}
}

func TestDefinitionRoundTripPreservesRevision(t *testing.T) {
	data := fixture(t, "definition-valid.md")
	def, _, err := ParseDefinition(data)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDefinition(def)
	if domain.ContentRevision(data) != domain.ContentRevision(got) {
		t.Fatalf("round-trip changed revision\n%s", got)
	}
}

// Cada tipo conserva sus campos particulares al releer lo que renderizó (el técnico perdía «Problema técnico»).
func TestDefinitionRoundTripKeepsKindSpecificFields(t *testing.T) {
	d := domain.Definition{ID: "HCD-001", Kind: domain.KindTechnical, Title: "T", Implementer: "dev", Playbook: "abc", Problem: "p", Actor: "a", ExpectedResult: "r", Included: "i", Excluded: "e", Restrictions: "x", TechnicalProblem: "resume lento", Beneficiary: "equipo", ImprovementCondition: "menos de 200 ms"}
	got, _, err := ParseDefinition(RenderDefinition(d))
	if err != nil {
		t.Fatal(err)
	}
	if got.TechnicalProblem != d.TechnicalProblem || got.Beneficiary != d.Beneficiary || got.ImprovementCondition != d.ImprovementCondition || got.Problem != "p" {
		t.Fatalf("round trip lost fields: %+v", got)
	}
}
