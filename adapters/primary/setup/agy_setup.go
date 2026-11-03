package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mem/domain"
)

// agyHookGroup conserva el nombre del grupo manual previo: reescribirlo lo
// reemplaza en su sitio y evita ejecutar los hooks dos veces.
const agyHookGroup = "gomemory-lifecycle"

// agyToolMatcher limita los hooks de herramienta a las que expand clasifica.
const agyToolMatcher = "run_command|write_file|edit_file|invoke_subagent"

const agySkillsPath = "~/.agents/skills"

func agyConfigDir(home string) string {
	return filepath.Join(home, ".gemini", "config")
}

func agyHookEntry(ref AgentRef, event string) map[string]any {
	return map[string]any{"type": "command", "command": ref.HookCommand + " hook agy " + event, "timeout": 15}
}

func agyToolHook(ref AgentRef, event, matcher string) []any {
	return []any{map[string]any{"matcher": matcher, "hooks": []any{agyHookEntry(ref, event)}}}
}

func agyLifecycleGroup(ref AgentRef) map[string]any {
	return map[string]any{
		"enabled":       true,
		"SessionStart":  []any{agyHookEntry(ref, "SessionStart")},
		"PreInvocation": []any{agyHookEntry(ref, "PreInvocation")},
		"PreToolUse":    agyToolHook(ref, "PreToolUse", agyToolMatcher),
		"PostToolUse":   agyToolHook(ref, "PostToolUse", "run_command"),
		"Stop":          []any{agyHookEntry(ref, "Stop")},
	}
}

// InstallAgyGlobal registra el MCP, los hooks y las skills de gomemory y
// devuelve los archivos que cambió. Es idempotente y conserva lo ajeno.
func InstallAgyGlobal(home string, ref AgentRef, now time.Time) ([]string, error) {
	dir := agyConfigDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("crear %s: %w", dir, err)
	}
	edits := []struct {
		name  string
		apply func(map[string]any)
	}{
		{"mcp_config.json", func(doc map[string]any) {
			servers, _ := doc["mcpServers"].(map[string]any)
			if servers == nil {
				servers = map[string]any{}
			}
			servers["gomemory"] = map[string]any{"command": ref.MCPCommand, "args": ref.MCPArgs}
			doc["mcpServers"] = servers
		}},
		{"hooks.json", func(doc map[string]any) { doc[agyHookGroup] = agyLifecycleGroup(ref) }},
		{"skills.json", addAgySkillsEntry},
	}
	var changed []string
	for _, edit := range edits {
		path := filepath.Join(dir, edit.name)
		wrote, err := editAgyConfig(path, now, edit.apply)
		if err != nil {
			return changed, err
		}
		if wrote {
			changed = append(changed, path)
		}
	}
	return changed, nil
}

// UninstallAgyGlobal retira el MCP y los hooks de gomemory, con copia previa.
// La entrada de skills.json se conserva: esa carpeta no es exclusiva de gomemory.
func UninstallAgyGlobal(home string, now time.Time) ([]string, error) {
	dir := agyConfigDir(home)
	edits := []struct {
		name  string
		apply func(map[string]any)
	}{
		{"mcp_config.json", func(doc map[string]any) {
			if servers, ok := doc["mcpServers"].(map[string]any); ok {
				delete(servers, "gomemory")
			}
		}},
		{"hooks.json", func(doc map[string]any) { delete(doc, agyHookGroup) }},
	}
	var changed []string
	for _, edit := range edits {
		path := filepath.Join(dir, edit.name)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		wrote, err := editAgyConfig(path, now, edit.apply)
		if err != nil {
			return changed, err
		}
		if wrote {
			changed = append(changed, path)
		}
	}
	return changed, nil
}

func addAgySkillsEntry(doc map[string]any) {
	entries, _ := doc["entries"].([]any)
	for _, entry := range entries {
		if item, ok := entry.(map[string]any); ok && item["path"] == agySkillsPath {
			return
		}
	}
	doc["entries"] = append(entries, map[string]any{"path": agySkillsPath, "exclude": []any{`\..*`}})
}

// editAgyConfig aplica edit y escribe solo si el contenido cambia. Un archivo
// inválido se rechaza sin tocarlo: reescribirlo perdería lo que contenga.
func editAgyConfig(path string, now time.Time, edit func(map[string]any)) (bool, error) {
	previous, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("leer %s: %w", path, err)
	}
	doc := map[string]any{}
	if len(bytes.TrimSpace(previous)) > 0 {
		if err := json.Unmarshal(previous, &doc); err != nil {
			return false, fmt.Errorf("%s no es JSON válido; se conserva intacto: %w", path, err)
		}
	}
	before, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}
	edit(doc)
	after, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}
	if bytes.Equal(before, after) {
		return false, nil
	}
	rendered, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	if len(previous) > 0 {
		backup := fmt.Sprintf("%s.bak-%s", path, now.Format("20060102-150405"))
		if err := os.WriteFile(backup, previous, 0o600); err != nil {
			return false, fmt.Errorf("copia previa de %s: %w", path, err)
		}
	}
	if err := WriteConfigFile(path, append(rendered, '\n')); err != nil {
		return false, fmt.Errorf("escribir %s: %w", path, err)
	}
	return true, nil
}

// agyCheckedVersion es la versión cuyo contrato de hooks se observó
// (docs/evidence/hcd015/agy-native.md).
const agyCheckedVersion = "1.4.12"

func agyInstalledVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "agy", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// InspectAgyEntry comprueba los hooks de agy y la versión con la que se
// observó su contrato. Sin integración instalada no hay nada que reparar.
func InspectAgyEntry(home, version string) domain.ActivationChannel {
	ch := domain.ActivationChannel{Arm: domain.ArmGomemory, Agent: "agy", Scope: domain.ScopeUser, Kind: domain.KindPlanEntry}
	path := filepath.Join(agyConfigDir(home), "hooks.json")
	data, err := os.ReadFile(path)
	switch {
	case err != nil || !bytes.Contains(data, []byte(agyHookGroup)):
		ch.State = domain.StateNotApplicable
		ch.Detail = "gomemory no está instalado en agy; actívalo con mem install --agents agy"
	case !bytes.Contains(data, []byte("hook agy PreInvocation")):
		ch.State = domain.StateOutdated
		ch.Detail = "grupo manual en " + path + "; reemplázalo con mem install --agents agy"
	case version != agyCheckedVersion:
		ch.State = domain.StateOutdated
		ch.Detail = fmt.Sprintf("agy %q sin comprobar; contrato observado en %s", version, agyCheckedVersion)
	default:
		ch.State = domain.StateOK
		ch.Detail = "hooks de gomemory en " + path + "; sin compresión automática de salidas ni aviso tras compactar"
	}
	return ch
}
