package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ExpandWiring reporta, por harness, si `mem hook expand` está cableado
// (feature 040, `mem expand status`). Lee los mismos artefactos que escribe
// el instalador: el settings.json de Claude Code (proyecto o global), el
// config.toml de Codex (todos los hooks de expand de la tabla) y el plugin
// de OpenCode y el ciclo de hooks de agy en ámbito de usuario.
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
	return map[string]bool{"claude": claude, "codex": codex, "opencode": opencode, "agy": agyExpandWired(home)}
}

// agyExpandWired deriva eventos y herramientas del contrato del instalador.
// Solo lee configuración: no ejecuta hooks ni exige que agy esté en el PATH.
func agyExpandWired(home string) bool {
	data, err := os.ReadFile(filepath.Join(agyConfigDir(home), "hooks.json"))
	if err != nil {
		return false
	}
	var doc map[string]any
	if json.Unmarshal(data, &doc) != nil {
		return false
	}
	group, _ := doc[agyHookGroup].(map[string]any)
	if enabled, _ := group["enabled"].(bool); !enabled {
		return false
	}
	for event, raw := range agyLifecycleGroup(AgentRef{HookCommand: "mem"}) {
		if event == "enabled" {
			continue
		}
		entries, _ := group[event].([]any)
		for _, required := range raw.([]any) {
			req := required.(map[string]any)
			matcher, grouped := req["matcher"].(string)
			if !grouped {
				if !agyEventHasHook(entries, event, "") {
					return false
				}
				continue
			}
			for _, tool := range strings.Split(matcher, "|") {
				if !agyEventHasHook(entries, event, tool) {
					return false
				}
			}
		}
	}
	return true
}

func agyEventHasHook(entries []any, event, tool string) bool {
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if tool == "" {
			if agyCommandEntryMatches(entry, event) {
				return true
			}
			continue
		}
		matcher, ok := entry["matcher"].(string)
		if !ok {
			continue
		}
		re, err := regexp.Compile("^(?:" + matcher + ")$")
		if err != nil || !re.MatchString(tool) {
			continue
		}
		hooks, _ := entry["hooks"].([]any)
		if agyEventHasHook(hooks, event, "") {
			return true
		}
	}
	return false
}

// Una invocación directa evita contar comentarios, echo o comandos de terceros.
var agyHookCommandRE = regexp.MustCompile(`^(?:"([^"\r\n]+)"|'([^'\r\n]+)'|([^\s"']+))[ \t]+hook[ \t]+agy[ \t]+([A-Za-z]+)[ \t]*$`)

func agyCommandEntryMatches(entry map[string]any, event string) bool {
	if entry["type"] != "command" {
		return false
	}
	command, _ := entry["command"].(string)
	m := agyHookCommandRE.FindStringSubmatch(strings.TrimSpace(command))
	if m == nil || m[4] != event {
		return false
	}
	bin := m[1] + m[2] + m[3]
	bin = filepath.Base(strings.ReplaceAll(bin, `\`, "/"))
	return bin == "mem" || bin == "mem.exe"
}
