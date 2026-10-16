package launcher

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mem/application/ports"
)

// fakeAgent espera a que exista <dir>/signal (lo crea onStarted) y deja sus
// argumentos y la variable de despacho en <dir>/<name>.out.
func fakeAgent(t *testing.T, dir, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("ejecutables falsos en shell")
	}
	script := "#!/bin/sh\n" +
		"i=0; while [ ! -f \"" + filepath.Join(dir, "signal") + "\" ] && [ $i -lt 100 ]; do sleep 0.05; i=$((i+1)); done\n" +
		"{ for a in \"$@\"; do printf '%s\\n' \"$a\"; done; printf 'env=%s\\n' \"$GOMEMORY_SOCRATICA_DISPATCH\"; } > \"" + filepath.Join(dir, name+".out") + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func isolatedPath(dir string) string {
	return dir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
}

func TestAgentLauncher(t *testing.T) {
	ctx := context.Background()
	req := func(client string) ports.LaunchRequest {
		return ports.LaunchRequest{Client: client, Prompt: "Envía el informe", Env: map[string]string{"GOMEMORY_SOCRATICA_DISPATCH": "d-1"}}
	}
	for client, wantArgs := range map[string][]string{
		"claude":   {"Envía el informe"},
		"codex":    {"Envía el informe"},
		"opencode": {"--prompt", "Envía el informe"},
	} {
		t.Run(client, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PATH", isolatedPath(dir))
			fakeAgent(t, dir, client)
			calls := 0
			err := NewExec(nil, nil, nil).Launch(ctx, req(client), func() error {
				calls++
				// El agente sigue vivo: todavía no escribió su salida.
				if _, err := os.Stat(filepath.Join(dir, client+".out")); !os.IsNotExist(err) {
					t.Error("onStarted llegó después de que el agente terminara")
				}
				return os.WriteFile(filepath.Join(dir, "signal"), nil, 0o600)
			})
			if err != nil || calls != 1 {
				t.Fatal(calls, err)
			}
			// Launch no vuelve hasta que el proceso sale (Wait).
			out, err := os.ReadFile(filepath.Join(dir, client+".out"))
			if err != nil {
				t.Fatal("Launch volvió antes de que el agente terminara", err)
			}
			want := strings.Join(append(wantArgs, "env=d-1"), "\n") + "\n"
			if string(out) != want {
				t.Fatalf("salida %q, se esperaba %q", out, want)
			}
		})
	}
	t.Run("missing binary", func(t *testing.T) {
		t.Setenv("PATH", isolatedPath(t.TempDir()))
		called := false
		err := NewExec(nil, nil, nil).Launch(ctx, req("claude"), func() error { called = true; return nil })
		if !errors.Is(err, ports.ErrLaunchFailed) || called {
			t.Fatal(called, err)
		}
	})
	t.Run("not executable", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PATH", isolatedPath(dir))
		if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		called := false
		err := NewExec(nil, nil, nil).Launch(ctx, req("claude"), func() error { called = true; return nil })
		if !errors.Is(err, ports.ErrLaunchFailed) || called {
			t.Fatal(called, err)
		}
	})
	t.Run("confirm failure waits and does not kill", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PATH", isolatedPath(dir))
		fakeAgent(t, dir, "codex")
		err := NewExec(nil, nil, nil).Launch(ctx, req("codex"), func() error {
			_ = os.WriteFile(filepath.Join(dir, "signal"), nil, 0o600)
			return errors.New("database locked")
		})
		if !errors.Is(err, ports.ErrConfirmFailed) || errors.Is(err, ports.ErrLaunchFailed) {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "codex.out")); err != nil {
			t.Fatal("el agente no terminó por sí mismo", err)
		}
	})
	t.Run("unknown client", func(t *testing.T) {
		if err := NewExec(nil, nil, nil).Launch(ctx, req("otro"), func() error { return nil }); !errors.Is(err, ports.ErrLaunchFailed) {
			t.Fatal(err)
		}
	})
	t.Run("print", func(t *testing.T) {
		for raw, want := range map[bool]string{false: ">Envía el informe\n", true: "Envía el informe\n"} {
			var out bytes.Buffer
			order := ""
			err := NewPrint(&out, raw).Launch(ctx, req("claude"), func() error {
				order = out.String()
				return nil
			})
			if err != nil || out.String() != want || order != want {
				t.Fatalf("raw=%v: %q (onStarted vio %q) %v", raw, out.String(), order, err)
			}
		}
	})
}
