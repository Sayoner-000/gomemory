package setup

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ExpandWiring reporta, por harness, si `mem hook expand` está cableado
// (feature 040, `mem expand status`). Lee los mismos artefactos que escribe
// el instalador: el settings.json de Claude Code (proyecto o global), el
// config.toml de Codex (todos los hooks de expand de la tabla) y el plugin
// de OpenCode en ámbito de usuario.
func ExpandWiring(home, root string) map[string]bool {
	claude := GomemoryHookSubs(filepath.Join(root, ".claude", "settings.json"))["expand"] ||
		GomemoryHookSubs(filepath.Join(home, ".claude", "settings.json"))["expand"]

	codex := false
	if data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err == nil {
		doc := map[string]any{}
		if toml.Unmarshal(data, &doc) == nil {
			hooks, _ := doc["hooks"].(map[string]any)
			codex = true
			for _, h := range codexGomemoryHooks {
				if strings.HasPrefix(h.Sub, "expand ") && !CodexHookPresente(hooks, h) {
					codex = false
				}
			}
		}
	}

	opencode := false
	if data, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "plugins", "gomemory.ts")); err == nil {
		opencode = strings.Contains(string(data), `"hook", "expand"`)
	}
	return map[string]bool{"claude": claude, "codex": codex, "opencode": opencode}
}
