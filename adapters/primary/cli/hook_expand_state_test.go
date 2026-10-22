package cli

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
)

func TestExpandSessionKey(t *testing.T) {
	if expandSessionKey("") != "default" || expandSessionKey("  ") != "default" {
		t.Fatal("sin session_id debe usar default")
	}
	a, b := expandSessionKey("s1"), expandSessionKey("s2")
	if len(a) != 8 || a == b || a != expandSessionKey("s1") {
		t.Fatalf("claves: %q %q", a, b)
	}
}

func TestExpandEpisodeIsolatedPerSession(t *testing.T) {
	root := t.TempDir()
	s1, s2 := expandSessionKey("s1"), expandSessionKey("s2")
	if expandEpisodeEmitted(root, s1, "suggest:verify-loop") {
		t.Fatal("episodio fresco no debe tener emitidas")
	}
	if !expandEpisodeClaim(root, s1, "suggest:verify-loop") {
		t.Fatal("la primera reserva debe emitir")
	}
	if expandEpisodeClaim(root, s1, "suggest:verify-loop") {
		t.Fatal("la segunda reserva no debe emitir")
	}
	if !expandEpisodeEmitted(root, s1, "suggest:verify-loop") {
		t.Fatal("tras marcar, debe constar como emitida")
	}
	if expandEpisodeEmitted(root, s1, "suggest:search-first") {
		t.Fatal("otra clave no debe constar")
	}
	if expandEpisodeEmitted(root, s2, "suggest:verify-loop") {
		t.Fatal("otra sesión no comparte episodio")
	}
	info, err := os.Stat(expandEpisodePath(root, s1))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos del marcador: %v %v", info.Mode().Perm(), err)
	}
	expandEpisodeReset(root, s1)
	if expandEpisodeEmitted(root, s1, "suggest:verify-loop") {
		t.Fatal("tras reset, episodio fresco")
	}
}

func TestExpandEpisodeCorruptMarkerSilences(t *testing.T) {
	root := t.TempDir()
	sid := expandSessionKey("s")
	path := expandEpisodePath(root, sid)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	for _, content := range []string{"\xff\xfe", "garbage line\n"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if !expandEpisodeEmitted(root, sid, "suggest:verify-loop") {
			t.Errorf("marcador corrupto %q debe silenciar", content)
		}
	}
}

func TestExpandToolCount(t *testing.T) {
	root := t.TempDir()
	sid := expandSessionKey("s")
	for i := 1; i <= 3; i++ {
		if n := expandToolCountIncr(root, sid); n != i {
			t.Fatalf("incremento %d → %d", i, n)
		}
	}
	expandToolCountReset(root, sid)
	if n := expandToolCountIncr(root, sid); n != 1 {
		t.Fatalf("tras reset → %d", n)
	}
	_ = os.WriteFile(expandToolsPath(root, sid), []byte("no-num"), 0o600)
	if n := expandToolCountIncr(root, sid); n != 1 {
		t.Fatalf("contador ilegible debe reiniciar: %d", n)
	}
}

func TestPurgeExpandMarkersOnlyOld(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	dir := filepath.Join(root, persistence.MemDir)
	_ = os.MkdirAll(dir, 0o700)
	files := map[string]time.Duration{
		".expand-episode-old":   8 * 24 * time.Hour,
		".expand-tools-old":     10 * 24 * time.Hour,
		".expand-episode-new":   1 * time.Hour,
		".plan-episode-denials": 30 * 24 * time.Hour, // ajeno: no se toca
	}
	for name, age := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, now.Add(-age), now.Add(-age))
	}
	purgeExpandMarkers(root, now)
	for name, shouldExist := range map[string]bool{
		".expand-episode-old": false, ".expand-tools-old": false,
		".expand-episode-new": true, ".plan-episode-denials": true,
	} {
		_, err := os.Stat(filepath.Join(dir, name))
		if exists := err == nil; exists != shouldExist {
			t.Errorf("%s: existe=%v, want %v", name, exists, shouldExist)
		}
	}
}

// C-004: hooks de herramientas en paralelo no pierden incrementos.
func TestExpandToolCountConcurrent(t *testing.T) {
	root := t.TempDir()
	const n = 20
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			expandToolCountIncr(root, "s")
		}()
	}
	wg.Wait()
	if got := expandToolCountIncr(root, "s"); got != n+1 {
		t.Fatalf("contador = %d, want %d", got, n+1)
	}
}

// B-002: hooks simultáneos de la misma sesión emiten una sola vez.
func TestExpandEpisodeClaimConcurrent(t *testing.T) {
	root := t.TempDir()
	const n = 20
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if expandEpisodeClaim(root, "s", "suggest:verify-loop") {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("emisiones = %d, want 1", wins)
	}
}
