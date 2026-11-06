package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
)

// HCD-036: dos agentes trabajan a la vez en el mismo proyecto. Cada uno es un
// proceso de host distinto (como dos Claude Code) que lanza sus comandos de
// gomemory como hijos directos. El cierre del agente B no debe quitarle al
// agente A su sesión, lo ya entregado ni la marca del bootstrap.

// hostStep es un comando que el host lanza como hijo directo, sin shell.
type hostStep struct {
	Args   []string `json:"args"`
	Stdin  string   `json:"stdin,omitempty"`
	Out    string   `json:"out,omitempty"`
	Signal string   `json:"signal,omitempty"` // archivo que se crea tras el paso
	Wait   string   `json:"wait,omitempty"`   // archivo que se espera antes del paso
}

// TestHCD036Host no es una prueba: es el proceso host simulado. Solo actúa
// cuando la prueba principal lo lanza con HCD036_HOST_STEPS.
func TestHCD036Host(t *testing.T) {
	raw := os.Getenv("HCD036_HOST_STEPS")
	if raw == "" {
		t.Skip("solo se ejecuta como host simulado de TestHCD036")
	}
	var steps []hostStep
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		t.Fatal(err)
	}
	// El host es el binario de pruebas y vuelve a pasar por TestMain, que le
	// asigna otro almacén y otro HOME: se restauran los de la prueba principal.
	for _, k := range []string{"GOMEMORY_DATA_HOME", "HOME", "USERPROFILE"} {
		_ = os.Setenv(k, os.Getenv("HCD036_"+k))
	}
	for _, s := range steps {
		if s.Wait != "" {
			deadline := time.Now().Add(60 * time.Second)
			for {
				if _, err := os.Stat(s.Wait); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("esperando %s", s.Wait)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}
		if len(s.Args) > 0 {
			cmd := exec.Command(os.Getenv("HCD036_MEM"), s.Args...)
			cmd.Stdin = strings.NewReader(s.Stdin)
			out, _ := cmd.Output()
			if s.Out != "" {
				if err := os.WriteFile(s.Out, out, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if s.Signal != "" {
			if err := os.WriteFile(s.Signal, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestHCD036(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el enlace por proceso del host se resuelve con ps; en Windows cada proceso usa su sesión local")
	}
	bin := buildMemBinary(t)
	repo, sync, dataHome, home := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	env := append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "GOMEMORY_DATA_HOME="+dataHome,
		"GOMEMORY_NO_CODE_REFRESH=1", "HCD036_MEM="+bin)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir, cmd.Env = repo, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mem %v: %v\n%s", args, err, out)
		}
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".memory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".memory", "settings.json"), []byte(`{"context_compression_level":"max"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tema := range []string{"capas", "pruebas", "persistencia"} {
		run("save", "-t", "Decisión sobre "+tema, "-y", "decision", "Se eligió una regla clara para "+tema+" porque reduce el acoplamiento y simplifica el mantenimiento.")
	}

	hook := func(event, session string, extra string) string {
		return `{"hook_event_name":"` + event + `","session_id":"` + session + `"` + extra + `}`
	}
	p := func(name string) string { return filepath.Join(sync, name) }
	host := func(steps []hostStep) *exec.Cmd {
		data, _ := json.Marshal(steps)
		cmd := exec.Command(os.Args[0], "-test.run=^TestHCD036Host$", "-test.count=1")
		cmd.Dir, cmd.Env = repo, append(env, "HCD036_HOST_STEPS="+string(data), "HCD036_GOMEMORY_DATA_HOME="+dataHome,
			"HCD036_HOME="+home, "HCD036_USERPROFILE="+home)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		return cmd
	}

	a := host([]hostStep{
		{Args: []string{"hook", "session-start"}, Stdin: hook("SessionStart", "agente-A", `,"source":"startup"`), Out: p("a-ss")},
		{Args: []string{"hook", "user-prompt-submit"}, Stdin: hook("UserPromptSubmit", "agente-A", `,"prompt":"empieza"`), Out: p("a-ups1")},
		{Args: []string{"context"}, Out: p("a-ctx1"), Signal: p("a-listo")},
		{Wait: p("b-hecho"), Args: []string{"hook", "user-prompt-submit"}, Stdin: hook("UserPromptSubmit", "agente-A", `,"prompt":"sigue"`), Out: p("a-ups2")},
		{Args: []string{"context"}, Out: p("a-ctx2")},
	})
	b := host([]hostStep{
		{Wait: p("a-listo"), Args: []string{"hook", "session-start"}, Stdin: hook("SessionStart", "agente-B", `,"source":"startup"`)},
		{Args: []string{"hook", "session-end"}, Stdin: hook("SessionEnd", "agente-B", ""), Signal: p("b-hecho")},
	})
	if err := b.Wait(); err != nil {
		t.Fatalf("host B: %v", err)
	}
	if err := a.Wait(); err != nil {
		t.Fatalf("host A: %v", err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(p(name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	// Línea base: antes de que B intervenga, A ya tiene su contexto entregado.
	if n := strings.Count(read("a-ctx1"), "ya entregad"); n == 0 {
		t.Fatalf("línea base: A debería ver «ya entregado» tras su arranque:\n%s", read("a-ctx1"))
	}
	// C-01 y C-04: la sesión de A, la primera que se abrió, sigue abierta tras el
	// arranque y el cierre de B; y A no ve menos «ya entregado» que antes.
	t.Setenv("GOMEMORY_DATA_HOME", dataHome)
	stores, _ := filepath.Glob(filepath.Join(dataHome, "projects", "*", persistence.DbName))
	if len(stores) != 1 {
		t.Fatalf("se esperaba un único almacén de proyecto: %v", stores)
	}
	db, err := persistence.OpenByKey(filepath.Base(filepath.Dir(stores[0])))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var abierta int
	if err := db.QueryRow(`SELECT ended_at IS NULL FROM sessions ORDER BY created_at, rowid LIMIT 1`).Scan(&abierta); err != nil {
		t.Fatal(err)
	}
	if abierta != 1 {
		t.Errorf("C-04: el cierre de B cerró la sesión de A")
	}
	if antes, despues := strings.Count(read("a-ctx1"), "ya entregad"), strings.Count(read("a-ctx2"), "ya entregad"); despues < antes {
		t.Errorf("C-01: tras la actividad de B, A ve menos contenido ya entregado (%d → %d):\n%s", antes, despues, read("a-ctx2"))
	}
	// C-02: el arranque y el cierre de B no le vuelven a mandar el bootstrap a A.
	if strings.Contains(read("a-ups2"), "PRIMERA ACCI") {
		t.Errorf("C-02: A recibió otra vez el bootstrap tras la actividad de B")
	}
}
