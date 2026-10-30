package cli

import (
	"strings"
	"testing"
)

func TestHCD001DiagnosticStates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	for _, enabled := range []bool{false, true} {
		got := toolOutputHookStates(t.TempDir(), enabled)["codex"]
		if !strings.Contains(got, "sin verificar") || !strings.Contains(got, "deshabilitada") {
			t.Errorf("enabled=%v: debe separar política local de evidencia actual: %q", enabled, got)
		}
		if strings.Contains(got, "no permite") || strings.Contains(got, "siempre") {
			t.Errorf("no puede extrapolar el rechazo histórico al host actual: %q", got)
		}
	}
	c := buildDoctorCompression(&Deps{}, t.TempDir())
	for _, key := range []string{"own_output", "selection", "session_delta", "external_output", "conversation_compaction"} {
		if c.Mechanisms[key] == "" {
			t.Errorf("mecanismo sin diagnóstico: %s", key)
		}
	}
	if !strings.Contains(c.ToolOutput["codex"], "0.159.0") {
		t.Fatal("el rechazo histórico necesita versión acotada")
	}
}
