package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

// La tabla de agentes instalables cubre todo el registro de capacidades: un
// agente nuevo en domain.KnownAgents no puede quedarse fuera de install.
func TestInstallAgents_CubrenElRegistro(t *testing.T) {
	names := map[string]bool{}
	for _, a := range installAgents {
		names[a.name] = true
	}
	for _, k := range domain.KnownAgents {
		if !names[k.Name] {
			t.Errorf("%s está en domain.KnownAgents y no en installAgents", k.Name)
		}
	}
}

func fakeLookPath(found ...string) func(string) (string, error) {
	return func(bin string) (string, error) {
		for _, f := range found {
			if f == bin {
				return "/usr/bin/" + bin, nil
			}
		}
		return "", errors.New("no encontrado")
	}
}

func detected(choices []agentChoice) map[string]bool {
	out := map[string]bool{}
	for _, c := range choices {
		out[c.name] = c.detected
	}
	return out
}

// Suposición de la spec: un agente está detectado si su ejecutable está en el
// PATH o existe su directorio de configuración de usuario.
func TestDetectAgents(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := detected(detectAgents(home, fakeLookPath("claude")))
	if !got["claude"] || !got["codex"] {
		t.Errorf("claude (PATH) y codex (~/.codex) deben detectarse: %v", got)
	}
	if got["opencode"] || got["cursor"] {
		t.Errorf("sin señales no se detecta: %v", got)
	}
}

// Windsurf y Cline se ofrecen, pero sin marcar aunque se detecten (feature
// 021: salieron de la instalación automática).
func TestDetectAgents_OpcionalesNuncaMarcados(t *testing.T) {
	home := t.TempDir()
	for _, c := range detectAgents(home, fakeLookPath("windsurf")) {
		if (c.name == "windsurf" || c.name == "cline") && (!c.optional || c.checked()) {
			t.Errorf("%s: optional=%v checked=%v", c.name, c.optional, c.checked())
		}
	}
}

// FR-022: validación de --agents antes de escribir nada.
func TestParseAgentList(t *testing.T) {
	got, err := parseAgentList("claude, codex")
	if err != nil || len(got) != 2 || got[0] != "claude" || got[1] != "codex" {
		t.Fatalf("parseAgentList = %v, %v", got, err)
	}
	if _, err := parseAgentList("claude,vscode"); err == nil {
		t.Error("un agente desconocido debe ser error")
	}
}
