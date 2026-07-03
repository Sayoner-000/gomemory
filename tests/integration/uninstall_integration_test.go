package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/primary/cli"
	"mem/adapters/secondary/persistence"
)

func uninstallTestDeps() *cli.Deps {
	return &cli.Deps{
		ProjectRepo:  persistence.NewProjectRepository(),
		SettingsRepo: persistence.NewSettingsRepository(),
	}
}

// buildFakeInstall reproduce lo que deja `mem install` en disco, sin invocar
// CmdInstall real: CmdInstall copia el binario en ejecución y lo ejecuta
// como subproceso (`mem init`), lo cual dentro de un test es el propio
// binario de test — ejecutarlo recursivamente reiniciaría toda la suite.
func buildFakeInstall(t *testing.T, target string) {
	t.Helper()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	if err := os.WriteFile(filepath.Join(target, "mem"), []byte("fake binary"), 0755); err != nil {
		t.Fatalf("write fake mem binary: %v", err)
	}

	mcpContent := `{"mcpServers":{"gomemory":{"command":"./mem","args":["mcp"]}}}`
	if err := os.WriteFile(filepath.Join(target, ".mcp.json"), []byte(mcpContent), 0644); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
	}

	agentContent := "# Instrucciones\n\nContenido del usuario que NO debe perderse.\n" +
		"\n<!-- gomemory-protocol-v3 -->\n## Memoria Persistente (`mem`) — Protocolo Activo\n\nTexto del protocolo...\n"
	if err := os.WriteFile(filepath.Join(target, "AGENTS.md"), []byte(agentContent), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	pluginDir := filepath.Join(target, ".claude", "plugins", "gomemory")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	settingsContent := fmt.Sprintf(`{"hooks":{"SessionStart":[%q]}}`, filepath.Join(pluginDir, "scripts", "session-start.sh"))
	if err := os.WriteFile(filepath.Join(target, ".claude", "settings.json"), []byte(settingsContent), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

func TestUninstallRemovesFullInstallation(t *testing.T) {
	target := t.TempDir()
	buildFakeInstall(t, target)
	deps := uninstallTestDeps()

	cli.CmdUninstall(deps, []string{target, "--yes"})

	requireGone := func(rel string) {
		if _, err := os.Stat(filepath.Join(target, rel)); err == nil {
			t.Fatalf("esperaba que %s NO exista tras mem uninstall", rel)
		}
	}
	requireGone("mem")
	requireGone(".memory")
	requireGone(filepath.Join(".claude", "plugins", "gomemory"))

	data, err := os.ReadFile(filepath.Join(target, ".mcp.json"))
	if err != nil {
		t.Fatalf(".mcp.json no debió eliminarse por completo (puede tener otras entradas): %v", err)
	}
	if strings.Contains(string(data), "gomemory") {
		t.Fatal(".mcp.json todavía contiene la entrada gomemory tras uninstall")
	}

	agentData, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md no debió eliminarse (tenía contenido del usuario): %v", err)
	}
	if strings.Contains(string(agentData), "gomemory-protocol-v3") {
		t.Fatal("AGENTS.md todavía contiene el bloque de protocolo tras uninstall")
	}
	if !strings.Contains(string(agentData), "Contenido del usuario que NO debe perderse") {
		t.Fatal("uninstall no debió borrar el contenido del usuario anterior al bloque")
	}
}

func TestUninstallDeletesAgentFileGeneratedEntirelyByInstall(t *testing.T) {
	target := t.TempDir()
	deps := uninstallTestDeps()

	// Reproduce defaultAgentFile("AGENTS.md"): solo título + bloque, sin
	// contenido previo del usuario — todo el archivo es generado por install.
	generated := "# Instrucciones para agentes AI\n" +
		"\n<!-- gomemory-protocol-v3 -->\n## Memoria Persistente (`mem`) — Protocolo Activo\n\nTexto del protocolo...\n"
	if err := os.WriteFile(filepath.Join(target, "AGENTS.md"), []byte(generated), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	cli.CmdUninstall(deps, []string{target, "--yes"})

	if _, err := os.Stat(filepath.Join(target, "AGENTS.md")); err == nil {
		t.Fatal("AGENTS.md generado enteramente por install debió eliminarse por completo, no solo vaciar el bloque")
	}
}

func TestUninstallCancelsWithoutConfirmation(t *testing.T) {
	target := t.TempDir()
	buildFakeInstall(t, target)
	deps := uninstallTestDeps()

	stdin, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	w.WriteString("no\n")
	w.Close()

	origStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = origStdin }()

	cli.CmdUninstall(deps, []string{target})

	if _, err := os.Stat(filepath.Join(target, "mem")); err != nil {
		t.Fatal("el binario no debió eliminarse al cancelar la confirmación")
	}
	if _, err := os.Stat(filepath.Join(target, ".memory")); err != nil {
		t.Fatal(".memory no debió eliminarse al cancelar la confirmación")
	}
}

func TestUninstallReportsMissingComponentsWithoutFailing(t *testing.T) {
	target := t.TempDir()
	deps := uninstallTestDeps()

	// Directorio vacío: gomemory nunca se instaló aquí.
	cli.CmdUninstall(deps, []string{target, "--yes"})
	// Si llegamos aquí sin panic/os.Exit, "reporta sin fallar" se cumple.
}

// TestUninstallRemovesPermissionsAndStopHook cubre dos bugs reales: (a) los
// permisos mcp__gomemory__* pre-aprobados por install sobrevivían a uninstall,
// y (b) el hook "Stop" (turn-end) no estaba en la lista de eventos limpiados,
// así que sobrevivía a la desinstalación.
func TestUninstallRemovesPermissionsAndStopHook(t *testing.T) {
	target := t.TempDir()
	deps := uninstallTestDeps()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	settingsContent := `{
		"hooks": {
			"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "mem hook turn-end"}]}],
			"SessionStart": [{"matcher": "", "hooks": [{"type": "command", "command": "mem hook session-start"}]}]
		},
		"permissions": {"allow": ["mcp__gomemory__save_memory", "mcp__other_server__some_tool"]},
		"enabledMcpjsonServers": ["gomemory", "other_server"]
	}`
	settingsPath := filepath.Join(target, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	cli.CmdUninstall(deps, []string{target, "--yes"})

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json no debió eliminarse: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json inválido tras uninstall: %v", err)
	}

	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		if _, has := hooks["Stop"]; has {
			t.Error("el hook Stop (turn-end) debió removerse en uninstall")
		}
	}

	if perms, ok := settings["permissions"].(map[string]interface{}); ok {
		allow, _ := perms["allow"].([]interface{})
		for _, v := range allow {
			if v == "mcp__gomemory__save_memory" {
				t.Error("el permiso mcp__gomemory__save_memory debió removerse en uninstall")
			}
		}
		hasOther := false
		for _, v := range allow {
			if v == "mcp__other_server__some_tool" {
				hasOther = true
			}
		}
		if !hasOther {
			t.Error("no debió eliminar el permiso de un servidor de terceros")
		}
	}

	if servers, ok := settings["enabledMcpjsonServers"].([]interface{}); ok {
		for _, v := range servers {
			if v == "gomemory" {
				t.Error("'gomemory' debió removerse de enabledMcpjsonServers")
			}
		}
	}
}
