package agentconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReaderJSONCPathsAndSkipped(t *testing.T) {
	ctx := context.Background()
	home, root := t.TempDir(), t.TempDir()
	r := New(home, root)
	config := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	raw := "{ // comment\n\"mcp\": {\"x\":{\"url\":\"https://example.invalid/a//b\",\"env\":{\"TOKEN\":\"literal\"},}}, /* tail */ }"
	if err := os.WriteFile(config, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(tomlPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tomlPath, []byte("approval_policy = 'never' # broad\n[mcp_servers.x.env]\nTOKEN = 'literal'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, skipped := r.Files(ctx, 32, 1<<20)
	if len(files) != 2 || len(skipped) > 0 {
		t.Fatal(len(files), skipped)
	}
	for _, f := range files {
		if !json.Valid(f.Content) {
			t.Fatalf("not normalized JSON: %s", f.Content)
		}
	}
	for i := range 34 {
		p := filepath.Join(root, fmt.Sprintf("config-%02d.json", i))
		if err := os.WriteFile(p, []byte(`{"hooks":{"PreToolUse":[{"hooks":[{"command":"/tmp/probe.sh"}]}]}}`), 0600); err != nil {
			t.Fatal(err)
		}
		r.Paths = append(r.Paths, p)
	}
	files, skipped = r.Files(ctx, 32, 1<<20)
	if len(files) != 32 || len(skipped) != 4 {
		t.Fatalf("file cap: %d %d", len(files), len(skipped))
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if r.Fingerprint(cancelCtx) != "" {
		t.Fatal("ignored cancellation")
	}
	files, skipped = r.Files(cancelCtx, 32, 1<<20)
	if len(files) != 0 || len(skipped) != 36 {
		t.Fatal("read after cancellation")
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	_, skipped = r.Files(ctx, 32, 1<<20)
	if !slices.Contains(skipped, "~/.claude.json") {
		t.Fatalf("malformed global config not skipped: %v", skipped)
	}
}

func TestReaderLimitsFingerprintAndReadOnly(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	ctx := context.Background()
	p := filepath.Join(home, ".claude.json")
	raw := `{"mcpServers":{"x":{"command":"mem"}},"history":"PRIVATE-HISTORY"}`
	if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(strings.Repeat("x", 100)), 0600); err != nil {
		t.Fatal(err)
	}
	r := New(home, root)
	hash := r.Fingerprint(ctx)
	before, _ := os.Stat(p)
	files, skipped := r.Files(ctx, 32, 1<<20)
	if len(files) != 2 || len(skipped) != 0 {
		t.Fatalf("%d %+v", len(files), skipped)
	}
	for _, f := range files {
		if strings.Contains(string(f.Content), "PRIVATE-HISTORY") {
			t.Fatal("exposed non-MCP settings")
		}
	}
	after, _ := os.Stat(p)
	again, _ := os.ReadFile(p)
	if !after.ModTime().Equal(before.ModTime()) || string(again) != raw {
		t.Fatal("modified config")
	}
	if r.Fingerprint(ctx) != hash {
		t.Fatal("unstable hash")
	}
	next := before.ModTime().Add(time.Second)
	if err := os.Chtimes(p, next, next); err != nil {
		t.Fatal(err)
	}
	if r.Fingerprint(ctx) == hash {
		t.Fatal("mtime not included")
	}
	files, skipped = r.Files(ctx, 1, 10)
	if len(files) != 0 || len(skipped) != 2 {
		t.Fatalf("limits: %d %+v", len(files), skipped)
	}
}

// R3-05: sin HOME absoluto no se escanea el home ni se resuelven sus rutas
// contra el directorio actual, y una ruta compartida se lee una sola vez.
func TestReaderWithoutHomeAndDedup(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, home := range []string{"", "."} {
		r := New(home, root)
		for _, p := range r.Paths {
			if !filepath.IsAbs(p) {
				t.Errorf("home %q: ruta relativa %q", home, p)
			}
		}
		files, skipped := r.Files(context.Background(), 20, 1<<20)
		if len(files) != 1 || len(skipped) != 1 || !strings.Contains(skipped[0], "HOME no definido") {
			t.Errorf("home %q: files=%d skipped=%v", home, len(files), skipped)
		}
	}
	// Proyecto en el propio home: settings.json, AGENTS.md y CLAUDE.md coinciden.
	r := New(root, root)
	seen := map[string]bool{}
	for _, p := range r.Paths {
		if seen[p] {
			t.Errorf("ruta duplicada %q", p)
		}
		seen[p] = true
	}
	if files, _ := r.Files(context.Background(), 20, 1<<20); len(files) != 1 {
		t.Errorf("el mismo archivo se escanea %d veces", len(files))
	}
}
