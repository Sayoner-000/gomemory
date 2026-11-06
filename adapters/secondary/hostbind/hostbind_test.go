package hostbind

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// HCD-036: el enlace por proceso del host. Tabla de procesos simulada:
//
//	1 launchd
//	├── 100 claude (agente A)  ← 200 zsh ← 300 mem hook   · 400 mem mcp · 600 zsh ← 500 mem context
//	└── 110 claude (agente B)  ← 310 mem hook             · 410 mem mcp
type proc struct {
	ppid  int
	start string
	comm  string
}

func tabla() map[int]proc {
	return map[int]proc{
		1:   {0, "t0", "launchd"},
		100: {1, "t1", "claude"},
		110: {1, "t2", "claude"},
		200: {100, "t3", "/bin/zsh"},
		300: {200, "t4", "mem"},
		310: {110, "t5", "mem"},
		400: {100, "t6", "mem"},
		410: {110, "t7", "mem"},
		600: {100, "t8", "-zsh"},
		500: {600, "t9", "mem"},
	}
}

func store(t *testing.T, dir string, procs map[int]proc, self int) *Store {
	t.Helper()
	s := New(dir)
	s.self = func() int { return self }
	s.info = func(pid int) (int, string, string, error) {
		p, ok := procs[pid]
		if !ok {
			return 0, "", "", errors.New("no existe")
		}
		return p.ppid, p.start, p.comm, nil
	}
	return s
}

// psInfo contra el sistema real: el padre que da ps es el de Go.
func TestHostBinding_PsReal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sin ps en Windows: cada proceso usa su conversación local")
	}
	ppid, start, comm, err := psInfo(os.Getpid())
	if err != nil || ppid != os.Getppid() || start == "" || comm == "" {
		t.Fatalf("psInfo(self) = %d %q %q %v; padre real %d", ppid, start, comm, err, os.Getppid())
	}
}

func TestHostBinding(t *testing.T) {
	dir := t.TempDir()
	procs := tabla()
	if err := store(t, dir, procs, 300).Publish("agente-A"); err != nil {
		t.Fatal(err)
	}
	if err := store(t, dir, procs, 310).Publish("agente-B"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "100.json")); err != nil {
		t.Fatalf("el hook de A, lanzado por un shell, publica para su host 100: %v", err)
	}
	for self, want := range map[int]string{400: "agente-A", 500: "agente-A", 410: "agente-B", 300: "agente-A"} {
		if got, ok := store(t, dir, procs, self).Lookup(); !ok || got != want {
			t.Errorf("Lookup desde %d = %q %v, want %q", self, got, ok, want)
		}
	}

	// PID reutilizado: el host 100 terminó y otro proceso tomó su número.
	reusado := tabla()
	reusado[100] = proc{1, "t99", "otro"}
	if got, ok := store(t, dir, reusado, 400).Lookup(); ok {
		t.Errorf("un PID reutilizado no hereda el enlace: %q", got)
	}

	// Al publicar, se podan los enlaces de hosts que ya no existen.
	sinA := tabla()
	delete(sinA, 100)
	if err := store(t, dir, sinA, 310).Publish("agente-B2"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "100.json")); !os.IsNotExist(err) {
		t.Errorf("el enlace del host muerto debía podarse: %v", err)
	}

	// Sin enlace en ningún ancestro: no hay conversación.
	if got, ok := store(t, t.TempDir(), procs, 400).Lookup(); ok {
		t.Errorf("sin enlaces no hay conversación: %q", got)
	}
	if err := store(t, dir, procs, 300).Publish(" "); err == nil {
		t.Error("publicar sin id es un error")
	}
}
