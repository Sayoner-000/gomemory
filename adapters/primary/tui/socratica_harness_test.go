package tui

import (
	"errors"
	"strings"
	"testing"

	"mem/adapters/secondary/generator"
	"mem/application/ports"
)

type namedTUIHarness struct {
	*tuiGen
	id string
}

func TestSocraticaProviderFailureDoesNotRetryAndOffersNativeConversation(t *testing.T) {
	s := newSocTUI(t)
	s.gen.fail = errors.New(`opencode: {"type":"provider.auth","message":"OpenCode's free tier can only be used from within OpenCode","status":403}`)
	s.m.soc.deps.Flow.Generator = generator.NewFirstAvailable([]ports.SocraticGenerator{namedTUIHarness{s.gen, "opencode"}})
	s.press("p", "r", "down", "down", "enter")
	s.typeText("busca grafos visuales")
	s.press("enter")
	before := s.gen.calls.Load()
	s.press("r")
	if s.gen.calls.Load() != before || strings.Contains(s.view(), "r Reintentar") || strings.Contains(s.view(), `"provider.auth"`) || !strings.Contains(s.view(), "Continuar asistido") {
		t.Fatal(s.view())
	}
	if cmd := s.press("a"); cmd == nil {
		t.Fatal("assisted session did not yield terminal")
	}
	if s.m.soc.current.Flow.DispatchAck || s.launcher.calls.Load() != 0 {
		t.Fatal("opened/executed before tea.Exec")
	}
}

func (g namedTUIHarness) Name() string { return g.id }

func TestSocraticaExplicitHarnessSelection(t *testing.T) {
	s := newSocTUI(t)
	s.m.soc.deps.Flow.Generator = generator.NewFirstAvailable([]ports.SocraticGenerator{namedTUIHarness{s.gen, "claude"}, namedTUIHarness{s.gen, "codex"}})
	s.press("p", "r")
	if !s.m.soc.choosingHarness || s.gen.calls.Load() != 0 || !strings.Contains(s.view(), "Elige el harness") {
		t.Fatal(s.view())
	}
	s.press("down", "down", "enter") // disabled OpenCode cannot be selected
	if !s.m.soc.choosingHarness || s.m.soc.inputMode != "" {
		t.Fatal("selected unavailable harness")
	}
	s.press("up", "enter")
	if s.m.soc.harness != "codex" || s.m.soc.inputMode != "prompt" {
		t.Fatal(s.view())
	}
	s.typeText("busca proyectos de grafos")
	s.press("enter")
	if s.m.soc.current.Flow.Client != "codex" || !strings.Contains(s.view(), "codex") {
		t.Fatal(s.view())
	}
	s.press("esc", "r")
	if !s.m.soc.choosingHarness || s.m.soc.harness != "" {
		t.Fatal("remembered selection for next flow")
	}
	s.press("esc")
	if s.m.soc.choosingHarness {
		t.Fatal("selector did not cancel")
	}
}

func TestSocraticaHarnessPickerFitsNarrowTerminal(t *testing.T) {
	s := newSocTUI(t)
	s.m.width = 35
	s.press("p", "r")
	for _, line := range strings.Split(s.view(), "\n") {
		if len([]rune(line)) > 35 {
			t.Fatalf("overflow: %q", line)
		}
	}
	s.press("esc")
	if s.gen.calls.Load() != 0 {
		t.Fatal("generation on cancellation")
	}
}
