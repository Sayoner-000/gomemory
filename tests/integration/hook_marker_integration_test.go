package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestHookUserPromptSubmit_BootstrapVaEnAdditionalContext cubre el bug real:
// Claude Code muestra el campo top-level "systemMessage" SOLO al humano en la
// terminal — nunca lo inyecta en el contexto del modelo (confirmado contra la
// documentación oficial de hooks). El bootstrap de ToolSearch que fuerza la
// carga de gomemory + codebase-memory-mcp vivía en "systemMessage", así que el
// agente jamás ejecutaba el ToolSearch forzado: la instrucción "PRIMERA ACCIÓN"
// nunca llegó a su contexto en ninguna sesión, solo se mostraba en la UI del
// usuario. Este test falla si el bootstrap vuelve a viajar por ese campo.
func TestHookUserPromptSubmit_BootstrapVaEnAdditionalContext(t *testing.T) {
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

	runHook(t, bin, target, "session-start")
	out := runHook(t, bin, target, "user-prompt-submit")

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida del hook no es JSON válido: %v\n%s", err, out)
	}

	if _, present := payload["systemMessage"]; present {
		t.Fatalf("el bootstrap de ToolSearch NO debe viajar en systemMessage (el modelo nunca lo ve): %s", out)
	}

	hso, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("esperaba hookSpecificOutput en la salida: %s", out)
	}
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "select:") || !strings.Contains(ctx, "mcp__gomemory__") {
		t.Fatalf("additionalContext debía incluir el select: de ToolSearch con las tools de gomemory: %q", ctx)
	}
}

// TestHookUserPromptSubmit_NudgeDeGuardadoVaEnAdditionalContext cubre el mismo
// bug para el recordatorio de guardado (prompts subsiguientes de la sesión):
// saveNudgeMessage está dirigido al agente en segunda persona ("llama a
// save_memory ahora"), así que debe viajar en additionalContext, no en
// systemMessage.
func TestHookUserPromptSubmit_NudgeDeGuardadoVaEnAdditionalContext(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	runHook(t, bin, target, "session-start")
	runHook(t, bin, target, "user-prompt-submit") // primer prompt: pone el marker

	// Empuja la sesión al pasado para superar el umbral de "overdue" (15 min)
	// sin esperar en tiempo real, y limpia el debounce por si el propio hook ya
	// escribió un timestamp de nudge.
	if _, err := db.Exec("UPDATE sessions SET created_at = datetime(created_at, '-1000 seconds')"); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
	db.Close()
	os.Remove(filepath.Join(target, ".memory", ".last-nudge"))

	out := runHook(t, bin, target, "user-prompt-submit") // segundo prompt: rama del nudge

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida del hook no es JSON válido: %v\n%s", err, out)
	}
	if _, present := payload["systemMessage"]; present {
		t.Fatalf("el recordatorio de guardado NO debe viajar en systemMessage (el modelo nunca lo ve): %s", out)
	}
	hso, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("esperaba hookSpecificOutput con el recordatorio de guardado: %s", out)
	}
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "save_memory") {
		t.Fatalf("additionalContext debía incluir el recordatorio de save_memory: %q", ctx)
	}
}
