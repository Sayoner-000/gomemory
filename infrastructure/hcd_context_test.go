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
	u, e := c.HCD.Start(context.Background(), usecases.HCDStartInput{Kind: domain.KindFix, Title: "Corregir guardado", Implementer: "dev", Client: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	text, e := c.ContextBuilder.Build()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(text, "## HCD activo") || !strings.Contains(text, u.ID) {
		t.Fatalf("container context lacks active HCD:\n%s", text)
	}
}
