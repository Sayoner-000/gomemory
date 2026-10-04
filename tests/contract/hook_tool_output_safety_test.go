package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// T045 (feature 035, US2): contra el binario real y las salidas reales que el
// hook destruyó en vivo (código Go sin indentación, «omitidos 23 de 30», una
// línea de mem doctor perdida), lo que el agente pidió llega intacto.
func TestHookToolOutputSafety(t *testing.T) {
	bin := buildLifecycleBinary(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".memory", "settings.json"), `{"tool_output_compression": true}`)
	data := t.TempDir()

	run := func(fixture string) (in map[string]any, out []byte) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(repoRootContract(t), "tests", "contract", "testdata", "tool_output", fixture))
		if err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(raw, &in)
		cmd := exec.Command(bin, "hook", "tool-output", "claude")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOMEMORY_DATA_HOME="+data, "GOMEMORY_NO_UPDATE_CHECK=1")
		cmd.Stdin = bytes.NewReader(raw)
		out, err = cmd.Output()
		if err != nil {
			t.Fatalf("%s: el hook debe terminar con código 0: %v", fixture, err)
		}
		return in, out
	}

	for _, f := range []string{"claude-bash-sed-go.json", "claude-bash-doctor.json"} {
		if _, out := run(f); len(out) != 0 {
			t.Errorf("%s: una lectura exacta no se reescribe; hubo %d bytes", f, len(out))
		}
	}

	in, out := run("claude-bash-awk-go.json")
	if len(out) > 0 {
		var got struct {
			HSO struct {
				Upd struct {
					Stdout string `json:"stdout"`
				} `json:"updatedToolOutput"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("salida no JSON: %s", out)
		}
		orig := in["tool_response"].(map[string]any)["stdout"].(string)
		lines := strings.Split(orig, "\n")
		i := 0
		for _, l := range strings.Split(got.HSO.Upd.Stdout, "\n") {
			if strings.Contains(l, "⟦mem⟧") || strings.HasPrefix(l, "> Marcas") || l == "" {
				continue
			}
			for i < len(lines) && lines[i] != l {
				i++
			}
			if i == len(lines) {
				t.Fatalf("línea de código alterada (indentación incluida): %q", l)
			}
			i++
		}
		if strings.Contains(got.HSO.Upd.Stdout, "frases omitidas") {
			t.Error("una sentencia de código nunca se sustituye por «frases omitidas»")
		}
	}

	if _, out := run("claude-mcp-search30.json"); len(out) > 0 {
		if n := strings.Count(string(out), `\"node\"`); n != 30 || strings.Contains(string(out), "omitidos") {
			t.Errorf("los 30 resultados deben llegar completos (hay %d)", n)
		}
	}
}
