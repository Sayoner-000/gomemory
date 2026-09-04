package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mem/adapters/primary/setup"
	"mem/domain"
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
// globalHookWriters asocia cada agente con el mecanismo que le escribe sus
// hooks de ámbito de usuario. La correspondencia vive en una tabla y no fijada
// en el flujo (FR-019): un agente sin entrada aquí simplemente no recibe hooks
// globales, y eso es un dato consultable, no una omisión escondida en un if.
var globalHookWriters = map[string]func(home string, ref setup.AgentRef) error{
	"claude": setup.WriteClaudeHooksGlobal,
}

var globalScopeAgents = map[string]bool{
	"claude":   true,
	"codex":    true,
	"opencode": true,
}

// defaultAgentList es el valor por defecto de --agents.
//
// Codex entró aquí al cerrarse su ciclo de inyección por turno. Antes estaba
// declarado en globalScopeAgents pero ausente del defecto, así que
// `mem setup-mcp --scope global` sin argumentos no lo registraba nunca: quien
// no supiera pasar `--agents codex` se quedaba sin su ciclo de memoria entero,
// y nada lo advertía porque el comando terminaba en éxito.
//
// La invariante que lo sostiene está en TestAgentesPorDefecto_IncluyenCodex:
// todo agente del defecto debe existir en globalScopeAgents, para que este
// valor no pueda volver a prometer lo que el flujo no cumple.
const defaultAgentList = "opencode,claude,codex"

var codexConfigMu sync.Mutex

func CmdMCPSetup(deps *Deps, args []string) {
	fs := flag.NewFlagSet("setup-mcp", flag.ContinueOnError)
	target := fs.String("target", ".", "Directorio del proyecto donde instalar configs (solo aplica a --scope project)")
	agents := fs.String("agents", defaultAgentList, "Agentes objetivo (separados por coma): opencode, claude, cursor, windsurf, cline, codex, all")
	scope := fs.String("scope", "project", "project (default, por repo) o global (una vez por máquina — claude, codex, opencode)")
	if err := fs.Parse(args); err != nil {
		return
	}

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
	solicitados := map[string]bool{}

	for _, agent := range agentList {
		agent = strings.TrimSpace(agent)
		if agent == "all" {
			for a := range globalScopeAgents {
				solicitados[a] = true
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
		solicitados[agent] = true
		if runGlobalScopeAgent(agent, ref) {
			generated++
		}
	}

	// Hooks del modo plan atómico en ámbito de usuario (feature 019, Historia
	// 4): independiente de si el registro MCP tuvo éxito (no depende de que
	// el CLI `claude` esté en PATH), recorre los agentes del registro de
	// capacidades que declaren ámbito de usuario y les escribe los hooks —
	// mismo mecanismo, misma idempotencia, que el ámbito de proyecto. Sin
	// esto, "habilitar una vez" solo cubría el texto, nunca el determinismo
	// de la Historia 1 (research.md §7).
	// Hooks del modo plan en ámbito de usuario, para los agentes SOLICITADOS.
	//
	// El bucle recorría el registro de capacidades pero fijaba el nombre de un
	// agente e ignoraba la selección recibida: parecía genérico y no lo era.
	// El efecto no era cosmético — creaba el directorio de configuración de un
	// agente que nadie había pedido, y eso hacía que InstallAtomicPlanGlobal,
	// que solo escribe donde el directorio ya existe, lo encontrara recién
	// creado y añadiera dos artefactos más. Un filtro fijo en cascada hacia
	// tres archivos ajenos a la petición.
	//
	// Ahora el despacho va por tabla: un agente con escritor de hooks de ámbito
	// de usuario lo recibe si fue solicitado, y ninguno más.
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		for _, agent := range domain.KnownAgents {
			if !solicitados[agent.Name] || !agent.Scopes[domain.ScopeUser] {
				continue
			}
			escribir, tiene := globalHookWriters[agent.Name]
			if !tiene {
				continue // sin mecanismo de hooks globales: declarado, no omitido
			}
			agentRef := setup.AgentRef{HookCommand: ref.HookCommand, MCPCommand: ref.MCPCommand, MCPArgs: ref.MCPArgs}
			if err := escribir(home, agentRef); err != nil {
				fmt.Printf("  ⚠️  hooks de modo plan (scope global, %s): %v\n", agent.Name, err)
			} else {
				fmt.Printf("  ✅ hooks de modo plan: %s\n", filepath.Join(home, ".claude", "settings.json"))
			}
		}
	}

	// Planificación atómica en scope global (spec 013, Historia 3): además del
	// registro del servidor MCP, se escribe el bloque de protocolo —que lleva el
	// disparador de modo plan— en el archivo de instrucciones de nivel usuario
	// de cada agente, y su envoltorio nativo. Con esto, habilitar una vez cubre
	// todos los proyectos presentes y futuros.
	//
	// Cursor, Windsurf y Cline quedan deliberadamente fuera: no aparecen en
	// globalScopeAgents porque no tienen un scope de usuario equivalente. Su
	// cobertura llega por scope de proyecto, donde `mem install` ya les escribe
	// el mismo bloque de protocolo en .cursorrules/.windsurfrules — así que no
	// pierden la funcionalidad, solo el "habilitar una sola vez".
	written, err := setup.InstallAtomicPlanGlobal(PlanMethod(), func(existing string) (string, bool) {
		return composeAgentFile(existing, embeddedTemplate("universal-agent-instructions.md"), buildIntegrationBlock())
	})
	if err != nil {
		fmt.Printf("  ⚠️  planificación atómica (scope global): %v\n", err)
	}
	for _, p := range written {
		fmt.Printf("  ✅ planificación atómica: %s\n", p)
	}

	// Guía de revisión adversarial por consenso (spec 027, FR-044): se
	// distribuye solo en scope global y no por proyecto, porque es un protocolo
	// de trabajo del usuario, no configuración de un repositorio concreto.
	//
	// Es autosuficiente: no depende de las herramientas de gomemory ni de que
	// haya memoria persistente, así que instalarla no promete nada que el
	// binario no pueda cumplir hoy.
	skills, err := setup.InstallAdversarialReviewSkill(embeddedTemplate("adversarial-consensus-review/SKILL.md"))
	if err != nil {
		fmt.Printf("  ⚠️  revisión adversarial (scope global): %v\n", err)
	}
	for _, p := range skills {
		fmt.Printf("  ✅ revisión adversarial: %s\n", p)
	}
	written = append(written, skills...)

	// Habilidades ya existentes hacia Codex y OpenCode. Cierra un hueco abierto
	// desde las features 013/021: gomemory escribía habilidades solo en
	// `.claude/skills/`, así que estos dos agentes nunca recibieron el método de
	// descomposición ni la constitución como habilidad descubrible.
	previas, err := setup.InstallExistingSkillsForAgents(PlanMethod(), setup.ConstitutionSkillBody())
	if err != nil {
		fmt.Printf("  ⚠️  habilidades existentes (scope global): %v\n", err)
	}
	for _, p := range previas {
		fmt.Printf("  ✅ habilidad: %s\n", p)
	}
	written = append(written, previas...)

	fmt.Println()
	if generated > 0 || len(written) > 0 {
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
	codexConfigMu.Lock()
	defer codexConfigMu.Unlock()

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
	hooksPath := filepath.Join(codexDir, "hooks.json")

	data, readErr := os.ReadFile(cfgPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		fmt.Printf("  ⚠️  codex: error al leer config.toml: %v\n", readErr)
		return false
	}
	originalData := append([]byte(nil), data...)
	original := string(data)
	hooksData, hooksReadErr := os.ReadFile(hooksPath)
	hooksMigrated := 0
	hooksConsolidated := false
	if hooksReadErr == nil {
		candidate, count, migrateErr := consolidateCodexHooks(data, hooksData)
		if migrateErr != nil {
			fmt.Printf("  ⚠️  codex: hooks.json no se migró; se conserva intacto: %v\n", migrateErr)
		} else {
			data = candidate
			original = string(candidate)
			hooksMigrated = count
			hooksConsolidated = true
		}
	} else if !os.IsNotExist(hooksReadErr) {
		fmt.Printf("  ⚠️  codex: error al leer hooks.json; se conserva intacto: %v\n", hooksReadErr)
	}
	migrated, removed := migrateLegacyCodexTables(original)
	if !hasCodexGlobalTable(migrated) {
		if migrated != "" && !strings.HasSuffix(migrated, "\n") {
			migrated += "\n"
		}
		migrated += fmt.Sprintf("\n[mcp_servers.gomemory]\ncommand = %q\nargs = [%q]\n", ref.MCPCommand, "mcp")
	}

	// El registro MCP da a Codex la CAPACIDAD de consultar memoria; los hooks
	// son los que hacen que gomemory EJERZA su ciclo (inyectar contexto al
	// arrancar, registrar la actividad al cerrar cada turno). Sin ellos, Codex
	// corría sobre un solo canal de cinco y `mem doctor` ni lo reportaba.
	hooksAdded := 0
	if conHooks, añadidos, err := ensureCodexGomemoryHooks([]byte(migrated), ref.MCPCommand); err != nil {
		fmt.Printf("  ⚠️  codex: no se pudieron registrar los hooks de gomemory; el resto del archivo no se toca: %v\n", err)
	} else {
		migrated = string(conHooks)
		hooksAdded = añadidos
	}

	if migrated == string(originalData) && !hooksConsolidated {
		fmt.Println("  ✅ codex: ~/.codex/config.toml ya tiene el registro global de gomemory")
		return true
	}

	mode := os.FileMode(0644)
	if info, statErr := os.Stat(cfgPath); statErr == nil {
		mode = info.Mode().Perm()
	}
	if removed > 0 || hooksConsolidated || hooksAdded > 0 {
		backupPath, backupErr := backupCodexConfig(cfgPath, originalData, mode)
		if backupErr != nil {
			fmt.Printf("  ⚠️  codex: no se pudo respaldar config.toml; no se modifica: %v\n", backupErr)
			return false
		}
		fmt.Printf("  ✅ codex: respaldo legado creado en %s\n", backupPath)
	}
	var hooksBackupPath string
	if hooksConsolidated {
		hooksMode := os.FileMode(0644)
		if info, statErr := os.Stat(hooksPath); statErr == nil {
			hooksMode = info.Mode().Perm()
		}
		var backupErr error
		hooksBackupPath, backupErr = backupExistingFile(hooksPath, hooksData, hooksMode)
		if backupErr != nil {
			fmt.Printf("  ⚠️  codex: no se pudo respaldar hooks.json; no se modifica: %v\n", backupErr)
			return false
		}
	}

	if err := writeFileAtomic(cfgPath, []byte(migrated), mode); err != nil {
		fmt.Printf("  ⚠️  codex: error al escribir config.toml: %v\n", err)
		return false
	}
	if hooksBackupPath != "" {
		if err := removeLegacyHooksJSON(hooksPath); err != nil {
			_ = writeFileAtomic(cfgPath, originalData, mode)
			fmt.Printf("  ⚠️  codex: no se pudo retirar hooks.json; config.toml fue restaurado: %v\n", err)
			return false
		}
		fmt.Printf("  ✅ codex: hooks.json consolidado en config.toml (%d grupo(s) añadido(s)); respaldo en %s\n", hooksMigrated, hooksBackupPath)
	}
	if removed > 0 {
		fmt.Printf("  ✅ codex: %d registro(s) gomemory_* legado(s) migrado(s)\n", removed)
	}
	if hooksAdded > 0 {
		fmt.Printf("  ✅ codex: %d hook(s) del ciclo de gomemory registrado(s) en config.toml\n", hooksAdded)
		fmt.Println("  ℹ️  codex: autoriza los hooks nuevos en tu próxima sesión de Codex para que queden activos")
	}
	fmt.Println("  ✅ codex: ~/.codex/config.toml actualizado con registro global (gomemory)")
	return true
}

func migrateLegacyCodexTables(content string) (string, int) {
	lines := strings.SplitAfter(content, "\n")
	var out strings.Builder
	dropping := false
	removed := 0
	for _, line := range lines {
		if isTOMLTableHeader(line) {
			dropping = isLegacyCodexTable(line)
			if dropping {
				removed++
				continue
			}
		}
		if !dropping {
			out.WriteString(line)
		}
	}
	return out.String(), removed
}

func isTOMLTableHeader(line string) bool {
	line = strings.TrimSpace(line)
	if comment := strings.IndexByte(line, '#'); comment >= 0 {
		line = strings.TrimSpace(line[:comment])
	}
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && !strings.Contains(line, "=")
}

func codexTableKey(line string) string {
	line = strings.TrimSpace(line)
	if comment := strings.IndexByte(line, '#'); comment >= 0 {
		line = strings.TrimSpace(line[:comment])
	}
	const prefix = "[mcp_servers."
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "]") {
		return ""
	}
	key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, prefix), "]"))
	if len(key) >= 2 && key[0] == '"' && key[len(key)-1] == '"' {
		key = key[1 : len(key)-1]
	}
	return key
}

func isLegacyCodexTable(line string) bool {
	key := codexTableKey(line)
	return strings.HasPrefix(key, "gomemory_") || strings.HasPrefix(key, `"gomemory_`)
}

func hasCodexGlobalTable(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if codexTableKey(line) == "gomemory" {
			return true
		}
	}
	return false
}

func backupCodexConfig(path string, data []byte, mode os.FileMode) (string, error) {
	for attempt := 0; attempt < 10; attempt++ {
		stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
		backupPath := fmt.Sprintf("%s.gomemory-legacy-%s.bak", path, stamp)
		f, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err = f.Write(data); err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		return backupPath, nil
	}
	return "", fmt.Errorf("no se pudo reservar un nombre único para el respaldo")
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.toml.gomemory-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	committed = true
	return nil
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
	fmt.Println("  ℹ️  codex: el registro MCP es global; se reutiliza para todos los proyectos")
	return setupCodexGlobal(binRefFor(root))
}

// RunGlobalScopeSetupForTest expone el registro de ámbito global a los
// contratos de tests/contract, que viven fuera del paquete. No añade
// comportamiento: solo hace verificable el contrato C8.
func RunGlobalScopeSetupForTest(agents []string) { runGlobalScopeSetup(agents) }
