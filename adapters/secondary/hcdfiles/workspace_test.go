package hcdfiles

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		run(t, root, "git", args...)
	}
	write(t, root, "a.go", "package a\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-qm", "init")
	return root
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintTracksCodeButNotHCDDocs(t *testing.T) {
	root := gitRepo(t)
	w, ctx := Workspace{Root: root}, context.Background()
	base, err := w.Fingerprint(ctx)
	if err != nil || base == "nogit" {
		t.Fatalf("fingerprint = %q, %v", base, err)
	}
	if err := w.WriteDoc("hcd/HCD-001-x.md", []byte("# x\n")); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.Fingerprint(ctx); got != base {
		t.Fatal("writing hcd/ changed the fingerprint")
	}
	write(t, root, "a.go", "package a // cambio\n")
	modified, _ := w.Fingerprint(ctx)
	if modified == base {
		t.Fatal("modifying tracked code did not change the fingerprint")
	}
	write(t, root, "b.go", "package a\n")
	if untracked, _ := w.Fingerprint(ctx); untracked == modified {
		t.Fatal("an untracked file did not change the fingerprint")
	}
	if got, _ := (Workspace{Root: t.TempDir()}).Fingerprint(ctx); got != "nogit" {
		t.Fatalf("outside git = %q", got)
	}
}

func TestPathsStayInsideAndRejectSymlinks(t *testing.T) {
	root := gitRepo(t)
	w := Workspace{Root: root}
	for _, bad := range []string{"../x.md", "hcd/../a.go", "/etc/passwd", "a.go", "hcd//x.md"} {
		if err := w.WriteDoc(bad, []byte("x")); err == nil {
			t.Errorf("WriteDoc(%q) accepted", bad)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "hcd")); err != nil {
		t.Skip("symlinks not supported")
	}
	if err := w.WriteDoc("hcd/x.md", []byte("x")); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlinked hcd/ accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "x.md")); err == nil {
		t.Fatal("wrote through the symlink")
	}
}

func TestRunReportsExitCodeAndTail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el caso usa un bucle de sh; en Windows Run delega en cmd /C")
	}
	w, ctx := Workspace{Root: t.TempDir()}, context.Background()
	code, out, err := w.Run(ctx, "echo ok")
	if err != nil || code != 0 || out != "ok" {
		t.Fatalf("Run ok = %d %q %v", code, out, err)
	}
	code, out, err = w.Run(ctx, "for i in $(seq 1 50); do echo l$i; done; exit 3")
	if err != nil || code != 3 || !strings.HasSuffix(out, "l50") || strings.Contains(out, "l1\n") {
		t.Fatalf("Run fail = %d %q %v", code, out, err)
	}
}

// HCD-043: la salida se protege entera antes de recortarla; si se recortara
// primero, la apertura <private> quedaría fuera del extracto y el contenido
// privado llegaría a la evidencia.
func TestRunProtegeLaSalidaAntesDeRecortar(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el caso usa printf y un bucle de sh; en Windows Run delega en cmd /C")
	}
	w, ctx := Workspace{Root: t.TempDir()}, context.Background()
	cases := map[string]struct{ command, canary string }{
		"apertura a más de 2000 bytes": {
			command: `printf '<private>'; printf '%2500s' 'SYNTH-PRIVATE-0001'; printf '</private>\npublico-final\n'`,
			canary:  "SYNTH-PRIVATE-0001",
		},
		"apertura a más de 20 líneas": {
			command: `echo '<private>'; for i in $(seq 1 30); do echo SYNTH-PRIVATE-0002-$i; done; echo '</private>'; echo publico-final`,
			canary:  "SYNTH-PRIVATE-0002",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			code, out, err := w.Run(ctx, c.command)
			if err != nil || code != 0 {
				t.Fatalf("Run = %d %v", code, err)
			}
			if strings.Contains(out, c.canary) {
				t.Fatalf("el extracto contiene el canario privado: %q", out)
			}
			if !strings.HasSuffix(out, "publico-final") {
				t.Fatalf("el extracto perdió el texto público final: %q", out)
			}
		})
	}
}

// HCD-045: al vencer el plazo, Run termina también a los descendientes que
// heredan la salida y vuelve dentro del plazo más el margen.
func TestRunTerminaLosDescendientesAlVencerElPlazo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el caso usa sleep y & de sh; en Windows Run delega en cmd /C")
	}
	root := t.TempDir()
	w := Workspace{Root: root, Timeout: 100 * time.Millisecond}
	started := time.Now()
	code, out, err := w.Run(context.Background(), "(sleep 2; touch canario-tardio) & wait")
	elapsed := time.Since(started)
	if err != nil || code != -1 || !strings.Contains(out, "tiempo límite agotado") {
		t.Fatalf("Run = %d %q %v, quiero -1 con aviso de tiempo agotado", code, out, err)
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("Run tardó %s con un plazo de 100ms", elapsed)
	}
	time.Sleep(2500*time.Millisecond - elapsed)
	if _, err := os.Stat(filepath.Join(root, "canario-tardio")); err == nil {
		t.Fatal("un descendiente siguió vivo y escribió después del plazo")
	}
}

func TestConfigureProcessTree_PidInvalido(t *testing.T) {
	cmd := exec.Command("echo", "test")
	configureProcessTree(cmd)
	if cmd.Cancel == nil {
		t.Fatal("configureProcessTree no configuró cmd.Cancel")
	}
	// Sin cmd.Process asignado:
	if err := cmd.Cancel(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("cmd.Cancel() sin Process = %v, quiero ErrProcessDone", err)
	}
	// Con cmd.Process con Pid <= 0:
	cmd.Process = &os.Process{Pid: 0}
	if err := cmd.Cancel(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("cmd.Cancel() con Pid 0 = %v, quiero ErrProcessDone", err)
	}
	cmd.Process = &os.Process{Pid: -1}
	if err := cmd.Cancel(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("cmd.Cancel() con Pid -1 = %v, quiero ErrProcessDone", err)
	}
}

func TestWorkspace_MaxUnitNumber(t *testing.T) {
	root := t.TempDir()
	w := Workspace{Root: root}
	if maxNum, err := w.MaxUnitNumber(); err != nil || maxNum != 0 {
		t.Fatalf("sin hcd/ expected 0, nil; got %d, %v", maxNum, err)
	}
	write(t, root, "hcd/HCD-001-algo.md", "# 1")
	write(t, root, "hcd/HCD-005-otro.md", "# 5")
	write(t, root, "hcd/HCD-029-ultimo.md", "# 29")
	write(t, root, "hcd/README.md", "# readme")
	write(t, root, "hcd/draft.txt", "draft")

	maxNum, err := w.MaxUnitNumber()
	if err != nil {
		t.Fatalf("MaxUnitNumber error: %v", err)
	}
	if maxNum != 29 {
		t.Fatalf("expected 29, got %d", maxNum)
	}
}

// HCD-034: PathExists responde solo dentro de la raíz del proyecto.
func TestWorkspacePathExists(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	write(t, root, "domain/a.go", "package domain\n")
	write(t, parent, "fuera.go", "package fuera\n")
	w := Workspace{Root: root}
	for rel, want := range map[string]bool{
		"domain/a.go":  true,
		"domain":       true,
		"domain/b.go":  false,
		"../fuera.go":  false,
		"/etc/hosts":   false,
		"":             false,
		"domain/../..": false,
	} {
		if got := w.PathExists(rel); got != want {
			t.Errorf("PathExists(%q) = %v, want %v", rel, got, want)
		}
	}
}
