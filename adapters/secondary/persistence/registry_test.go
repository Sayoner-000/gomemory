package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"mem/domain"
)

// FR-013: install registra la ruta de cada proyecto en el almacén global, de
// forma idempotente y con permisos privados.
func TestRegisterProject_EscribeRootPrivadoEIdempotente(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, t.TempDir())
	root := t.TempDir()

	for i := 0; i < 2; i++ {
		if err := RegisterProject(root); err != nil {
			t.Fatalf("RegisterProject (%d): %v", i, err)
		}
	}
	dir, _ := GlobalProjectDir(ProjectKey(root))
	path := filepath.Join(dir, registryFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != filepath.Clean(root) {
		t.Errorf("root registrado = %q", data)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Errorf("permisos = %o; quiero 0600", info.Mode().Perm())
		}
	}

	regs, err := ListRegisteredProjects()
	if err != nil || len(regs) != 1 || regs[0].Root != filepath.Clean(root) || !regs[0].Exists {
		t.Fatalf("ListRegisteredProjects = %+v, %v", regs, err)
	}
}

// Un proyecto registrado que ya no existe queda huérfano: sus datos del
// almacén se retiran igualmente (caso límite de la spec).
func TestListRegisteredProjects_Huerfano(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, t.TempDir())
	root := filepath.Join(t.TempDir(), "borrado")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProject(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	regs, _ := ListRegisteredProjects()
	if len(regs) != 1 || regs[0].Exists {
		t.Fatalf("el registro de un proyecto borrado debe marcarse huérfano: %+v", regs)
	}
}

func mkProject(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".memory", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Aclaración 2026-09-26: escaneo con profundidad 6, sin seguir enlaces,
// omitiendo directorios pesados; solo cuenta .memory/settings.json.
func TestScanProjects(t *testing.T) {
	home := t.TempDir()
	mkProject(t, filepath.Join(home, "a"))
	mkProject(t, filepath.Join(home, "x", "y", "z", "b")) // profundidad 4
	mkProject(t, filepath.Join(home, "1", "2", "3", "4", "5", "6", "hondo"))
	mkProject(t, filepath.Join(home, "app", "node_modules", "dep"))
	mkProject(t, filepath.Join(home, "repo", ".git", "falso"))
	if err := os.MkdirAll(filepath.Join(home, "solo-memory", ".memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		fuera := t.TempDir()
		mkProject(t, filepath.Join(fuera, "enlazado"))
		if err := os.Symlink(fuera, filepath.Join(home, "enlace")); err != nil {
			t.Fatal(err)
		}
	}

	found, _, err := ScanProjects(home, domain.UninstallScanMaxDepth)
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	for _, f := range found {
		r, _ := filepath.Rel(home, f)
		rel = append(rel, filepath.ToSlash(r))
	}
	sort.Strings(rel)
	want := []string{"a", "x/y/z/b"}
	if strings.Join(rel, ",") != strings.Join(want, ",") {
		t.Fatalf("encontrados = %v; quiero %v", rel, want)
	}
}

// Un directorio sin permiso de lectura se omite y se informa; el resto sigue.
func TestScanProjects_SinPermisoSeOmite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permisos de Unix")
	}
	home := t.TempDir()
	mkProject(t, filepath.Join(home, "ok"))
	cerrado := filepath.Join(home, "cerrado")
	mkProject(t, filepath.Join(cerrado, "dentro"))
	if err := os.Chmod(cerrado, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cerrado, 0o755) })

	found, skipped, err := ScanProjects(home, domain.UninstallScanMaxDepth)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || filepath.Base(found[0]) != "ok" {
		t.Errorf("encontrados = %v", found)
	}
	if len(skipped) != 1 || skipped[0] != cerrado {
		t.Errorf("omitidos = %v; quiero [%s]", skipped, cerrado)
	}
}
