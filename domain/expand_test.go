package domain

import "testing"

func TestParseExpandEvent(t *testing.T) {
	for _, s := range []string{"session-start", "prompt", "pre-tool", "post-tool", "stop"} {
		if ev, ok := ParseExpandEvent(s); !ok || string(ev) != s {
			t.Errorf("ParseExpandEvent(%q) = %q, %v", s, ev, ok)
		}
	}
	for _, s := range []string{"", "bogus", "PreToolUse", "pre_tool"} {
		if _, ok := ParseExpandEvent(s); ok {
			t.Errorf("ParseExpandEvent(%q) debería ser inválido", s)
		}
	}
}

func TestParseSupertoolName(t *testing.T) {
	for _, s := range []string{"rules", "shield", "flow", "instincts"} {
		if n, ok := ParseSupertoolName(s); !ok || string(n) != s {
			t.Errorf("ParseSupertoolName(%q) = %q, %v", s, n, ok)
		}
	}
	for _, s := range []string{"", "flow-skills", "Rules", "bogus"} {
		if _, ok := ParseSupertoolName(s); ok {
			t.Errorf("ParseSupertoolName(%q) debería ser inválido", s)
		}
	}
}

func TestClassifyTool(t *testing.T) {
	cases := map[string]ToolKind{
		// Claude Code
		"Bash": ToolShell, "Write": ToolFile, "Edit": ToolFile, "MultiEdit": ToolFile,
		"NotebookEdit": ToolFile, "Task": ToolDelegate, "Agent": ToolDelegate, "Read": ToolOther,
		// Codex (verificados en specs/040-expand-supertools/evidence/codex-probe.md)
		"apply_patch": ToolFile, "collaborationspawn_agent": ToolDelegate, "collaborationwait_agent": ToolOther,
		// OpenCode
		"bash": ToolShell, "write": ToolFile, "edit": ToolFile, "patch": ToolFile, "task": ToolDelegate, "read": ToolOther,
		// Desconocidos
		"": ToolOther, "mcp__gomemory__save_memory": ToolOther, "WebFetch": ToolOther,
	}
	for name, want := range cases {
		if got := ClassifyTool(name); got != want {
			t.Errorf("ClassifyTool(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIsCommitCommand(t *testing.T) {
	cases := map[string]bool{
		"git commit -m x":                   true,
		"ls && git commit":                  true,
		"git -C repo commit -am 'msg'":      true,
		"gh pr create --fill":               true,
		"glab mr create":                    true,
		"GIT_AUTHOR_NAME=x git commit -m y": true,
		"sudo -u app git commit -m x":       true,
		"sudo -E -H git commit":             true,
		"sudo --user=app git commit":        true,
		"sudo -- git commit":                true,
		`echo "git commit"`:                 false,
		"git commit-tree abc":               false,
		"git status":                        false,
		"gh pr view":                        false,
		"":                                  false,
		"grep -r 'git commit' docs":         false,
		"git log --grep commit":             false,
	}
	for cmd, want := range cases {
		if got := IsCommitCommand(cmd); got != want {
			t.Errorf("IsCommitCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestMatchesDependencyIntent(t *testing.T) {
	positives := []string{
		"añade la librería zod",
		"Agrega una dependencia para parsear YAML",
		"instala el paquete lodash",
		"integra el SDK de Stripe",
		"incorpora un servidor MCP de GitHub",
		"necesito añadir una integración con Slack",
		"add a dependency for retries",
		"install the axios package",
		"integrate the Sentry SDK",
		"please add a library to parse TOML",
		"ejecuta go get github.com/foo/bar",
		"run npm install react",
	}
	negatives := []string{
		"explica este archivo",
		"añade un test para el router",
		"agrega un comentario aquí",
		"explica la dependencia entre módulos",
		"instala el binario en /usr/local",
		"add a comment to the function",
		"install the hook and run it",
		"what does this package do?",
		"refactoriza la integración existente",
		"fix the bug in the library loader",
		"",
		"commit the changes",
	}
	for _, p := range positives {
		if !MatchesDependencyIntent(p) {
			t.Errorf("MatchesDependencyIntent(%q) = false, want true", p)
		}
	}
	for _, p := range negatives {
		if MatchesDependencyIntent(p) {
			t.Errorf("MatchesDependencyIntent(%q) = true, want false", p)
		}
	}
}
