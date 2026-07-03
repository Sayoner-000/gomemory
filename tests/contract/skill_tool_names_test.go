package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillReferencesRealToolNames evita que el SKILL.md instruya al agente a
// llamar tools MCP que no existen en el server (bug real encontrado:
// mem_save/mem_search en vez de save_memory/search_memories).
func TestSkillReferencesRealToolNames(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	skillPath := filepath.Join(wd, "..", "..", "infrastructure", "plugin", "claude-code", "skills", "memory", "SKILL.md")

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(data)

	for _, ghost := range []string{"mem_save", "mem_search"} {
		if strings.Contains(content, ghost) {
			t.Errorf("SKILL.md menciona %q, que no existe como tool MCP", ghost)
		}
	}
	for _, real := range []string{"save_memory", "search_memories"} {
		if !strings.Contains(content, real) {
			t.Errorf("SKILL.md debería mencionar la tool real %q", real)
		}
	}
}
