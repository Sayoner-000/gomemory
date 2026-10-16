package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// runSocraticaHook ejecuta `mem hook <event>` con el payload por stdin.
func runSocraticaHook(t *testing.T, bin, dir string, env []string, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("mem %v: %v (%s)", args, err, errOut.String())
	}
	return out.String()
}

// TestSocraticaHook_CapturaConElBinarioReal (US6): con la intercepción activada
// sin verificar, user-prompt-submit añade la directiva y socratica-decide la
// devuelve al plugin; apagada, ninguno de los dos captura.
func TestSocraticaHook_CapturaConElBinarioReal(t *testing.T) {
	bin := buildMemBinary(t)
	dir := dirDeProyecto(t)
	if err := persistence.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	env := []string{"GOMEMORY_DATA_HOME=" + t.TempDir(), "HOME=" + t.TempDir(), "GOMEMORY_NO_UPDATE_CHECK=1"}
	runSocraticaHook(t, bin, dir, env, "", "init")
	prompt := `{"prompt":"envía el informe trimestral al cliente antes del cierre"}`

	off := runSocraticaHook(t, bin, dir, env, prompt, "hook", "user-prompt-submit")
	if strings.Contains(off, "Socratica (intercepción automática)") {
		t.Fatalf("capturó con la intercepción apagada: %s", off)
	}
	runSocraticaHook(t, bin, dir, env, "", "socratica", "on", "--client", "claude", "--unverified")
	on := runSocraticaHook(t, bin, dir, env, prompt, "hook", "user-prompt-submit")
	var payload struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(on), &payload); err != nil || !strings.Contains(payload.HookSpecificOutput.AdditionalContext, "Socratica (intercepción automática)") {
		t.Fatalf("sin directiva con la intercepción activa: %v\n%s", err, on)
	}

	decideOff := runSocraticaHook(t, bin, dir, env, `{"prompt":"envía el informe trimestral al cliente antes del cierre","client":"opencode"}`, "hook", "socratica-decide")
	if !strings.Contains(decideOff, `"capture":false`) {
		t.Fatalf("opencode sin activar: %s", decideOff)
	}
	runSocraticaHook(t, bin, dir, env, "", "socratica", "on", "--client", "opencode", "--unverified")
	decide := runSocraticaHook(t, bin, dir, env, `{"prompt":">envía el informe","client":"opencode"}`, "hook", "socratica-decide")
	var d struct {
		Capture   bool   `json:"capture"`
		Forward   string `json:"forward"`
		Forwarded bool   `json:"forwarded"`
	}
	if err := json.Unmarshal([]byte(decide), &d); err != nil || d.Capture || !d.Forwarded || d.Forward != "envía el informe" {
		t.Fatalf("decisión con >: %v %s", err, decide)
	}
}
