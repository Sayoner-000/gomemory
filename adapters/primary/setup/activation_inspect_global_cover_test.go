package setup

import (
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

// HCD-063: desde la feature 035, `mem install .` no registra en el proyecto
// los hooks que ya cubre ~/.claude/settings.json. El inspector debe contarlos
// como cubiertos en el ámbito proyecto; si no, `mem doctor` los marca
// missing y su remedio (`mem install .`) nunca converge.
func TestInspectClaudeHook_ProyectoCubiertoPorGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	root := t.TempDir()

	global := `{"hooks":{
	  "PreToolUse":[{"matcher":"ExitPlanMode","hooks":[{"type":"command","command":"mem hook plan-guard"}]}],
	  "PostToolUse":[{"matcher":"EnterPlanMode","hooks":[{"type":"command","command":"mem hook plan-entered"}]}]}}`
	writeSettings(t, home, global)
	// El proyecto existe pero sin esos hooks: install los omitió por el global.
	writeSettings(t, root, `{"hooks":{}}`)

	channels := NewActivationInspector().Inspect(root)
	for _, kind := range []domain.ChannelKind{domain.KindPlanGuard, domain.KindPlanEntry} {
		c := findChannel(t, channels, "claude", domain.ScopeProject, kind)
		if c.State != domain.StateOK {
			t.Errorf("%s/project cubierto por el global: state=%v detail=%q", kind, c.State, c.Detail)
		}
	}
}

func TestInspectClaudeHook_ProyectoSinGlobalSigueFaltando(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	root := t.TempDir()
	writeSettings(t, root, `{"hooks":{}}`)

	c := findChannel(t, NewActivationInspector().Inspect(root), "claude", domain.ScopeProject, domain.KindPlanGuard)
	if c.State != domain.StateMissing {
		t.Fatalf("sin hook en proyecto ni global debe faltar: state=%v", c.State)
	}
}

func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findChannel(t *testing.T, channels []domain.ActivationChannel, agent string, scope domain.AgentScope, kind domain.ChannelKind) domain.ActivationChannel {
	t.Helper()
	for _, c := range channels {
		if c.Arm == domain.ArmGomemory && c.Agent == agent && c.Scope == scope && c.Kind == kind {
			return c
		}
	}
	t.Fatalf("no se encontró el canal %s/%s/%s", agent, scope, kind)
	return domain.ActivationChannel{}
}
