package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
)

// T023 (feature 035, US1, FR-002): con los hooks de gomemory registrados en
// usuario y proyecto, Claude Code ejecuta cada evento dos veces EN PARALELO con
// el mismo stdin. La segunda invocación debe salir en silencio; dos
// invocaciones secuenciales idénticas, en cambio, son legítimas.

// runHookConcurrente lanza n copias del mismo hook a la vez y devuelve sus
// salidas.
func runHookConcurrente(t *testing.T, bin, dir string, args []string, stdin string, n int) []string {
	t.Helper()
	cmds := make([]*exec.Cmd, n)
	outs := make([]*bytes.Buffer, n)
	for i := range cmds {
		outs[i] = &bytes.Buffer{}
		cmds[i] = exec.Command(bin, args...)
		cmds[i].Dir = dir
		cmds[i].Stdin = strings.NewReader(stdin)
		cmds[i].Stdout = outs[i]
	}
	for _, c := range cmds {
		if err := c.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
	}
	res := make([]string, n)
	for i, c := range cmds {
		if err := c.Wait(); err != nil {
			t.Fatalf("mem %v: %v", args, err)
		}
		res[i] = outs[i].String()
	}
	return res
}

// registrarHooksDuplicados reproduce la causa: los mismos hooks de gomemory en
// ~/.claude/settings.json y en el del proyecto. El HOME es el temporal que fija
// TestMain y lo comparten todas las pruebas, así que se limpia al terminar.
func registrarHooksDuplicados(t *testing.T, target string) {
	t.Helper()
	settings := `{"hooks":{"UserPromptSubmit":[{"matcher":"","hooks":[{"type":"command","command":"mem hook user-prompt-submit"}]}],` +
		`"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook turn-end"}]}]}}`
	home, _ := os.UserHomeDir()
	for _, dir := range []string{home, target} {
		if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(home, ".claude", "settings.json")) })
}

func TestHookReentry_ConcurrentesIdenticosSoloUnoProduceSalida(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	registrarHooksDuplicados(t, target)
	// Primer prompt aparte: crea el almacén y el marcador de la conversación.
	// En el dialecto json (Codex) cada turno lleva el recordatorio de plan, así
	// que la invocación procesada siempre tiene contenido y el duplicado no.
	runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"inicio"}`)
	payload := `{"session_id":"r1","prompt":"hola"}`
	outs := runHookConcurrente(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, payload, 2)

	conContenido, silencio := 0, 0
	for _, o := range outs {
		switch strings.TrimSpace(o) {
		case "{}":
			silencio++
		default:
			if strings.Contains(o, "additionalContext") {
				conContenido++
			}
		}
	}
	if conContenido != 1 || silencio != 1 {
		t.Fatalf("de dos invocaciones paralelas idénticas, una con contenido y otra en silencio; got %q", outs)
	}
	if doc := runMem(t, bin, target, "doctor"); !strings.Contains(doc, "duplicate_dropped") {
		t.Errorf("el duplicado descartado debe verse en mem doctor:\n%s", doc)
	}
}

func TestHookReentry_BloqueoViejoNoDaDosDuenos(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	registrarHooksDuplicados(t, target)
	runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"inicio"}`)
	payload := `{"prompt":"bloqueo viejo"}`
	sum := sha256.Sum256([]byte(payload))
	p := filepath.Join(target, ".memory", ".hook-lock-user-prompt-submit-"+hex.EncodeToString(sum[:])[:12])
	if err := os.WriteFile(p, []byte("archivo antiguo"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	outs := runHookConcurrente(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, payload, 2)
	silence := 0
	for _, out := range outs {
		if strings.TrimSpace(out) == "{}" {
			silence++
		}
	}
	if silence != 1 {
		t.Fatalf("un archivo viejo no debe dar dos dueños: %q", outs)
	}
}

func TestHookReentry_SecuencialesIdenticosSeProcesanAmbos(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	registrarHooksDuplicados(t, target)
	payload := `{"session_id":"r2","prompt":"sí"}`
	primero := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit"}, payload)
	if !strings.Contains(primero, "PRIMERA ACCIÓN") {
		t.Fatalf("el primer prompt debía llevar la carga de tools: %q", primero)
	}
	// El mismo prompt otra vez, ya terminada la primera invocación: no es un
	// duplicado sino un turno nuevo, y debe llegar al recordatorio por turno.
	segundo := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, payload)
	if !strings.Contains(segundo, "Si vas a entrar en modo plan") {
		t.Errorf("una invocación secuencial idéntica es legítima y debe procesarse: %q", segundo)
	}
}

func TestHookReentry_PromptIDDistingueGemeloTardioDeTurnoNuevo(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	registrarHooksDuplicados(t, target)
	one := `{"session_id":"s","prompt_id":"p-1","prompt":"igual"}`
	first := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, one)
	if !strings.Contains(first, "additionalContext") {
		t.Fatalf("la primera copia debe ejecutarse: %q", first)
	}
	time.Sleep(350 * time.Millisecond)
	if second := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, one); strings.TrimSpace(second) != "{}" {
		t.Fatalf("la copia tardía del mismo prompt debe callar: %q", second)
	}
	two := `{"session_id":"s","prompt_id":"p-2","prompt":"igual"}`
	if third := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, two); strings.TrimSpace(third) == "{}" {
		t.Fatal("otro prompt_id con el mismo texto debe procesarse")
	}
}

func TestHookReentry_TurnEndConcurrenteSeDescartaUnaVez(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	registrarHooksDuplicados(t, target)
	runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit"}, `{"prompt":"inicio"}`)
	payload := `{"files":["a.go"],"commands":["go test ./..."]}`
	runHookConcurrente(t, bin, target, []string{"hook", "turn-end", "--emit=text"}, payload, 2)
	if doc := runMem(t, bin, target, "doctor"); !strings.Contains(doc, "duplicate_dropped") {
		t.Errorf("el turn-end duplicado debe descartarse y registrarse:\n%s", doc)
	}
}

func TestHookReentry_SinDuplicacionNoHayGuarda(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"inicio"}`)
	outs := runHookConcurrente(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, `{"prompt":"p"}`, 2)
	for _, o := range outs {
		if strings.TrimSpace(o) == "{}" {
			t.Errorf("sin hooks registrados dos veces no hay duplicados que descartar: %q", outs)
		}
	}
}
