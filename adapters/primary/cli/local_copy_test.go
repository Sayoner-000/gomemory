package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mem/adapters/primary/console"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("las copias simuladas son scripts de shell")
	}
}

// fakeMem escribe en path un ejecutable que responde a `version` con out.
func fakeMem(t *testing.T, path, out string) {
	t.Helper()
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then printf '%s\\n' '" + out + "'; fi\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func globalFake(t *testing.T) string {
	t.Helper()
	g := filepath.Join(t.TempDir(), memBinaryName())
	fakeMem(t, g, "gomemory 2.26.4")
	return g
}

// FR-003: una copia de gomemory distinta del global se retira.
func TestRetireLocalCopy_CopiaDeGomemory(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	fakeMem(t, filepath.Join(root, memBinaryName()), "gomemory 2.8.0")

	got, retired, err := retireLocalCopy(root, globalFake(t))
	if err != nil || !retired {
		t.Fatalf("debía retirarse: retired=%v err=%v", retired, err)
	}
	if got.Version != "2.8.0" {
		t.Errorf("versión retirada = %q; quiero 2.8.0", got.Version)
	}
	if _, err := os.Stat(filepath.Join(root, memBinaryName())); !os.IsNotExist(err) {
		t.Fatal("la copia sigue en disco")
	}
}

// FR-003: nada que no sea una copia verificada de gomemory se toca.
func TestRetireLocalCopy_NoTocaLoAjeno(t *testing.T) {
	skipOnWindows(t)
	global := globalFake(t)

	cases := map[string]func(t *testing.T, path string){
		"script propio": func(t *testing.T, path string) { fakeMem(t, path, "hola") },
		"versión sin semver": func(t *testing.T, path string) {
			fakeMem(t, path, "gomemory dev")
		},
		"enlace simbólico al global": func(t *testing.T, path string) {
			if err := os.Symlink(global, path); err != nil {
				t.Fatal(err)
			}
		},
		"directorio": func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"mismo archivo que el global": func(t *testing.T, path string) {
			if err := os.Link(global, path); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, memBinaryName())
			mk(t, path)
			if _, retired, _ := retireLocalCopy(root, global); retired {
				t.Fatal("no debía retirarse")
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatalf("el archivo desapareció: %v", err)
			}
		})
	}
}

func TestRetireLocalCopy_SinGlobalNoRetira(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	fakeMem(t, filepath.Join(root, memBinaryName()), "gomemory 2.8.0")
	if _, retired, _ := retireLocalCopy(root, ""); retired {
		t.Fatal("sin global, la copia es la instalación legítima (FR-002)")
	}
}

func TestRetireLocalCopy_SinCopiaNoHaceNada(t *testing.T) {
	if _, retired, err := retireLocalCopy(t.TempDir(), "/no/importa"); retired || err != nil {
		t.Fatalf("sin copia: retired=%v err=%v", retired, err)
	}
}

// R2: un ejecutable que no responde no bloquea; se deja como está.
func TestRetireLocalCopy_TiempoLimite(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	path := filepath.Join(root, memBinaryName())
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, retired, _ := retireLocalCopy(root, globalFake(t))
	if retired {
		t.Fatal("un ejecutable que no responde no se identifica como gomemory")
	}
	if d := time.Since(start); d > localCopyVersionTimeout+2*time.Second {
		t.Fatalf("la identificación tardó %v; el límite es %v", d, localCopyVersionTimeout)
	}
}

// FR-004a y FR-015: el paso Binario del resumen de install refleja la
// retirada, y un fallo al retirar es ⚠ con la acción manual.
func TestInstallBinary_PasoDelResumenRefleLaRetirada(t *testing.T) {
	skipOnWindows(t)
	global := globalFake(t)
	t.Setenv("PATH", filepath.Dir(global))

	root := t.TempDir()
	copia := filepath.Join(root, memBinaryName())
	fakeMem(t, copia, "gomemory 2.8.0")
	var res console.StepResult
	captureStdout(t, func() { _, res = installBinary(root, global) })
	if res.Status != console.StepOK || !strings.Contains(res.Detail, "Se retiró "+copia+" (v2.8.0)") {
		t.Fatalf("retirada correcta: %+v", res)
	}

	bloqueado := t.TempDir()
	atascada := filepath.Join(bloqueado, memBinaryName())
	fakeMem(t, atascada, "gomemory 2.8.0")
	if err := os.Chmod(bloqueado, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bloqueado, 0o755) })
	captureStdout(t, func() { _, res = installBinary(bloqueado, global) })
	if res.Status != console.StepWarn || !strings.Contains(res.Manual, atascada) {
		t.Fatalf("un fallo al retirar debe ser ⚠ con la acción manual: %+v", res)
	}
}
