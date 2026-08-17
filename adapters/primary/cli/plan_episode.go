package cli

import (
	"os"
	"path/filepath"
	"strings"

	"mem/adapters/secondary/persistence"
)

// planEpisodeStatePath es el marcador de episodio de plan: mismo patrón que
// sessionMarkerPath y compactNudgeStatePath (archivo bajo .memory/, sin BD).
// El estado del guard debe resolverse en < 50ms y no puede abrir SQLite
// (specs/019-deterministic-plan-trigger/plan.md, Technical Context).
func planEpisodeStatePath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".plan-episode-denials")
}

// planEpisodeDenied reporta si el episodio de plan en curso ya emitió una
// devolución (data-model.md §2). Tres casos, todos sesgados a permitir ante
// la duda:
//   - marcador ausente → false: episodio fresco, arranca en 0.
//   - marcador con contenido "0" → false: episodio reiniciado explícitamente.
//   - cualquier otro contenido (ilegible, corrupto, o "1") → true: ya se
//     devolvió, o el estado es dudoso — en ambos casos se permite.
func planEpisodeDenied(root string) bool {
	data, err := os.ReadFile(planEpisodeStatePath(root))
	if err != nil {
		return !os.IsNotExist(err)
	}
	return strings.TrimSpace(string(data)) != "0"
}

// planEpisodeMarkDenied registra que el episodio en curso ya devolvió un plan.
// Best-effort: un fallo de escritura no debe bloquear el hook (regla de oro de
// CmdHook, cmd_hook.go).
func planEpisodeMarkDenied(root string) {
	path := planEpisodeStatePath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("1"), 0o644)
}

// planEpisodeReset abre un episodio de plan nuevo: entrar en modo plan
// (plan-entered) o aprobar el plan (plan-approved) reinician el contador.
func planEpisodeReset(root string) {
	path := planEpisodeStatePath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("0"), 0o644)
}
