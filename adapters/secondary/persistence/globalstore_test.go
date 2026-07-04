package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectKeyIsDeterministic(t *testing.T) {
	root := "/home/user/projects/foo"
	if ProjectKey(root) != ProjectKey(root) {
		t.Fatal("ProjectKey debe ser determinística para la misma ruta")
	}
}

func TestProjectKeyDoesNotCollideOnSharedFolderName(t *testing.T) {
	keyA := ProjectKey("/home/user/group1/shared-name")
	keyB := ProjectKey("/home/user/group2/shared-name")
	if keyA == keyB {
		t.Fatalf("dos rutas distintas con el mismo nombre de carpeta final no deben producir la misma clave: %q", keyA)
	}
}

func TestProjectKeySanitizesSpecialCharacters(t *testing.T) {
	key := ProjectKey("/home/user/my project (v2)!")
	for _, r := range key {
		isSafe := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
		if !isSafe {
			t.Fatalf("ProjectKey %q contiene un carácter no seguro para filesystem: %q", key, r)
		}
	}
}

func TestDataHomeRespectsExplicitOverride(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, "/tmp/gomemory-override-test")
	home, err := DataHome()
	if err != nil {
		t.Fatalf("DataHome: %v", err)
	}
	if home != "/tmp/gomemory-override-test" {
		t.Fatalf("esperaba respetar GOMEMORY_DATA_HOME, obtuve %q", home)
	}
}

func TestDataHomeFallsBackToXDGDataHome(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-test")
	home, err := DataHome()
	if err != nil {
		t.Fatalf("DataHome: %v", err)
	}
	if home != filepath.Join("/tmp/xdg-test", "gomemory") {
		t.Fatalf("esperaba %q, obtuve %q", filepath.Join("/tmp/xdg-test", "gomemory"), home)
	}
}

func TestGlobalDbPathIsUnderProjectsSubdir(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, "/tmp/gomemory-datahome-test")
	path, err := GlobalDbPath("myproject-abcd1234")
	if err != nil {
		t.Fatalf("GlobalDbPath: %v", err)
	}
	want := filepath.Join("/tmp/gomemory-datahome-test", "projects", "myproject-abcd1234", "mem.db")
	if path != want {
		t.Fatalf("esperaba %q, obtuve %q", want, path)
	}
}

func TestFindProjectRootUsesGitRoot(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, ".git"), 0755); err != nil {
		t.Fatalf("crear .git: %v", err)
	}
	sub := filepath.Join(base, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("crear subdir: %v", err)
	}

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(sub); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	resolvedBase, _ := filepath.EvalSymlinks(base)
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if resolvedRoot != resolvedBase {
		t.Fatalf("esperaba root=%q (git root), obtuve %q", resolvedBase, resolvedRoot)
	}
}

func TestFindProjectRootFallsBackToCwdWithoutGit(t *testing.T) {
	dir := t.TempDir()

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if resolvedRoot != resolvedDir {
		t.Fatalf("esperaba root=%q (cwd, sin .git), obtuve %q", resolvedDir, resolvedRoot)
	}
}
