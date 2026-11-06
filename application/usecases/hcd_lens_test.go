package usecases_test

import (
	"context"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// HCD-034: analyze consulta en el repositorio las rutas citadas y avisa de las
// ausentes desde verify; las lentes nunca bloquean el cierre.
func TestHCDAnalyzeLenses(t *testing.T) {
	ctx := context.Background()
	env := newHCDEnv(t)
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "fast", Title: "Lentes", Why: "motivo", Intent: "avisar sin tokens"})
	if err != nil {
		t.Fatal(err)
	}
	editDoc(t, env, view, append(append([]string{}, commonFill...),
		"## Checklist\n", "## Checklist\n- [ ] 1. Tocar `save.go` y `falta/nuevo.go` → git grep -q save -- save.go\n")...)
	view, _, err = env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q save -- save.go"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Stage != domain.StageClose {
		t.Fatalf("stage = %s", view.Stage)
	}
	_, findings, err := env.uc.Analyze(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.Severity{}
	for _, f := range findings {
		got[f.Code+":"+f.Ref] = f.Severity
	}
	if got["PATH-MISSING:falta/nuevo.go"] != domain.SeverityMedium || got["PATH-MISSING:save.go"] != "" {
		t.Fatalf("PATH-MISSING: %v", got)
	}
	if got["CHEAPEST-FILLER:"+domain.SectionCheapest] != domain.SeverityMedium {
		t.Fatalf("CHEAPEST-FILLER: %v", got)
	}
	if domain.HasCritical(findings) {
		t.Fatalf("las lentes no bloquean: %+v", findings)
	}
}
