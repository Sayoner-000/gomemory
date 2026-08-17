package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runDoctor(t *testing.T, bin, dir string, args ...string) (stdout string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"doctor"}, args...)...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("ejecutar mem doctor: %v", err)
		}
	}
	return out.String(), code
}

func TestDoctor_JSONEsEstableEntreEjecuciones(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	t.Setenv("HOME", t.TempDir())

	first, _ := runDoctor(t, bin, dir, "--json")
	second, _ := runDoctor(t, bin, dir, "--json")
	if first != second {
		t.Errorf("dos ejecuciones sin cambios deben producir el mismo JSON:\n%s\n---\n%s", first, second)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(first), &parsed); err != nil {
		t.Fatalf("--json no produjo JSON válido: %v (%q)", err, first)
	}
	if _, ok := parsed["channels"]; !ok {
		t.Error("falta el campo channels")
	}
	if _, ok := parsed["problems"]; !ok {
		t.Error("falta el campo problems")
	}
}

func TestDoctor_StrictExitCodeReflejaProblems(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	t.Setenv("HOME", t.TempDir())

	// Sin nada instalado: hay canales missing → problems > 0 → --strict falla.
	_, code := runDoctor(t, bin, dir, "--strict", "--json")
	if code == 0 {
		t.Error("--strict con canales missing debe salir con código != 0")
	}

	// Sin --strict, el mismo escenario sale con 0 (diagnóstico no rompe el flujo).
	_, codeNonStrict := runDoctor(t, bin, dir, "--json")
	if codeNonStrict != 0 {
		t.Errorf("sin --strict siempre debe salir con 0, got %d", codeNonStrict)
	}
}

func TestDoctor_CodegraphAusenteSinAvisos(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	t.Setenv("HOME", t.TempDir())

	out, _ := runDoctor(t, bin, dir, "--json")
	var parsed struct {
		Channels []struct {
			Arm string `json:"arm"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	for _, c := range parsed.Channels {
		if c.Arm == "codegraph" {
			t.Error("sin el brazo extensor instalado, no debe aparecer ningún canal codegraph")
		}
	}
}
