package cli

import (
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
)

// TestPlanEpisodeStartsAtZero cubre el arranque limpio: sin marcador, el
// episodio no registra ninguna devolución todavía (data-model.md §2).
func TestPlanEpisodeStartsAtZero(t *testing.T) {
	root := t.TempDir()
	if planEpisodeDenied(root) {
		t.Error("un episodio sin marcador no debe reportar devolución ya emitida")
	}
}

// TestPlanEpisodeMarkDeniedThenReset cubre incrementar y reiniciar: tras
// marcar una devolución, el episodio la recuerda; tras reiniciar, vuelve a
// permitir una devolución nueva.
func TestPlanEpisodeMarkDeniedThenReset(t *testing.T) {
	root := t.TempDir()

	planEpisodeMarkDenied(root)
	if !planEpisodeDenied(root) {
		t.Error("tras marcar una devolución, el episodio debe recordarla")
	}

	planEpisodeReset(root)
	if planEpisodeDenied(root) {
		t.Error("tras reiniciar el episodio, debe volver a permitir una devolución nueva")
	}
}

// TestPlanEpisodeInvalidContentIsTreatedAsAlreadyDenied cubre la invariante
// de data-model.md §2: contenido ilegible se resuelve permitiendo, nunca
// bloqueando. Un archivo corrupto se trata como "ya devuelto" (denials >= 1),
// no como "arranca en 0".
func TestPlanEpisodeInvalidContentIsTreatedAsAlreadyDenied(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, persistence.MemDir, ".plan-episode-denials")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("esto-no-es-un-contador"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !planEpisodeDenied(root) {
		t.Error("contenido inválido debe tratarse como ya devuelto (permitir), nunca como episodio fresco")
	}
}
