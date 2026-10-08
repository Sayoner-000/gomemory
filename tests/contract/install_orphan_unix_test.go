//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// C-002: si el puente `install --events` muere por cualquier vía (la terminal
// se cierra, el cliente se va, o lo matan con SIGKILL), el instalador real y
// sus descendientes no pueden seguir trabajando huérfanos.
func TestInstallEvents_ElInstaladorNoQuedaHuerfano(t *testing.T) {
	bin := buildLifecycleBinary(t)
	for _, name := range []string{"SIGINT de control", "SIGHUP", "SIGKILL", "el cliente muere", "arranca adoptado y el lector se cierra"} {
		t.Run(name, func(t *testing.T) {
			s := newLifecycleSandbox(t)
			p := s.Project("p")
			beat := filepath.Join(s.Root, "latido")
			// Descendiente lento: el paso CodeGraph MCP lo invoca porque
			// Claude está presente sin el servidor registrado.
			mustMkdir(t, s.BinDir)
			script := "#!/bin/sh\nwhile true; do echo x >> '" + beat + "'; sleep 0.1; done\n"
			if err := os.WriteFile(filepath.Join(s.BinDir, "codegraph"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
				t.Fatal(err)
			}

			// El cliente es un shell que lanza el puente y lo espera, como la
			// consola nativa o el instalador TypeScript.
			client := exec.Command("sh", "-c", `"$0" install "$1" --events --yes --agents none > /dev/null & wait`, bin, p)
			var reader *os.File
			if name == "arranca adoptado y el lector se cierra" {
				// El shell sale en seguida: el puente queda adoptado (ppid 1)
				// y su salida es una pipe que este test lee.
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				client = exec.Command("sh", "-c", `"$0" install "$1" --events --yes --agents none & exit 0`, bin, p)
				client.Stdout, reader = w, r
				defer w.Close()
			}
			client.Env = s.Env()
			if err := client.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = client.Process.Kill()
				_ = exec.Command("pkill", "-9", "-f", beat).Run()
				_ = exec.Command("pkill", "-9", "-f", p).Run()
			})
			deadline := time.Now().Add(20 * time.Second)
			for beats(beat) < 3 {
				if time.Now().After(deadline) {
					t.Fatal("el instalador no llegó al paso lento")
				}
				time.Sleep(50 * time.Millisecond)
			}
			bridgePID := pidOf(t, "install "+p+" --events")
			switch name {
			case "SIGINT de control":
				_ = syscall.Kill(bridgePID, syscall.SIGINT)
			case "SIGHUP":
				_ = syscall.Kill(bridgePID, syscall.SIGHUP)
			case "SIGKILL":
				_ = syscall.Kill(bridgePID, syscall.SIGKILL)
			case "el cliente muere":
				_ = client.Process.Kill()
			case "arranca adoptado y el lector se cierra":
				_ = reader.Close()
			}
			time.Sleep(1500 * time.Millisecond)
			before := beats(beat)
			time.Sleep(700 * time.Millisecond)
			if after := beats(beat); after != before {
				t.Fatalf("el descendiente sigue trabajando tras la muerte del puente: latido %d → %d", before, after)
			}
			if out, _ := exec.Command("pgrep", "-f", "install --yes "+p).Output(); strings.TrimSpace(string(out)) != "" {
				t.Fatalf("instalador real huérfano: pid %s", out)
			}
		})
	}
}

func pidOf(t *testing.T, pattern string) int {
	t.Helper()
	out, err := exec.Command("pgrep", "-f", pattern).Output()
	fields := strings.Fields(string(out))
	if err != nil || len(fields) == 0 {
		t.Fatalf("sin proceso %q", pattern)
	}
	pid, _ := strconv.Atoi(fields[0])
	return pid
}

func beats(path string) int {
	data, _ := os.ReadFile(path)
	return strings.Count(string(data), "\n")
}
