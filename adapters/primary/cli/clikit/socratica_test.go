package clikit

import (
	"errors"
	"strings"
	"testing"

	"mem/adapters/primary/console"
	"mem/application/ports"
)

func TestSelectSocraticHarness(t *testing.T) {
	catalog := []ports.SocraticHarness{
		{ID: "claude", Label: "Claude", Available: true},
		{ID: "opencode", Label: "OpenCode", Reason: "no instalado", Available: false},
		{ID: "codex", Label: "Codex", Available: true},
	}

	t.Run("preserves catalog and unavailable reason", func(t *testing.T) {
		var out strings.Builder
		ui := console.NewPlain(strings.NewReader("2\n"), &out)
		got, err := SelectSocraticHarness(catalog, ui, nil, &out, func(label, reason string) {
			_, _ = out.WriteString("  − " + label + ": " + reason + "\n")
		})
		if err != nil || got != "codex" {
			t.Fatalf("got %q, %v", got, err)
		}
		if !strings.Contains(out.String(), "OpenCode: no instalado") || !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") {
			t.Fatalf("selector perdió opciones o razón: %q", out.String())
		}
	})

	t.Run("excludes a previous harness", func(t *testing.T) {
		var out strings.Builder
		ui := console.NewPlain(strings.NewReader("1\n"), &out)
		got, err := SelectSocraticHarness(catalog, ui, nil, &out, nil, "claude")
		if err != nil || got != "codex" {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("returns cancellation", func(t *testing.T) {
		var out strings.Builder
		ui := console.NewPlain(strings.NewReader("3\n"), &out)
		_, err := SelectSocraticHarness(catalog, ui, nil, &out, nil)
		if !errors.Is(err, console.ErrCanceled) {
			t.Fatalf("expected canceled, got %v", err)
		}
	})

	t.Run("returns unavailable when catalog has no choices", func(t *testing.T) {
		_, err := SelectSocraticHarness([]ports.SocraticHarness{{ID: "x", Reason: "offline"}}, nil, nil, nil, nil)
		if !errors.Is(err, ports.ErrNoGenerator) {
			t.Fatalf("expected no generator, got %v", err)
		}
	})
}
