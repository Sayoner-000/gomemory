package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// T024/T025 (feature 035, US1, FR-001/FR-001a): en Claude Code cada subcomando
// de hook de gomemory queda registrado UNA vez entre ~/.claude/settings.json y
// el del proyecto. El global sirve a todos los proyectos y nunca se toca desde
// uno: el proyecto solo añade lo que el global no cubre (tool-output).

// gomemorySubsGlobales es el juego que escribe `setup-mcp --scope global`: sin
// tool-output, porque lee los ajustes del HOME y no los del proyecto.
var gomemorySubsGlobales = map[string][]string{
	"SessionStart":     {"session-start", "post-compact"},
	"SessionEnd":       {"session-end"},
	"PostCompact":      {"compact-summary"},
	"UserPromptSubmit": {"user-prompt-submit"},
	"Stop":             {"turn-end"},
	"SubagentStart":    {"subagent-start"},
	"SubagentStop":     {"subagent-stop"},
	"PreToolUse":       {"plan-guard"},
	"PostToolUse":      {"plan-approved", "plan-entered"},
}

var hookSubRe = regexp.MustCompile(`mem hook ([a-z-]+)`)

// writeClaudeSettings escribe un settings.json con los subcomandos dados más
// dos hooks ajenos que nunca deben desaparecer.
func writeClaudeSettings(t *testing.T, path string, subs map[string][]string) {
	t.Helper()
	hooks := map[string]any{}
	for event, list := range subs {
		var entries []any
		for _, sub := range list {
			entries = append(entries, map[string]any{"matcher": "", "hooks": []any{map[string]any{"type": "command", "command": "mem hook " + sub}}})
		}
		hooks[event] = entries
	}
	hooks["UserPromptSubmit"] = append(asSlice(hooks["UserPromptSubmit"]), map[string]any{"matcher": "", "hooks": []any{map[string]any{"type": "command", "command": "cbm-augment --prompt"}}})
	hooks["Stop"] = append(asSlice(hooks["Stop"]), map[string]any{"matcher": "", "hooks": []any{map[string]any{"type": "command", "command": "herdr-agent-state stop"}}})
	data, _ := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	mustWrite(t, path, string(data))
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// gomemorySubsIn devuelve los subcomandos de gomemory registrados en un
// settings.json, con repetición si un subcomando aparece más de una vez.
func gomemorySubsIn(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var subs []string
	for _, m := range hookSubRe.FindAllStringSubmatch(string(data), -1) {
		subs = append(subs, m[1])
	}
	sort.Strings(subs)
	return subs
}

func currentVersion(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootContract(t), "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`Version\s*=\s*"v?([0-9.]+)"`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("no se encontró la versión")
	}
	return "v" + string(m[1])
}

func TestScopeDedup_InstallProyectoComplementaAlGlobal(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("dedup-install")
	global := filepath.Join(s.Home, ".claude", "settings.json")
	writeClaudeSettings(t, global, gomemorySubsGlobales)
	globalAntes := readAll(t, global)
	mustWrite(t, filepath.Join(p, ".memory", "settings.json"), `{"tool_output_compression":true}`)

	if r := s.Run("", p, "install", "--yes", "--agents", "claude", "--scope", "project", p); r.ExitCode != 0 {
		t.Fatalf("install: %d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	proyecto := filepath.Join(p, ".claude", "settings.json")
	if got := gomemorySubsIn(t, proyecto); strings.Join(got, ",") != "tool-output" {
		t.Errorf("con hooks globales activos, el proyecto solo debe añadir tool-output; got %v", got)
	}
	if readAll(t, global) != globalAntes {
		t.Error("el settings.json global nunca se toca desde un proyecto")
	}
	if !strings.Contains(readAll(t, global), "cbm-augment") || !strings.Contains(readAll(t, global), "herdr-agent-state") {
		t.Error("los hooks ajenos del global deben quedar intactos")
	}
}

func TestScopeDedup_SinGlobalElProyectoRegistraTodo(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("dedup-sin-global")
	if r := s.Run("", p, "install", "--yes", "--agents", "claude", "--scope", "project", p); r.ExitCode != 0 {
		t.Fatalf("install: %d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	got := strings.Join(gomemorySubsIn(t, filepath.Join(p, ".claude", "settings.json")), ",")
	for _, want := range []string{"session-start", "user-prompt-submit", "turn-end"} {
		if !strings.Contains(got, want) {
			t.Errorf("sin hooks globales el proyecto debe registrar %s; got %s", want, got)
		}
	}
}

func TestScopeDedup_UpdateYaActualizadoRetiraDuplicados(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("dedup-update")
	writeClaudeSettings(t, filepath.Join(s.Home, ".claude", "settings.json"), gomemorySubsGlobales)
	proyecto := filepath.Join(p, ".claude", "settings.json")
	writeClaudeSettings(t, proyecto, gomemorySubsGlobales)
	mustMkdir(t, filepath.Join(p, ".memory"))

	r := s.Run("", p, "update", "--version", currentVersion(t))
	if r.ExitCode != 0 {
		t.Fatalf("update: %d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	if got := gomemorySubsIn(t, proyecto); len(got) != 0 {
		t.Errorf("update debía retirar del proyecto los subcomandos que ya cubre el global; quedan %v", got)
	}
	if c := readAll(t, proyecto); !strings.Contains(c, "cbm-augment") || !strings.Contains(c, "herdr-agent-state") {
		t.Error("los hooks ajenos del proyecto deben quedar intactos")
	}
}

func TestScopeDedup_SessionStartAvisaSinTocarLaConfiguracion(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("dedup-aviso")
	global := filepath.Join(s.Home, ".claude", "settings.json")
	proyecto := filepath.Join(p, ".claude", "settings.json")
	writeClaudeSettings(t, global, gomemorySubsGlobales)
	writeClaudeSettings(t, proyecto, gomemorySubsGlobales)
	mustMkdir(t, filepath.Join(p, ".memory"))
	globalAntes, proyectoAntes := readAll(t, global), readAll(t, proyecto)
	s.Setenv("CLAUDE_PROJECT_DIR", p)

	r := s.RunInput("", p, `{"session_id":"dup-1","source":"startup"}`, "hook", "session-start")
	var out struct {
		SystemMessage string `json:"systemMessage"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("con un aviso, session-start debe usar el sobre JSON: %v\n%s", err, firstRunes(r.Stdout, 300))
	}
	if !strings.Contains(out.SystemMessage, "hooks duplicados") || !strings.Contains(out.SystemMessage, "mem update") {
		t.Errorf("el aviso a la persona debe nombrar el problema y el remedio: %q", out.SystemMessage)
	}
	if readAll(t, global) != globalAntes || readAll(t, proyecto) != proyectoAntes {
		t.Error("session-start nunca escribe la configuración del host")
	}
}
