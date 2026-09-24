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

// TestDoctor_PluginV1EnOpenCode2MarcaCanalesYCuentaProblems cubre los
// hallazgos C-002 y C-003 de la ACR sobre la feature 032, de extremo a
// extremo con el binario: un plugin solo v1 con OpenCode 2.x instalado deja
// outdated los canales que sostiene (plan_entry y turn_reminder de
// opencode/user), y `problems` sigue siendo exactamente el número de canales
// outdated/duplicated/missing (contrato 019), sin sumas por fuera.
func TestDoctor_PluginV1EnOpenCode2MarcaCanalesYCuentaProblems(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	plugins := filepath.Join(home, ".config", "opencode", "plugins")
	if err := os.MkdirAll(plugins, 0755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	v1 := "export const GomemoryPlugin = async ({ $, directory, client }) => {\n  return {};\n};\n"
	if err := os.WriteFile(filepath.Join(plugins, "gomemory.ts"), []byte(v1), 0644); err != nil {
		t.Fatalf("escribir plugin: %v", err)
	}
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "opencode"), []byte("#!/bin/sh\necho 'opencode v2.0.16'\n"), 0755); err != nil {
		t.Fatalf("escribir opencode falso: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, _ := runDoctor(t, bin, dir, "--json")
	var parsed struct {
		Problems int `json:"problems"`
		Channels []struct {
			Agent, Scope, Kind, State, Detail string
		} `json:"channels"`
		OpenCode *struct {
			Compatible bool `json:"compatible"`
		} `json:"opencode"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("--json no produjo JSON válido: %v (%q)", err, out)
	}
	cuenta := 0
	marcados := 0
	for _, c := range parsed.Channels {
		switch c.State {
		case "outdated", "duplicated", "missing":
			cuenta++
		}
		if c.Agent == "opencode" && c.Scope == "user" && (c.Kind == "plan_entry" || c.Kind == "turn_reminder") {
			if c.State != "outdated" {
				t.Errorf("%s de opencode/user = %s (%q); want outdated", c.Kind, c.State, c.Detail)
			}
			marcados++
		}
	}
	if marcados != 2 {
		t.Errorf("esperaba plan_entry y turn_reminder de opencode/user; vistos=%d", marcados)
	}
	if parsed.Problems != cuenta {
		t.Errorf("problems=%d; los canales outdated/duplicated/missing son %d (contrato 019)", parsed.Problems, cuenta)
	}
	if parsed.OpenCode == nil || parsed.OpenCode.Compatible {
		t.Errorf("la sección opencode debía marcar el plugin como incompatible: %+v", parsed.OpenCode)
	}
}
