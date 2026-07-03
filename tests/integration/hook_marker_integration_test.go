package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
)

// buildMemBinary compila el binario mem una sola vez por corrida de tests y
// devuelve su ruta. hookSessionStart/hookUserPromptSubmit terminan con
// os.Exit(0), así que probarlas requiere un subproceso real (llamarlas
// in-process mataría el binario de test).
var (
	memBinOnce sync.Once
	memBinPath string
	memBinErr  error
)

func buildMemBinary(t *testing.T) string {
	t.Helper()
	memBinOnce.Do(func() {
		// os.MkdirTemp (no t.TempDir()): debe sobrevivir a todos los tests del
		// binario, no solo al que dispara el sync.Once.
		dir, err := os.MkdirTemp("", "gomemory-test-bin-*")
		if err != nil {
			memBinErr = err
			return
		}
		memBinPath = filepath.Join(dir, "mem-test-bin")
		cmd := exec.Command("go", "build", "-o", memBinPath, "./infrastructure")
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			memBinErr = err
			t.Logf("go build output: %s", out)
		}
	})
	if memBinErr != nil {
		t.Fatalf("build mem binary: %v", memBinErr)
	}
	return memBinPath
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// tests/integration -> raíz del repo
	return filepath.Join(wd, "..", "..")
}

func runHook(t *testing.T, bin, dir, event string) string {
	t.Helper()
	cmd := exec.Command(bin, "hook", event)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("mem hook %s: %v (%s)", event, err, out.String())
	}
	return out.String()
}

// TestHookMarkerResetsPerSession verifica el fix del bug: el recordatorio de
// protocolo (marker .session-tools-injected) debía inyectarse una sola vez en
// toda la vida del proyecto porque nada lo borraba. Ahora session-start lo
// resetea, así que debe re-inyectarse en el primer prompt de CADA sesión.
func TestHookMarkerResetsPerSession(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	marker := filepath.Join(target, ".memory", ".session-tools-injected")

	// Sesión 1: session-start, luego dos prompts.
	runHook(t, bin, target, "session-start")
	first := runHook(t, bin, target, "user-prompt-submit")
	if !bytes.Contains([]byte(first), []byte("additionalContext")) {
		t.Fatalf("primer prompt de la sesión debía inyectar additionalContext, got: %s", first)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker debía existir tras el primer prompt: %v", err)
	}

	second := runHook(t, bin, target, "user-prompt-submit")
	if bytes.Contains([]byte(second), []byte("additionalContext")) {
		t.Fatalf("segundo prompt de la misma sesión NO debía re-inyectar, got: %s", second)
	}

	// Sesión 2: session-start debe resetear el marker.
	runHook(t, bin, target, "session-start")
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("marker debía eliminarse al iniciar una nueva sesión")
	}

	third := runHook(t, bin, target, "user-prompt-submit")
	if !bytes.Contains([]byte(third), []byte("additionalContext")) {
		t.Fatalf("primer prompt de la NUEVA sesión debía re-inyectar el recordatorio, got: %s", third)
	}
}

// TestHookSessionEndResetsMarker cubre el caso defensivo: cerrar sesión
// también debe resetear el marker (compactación/cierre sin un session-start
// intermedio no debe dejar el recordatorio inyectado para siempre).
func TestHookSessionEndResetsMarker(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	marker := filepath.Join(target, ".memory", ".session-tools-injected")

	runHook(t, bin, target, "session-start")
	runHook(t, bin, target, "user-prompt-submit")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker debía existir tras el primer prompt: %v", err)
	}

	runHook(t, bin, target, "session-end")
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("marker debía eliminarse al cerrar sesión")
	}
}
