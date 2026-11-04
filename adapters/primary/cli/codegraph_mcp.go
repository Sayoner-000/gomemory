package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"mem/adapters/primary/console"
	"mem/application/ports"
)

// codeGraphTargets traduce los agentes de gomemory a los ids de target del
// instalador de CodeGraph.
var codeGraphTargets = map[string]string{"claude": "claude", "codex": "codex", "opencode": "opencode", "agy": "antigravity"}

// codeGraphInstallArgs conecta el servidor MCP de CodeGraph solo a los agentes
// que faltan. No usa --target auto: también conecta Gemini CLI, que crea
// ~/.gemini/GEMINI.md y ~/.gemini/settings.json (HCD-020, Q1).
func codeGraphInstallArgs(missing []string) []string {
	targets := make([]string, 0, len(missing))
	for _, agent := range missing {
		targets = append(targets, codeGraphTargets[agent])
	}
	return []string{"install", "--target", strings.Join(targets, ","), "--location", "global", "--yes"}
}

// codeGraphManual es el comando que la persona puede ejecutar a mano.
func codeGraphManual(missing []string) string {
	return "codegraph " + strings.Join(codeGraphInstallArgs(missing), " ")
}

// codeGraphOn indica si CodeGraph forma parte del grafo externo activo: el
// grafo no está desactivado, CodeGraph está entre los proveedores (o la
// lista vacía lo autodetecta) y su binario está en PATH.
func codeGraphOn(s ports.SettingsData) bool {
	if s.CodeGraphDisabled {
		return false
	}
	if _, err := exec.LookPath("codegraph"); err != nil {
		return false
	}
	if len(s.CodeGraphProviders) == 0 {
		return true
	}
	for _, p := range s.CodeGraphProviders {
		if strings.EqualFold(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)), "codegraph") {
			return true
		}
	}
	return false
}

// codeGraphMCPState devuelve, para cada agente con configuración global en
// home, si su servidor MCP de CodeGraph está registrado. Un agente sin
// configuración no aparece: no hay nada que conectar. root, si no es vacío,
// suma el registro de Claude a nivel proyecto (projects[root] y .mcp.json).
func codeGraphMCPState(home, root string) map[string]bool {
	state := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(home, ".claude.json")); err == nil {
		var cfg struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
			Projects   map[string]struct {
				MCPServers map[string]json.RawMessage `json:"mcpServers"`
			} `json:"projects"`
		}
		_ = json.Unmarshal(data, &cfg)
		global := jsonServerEntry(cfg.MCPServers["codegraph"])
		project := root != "" && jsonServerEntry(cfg.Projects[root].MCPServers["codegraph"])
		state["claude"] = global || project || projectMCPHasCodeGraph(root)
	}
	if data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err == nil {
		// Se interpreta el TOML: una sección comentada no es un registro.
		var cfg struct {
			MCPServers map[string]any `toml:"mcp_servers"`
		}
		_ = toml.Unmarshal(data, &cfg)
		entry, _ := cfg.MCPServers["codegraph"].(map[string]any)
		state["codex"] = serverEntry(entry)
	}
	if data, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json")); err == nil {
		// CodeGraph escribe mcp.servers.codegraph, que OpenCode acepta igual
		// que mcp.codegraph.
		var cfg struct {
			MCP map[string]json.RawMessage `json:"mcp"`
		}
		_ = json.Unmarshal(data, &cfg)
		var nested map[string]json.RawMessage
		_ = json.Unmarshal(cfg.MCP["servers"], &nested)
		state["opencode"] = jsonServerEntry(cfg.MCP["codegraph"]) || jsonServerEntry(nested["codegraph"])
	}
	if entry, ok := agyMCPServer(home, "codegraph"); ok {
		state["agy"] = entry
	}
	return state
}

// agyMCPServer dice si el mcp_config.json de agy registra el servidor name.
// ok=false si agy no tiene configuración: no hay nada que conectar.
func agyMCPServer(home, name string) (registered, ok bool) {
	data, err := os.ReadFile(filepath.Join(home, ".gemini", "config", "mcp_config.json"))
	if err != nil {
		return false, false
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	_ = json.Unmarshal(data, &cfg)
	return jsonServerEntry(cfg.MCPServers[name]), true
}

// projectMCPHasCodeGraph mira el .mcp.json de Claude en la raíz del proyecto.
func projectMCPHasCodeGraph(root string) bool {
	if root == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		return false
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	_ = json.Unmarshal(data, &cfg)
	return jsonServerEntry(cfg.MCPServers["codegraph"])
}

// serverEntry exige un servidor de verdad: un comando local o una URL
// remota. Una tabla vacía o creada solo por sus subtablas no conecta nada.
func serverEntry(entry map[string]any) bool {
	_, command := entry["command"]
	_, url := entry["url"]
	return command || url
}

func jsonServerEntry(raw json.RawMessage) bool {
	var entry map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &entry) != nil {
		return false
	}
	return serverEntry(entry)
}

// codeGraphInstallerOK comprueba que el codegraph del PATH es el de CodeGraph
// y ofrece el instalador del manual antes de ejecutarlo con --yes.
func codeGraphInstallerOK(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "codegraph", "install", "--help").CombinedOutput()
	help := string(out)
	return err == nil && strings.Contains(help, "MCP") && strings.Contains(help, "--target") && strings.Contains(help, "--location")
}

// ensureCodeGraphMCP deja activo el MCP de CodeGraph cuando el grafo externo
// lo usa: si algún agente presente no lo tiene, ejecuta el instalador de
// CodeGraph. Devuelve false si CodeGraph no está en uso (no hay paso que
// informar).
func ensureCodeGraphMCP(s ports.SettingsData, home, root string) (console.StepResult, bool) {
	step := console.StepResult{Name: "CodeGraph MCP", Status: console.StepOK}
	if !codeGraphOn(s) {
		return step, false
	}
	state := codeGraphMCPState(home, root)
	if len(state) == 0 {
		return step, false
	}
	if missing := codeGraphMissing(state); len(missing) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if !codeGraphInstallerOK(ctx) {
			step.Status, step.Manual = console.StepWarn, codeGraphManual(missing)
			step.Detail = "el codegraph del PATH no ofrece install --target/--location; sin registrar en " + strings.Join(missing, ", ")
			return step, true
		}
		cmd := exec.CommandContext(ctx, "codegraph", codeGraphInstallArgs(missing)...)
		out, err := cmd.CombinedOutput()
		state = codeGraphMCPState(home, root)
		requested := missing
		if missing = codeGraphMissing(state); err != nil || len(missing) > 0 {
			if len(missing) == 0 {
				missing = requested
			}
			step.Status, step.Manual = console.StepWarn, codeGraphManual(missing)
			step.Detail = "sin registrar en " + strings.Join(missing, ", ")
			if err != nil {
				step.Detail = strings.TrimSpace(err.Error() + " " + lastLine(string(out)))
			}
			return step, true
		}
	}
	step.Detail = "activo en " + strings.Join(codeGraphRegistered(state), ", ")
	return step, true
}

// codeGraphAgents son los agentes cuya configuración global se revisa.
var codeGraphAgents = []string{"claude", "codex", "opencode", "agy"}

func codeGraphMissing(state map[string]bool) []string {
	var out []string
	for _, agent := range codeGraphAgents {
		if registered, present := state[agent]; present && !registered {
			out = append(out, agent)
		}
	}
	return out
}

func codeGraphRegistered(state map[string]bool) []string {
	var out []string
	for _, agent := range codeGraphAgents {
		if state[agent] {
			out = append(out, agent)
		}
	}
	return out
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// printDoctorCodeGraphMCP informa, cuando CodeGraph está en uso, en qué
// agentes está conectado su servidor MCP y cómo conectarlo donde falte. Antes
// avisa si codebase-memory-mcp no está conectado a agy (solo diagnóstico).
func printDoctorCodeGraphMCP(s ports.SettingsData, root string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	if msg, warn := agyCodebaseMemoryWarning(home); warn {
		humanln("\ncodebase-memory-mcp:\n  ⚠ " + msg)
	}
	if !codeGraphOn(s) {
		return
	}
	state := codeGraphMCPState(home, root)
	if len(state) == 0 {
		return
	}
	humanln("\nCodeGraph MCP:")
	for _, agent := range codeGraphAgents {
		registered, present := state[agent]
		switch {
		case !present:
		case registered:
			humanf("  ✓ %s: conectado\n", agent)
		default:
			humanf("  ✗ %s: sin conectar → %s\n", agent, codeGraphManual([]string{agent}))
		}
	}
}

// codebaseMemoryAgyManual es el comando del proveedor para conectarse a agy.
// gomemory no lo ejecuta: también escribe ~/.gemini/GEMINI.md (HCD-020).
const codebaseMemoryAgyManual = "codebase-memory-mcp install --clients=antigravity"

// agyCodebaseMemoryWarning avisa cuando codebase-memory-mcp está instalado
// pero no conectado a agy. Sin configuración de agy o sin binario no aplica.
func agyCodebaseMemoryWarning(home string) (string, bool) {
	if _, err := exec.LookPath("codebase-memory-mcp"); err != nil {
		return "", false
	}
	registered, ok := agyMCPServer(home, "codebase-memory-mcp")
	if !ok || registered {
		return "", false
	}
	return "agy sin codebase-memory-mcp → " + codebaseMemoryAgyManual + " (ese instalador también escribe ~/.gemini/GEMINI.md)", true
}
