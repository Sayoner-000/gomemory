package usecases_test

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// T073 — la directiva de concisión solo aparece con el ajuste, siempre tras el
// separador volátil, y no altera el prefijo estable.
func TestBuildContextConciseDirective(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	memRepo := persistence.NewMemoryRepository(db)
	if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "d", Content: "c"}); err != nil {
		t.Fatal(err)
	}
	build := func(stable, concise bool) string {
		b := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
		b.StableOrder, b.ConciseDirective = stable, concise
		out, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	off := build(true, false)
	on := build(true, true)
	if strings.Contains(off, "Estilo de respuesta") {
		t.Error("desactivada por defecto")
	}
	sep := strings.Index(on, usecases.VolatileSeparator)
	dir := strings.Index(on, "Estilo de respuesta")
	if dir < 0 || sep < 0 || dir < sep {
		t.Fatalf("la directiva debe ir tras el separador volátil:\n%s", on)
	}
	offPrefix := off
	if i := strings.Index(off, usecases.VolatileSeparator); i >= 0 {
		offPrefix = off[:i]
	}
	if on[:sep] != offPrefix {
		t.Error("la directiva no debe alterar el prefijo estable")
	}
	// Sin orden estable (nivel structural) también va al final, tras un separador.
	legacy := build(false, true)
	if !strings.HasSuffix(strings.TrimSpace(legacy), strings.TrimSpace(usecases.ConciseOutputDirective)) {
		t.Error("sin zona volátil, la directiva va al final del documento")
	}
}
