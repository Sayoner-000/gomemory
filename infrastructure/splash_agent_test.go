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
