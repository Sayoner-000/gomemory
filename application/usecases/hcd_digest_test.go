package usecases_test

import (
	"context"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// HCD-046: la unidad mantiene una sola ficha, actualizada en cada paso, sin
// texto privado y enlazada a los archivos que cita.
func TestHCDDigest_UnaFichaVivaPorUnidad(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Guardado pierde datos", Why: "fix claro", Intent: "El guardado pierde datos <private>SYNTH-DIGEST-0046</private>"})
	if err != nil {
		t.Fatal(err)
	}
	if d := env.decisions.byTopic["hcd/HCD-001"]; d == nil || !strings.Contains(d.Content, "Intención: El guardado pierde datos") || d.Type != domain.Decision || d.Filepath != view.Unit.DocPath {
		t.Fatalf("ficha tras Start = %+v", d)
	}
	editDoc(t, env, view, commonFill...)
	editDoc(t, env, view, "Incluido: guardado", "Incluido: guardado en `save.go`")
	if _, _, err = env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go", ExpectFail: true}); err != nil {
		t.Fatal(err)
	}
	writeIn(t, env.root, "save.go", "package save // fixed\n")
	if _, _, err = env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go"}); err != nil {
		t.Fatal(err)
	}
	if _, err = env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo", Client: "claude"}); err != nil {
		t.Fatal(err)
	}

	if len(env.decisions.byTopic) != 1 || strings.Join(env.decisions.steps, " ") != "start close" {
		t.Fatalf("upserts = %v en %d fichas", env.decisions.steps, len(env.decisions.byTopic))
	}
	d := env.decisions.byTopic["hcd/HCD-001"]
	if d.Title != "HCD-001 · Guardado pierde datos (cerrada)" || !strings.Contains(d.Content, "Aprobaciones: cierre ") {
		t.Fatalf("ficha final = %+v", d)
	}
	if strings.Contains(d.Content, "SYNTH-DIGEST") {
		t.Fatalf("la ficha contiene texto privado:\n%s", d.Content)
	}
	if files := env.decisions.files["hcd/HCD-001"]; strings.Join(files, ",") != "save.go" {
		t.Fatalf("archivos enlazados = %v", files)
	}
}

func TestHCDDigest_PromoverYCancelarActualizanLaFicha(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "fast", Title: "Exportar", Why: "simple", Intent: "Exportar a CSV"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.uc.Promote(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if d := env.decisions.byTopic["hcd/HCD-001"]; !strings.Contains(d.Content, "Carril: full") {
		t.Fatalf("la ficha no refleja la promoción:\n%s", d.Content)
	}
	if _, err := env.uc.Cancel(ctx, usecases.HCDCloseInput{Quote: "ya no hace falta"}); err != nil {
		t.Fatal(err)
	}
	if d := env.decisions.byTopic["hcd/HCD-001"]; d.Title != "HCD-001 · Exportar (cancelada)" || len(env.decisions.steps) != 3 {
		t.Fatalf("ficha tras cancelar = %+v, upserts %v", d, env.decisions.steps)
	}
}
