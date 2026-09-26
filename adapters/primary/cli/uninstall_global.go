package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"mem/adapters/primary/setup"
)

// Retirada del ámbito de usuario (feature 034, US2). Regla común (FR-012):
// en un archivo compartido con otras herramientas solo se quitan las entradas
// de gomemory; si el archivo no se puede interpretar, no se toca y quien llama
// lo informa con ⚠ y el paso manual.

// fileMode conserva los permisos de un archivo existente al reescribirlo.
func fileMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

// removeJSONServerEntry quita el servidor "gomemory" de la tabla key de un
// JSON de configuración MCP (~/.claude.json, opencode.json…).
func removeJSONServerEntry(path, key string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, fmt.Errorf("JSON inválido: %w", err)
	}
	servers, ok := cfg[key].(map[string]any)
	if !ok {
		return false, nil
	}
	if _, has := servers["gomemory"]; !has {
		return false, nil
	}
	delete(servers, "gomemory")
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return false, err
	}
	return true, writeFileAtomic(path, append(out, '\n'), fileMode(path))
}

// isGomemoryHookCommand reconoce el comando de un hook de gomemory:
// `mem hook <sub>` con el binario por nombre o por ruta.
func isGomemoryHookCommand(cmd string) bool {
	f := strings.Fields(cmd)
	if len(f) < 2 || f[1] != "hook" {
		return false
	}
	base := filepath.Base(strings.Trim(f[0], `"'`))
	base = strings.TrimSuffix(base, ".exe")
	return base == "mem"
}

// removeCodexGomemory quita de ~/.codex/config.toml la tabla
// [mcp_servers.gomemory] (y sus subtablas) y los hooks de gomemory,
// conservando servidores, hooks y claves ajenos. El resultado se valida como
// TOML antes de devolverlo.
func removeCodexGomemory(data []byte) ([]byte, bool, error) {
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, false, fmt.Errorf("config.toml inválido: %w", err)
	}

	// 1. Servidor MCP: se retira por texto para no reformatear lo ajeno.
	var b strings.Builder
	dropping, changed := false, false
	for _, line := range strings.SplitAfter(string(data), "\n") {
		if isTOMLTableHeader(line) {
			k := codexTableKey(line)
			dropping = k == "gomemory" || strings.HasPrefix(k, "gomemory.")
			if dropping {
				changed = true
			}
		}
		if !dropping {
			b.WriteString(line)
		}
	}
	text := b.String()

	// 2. Hooks: se filtran los de gomemory y se reescribe la tabla con lo que
	// queda, igual que hace la instalación (ensureCodexGomemoryHooks).
	hooks, _ := stringMap(doc["hooks"])
	if len(hooks) > 0 {
		kept := map[string]any{}
		for event, v := range hooks {
			groups, _ := anySlice(v)
			var keptGroups []any
			for _, g := range groups {
				gm, ok := stringMap(g)
				if !ok {
					keptGroups = append(keptGroups, g)
					continue
				}
				entries, _ := anySlice(gm["hooks"])
				var keptEntries []any
				for _, e := range entries {
					em, _ := stringMap(e)
					if cmd, _ := em["command"].(string); isGomemoryHookCommand(cmd) {
						changed = true
						continue
					}
					keptEntries = append(keptEntries, e)
				}
				if len(keptEntries) == 0 && len(entries) > 0 {
					continue
				}
				gm["hooks"] = keptEntries
				keptGroups = append(keptGroups, gm)
			}
			if len(keptGroups) > 0 {
				kept[event] = keptGroups
			}
		}
		if changed {
			text = stripCodexHooksTables(text)
			if len(kept) > 0 {
				bloque, err := toml.Marshal(map[string]any{"hooks": kept})
				if err != nil {
					return nil, false, err
				}
				text = strings.TrimRight(text, "\n") + "\n\n" + strings.TrimLeft(string(bloque), "\n")
			}
		}
	}
	if !changed {
		return data, false, nil
	}
	out := []byte(strings.TrimRight(text, "\n") + "\n")
	var check map[string]any
	if err := toml.Unmarshal(out, &check); err != nil {
		return nil, false, fmt.Errorf("el resultado no es TOML válido: %w", err)
	}
	return out, true, nil
}

// removeCodexConfigFile aplica removeCodexGomemory al archivo.
func removeCodexConfigFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	out, changed, err := removeCodexGomemory(data)
	if err != nil || !changed {
		return false, err
	}
	return true, writeFileAtomic(path, out, fileMode(path))
}

// removeProtocolBlockFile quita el bloque de protocolo de gomemory de un
// archivo de instrucciones y conserva lo que haya antes y después. Si no queda
// nada propio de la persona, el archivo se elimina.
func removeProtocolBlockFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	rest := string(data)
	changed := false
	cut := func(start, end int) {
		before := strings.TrimRight(rest[:start], "\n")
		after := strings.TrimLeft(rest[end:], "\n")
		rest = before
		if after != "" {
			if rest != "" {
				rest += "\n\n"
			}
			rest += after
		}
		changed = true
	}
	// El protocolo de memoria y, en las instrucciones globales, el baseline
	// universal que composeAgentFile escribe justo antes: ambos son de gomemory.
	if idx := protocolStart(rest); idx != -1 {
		cut(idx, protocolEnd(rest, idx))
	}
	if idx := universalInstructionsStart(rest); idx != -1 {
		cut(idx, universalInstructionsEnd(rest, idx))
	}
	if !changed {
		return false, nil
	}
	if strings.TrimSpace(rest) == "" || isAutoGeneratedTitle(strings.TrimSpace(rest)) {
		return true, os.Remove(path)
	}
	return true, writeFileAtomic(path, []byte(strings.TrimRight(rest, "\n")+"\n"), fileMode(path))
}

// removeGlobalGeneratedArtifacts retira las habilidades y envoltorios de
// usuario que escribe gomemory. Devuelve las rutas retiradas.
func removeGlobalGeneratedArtifacts(home string) ([]string, []error) {
	var removed []string
	var errs []error
	for _, rel := range setup.GlobalGeneratedArtifacts() {
		p := filepath.Join(append([]string{home}, rel...)...)
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		removed = append(removed, p)
	}
	return removed, errs
}

// removeCodexBackups retira los respaldos que la instalación dejó junto a la
// configuración de Codex (*.gomemory-*.bak): contienen la configuración con
// las entradas de gomemory.
func removeCodexBackups(home string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(home, ".codex", "*.gomemory-*.bak"))
	if err != nil {
		return nil, err
	}
	var errs []error
	var removed []string
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			errs = append(errs, err)
			continue
		}
		removed = append(removed, m)
	}
	return removed, errors.Join(errs...)
}

// removeLegacyCodexHooksJSON limpia ~/.codex/hooks.json (versiones < 2.12.0):
// quita los hooks de gomemory y borra el archivo si no queda nada.
func removeLegacyCodexHooksJSON(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !bytes.Contains(data, []byte("hook ")) {
		return false, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("JSON inválido: %w", err)
	}
	hooks, _ := stringMap(doc["hooks"])
	changed := false
	for event, v := range hooks {
		groups, _ := anySlice(v)
		var keptGroups []any
		for _, g := range groups {
			gm, _ := stringMap(g)
			entries, _ := anySlice(gm["hooks"])
			var kept []any
			for _, e := range entries {
				em, _ := stringMap(e)
				if cmd, _ := em["command"].(string); isGomemoryHookCommand(cmd) {
					changed = true
					continue
				}
				kept = append(kept, e)
			}
			if len(kept) > 0 {
				gm["hooks"] = kept
				keptGroups = append(keptGroups, gm)
			}
		}
		if len(keptGroups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = keptGroups
		}
	}
	if !changed {
		return false, nil
	}
	if len(hooks) == 0 && len(doc) <= 1 {
		return true, os.Remove(path)
	}
	doc["hooks"] = hooks
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	return true, writeFileAtomic(path, append(out, '\n'), fileMode(path))
}
