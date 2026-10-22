package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/primary/setup"
)

// R2-04: un hook de terceros que pasa `hook <sub>` a su propio script no es
// de gomemory. install (Claude y Codex) y uninstall lo dejan intacto.
func TestThirdPartyHookSubcommandSurvivesInstallAndUninstall(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	const third = "bash /opt/acme/hooks/entry.sh hook user-prompt-submit"
	settings := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o700); err != nil {
		t.Fatal(err)
	}
	seed := `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"` + third + `"}]}],"Stop":[{"hooks":[{"type":"command","command":"bash /opt/acme/hooks/entry.sh hook turn-end"}]}]}}`
	if err := os.WriteFile(settings, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	codexPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatal(err)
	}
	codexSeed := "[hooks]\n[[hooks.UserPromptSubmit]]\n[[hooks.UserPromptSubmit.hooks]]\ntype = 'command'\ncommand = '" + third + "'\n"
	if err := os.WriteFile(codexPath, []byte(codexSeed), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := setup.WriteClaudeHooksGlobal(root, setup.AgentRef{HookCommand: "mem"}); err != nil {
			t.Fatal(err)
		}
		if !setupCodexGlobal(BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}) {
			t.Fatal("codex setup")
		}
	}
	raw, _ := os.ReadFile(settings)
	for _, want := range []string{third, "entry.sh hook turn-end", "mem hook user-prompt-submit", "mem hook turn-end"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("tras install, settings.json pierde %q:\n%s", want, raw)
		}
	}
	codex, _ := os.ReadFile(codexPath)
	if !strings.Contains(string(codex), "'"+third+"'") && !strings.Contains(string(codex), `"`+third+`"`) {
		t.Errorf("la migración de Codex reescribe el hook de terceros:\n%s", codex)
	}
	if !strings.Contains(string(codex), "mem hook user-prompt-submit") {
		t.Errorf("el hook de terceros no debe pasar por el de gomemory:\n%s", codex)
	}
	removeClaudePlugin(root)
	raw, _ = os.ReadFile(settings)
	if !strings.Contains(string(raw), third) || !strings.Contains(string(raw), "entry.sh hook turn-end") {
		t.Errorf("uninstall borra hooks de terceros:\n%s", raw)
	}
	if strings.Contains(string(raw), "mem hook") {
		t.Errorf("uninstall deja hooks de gomemory:\n%s", raw)
	}
}
