package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// readClaudeSettings lee y parsea .claude/settings.json de target.
func readClaudeSettings(t *testing.T, target string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(target, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("leer .claude/settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parsear .claude/settings.json: %v", err)
	}
	return settings
}

// preToolUseCommands extrae, del hook PreToolUse ya parseado, los comandos
// registrados para el matcher dado.
func preToolUseCommands(settings map[string]any, event, matcher string) []string {
	hooks, _ := settings["hooks"].(map[string]any)
	raw, _ := hooks[event].([]any)
	var out []string
	for _, e := range raw {
		m, _ := e.(map[string]any)
		if m["matcher"] != matcher {
			continue
		}
		inner, _ := m["hooks"].([]any)
		for _, h := range inner {
			hm, _ := h.(map[string]any)
			if cmd, _ := hm["command"].(string); cmd != "" {
				out = append(out, cmd)
			}
		}
	}
	return out
}

// TestInstallRegistersPlanGuardOnExitPlanMode cubre T023 (feature 019,
// Historia 1): PreToolUse con matcher ExitPlanMode debe registrar
// `mem hook plan-guard` — el borde de salida determinista solo funciona si
// esta entrada existe en settings.json.
func TestInstallRegistersPlanGuardOnExitPlanMode(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	target := t.TempDir()

	cmd := exec.Command(bin, "install", target)
	cmd.Dir = target
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mem install: %v\n%s", err, out)
	}

	settings := readClaudeSettings(t, target)
	cmds := preToolUseCommands(settings, "PreToolUse", "ExitPlanMode")
	found := false
	for _, c := range cmds {
		if hasPlanGuardSuffix(c) {
			found = true
		}
	}
	if !found {
		t.Errorf("PreToolUse:ExitPlanMode debe registrar plan-guard, got comandos=%v", cmds)
	}
}

func hasPlanGuardSuffix(cmd string) bool {
	const suffix = " hook plan-guard"
	return len(cmd) >= len(suffix) && cmd[len(cmd)-len(suffix):] == suffix
}

// TestInstallPreservesForeignPreToolUseEntries cubre la preservación de
// entradas ajenas: un settings.json con un hook PreToolUse de un tercero
// (mismo matcher u otro) debe sobrevivir intacto tras `mem install`, igual
// que ya garantizan los hooks existentes (filterOutGomemoryHooks).
func TestInstallPreservesForeignPreToolUseEntries(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	target := t.TempDir()

	claudeDir := filepath.Join(target, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	existing := `{
		"hooks": {
			"PreToolUse": [
				{"matcher": "Bash", "hooks": [{"type": "command", "command": "otra-tool hook antes-de-bash"}]}
			]
		}
	}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	cmd := exec.Command(bin, "install", target)
	cmd.Dir = target
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mem install: %v\n%s", err, out)
	}

	settings := readClaudeSettings(t, target)
	foreign := preToolUseCommands(settings, "PreToolUse", "Bash")
	if len(foreign) != 1 || foreign[0] != "otra-tool hook antes-de-bash" {
		t.Errorf("la entrada PreToolUse:Bash de un tercero debió preservarse intacta, got %v", foreign)
	}

	guard := preToolUseCommands(settings, "PreToolUse", "ExitPlanMode")
	if len(guard) == 0 {
		t.Error("plan-guard debió registrarse igual, sin pisar la entrada ajena")
	}
}

// TestInstallTwiceDoesNotDuplicatePlanGuard cubre la idempotencia exigida
// por FR-012/T006: reinstalar dos veces no debe dejar dos entradas de
// plan-guard en PreToolUse:ExitPlanMode, ni de plan-entered en
// PostToolUse:EnterPlanMode.
func TestInstallTwiceDoesNotDuplicatePlanGuard(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	target := t.TempDir()

	for i := 0; i < 2; i++ {
		cmd := exec.Command(bin, "install", target)
		cmd.Dir = target
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mem install (corrida %d): %v\n%s", i+1, err, out)
		}
	}

	settings := readClaudeSettings(t, target)

	guard := preToolUseCommands(settings, "PreToolUse", "ExitPlanMode")
	if len(guard) != 1 {
		t.Errorf("tras dos instalaciones esperaba exactamente 1 entrada de plan-guard, got %d: %v", len(guard), guard)
	}

	entered := preToolUseCommands(settings, "PostToolUse", "EnterPlanMode")
	if len(entered) != 1 {
		t.Errorf("tras dos instalaciones esperaba exactamente 1 entrada de plan-entered, got %d: %v", len(entered), entered)
	}

	approved := preToolUseCommands(settings, "PostToolUse", "ExitPlanMode")
	if len(approved) != 1 {
		t.Errorf("tras dos instalaciones esperaba exactamente 1 entrada de plan-approved (feature 007), got %d: %v", len(approved), approved)
	}
}

// TestInstallRegistersPlanEnteredOnEnterPlanMode cubre T031: PostToolUse con
// matcher EnterPlanMode debe registrar `mem hook plan-entered` — el borde de
// entrada de la Historia 2, verificado en vivo (T001-T004) que este evento
// acepta additionalContext.
func TestInstallRegistersPlanEnteredOnEnterPlanMode(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	target := t.TempDir()

	cmd := exec.Command(bin, "install", target)
	cmd.Dir = target
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mem install: %v\n%s", err, out)
	}

	settings := readClaudeSettings(t, target)
	cmds := preToolUseCommands(settings, "PostToolUse", "EnterPlanMode")
	found := false
	for _, c := range cmds {
		const suffix = " hook plan-entered"
		if len(c) >= len(suffix) && c[len(c)-len(suffix):] == suffix {
			found = true
		}
	}
	if !found {
		t.Errorf("PostToolUse:EnterPlanMode debe registrar plan-entered, got comandos=%v", cmds)
	}

	// El hook existente de plan-approved (feature 007) debe seguir intacto.
	approved := preToolUseCommands(settings, "PostToolUse", "ExitPlanMode")
	if len(approved) == 0 {
		t.Error("registrar plan-entered no debe desplazar el hook existente de plan-approved")
	}
}
