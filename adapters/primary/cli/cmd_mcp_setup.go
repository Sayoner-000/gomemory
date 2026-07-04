package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mem/adapters/primary/setup"
)

// globalScopeAgents son los agentes que soportan registrar gomemory una sola
// vez a nivel de usuario/máquina, en vez de por proyecto (ver
// specs/005-global-mcp-store). Cursor, Windsurf y Cline no tienen un
// mecanismo de config MCP a nivel de usuario conocido — se documenta como
// limitación de esos agentes, no de gomemory (contracts/cli-contracts.md).
//
// OpenCode se confirmó empíricamente con `opencode debug config`: mergea
// ~/.config/opencode/opencode.json (scope usuario) con el opencode.json del
// proyecto, mismo esquema "mcp". La limitación documentada en
// specs/005-global-mcp-store/tasks.md T027 quedó obsoleta.
var globalScopeAgents = map[string]bool{
	"claude":   true,
	"codex":    true,
	"opencode": true,
}

func CmdMCPSetup(deps *Deps, args []string) {
	fs := flag.NewFlagSet("setup-mcp", flag.ContinueOnError)
	target := fs.String("target", ".", "Directorio del proyecto donde instalar configs (solo aplica a --scope project)")
	agents := fs.String("agents", "opencode,claude", "Agentes objetivo (separados por coma): opencode, claude, cursor, windsurf, cline, codex, all")
	scope := fs.String("scope", "project", "project (default, por repo) o global (una vez por máquina — claude, codex, opencode)")
	fs.Parse(args)

	agentList := strings.Split(*agents, ",")

	if *scope == "global" {
		runGlobalScopeSetup(agentList)
		return
	}
	if *scope != "project" {
		fail("--scope inválido: %q (valores válidos: project, global)", *scope)
	}

	root := *target
	if root == "." {
		var err error
		root, err = deps.ProjectRepo.FindRoot()
		if err != nil {
			root = "."
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fail("ruta inválida: %v", err)
	}

	fmt.Printf("🔌 Configurando MCP para gomemory en %s\n\n", absRoot)

	generated := 0

	for _, agent := range agentList {
		agent = strings.TrimSpace(agent)
		switch agent {
		case "opencode":
			if setupOpenCode(absRoot) {
				generated++
			}
		case "claude":
			if setupClaude(absRoot) {
				generated++
			}
		case "cursor":
			if setupCursor(absRoot) {
				generated++
			}
		case "windsurf":
			if setupWindsurf(absRoot) {
				generated++
			}
		case "cline":
			if setupCline(absRoot) {
				generated++
			}
		case "codex":
			if setupCodex(absRoot) {
				generated++
			}
		case "all":
			if setupOpenCode(absRoot) {
				generated++
			}
			if setupClaude(absRoot) {
				generated++
			}
			if setupCursor(absRoot) {
				generated++
			}
			if setupWindsurf(absRoot) {
				generated++
			}
			if setupCline(absRoot) {
				generated++
			}
			if setupCodex(absRoot) {
				generated++
			}
		default:
			fmt.Printf("  ⚠️  Agente desconocido: %s (opciones: opencode, claude, cursor, windsurf, cline, codex, all)\n", agent)
		}
	}

	fmt.Println()
	if generated > 0 {
		fmt.Printf("✅ %d configuraciones MCP generadas. Reinicia el agente para que las detecte.\n", generated)
	} else {
		fmt.Println("ℹ️  No se generaron configuraciones nuevas (ya existen o agentes no encontrados).")
	}
}

// runGlobalScopeSetup registra gomemory una sola vez a nivel de usuario, para
// los agentes que lo soportan. cwd es irrelevante aquí: no hay "target",
// porque el registro global aplica a todos los proyectos por igual.
func runGlobalScopeSetup(agentList []string) {
	fmt.Println("🔌 Registrando gomemory en scope global (una vez, para todos los proyectos)")
	fmt.Println()

	ref := binRefFor(".")
	generated := 0

	for _, agent := range agentList {
		agent = strings.TrimSpace(agent)
		if agent == "all" {
			for a := range globalScopeAgents {
				if runGlobalScopeAgent(a, ref) {
					generated++
				}
			}
			continue
		}
		if !globalScopeAgents[agent] {
			fmt.Printf("  ⚠️  %s no soporta --scope global (solo por proyecto): usa 'mem setup-mcp --scope project --agents %s --target <dir>'\n", agent, agent)
			continue
		}
		if runGlobalScopeAgent(agent, ref) {
			generated++
		}
	}

	fmt.Println()
	if generated > 0 {
		fmt.Printf("✅ %d registro(s) global(es) completados. Reinicia el agente para que los detecte.\n", generated)
	} else {
		fmt.Println("ℹ️  No se completó ningún registro global nuevo.")
	}
}

func runGlobalScopeAgent(agent string, ref BinRef) bool {
	switch agent {
	case "claude":
		return setupClaudeGlobal(ref)
	case "codex":
		return setupCodexGlobal(ref)
	case "opencode":
		return setupOpenCodeGlobal(ref)
	default:
		return false
	}
}

// setupOpenCodeGlobal instala el plugin (ya global por naturaleza) y registra
// el MCP en ~/.config/opencode/opencode.json, para todos los proyectos.
func setupOpenCodeGlobal(ref BinRef) bool {
	agentRef := setup.AgentRef{
		HookCommand: ref.HookCommand,
		MCPCommand:  ref.MCPCommand,
		MCPArgs:     ref.MCPArgs,
	}
	if err := setup.InstallOpenCodeGlobal(agentRef); err != nil {
		fmt.Printf("  ⚠️  opencode: %v\n", err)
		return false
	}
	return true
}

// setupClaudeGlobal registra gomemory en el scope de usuario de Claude Code
// (aplica a todos los proyectos, `~/.claude.json` → mcpServers.gomemory) vía
// el propio CLI `claude mcp add` — se delega la escritura del archivo a la
// herramienta que lo posee, en vez de que gomemory edite ese JSON a mano
// (es un archivo grande y con formato propio; editarlo directamente arriesga
// corromper estado no relacionado con gomemory).
//
// Antes de registrar, verifica si ya existe una entrada "gomemory" en scope
// user que apunte a un comando distinto (colisión de nombre con otra
// herramienta, ver FR-008) y se detiene pidiendo resolución manual en vez de
// sobrescribirla en silencio.
func setupClaudeGlobal(ref BinRef) bool {
	if _, err := exec.LookPath("claude"); err != nil {
		fmt.Println("  ⚠️  claude: no se encontró el CLI 'claude' en el PATH, no se puede registrar en scope global")
		return false
	}

	existing, err := readClaudeUserMCPEntry("gomemory")
	if err != nil {
		fmt.Printf("  ⚠️  claude: no se pudo leer ~/.claude.json: %v\n", err)
		return false
	}
	if existing != nil {
		if existing.Command == ref.MCPCommand {
			fmt.Println("  ✅ claude: ya registrado en scope global (~/.claude.json)")
			return true
		}
		fmt.Printf("  ⚠️  claude: ya existe una entrada global 'gomemory' apuntando a %q (no a %q) — "+
			"probablemente de otra herramienta. Resuélvelo manualmente antes de continuar: "+
			"'claude mcp remove gomemory -s user' o renombra la entrada existente.\n", existing.Command, ref.MCPCommand)
		return false
	}

	cmdArgs := append([]string{"mcp", "add", "-s", "user", "gomemory", ref.MCPCommand}, ref.MCPArgs...)
	cmd := exec.Command("claude", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ⚠️  claude: error al registrar en scope global: %v\n%s\n", err, out)
		return false
	}
	fmt.Println("  ✅ claude: registrado en scope global (~/.claude.json)")
	return true
}

type claudeMCPEntry struct {
	Command string `json:"command"`
}

// claudeUserConfigPath resuelve la ruta de ~/.claude.json. Acepta un override
// por variable de entorno únicamente para poder probar la lógica de
// lectura/conflicto sin tocar el ~/.claude.json real de la máquina que corre
// los tests.
func claudeUserConfigPath() (string, error) {
	if v := os.Getenv("GOMEMORY_CLAUDE_CONFIG"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude.json"), nil
}

// readClaudeUserMCPEntry lee (sin escribir nada) la entrada `name` del
// mcpServers de nivel usuario en ~/.claude.json, si existe.
func readClaudeUserMCPEntry(name string) (*claudeMCPEntry, error) {
	path, err := claudeUserConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var doc struct {
		McpServers map[string]claudeMCPEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsear ~/.claude.json: %w", err)
	}
	entry, ok := doc.McpServers[name]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

// setupCodexGlobal simplifica el registro de Codex a una sola tabla TOML
// global `[mcp_servers.gomemory]`, sin `cwd` ni sufijo por proyecto: el
// server ya resuelve el proyecto por git-root del cwd del proceso que Codex
// lance, así que una entrada por proyecto (el esquema anterior,
// `gomemory_<key>` con `cwd` fijo) ya no es necesaria.
func setupCodexGlobal(ref BinRef) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("  ⚠️  codex: no se pudo determinar el home: %v\n", err)
		return false
	}
	codexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		fmt.Printf("  ⚠️  codex: error al crear %s: %v\n", codexDir, err)
		return false
	}
	cfgPath := filepath.Join(codexDir, "config.toml")
	const tableHeader = `[mcp_servers.gomemory]`

	if data, err := os.ReadFile(cfgPath); err == nil {
		if strings.Contains(string(data), tableHeader) {
			fmt.Println("  ✅ codex: ~/.codex/config.toml ya tiene el registro global de gomemory")
			return true
		}
	}

	block := fmt.Sprintf("\n%s\ncommand = %q\nargs = [%q]\n", tableHeader, ref.MCPCommand, "mcp")

	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("  ⚠️  codex: error al escribir config.toml: %v\n", err)
		return false
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		fmt.Printf("  ⚠️  codex: error al escribir config.toml: %v\n", err)
		return false
	}
	fmt.Println("  ✅ codex: ~/.codex/config.toml actualizado con registro global (gomemory)")
	return true
}

func setupOpenCode(root string) bool {
	br := binRefFor(root)
	ref := setup.AgentRef{
		HookCommand: br.HookCommand,
		MCPCommand:  br.MCPCommand,
		MCPArgs:     br.MCPArgs,
	}
	if err := setup.WriteOpenCodeMCP(root, ref); err != nil {
		fmt.Printf("  ⚠️  opencode: %v\n", err)
		return false
	}
	fmt.Printf("  ✅ opencode: MCP configurado en %s\n", filepath.Join(root, "opencode.json"))
	return true
}

func setupClaude(root string) bool {
	ref := binRefFor(root)
	mcpPath := filepath.Join(root, ".mcp.json")
	if data, err := os.ReadFile(mcpPath); err == nil {
		var existing map[string]interface{}
		if json.Unmarshal(data, &existing) == nil {
			if ms, ok := existing["mcpServers"].(map[string]interface{}); ok {
				if _, has := ms["gomemory"]; has {
					fmt.Println("  ✅ claude: .mcp.json ya configurado")
					return true
				}
			}
		}
	}

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command": ref.MCPCommand,
				"args":    ref.MCPArgs,
			},
		},
	}
	data, _ := json.MarshalIndent(mcpCfg, "", "  ")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		fmt.Printf("  ⚠️  claude: error al escribir .mcp.json: %v\n", err)
		return false
	}
	fmt.Println("  ✅ claude: .mcp.json creado/actualizado")

	return true
}

func setupCursor(root string) bool {
	ref := binRefFor(root)
	cursorDir := filepath.Join(root, ".cursor")
	os.MkdirAll(cursorDir, 0755)
	mcpPath := filepath.Join(cursorDir, "mcp.json")

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command": ref.MCPCommand,
				"args":    ref.MCPArgs,
			},
		},
	}

	var existing map[string]interface{}
	if data, _ := os.ReadFile(mcpPath); data != nil {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = mcpCfg
	} else {
		ms, _ := existing["mcpServers"].(map[string]interface{})
		if ms == nil {
			ms = make(map[string]interface{})
		}
		if _, has := ms["gomemory"]; has {
			fmt.Println("  ✅ cursor: .cursor/mcp.json ya configurado")
			return true
		}
		ms["gomemory"] = map[string]interface{}{
			"command": ref.MCPCommand,
			"args":    ref.MCPArgs,
		}
		existing["mcpServers"] = ms
	}

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		fmt.Printf("  ⚠️  cursor: error al escribir .cursor/mcp.json: %v\n", err)
		return false
	}
	fmt.Println("  ✅ cursor: .cursor/mcp.json creado/actualizado")
	return true
}

func setupWindsurf(root string) bool {
	ref := binRefFor(root)
	windsufDir := filepath.Join(root, ".windsurf")
	os.MkdirAll(windsufDir, 0755)
	mcpPath := filepath.Join(windsufDir, "mcp_config.json")

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command": ref.MCPCommand,
				"args":    ref.MCPArgs,
			},
		},
	}

	var existing map[string]interface{}
	if data, _ := os.ReadFile(mcpPath); data != nil {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = mcpCfg
	} else {
		ms, _ := existing["mcpServers"].(map[string]interface{})
		if ms == nil {
			ms = make(map[string]interface{})
		}
		if _, has := ms["gomemory"]; has {
			fmt.Println("  ✅ windsurf: .windsurf/mcp_config.json ya configurado")
			return true
		}
		ms["gomemory"] = map[string]interface{}{
			"command": ref.MCPCommand,
			"args":    ref.MCPArgs,
		}
		existing["mcpServers"] = ms
	}

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		fmt.Printf("  ⚠️  windsurf: error al escribir .windsurf/mcp_config.json: %v\n", err)
		return false
	}
	fmt.Println("  ✅ windsurf: .windsurf/mcp_config.json creado/actualizado")
	return true
}

func setupCline(root string) bool {
	ref := binRefFor(root)
	clineDir := filepath.Join(root, ".cline")
	os.MkdirAll(clineDir, 0755)
	mcpPath := filepath.Join(clineDir, "mcp_settings.json")

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command":     ref.MCPCommand,
				"args":        ref.MCPArgs,
				"disabled":    false,
				"autoApprove": []string{},
			},
		},
	}

	var existing map[string]interface{}
	if data, _ := os.ReadFile(mcpPath); data != nil {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = mcpCfg
	} else {
		ms, _ := existing["mcpServers"].(map[string]interface{})
		if ms == nil {
			ms = make(map[string]interface{})
		}
		if _, has := ms["gomemory"]; has {
			fmt.Println("  ✅ cline: .cline/mcp_settings.json ya configurado")
			return true
		}
		ms["gomemory"] = map[string]interface{}{
			"command":     ref.MCPCommand,
			"args":        ref.MCPArgs,
			"disabled":    false,
			"autoApprove": []string{},
		}
		existing["mcpServers"] = ms
	}

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		fmt.Printf("  ⚠️  cline: error al escribir .cline/mcp_settings.json: %v\n", err)
		return false
	}
	fmt.Println("  ✅ cline: .cline/mcp_settings.json creado/actualizado")
	return true
}

func setupCodex(root string) bool {
	ref := binRefFor(root)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("  ⚠️  codex: no se pudo determinar el home: %v\n", err)
		return false
	}
	codexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		fmt.Printf("  ⚠️  codex: error al crear %s: %v\n", codexDir, err)
		return false
	}
	cfgPath := filepath.Join(codexDir, "config.toml")

	key := sanitizeTomlKey(filepath.Base(root))
	tableHeader := fmt.Sprintf(`[mcp_servers."gomemory_%s"]`, key)

	if data, err := os.ReadFile(cfgPath); err == nil {
		if strings.Contains(string(data), tableHeader) {
			fmt.Println("  ✅ codex: ~/.codex/config.toml ya configurado para este proyecto")
			return true
		}
	}

	// ~/.codex/config.toml es un archivo global de la máquina (nunca se
	// commitea), así que cwd=root es aceptable y necesario para ubicar el
	// proyecto; el command se referencia de forma portable vía PATH.
	block := fmt.Sprintf("\n%s\ncommand = %q\nargs = [%q]\ncwd = %q\n",
		tableHeader, ref.MCPCommand, "mcp", root)

	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("  ⚠️  codex: error al escribir config.toml: %v\n", err)
		return false
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		fmt.Printf("  ⚠️  codex: error al escribir config.toml: %v\n", err)
		return false
	}
	fmt.Printf("  ✅ codex: ~/.codex/config.toml actualizado (tabla gomemory_%s)\n", key)
	return true
}

func sanitizeTomlKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "project"
	}
	return b.String()
}
