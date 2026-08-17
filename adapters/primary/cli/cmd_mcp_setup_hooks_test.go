package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

// TestRunGlobalScopeSetup_InstructionsQuedaEnVersionVigente cubre T054: el
// archivo de instrucciones de nivel usuario, tras habilitar el ámbito global,
// contiene el marcador de la versión VIGENTE del protocolo (domain.ProtocolVersionMarker)
// — la misma fuente única que consulta el inspector de cobertura, para que
// "vigente" nunca pueda significar cosas distintas en dos sitios.
func TestRunGlobalScopeSetup_InstructionsQuedaEnVersionVigente(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir ~/.claude: %v", err)
	}

	runGlobalScopeSetup([]string{"claude"})

	data, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("no se escribió ~/.claude/CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(data), domain.ProtocolVersionMarker) {
		t.Errorf("~/.claude/CLAUDE.md no contiene la versión vigente %q", domain.ProtocolVersionMarker)
	}
}

// TestRunGlobalScopeSetup_AgenteSinDirectorioSeOmiteSinError cubre la otra
// mitad de T054: opencode no tiene ~/.config/opencode en esta máquina de
// prueba, así que habilitar el ámbito global no debe fallar ni crearle el
// directorio (mismo criterio que TestInstallAtomicPlanGlobal_NoCreaConfigDeAgentesNoUsados,
// pero ejercido a través del flujo real de runGlobalScopeSetup).
func TestRunGlobalScopeSetup_AgenteSinDirectorioSeOmiteSinError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir ~/.claude: %v", err)
	}
	// Deliberadamente NO se crea ~/.config/opencode.

	runGlobalScopeSetup([]string{"claude"})

	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); err == nil {
		t.Error("no debía crearse ~/.config/opencode para un agente que la persona no usa")
	}
}

// TestRunGlobalScopeSetup_WritesClaudeHooksInHome cubre T052 (feature 019,
// Historia 4): habilitar el ámbito global debe escribir también los hooks del
// modo plan atómico en ~/.claude/settings.json — antes de esta feature, el
// ámbito global registraba el servidor MCP y el texto de instrucciones, pero
// NUNCA los hooks (research.md §7), así que el determinismo de la Historia 1
// nunca llegaba a un proyecto nuevo sin instalación propia.
func TestRunGlobalScopeSetup_WritesClaudeHooksInHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir ~/.claude: %v", err)
	}

	runGlobalScopeSetup([]string{"claude"})

	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("no se escribió ~/.claude/settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json inválido: %v", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("falta la clave hooks en ~/.claude/settings.json")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("falta PreToolUse (plan-guard) en el ámbito de usuario")
	}
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("falta PostToolUse (plan-entered/plan-approved) en el ámbito de usuario")
	}
}

// TestRunGlobalScopeSetup_PreservesForeignHooksAndIsIdempotent cubre que la
// escritura de hooks globales preserva entradas ajenas y no duplica en una
// segunda ejecución — mismo criterio que el ámbito de proyecto.
func TestRunGlobalScopeSetup_PreservesForeignHooksAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"otra-tool hook antes-de-bash"}]}]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	runGlobalScopeSetup([]string{"claude"})
	runGlobalScopeSetup([]string{"claude"})

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]any
	json.Unmarshal(data, &settings)
	hooks, _ := settings["hooks"].(map[string]any)
	preToolUse, _ := hooks["PreToolUse"].([]any)

	foreignFound, guardCount := 0, 0
	for _, e := range preToolUse {
		m, _ := e.(map[string]any)
		matcher, _ := m["matcher"].(string)
		if matcher == "Bash" {
			foreignFound++
		}
		if matcher == "ExitPlanMode" {
			guardCount++
		}
	}
	if foreignFound != 1 {
		t.Errorf("la entrada ajena (Bash) debió preservarse exactamente una vez, got %d", foreignFound)
	}
	if guardCount != 1 {
		t.Errorf("tras dos ejecuciones, plan-guard debe aparecer exactamente una vez, got %d", guardCount)
	}
}
