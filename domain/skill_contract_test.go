package domain

import (
	"strings"
	"testing"
)

func TestValidateSkillBody(t *testing.T) {
	valid := "---\nname: sample\ndescription: Revisa cambios\n---\n## Capacidades\nMCP `save_memory`; CLI mem save; manual registrar decisión.\n## Procedimiento\n1. Guardar resultado. Salida: decisión registrada.\nInspirada en test de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.\n"
	if errs := ValidateSkillBody("sample", valid, AllMCPTools()); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, bad := range []string{strings.Replace(valid, "name: sample", "name: wrong", 1), strings.Replace(valid, "save_memory", "fake_tool", 1), valid + "\nnpm\n", valid + "\n~/.codex\n", valid + "\n/compact\n", strings.Replace(valid, "Inspirada en", "Derivada de", 1), strings.Replace(valid, "## Capacidades", "## Alternativas", 1), strings.Replace(valid, "description: Revisa cambios", "description: "+strings.Repeat("x", 301), 1)} {
		if errs := ValidateSkillBody("sample", bad, AllMCPTools()); len(errs) == 0 {
			t.Errorf("accepted invalid skill %q", bad)
		}
	}
}

func TestAllMCPToolsUniqueAndIncludesOptional(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range AllMCPTools() {
		if seen[n] {
			t.Fatal(n)
		}
		seen[n] = true
	}
	for _, n := range append(MCPAllTools(), MCPOctopusTools...) {
		if !seen[n] {
			t.Fatal(n)
		}
	}
}
