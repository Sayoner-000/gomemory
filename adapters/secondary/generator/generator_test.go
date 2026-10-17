package generator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// fakeAgent escribe en dir un ejecutable que registra sus argumentos y su
// stdin en <dir>/<name>.args y <dir>/<name>.stdin, y emite out.
func fakeAgent(t *testing.T, dir, name, out string, sleep time.Duration) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("ejecutables falsos en shell")
	}
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do printf '%s\\n' \"$a\"; done > \"" + filepath.Join(dir, name+".args") + "\"\n" +
		"cat > \"" + filepath.Join(dir, name+".stdin") + "\"\n"
	if sleep > 0 {
		script += "sleep " + strings.TrimSuffix(sleep.String(), "s") + "\n"
	}
	script += "cat <<'JSON'\n" + out + "\nJSON\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func compactSchemaForTest(t *testing.T) string {
	t.Helper()
	var schema map[string]json.RawMessage
	if err := json.Unmarshal([]byte(domain.SocraticaGenerationSchema), &schema); err != nil {
		t.Fatal(err)
	}
	delete(schema, "$schema")
	b, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

var flow = ports.SocraticGenerationRequest{Flow: domain.RefinementFlow{Mode: domain.ModeRefine, OriginalPrompt: "envía el informe"}}

func TestGenerators(t *testing.T) {
	dir := t.TempDir()
	// Solo el directorio de los agentes falsos más las utilidades del sistema
	// que usan sus scripts; ahí no viven claude ni codex reales.
	isolated := dir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
	t.Setenv("PATH", isolated)
	ctx := context.Background()

	t.Run("claude", func(t *testing.T) {
		fakeAgent(t, dir, "claude", `{"type":"result","result":"","structured_output":{"questions":[{"text":"¿Para quién?","options":["Equipo","Cliente"]}]}}`, 0)
		g := NewClaude(time.Second)
		if !g.Available() || g.Name() != "claude" {
			t.Fatal("claude no disponible")
		}
		got, err := g.GenerateQuestions(ctx, flow)
		if err != nil || len(got.Questions) != 1 || got.Questions[0].Text != "¿Para quién?" {
			t.Fatal(got, err)
		}
		args := readLines(t, filepath.Join(dir, "claude.args"))
		if len(args) != 7 || args[0] != "-p" || args[1] != "--output-format" || args[2] != "json" || args[3] != "--json-schema" || args[4] != compactSchemaForTest(t) || args[5] != "--tools" || args[6] != "" {
			t.Fatalf("argumentos de claude: %q", args)
		}
		if stdin, _ := os.ReadFile(filepath.Join(dir, "claude.stdin")); !strings.Contains(string(stdin), "envía el informe") {
			t.Fatalf("el prompt no llegó por stdin: %q", stdin)
		}
	})
	t.Run("claude result text", func(t *testing.T) {
		fakeAgent(t, dir, "claude", `{"type":"result","result":"{\"proposed\":\"Envía el informe a [destinatario]\"}"}`, 0)
		got, err := NewClaude(time.Second).Rewrite(ctx, flow)
		if err != nil || got.Proposed != "Envía el informe a [destinatario]" {
			t.Fatal(got, err)
		}
	})
	t.Run("codex", func(t *testing.T) {
		fakeAgent(t, dir, "codex", `{"proposed":"Envía el informe trimestral","additions":["periodo"]}`, 0)
		g := NewCodex(time.Second)
		got, err := g.Rewrite(ctx, flow)
		if err != nil || got.Proposed != "Envía el informe trimestral" {
			t.Fatal(got, err)
		}
		args := readLines(t, filepath.Join(dir, "codex.args"))
		want := []string{"exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "--output-schema"}
		if len(args) != 8 || strings.Join(args[:6], " ") != strings.Join(want, " ") || args[7] != "-" {
			t.Fatalf("argumentos de codex: %q", args)
		}
		// El esquema se escribió en un archivo temporal que ya no existe.
		if _, err := os.Stat(args[6]); !os.IsNotExist(err) {
			t.Fatalf("el esquema temporal %s sigue en disco (%v)", args[6], err)
		}
	})
	t.Run("irreparable output", func(t *testing.T) {
		fakeAgent(t, dir, "codex", `no json here`, 0)
		if _, err := NewCodex(time.Second).Rewrite(ctx, flow); !errors.Is(err, domain.ErrInvalidGeneration) {
			t.Fatal(err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		fakeAgent(t, dir, "codex", `{"proposed":"tarde"}`, 2*time.Second)
		start := time.Now()
		if _, err := NewCodex(200*time.Millisecond).Rewrite(ctx, flow); err == nil || time.Since(start) > 1500*time.Millisecond {
			t.Fatalf("sin timeout: %v en %s", err, time.Since(start))
		}
	})
	t.Run("FirstAvailable", func(t *testing.T) {
		empty := t.TempDir()
		t.Setenv("PATH", empty)
		if NewClaude(time.Second).Available() {
			t.Fatal("disponible sin binario")
		}
		none := NewFirstAvailable([]ports.SocraticGenerator{NewClaude(time.Second), NewCodex(time.Second)})
		if none.Available() {
			t.Fatal("disponible sin generadores")
		}
		if _, err := none.GenerateQuestions(ctx, flow); !errors.Is(err, ports.ErrNoGenerator) {
			t.Fatal(err)
		}
		t.Setenv("PATH", isolated)
		fakeAgent(t, dir, "codex", `{"proposed":"x"}`, 0)
		both := NewFirstAvailable([]ports.SocraticGenerator{NewCodex(time.Second), NewClaude(time.Second)})
		if both.Name() != "codex" {
			t.Fatalf("respeta el orden recibido: %s", both.Name())
		}
	})
}
