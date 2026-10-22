package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"mem/adapters/primary/setup"
	"mem/domain"
)

// TestHookSubcommandsRegistry cierra la brecha de la feature 040 (research
// R2): cada subcomando de `mem hook` que instala cualquiera de los tres
// harnesses debe figurar en el registro único, porque es lo que consulta el
// desinstalador y la deduplicación. Un subcomando fuera del registro deja
// hooks huérfanos tras `mem uninstall`.
func TestHookSubcommandsRegistry(t *testing.T) {
	var subs []string
	subs = append(subs, setup.ClaudeHookSubs()...)
	for _, h := range setup.CodexGomemoryHooks() {
		subs = append(subs, firstWord(h.Sub))
	}
	plugin, err := os.ReadFile(filepath.Join(repoRootContract(t), "infrastructure", "plugin", "opencode", "gomemory.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`\["hook", "([a-z-]+)"`).FindAllStringSubmatch(string(plugin), -1) {
		subs = append(subs, m[1])
	}
	if len(subs) == 0 {
		t.Fatal("no se encontró ningún subcomando instalado")
	}
	for _, sub := range subs {
		if !domain.IsGomemoryHookSubcommand(sub) {
			t.Errorf("subcomando instalado %q no figura en domain.GomemoryHookSubcommands", sub)
		}
	}

	for _, sub := range append(append([]string{}, domain.GomemoryHookSubcommands...), "expand pre-tool") {
		entry := map[string]interface{}{"hooks": []interface{}{
			map[string]interface{}{"type": "command", "command": "mem hook " + sub},
		}}
		if !setup.IsGomemoryHookEntry(entry) {
			t.Errorf("IsGomemoryHookEntry no reconoce %q", "mem hook "+sub)
		}
	}
	if setup.IsGomemoryHookEntry("some-other-tool hook whatever") {
		t.Error("no debe reconocer comandos de terceros")
	}
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	return s
}
