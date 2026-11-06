package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HCD-033: la skill ACR enseña a encadenar revisiones (HCD-032). Sin esto el
// motor tiene la revisión acumulativa, pero ningún orquestador la usa.
func TestACRSkill_RevisionDeSeguimiento(t *testing.T) {
	for _, p := range []string{
		"skills/adversarial-consensus-review/SKILL.md",
		"infrastructure/templates/adversarial-consensus-review/SKILL.md",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", p))
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		seguimiento := strings.Index(s, "## Follow-up Review")
		plantilla := strings.Index(s, "# 44. Reviewer Template")
		if seguimiento < 0 || plantilla < 0 || seguimiento > plantilla {
			t.Fatalf("%s: falta la subsección «Follow-up Review» en la orquestación", p)
		}
		bloque := s[seguimiento:plantilla]
		for _, want := range []string{"since_review_id", "inherited", "delta", "settled_ref", "INCOMPLETE"} {
			if !strings.Contains(bloque, want) {
				t.Errorf("%s: la revisión de seguimiento debe mencionar %q", p, want)
			}
		}
		fin := strings.Index(s[plantilla:], "# 45.")
		if fin < 0 || !strings.Contains(s[plantilla:plantilla+fin], "settled_ref") {
			t.Errorf("%s: la plantilla del revisor debe decirle cómo citar lo heredado (settled_ref)", p)
		}
	}
}
