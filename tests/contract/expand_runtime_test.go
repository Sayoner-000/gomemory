package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandRuntimeContract(t *testing.T) {
	s := newLifecycleSandbox(t)
	root := s.Project("expand")
	s.globalSetup()
	mustWrite(t, filepath.Join(root, ".gomemory", "rules", "warn.md"), "---\nevent: prompt\npattern: marker\naction: warn\n---\nreview marker\n")
	run := func(input string, args ...string) lifecycleResult {
		t.Helper()
		return s.RunInput("", root, input, args...)
	}
	for _, tc := range []struct {
		input string
		args  []string
		code  int
		want  string
	}{
		{"", []string{"expand", "status", "--json"}, 0, `"wiring"`},
		{"", []string{"expand"}, 0, "destructive-shell"},
		{"", []string{"expand", "rules"}, 0, "warn"},
		{"git push --force", []string{"expand", "rules", "--check", "--event", "bash", "--json"}, 2, `"block"`},
		{"marker", []string{"hook", "expand", "bogus"}, 0, ""},
		{`{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/synthetic"}}`, []string{"hook", "expand", "pre-tool", "--emit=json"}, 0, "deny"},
		{`{"prompt":"marker","session_id":"warn"}`, []string{"hook", "expand", "prompt", "--emit=json"}, 0, "review marker"},
		{`{"tool_name":"Bash","session_id":"flow","tool_input":{"command":"git commit --dry-run"}}`, []string{"hook", "expand", "pre-tool", "--emit=json"}, 0, "verify-loop"},
		{`{"tool_name":"Bash","session_id":"flow","tool_input":{"command":"git commit --dry-run"}}`, []string{"hook", "expand", "pre-tool", "--emit=json"}, 0, "{}"},
		{`{"source":"compact","session_id":"flow"}`, []string{"hook", "expand", "session-start", "--emit=json"}, 0, ""},
		{"not json", []string{"hook", "expand", "pre-tool", "--emit=text"}, 0, ""},
		{"", []string{"expand", "off", "instincts"}, 0, ""},
		{`{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}`, []string{"hook", "expand", "post-tool"}, 0, ""},
		{"", []string{"expand", "on"}, 0, "todas"},
	} {
		r := run(tc.input, tc.args...)
		if r.ExitCode != tc.code || !strings.Contains(r.Stdout, tc.want) {
			t.Fatalf("%v: %d %s %s", tc.args, r.ExitCode, r.Stdout, r.Stderr)
		}
	}
	for range 16 {
		r := run(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}`, "hook", "expand", "post-tool")
		if r.ExitCode != 0 || r.Stdout != "" || r.Stderr != "" {
			t.Fatalf("post-tool: %+v", r)
		}
	}
	if r := run(`{"last_assistant_message":"done"}`, "hook", "expand", "stop"); r.ExitCode != 0 {
		t.Fatal(r)
	}
	r := run("", "expand", "instincts", "list", "--json")
	var list []struct {
		Slug         string
		Observations int
	}
	if json.Unmarshal([]byte(r.Stdout), &list) != nil || len(list) != 1 || list[0].Observations != 16 {
		t.Fatalf("fold: %+v", r)
	}
	if r := run("", "expand", "instincts", "confirm", "cmd:go-test"); r.ExitCode != 0 {
		t.Fatal(r)
	}
	if r := run("", "expand", "instincts", "list", "--all"); !strings.Contains(r.Stdout, "confirmed") {
		t.Fatal(r)
	}
	if r := run("", "expand", "instincts", "evolve", "cmd:go-test"); r.ExitCode != 2 {
		t.Fatal(r)
	}
	if r := run("", "doctor", "--shield", "--json"); r.ExitCode != 0 || !strings.Contains(r.Stdout, `"shield"`) {
		t.Fatal(r)
	}
}
