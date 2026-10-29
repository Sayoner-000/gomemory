package main

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCDContextRealContainer(t *testing.T) {
	c, _ := containerCloseFixture(t, "cli")
	v, e := c.HCD.Start(context.Background(), usecases.HCDStartInput{Kind: string(domain.KindFix), Lane: string(domain.LaneFast), Title: "Corregir guardado", Why: "fix claro", Client: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	text, e := c.ContextBuilder.Build()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(text, "## HCD activo") || !strings.Contains(text, v.Unit.ID) {
		t.Fatalf("container context lacks active HCD:\n%s", text)
	}
}
