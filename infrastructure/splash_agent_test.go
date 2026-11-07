package main

import (
	"strings"
	"testing"
)

// HCD-047: el agente splash de Claude Code lee solo archivos: sin tools MCP
// (ni de memoria ni de grafo) y sin la jerarquía CLAUDE.md, que llevaría el
// protocolo de gomemory.
func TestSplashAgentTemplate(t *testing.T) {
	data, err := templatesFS.ReadFile("templates/agents/claude/splash.md")
	if err != nil {
		t.Fatalf("falta la plantilla del agente splash: %v", err)
	}
	text := string(data)
	front, body, ok := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	if !strings.HasPrefix(text, "---\n") || !ok {
		t.Fatalf("la plantilla debe empezar con frontmatter YAML:\n%s", text)
	}
	for _, want := range []string{"name: splash", "tools: Read, Grep, Glob", "omitClaudeMd: true", "description: "} {
		if !strings.Contains(front, want) {
			t.Errorf("el frontmatter no contiene %q:\n%s", want, front)
		}
	}
	for _, banned := range []string{"mcp__", "gomemory", "get_context", "search_memories"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(banned)) {
			t.Errorf("la plantilla no debe nombrar %q: el splash no usa memoria:\n%s", banned, text)
		}
	}
	if strings.TrimSpace(body) == "" {
		t.Fatal("la plantilla necesita instrucciones para el revisor")
	}
}

// HCD-051: los formatos que leen Codex (TOML en ~/.codex/agents) y agy 1.5.3
// (agent.md con frontmatter y el prompt tras un H1), según el sondeo real.
// Codex hereda el MCP global aunque el TOML lo intente quitar: la plantilla
// se lo prohíbe por instrucción.
func TestSplashAgentTemplates_CodexYAgy(t *testing.T) {
	for path, wants := range map[string][]string{
		"templates/agents/codex/splash.toml": {`name = "splash"`, `sandbox_mode = "read-only"`, `developer_instructions = """`, "No uses herramientas MCP", "Blocker, Major o Nit"},
		"templates/agents/agy/splash.md":     {"---\nname: splash\n", "\nsubagent: true\n", "\ninheritMcp: false\n", "\n# splash\n", "Blocker, Major o Nit"},
	} {
		data, err := templatesFS.ReadFile(path)
		if err != nil {
			t.Fatalf("falta la plantilla %s: %v", path, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(data), want) {
				t.Errorf("%s no contiene %q", path, want)
			}
		}
	}
}
