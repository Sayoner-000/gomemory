package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// HCD-032: el delta de una revisión enlazada son los archivos que cambiaron
// entre los commits de la revisión anterior y la nueva.

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChangedFiles(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, dir, "domain/rules.go", "package domain\n")
	write(t, dir, "adapters/x.go", "package adapters\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "uno")
	from := git(t, dir, "rev-parse", "HEAD")
	write(t, dir, "domain/rules.go", "package domain\n// cambio\n")
	write(t, dir, "docs/nuevo.md", "x\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "dos")
	to := git(t, dir, "rev-parse", "HEAD")

	d := New(dir)
	got, err := d.ChangedFiles(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"docs/nuevo.md", "domain/rules.go"}) {
		t.Fatalf("solo los archivos cambiados, ordenados: %v", got)
	}
	if same, err := d.ChangedFiles(to, to); err != nil || len(same) != 0 {
		t.Fatalf("entre el mismo commit no hay delta: %v %v", same, err)
	}
	for _, bad := range [][2]string{{"no-es-un-commit", to}, {from, "worktree"}, {"", to}} {
		if _, err := d.ChangedFiles(bad[0], bad[1]); err == nil {
			t.Errorf("%v: una revisión que no es commit debe dar error", bad)
		}
	}
	if _, err := New(t.TempDir()).ChangedFiles(from, to); err == nil {
		t.Error("fuera de un repositorio git debe dar error")
	}
}

// HCD-035: el contenido de texto del alcance en una revisión, para medir el
// ahorro. Los binarios no cuentan y lo que está fuera del alcance tampoco.
func TestTextFilesAt(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, dir, "domain/rules.go", "package domain\n")
	write(t, dir, "domain/logo.png", "\x89PNG\x00\x01")
	write(t, dir, "docs/a.md", "doc\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "uno")
	rev := git(t, dir, "rev-parse", "HEAD")
	write(t, dir, "domain/rules.go", "package domain\n// sin commit\n")

	d := New(dir)
	got, err := d.TextFilesAt(rev, []string{"domain"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["domain/rules.go"] != "package domain\n" {
		t.Fatalf("solo el texto del alcance, tal como está en la revisión: %q", got)
	}
	all, err := d.TextFilesAt(rev, []string{"."})
	if err != nil || len(all) != 2 {
		t.Fatalf("alcance «.»: %v %v", all, err)
	}
	if _, err := d.TextFilesAt("worktree", []string{"."}); err == nil {
		t.Error("una revisión que no es commit debe dar error")
	}
}
