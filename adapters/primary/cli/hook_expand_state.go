package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mem/adapters/secondary/persistence"
)

// Estado de `mem hook expand` que pertenece al adaptador, no a la
// aplicación (feature 040, research R6): qué sugerencias ya se emitieron en
// el episodio y cuántas herramientas se interceptaron desde la última
// compactación. Mismo patrón que plan_episode.go: archivos bajo .memory/,
// sin abrir SQLite, best-effort y sesgado al silencio ante la duda.

// expandMarkerTTL es la edad a partir de la cual se purga un marcador
// huérfano (sesión que no volvió a disparar ningún hook).
const expandMarkerTTL = 7 * 24 * time.Hour

var expandEpisodeKeyRe = regexp.MustCompile(`^suggest:[a-z-]+$`)

// expandSessionKey separa el estado por sesión: sesiones simultáneas en el
// mismo proyecto no comparten episodio ni contador.
func expandSessionKey(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "default"
	}
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])[:8]
}

func expandEpisodePath(root, sid string) string {
	return filepath.Join(root, persistence.MemDir, ".expand-episode-"+sid)
}

func expandToolsPath(root, sid string) string {
	return filepath.Join(root, persistence.MemDir, ".expand-tools-"+sid)
}

// expandEpisodeEmitted reporta si key ya se emitió en el episodio. Un
// marcador ilegible o corrupto cuenta como «ya emitido»: ante la duda, no
// se repite una sugerencia.
func expandEpisodeEmitted(root, sid, key string) bool {
	data, err := os.ReadFile(expandEpisodePath(root, sid))
	if err != nil {
		return !os.IsNotExist(err)
	}
	return episodeHasKey(data, key)
}

func episodeHasKey(data []byte, key string) bool {
	if !utf8.Valid(data) {
		return true
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		if !expandEpisodeKeyRe.MatchString(line) {
			return true
		}
		if line == key {
			return true
		}
	}
	return false
}

// expandEpisodeClaim reserva key en el episodio y dice si este proceso debe
// emitirla. Comprobar y marcar van bajo el mismo candado de archivo: dos
// hooks simultáneos de la misma sesión no emiten la misma sugerencia (B-002).
// Ante cualquier error, no se emite.
func expandEpisodeClaim(root, sid, key string) bool {
	path := expandEpisodePath(root, sid)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	if lockHookFile(f) != nil {
		return false
	}
	defer func() { _ = unlockHookFile(f) }()
	data, err := io.ReadAll(f)
	if err != nil || episodeHasKey(data, key) {
		return false
	}
	_, err = f.Write([]byte(key + "\n"))
	return err == nil
}

func expandEpisodeReset(root, sid string) {
	_ = os.Remove(expandEpisodePath(root, sid))
}

// expandToolCountIncr suma una herramienta interceptada y devuelve el total.
// Un contador ilegible vuelve a empezar desde cero. La lectura y la
// escritura van bajo candado de archivo: los hooks de herramientas en
// paralelo no deben perder incrementos (C-004).
func expandToolCountIncr(root, sid string) int {
	path := expandToolsPath(root, sid)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	if lockHookFile(f) != nil {
		return 0
	}
	defer func() { _ = unlockHookFile(f) }()
	n := 0
	if data, err := io.ReadAll(io.LimitReader(f, 32)); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v >= 0 {
			n = v
		}
	}
	n++
	if f.Truncate(0) == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(n)), 0)
	}
	return n
}

func expandToolCountReset(root, sid string) {
	_ = os.Remove(expandToolsPath(root, sid))
}

// purgeExpandMarkers borra los marcadores de sesiones que llevan más de
// expandMarkerTTL sin actividad. now es inyectado (constitución §10).
func purgeExpandMarkers(root string, now time.Time) {
	dir := filepath.Join(root, persistence.MemDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, ".expand-episode-") && !strings.HasPrefix(name, ".expand-tools-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > expandMarkerTTL {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}
