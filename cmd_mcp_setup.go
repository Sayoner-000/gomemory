package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/store"
)

func cmdMCPSetup(args []string) {
	fs := flag.NewFlagSet("setup-mcp", flag.ContinueOnError)
	target := fs.String("target", ".", "Directorio del proyecto donde instalar configs")
	agents := fs.String("agents", "opencode,claude", "Agentes objetivo (separados por coma): opencode, claude, cursor, windsurf, cline, codex, all")
	fs.Parse(args)

	root := *target
	if root == "." {
		var err error
		root, err = store.FindRoot()
		if err != nil {
			root = "."
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fail("ruta inválida: %v", err)
	}

	binPath := filepath.Join(absRoot, "mem")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		// Try the current binary
		self, err := os.Executable()
		if err == nil {
			binPath = self
		}
	}

	fmt.Printf("🔌 Configurando MCP para gomemory en %s\n\n", absRoot)

	agentList := strings.Split(*agents, ",")
	generated := 0

	for _, agent := range agentList {
		agent = strings.TrimSpace(agent)
		switch agent {
		case "opencode":
			if setupOpenCode(absRoot, binPath) {
				generated++
			}
		case "claude":
			if setupClaude(absRoot, binPath) {
				generated++
			}
		case "cursor":
			if setupCursor(absRoot, binPath) {
				generated++
			}
		case "windsurf":
			if setupWindsurf(absRoot, binPath) {
				generated++
			}
		case "cline":
			if setupCline(absRoot, binPath) {
				generated++
			}
		case "codex":
			if setupCodex(absRoot, binPath) {
				generated++
			}
		case "all":
			if setupOpenCode(absRoot, binPath) {
				generated++
			}
			if setupClaude(absRoot, binPath) {
				generated++
			}
			if setupCursor(absRoot, binPath) {
				generated++
			}
			if setupWindsurf(absRoot, binPath) {
				generated++
			}
			if setupCline(absRoot, binPath) {
				generated++
			}
			if setupCodex(absRoot, binPath) {
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

func setupOpenCode(root, binPath string) bool {
	cfgPath := filepath.Join(root, ".opencode.json")

	entry := MCPEntry{
		Command: binPath,
		Args:    []string{"mcp", "--root", root},
	}

	var cfg OpenCodeConfig
	if data, err := os.ReadFile(cfgPath); err == nil {
		json.Unmarshal(data, &cfg)
	}

	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPEntry)
	}
	if _, exists := cfg.MCPServers["gomemory"]; exists {
		cfg.MCPServers["gomemory"] = entry
		fmt.Println("  ✅ opencode: .opencode.json actualizado")
		return true
	}

	cfg.MCPServers["gomemory"] = entry
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		fmt.Printf("  ⚠️  opencode: error al escribir %s: %v\n", cfgPath, err)
		return false
	}
	fmt.Println("  ✅ opencode: .opencode.json creado/actualizado")
	return true
}

func setupClaude(root, binPath string) bool {
	// Per-project .mcp.json (Claude Code standard)
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
				"command": binPath,
				"args":    []string{"mcp", "--root", root},
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

func setupCursor(root, binPath string) bool {
	cursorDir := filepath.Join(root, ".cursor")
	os.MkdirAll(cursorDir, 0755)
	mcpPath := filepath.Join(cursorDir, "mcp.json")

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command": binPath,
				"args":    []string{"mcp", "--root", root},
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
			"command": binPath,
			"args":    []string{"mcp", "--root", root},
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

func setupWindsurf(root, binPath string) bool {
	windsufDir := filepath.Join(root, ".windsurf")
	os.MkdirAll(windsufDir, 0755)
	mcpPath := filepath.Join(windsufDir, "mcp_config.json")

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command": binPath,
				"args":    []string{"mcp", "--root", root},
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
			"command": binPath,
			"args":    []string{"mcp", "--root", root},
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

func setupCline(root, binPath string) bool {
	clineDir := filepath.Join(root, ".cline")
	os.MkdirAll(clineDir, 0755)
	mcpPath := filepath.Join(clineDir, "mcp_settings.json")

	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command":     binPath,
				"args":        []string{"mcp", "--root", root},
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
			"command":     binPath,
			"args":        []string{"mcp", "--root", root},
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

// setupCodex registra gomemory en ~/.codex/config.toml. Es un único archivo
// global compartido entre todos los proyectos, por lo que cada proyecto usa
// una tabla [mcp_servers."gomemory_<key>"] propia para no pisar a otros.
// Solo se agrega (append) si la tabla todavía no existe — nunca se reescribe
// el archivo completo, para no arriesgar corromper TOML ya presente.
func setupCodex(root, binPath string) bool {
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

	block := fmt.Sprintf("\n%s\ncommand = %q\nargs = [%q, %q, %q]\ncwd = %q\n",
		tableHeader, binPath, "mcp", "--root", root, root)

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
