package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandWiringAgy(t *testing.T) {
	cases := []struct {
		name, command string
		edit          func(map[string]any)
		want          bool
	}{
		{name: "complete", want: true},
		{name: "quoted path", command: `"/Program Files/gomemory/mem"`, want: true},
		{name: "windows path", command: `"C:\Program Files\gomemory\mem.exe"`, want: true},
		{name: "single quoted path", command: `'/home/user/my tools/mem'`, want: true},
		{name: "disabled", edit: func(g map[string]any) { g["enabled"] = false }},
		{name: "enabled missing", edit: func(g map[string]any) { delete(g, "enabled") }},
		{name: "enabled string", edit: func(g map[string]any) { g["enabled"] = "true" }},
		{name: "third party", command: "other"},
		{name: "incidental text", command: "echo mem"},
		{name: "comment", command: "# mem"},
		{name: "echo quoted", command: `echo "mem`},
		{name: "wrong event", edit: func(g map[string]any) {
			g["Stop"].([]any)[0].(map[string]any)["command"] = "mem hook agy SessionStart"
		}},
		{name: "suffix", edit: func(g map[string]any) {
			g["Stop"].([]any)[0].(map[string]any)["command"] = "mem hook agy StopExtra"
		}},
		{name: "echo full invocation", edit: func(g map[string]any) {
			g["Stop"].([]any)[0].(map[string]any)["command"] = `echo "mem hook agy Stop"`
		}},
		{name: "wrong type", edit: func(g map[string]any) {
			g["Stop"].([]any)[0].(map[string]any)["type"] = "prompt"
		}},
		{name: "malformed entry", edit: func(g map[string]any) { g["Stop"] = []any{nil, "mem hook agy Stop"} }},
		{name: "insufficient matcher", edit: func(g map[string]any) {
			g["PreToolUse"].([]any)[0].(map[string]any)["matcher"] = "run_command"
		}},
		{name: "invalid matcher", edit: func(g map[string]any) {
			g["PreToolUse"].([]any)[0].(map[string]any)["matcher"] = "["
		}},
		{name: "broad matcher", want: true, edit: func(g map[string]any) {
			g["PreToolUse"].([]any)[0].(map[string]any)["matcher"] = ".*"
		}},
		{name: "split matchers", want: true, edit: func(g map[string]any) {
			g["PreToolUse"] = append(agyToolHook(AgentRef{HookCommand: "mem"}, "PreToolUse", "run_command"),
				agyToolHook(AgentRef{HookCommand: "mem"}, "PreToolUse", "write_file|edit_file|invoke_subagent")...)
		}},
		{name: "nested hook missing", edit: func(g map[string]any) {
			delete(g["PostToolUse"].([]any)[0].(map[string]any), "hooks")
		}},
	}
	for _, event := range []string{"SessionStart", "PreInvocation", "PreToolUse", "PostToolUse", "Stop"} {
		cases = append(cases, struct {
			name, command string
			edit          func(map[string]any)
			want          bool
		}{name: "missing " + event, edit: func(g map[string]any) { delete(g, event) }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, root := t.TempDir(), t.TempDir()
			command := tc.command
			if command == "" {
				command = "mem"
			}
			group := agyLifecycleGroup(AgentRef{HookCommand: command})
			if tc.edit != nil {
				tc.edit(group)
			}
			data, err := json.Marshal(map[string]any{agyHookGroup: group})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(agyConfigDir(home), "hooks.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			wiring := ExpandWiring(home, root)
			if got, exists := wiring["agy"]; !exists || got != tc.want || len(wiring) != 4 {
				t.Fatalf("wiring=%v, want agy=%t", wiring, tc.want)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatalf("el diagnóstico modificó la configuración: %v", err)
			}
		})
	}
	for _, mode := range []string{"absent", "invalid", "wrong group", "read error"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(agyConfigDir(home), "hooks.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "invalid", "wrong group":
				data := []byte("{")
				if mode == "wrong group" {
					data = []byte(`{"other":{"enabled":true}}`)
				}
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			case "read error":
				// Leer un directorio falla también en Windows y bajo root.
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if wiring := ExpandWiring(home, t.TempDir()); wiring["agy"] || len(wiring) != 4 {
				t.Fatalf("wiring=%v", wiring)
			}
		})
	}
}
