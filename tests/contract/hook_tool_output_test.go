package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// T057 — contrato de `mem hook tool-output` contra el binario real y los
// fixtures de tests/contract/testdata/tool_output.
func TestHookToolOutput(t *testing.T) {
	root := repoRootContract(t)
	bin := filepath.Join(t.TempDir(), "mem-hook-tool-output")
	build := exec.Command("go", "build", "-o", bin, "./infrastructure")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar: %v\n%s", err, out)
	}
	proyecto := func(settings string) string {
		dir := t.TempDir()
		_ = os.MkdirAll(filepath.Join(dir, ".memory"), 0o700)
		_ = os.WriteFile(filepath.Join(dir, ".memory", "settings.json"), []byte(settings), 0o600)
		return dir
	}
	on := proyecto(`{"tool_output_compression": true}`)
	off := proyecto(`{}`)
	fixture := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(root, "tests", "contract", "testdata", "tool_output", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	run := func(dir, runtime string, in []byte) ([]byte, time.Duration) {
		cmd := exec.Command(bin, "hook", "tool-output", runtime)
		cmd.Dir = dir
		cmd.Stdin = bytes.NewReader(in)
		start := time.Now()
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("el hook debe terminar con código 0 (H1): %v", err)
		}
		return out, time.Since(start)
	}
	keysOf := func(v any) []string {
		m, _ := v.(map[string]any)
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return ks
	}

	for _, c := range []struct{ fixture, runtime, field string }{
		{"claude-bash.json", "claude", "updatedToolOutput"},
		{"claude-grep.json", "claude", "updatedToolOutput"},
		{"claude-mcp.json", "claude", "updatedToolOutput"},
		{"codex-mcp.json", "codex", "updatedMCPToolOutput"},
	} {
		in := fixture(c.fixture)
		out, took := run(on, c.runtime, in)
		var ev struct {
			TR any `json:"tool_response"`
		}
		_ = json.Unmarshal(in, &ev)
		var got map[string]map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Errorf("%s: salida no JSON: %q", c.fixture, out)
			continue
		}
		upd := got["hookSpecificOutput"][c.field]
		if upd == nil {
			t.Errorf("%s: falta %s", c.fixture, c.field)
			continue
		}
		if reflect.TypeOf(upd) != reflect.TypeOf(ev.TR) || !reflect.DeepEqual(keysOf(upd), keysOf(ev.TR)) {
			t.Errorf("%s: H2 violada, forma distinta: %v frente a %v", c.fixture, keysOf(upd), keysOf(ev.TR))
		}
		updJSON, _ := json.Marshal(upd)
		origJSON, _ := json.Marshal(ev.TR)
		if len(updJSON) >= len(origJSON) {
			t.Errorf("%s: la salida no se redujo", c.fixture)
		}
		t.Logf("%s: %d → %d bytes en %v", c.fixture, len(origJSON), len(updJSON), took)
	}

	if out, _ := run(on, "opencode", fixture("opencode-bash.json")); !bytes.Contains(out, []byte(`"output":`)) || !bytes.Contains(out, []byte("⟦mem⟧")) {
		t.Errorf("opencode: %q", out)
	}
	for _, c := range []struct{ dir, fixture, runtime, why string }{
		{on, "claude-read.json", "claude", "Read excluida (H3)"},
		{on, "codex-shell.json", "codex", "Codex no reescribe shell"},
		{off, "claude-bash.json", "claude", "ajuste apagado"},
	} {
		if out, _ := run(c.dir, c.runtime, fixture(c.fixture)); len(out) != 0 {
			t.Errorf("%s: se esperaba salida vacía, hubo %d bytes", c.why, len(out))
		}
	}
	if out, _ := run(on, "claude", []byte("basura")); len(out) != 0 {
		t.Error("H1: entrada inválida → salida vacía")
	}
}
