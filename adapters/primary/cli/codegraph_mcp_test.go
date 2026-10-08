package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/primary/console"
	"mem/application/ports"
)

// fakeCodeGraph instala un `codegraph` falso que registra sus argumentos y,
// como el real, añade su servidor MCP a ~/.claude.json.
func fakeCodeGraph(t *testing.T, home string) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	script := "#!/bin/sh\nif [ \"$2\" = --help ]; then echo 'Install codegraph MCP server --target --location'; exit 0; fi\necho \"$@\" >> " + log + "\nprintf '{\"mcpServers\":{\"codegraph\":{\"command\":\"codegraph\"}}}' > " + filepath.Join(home, ".claude.json") + "\n"
	if err := os.WriteFile(filepath.Join(bin, "codegraph"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return log
}

func TestEnsureCodeGraphMCP_RegistraSoloCuandoFalta(t *testing.T) {
	home := t.TempDir()
	log := fakeCodeGraph(t, home)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{"gomemory":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	step, ok := ensureCodeGraphMCP(ports.SettingsData{}, home, "")
	if !ok || step.Status != console.StepOK || step.Detail != "activo en claude" {
		t.Fatalf("paso = %+v ok=%v", step, ok)
	}
	calls, _ := os.ReadFile(log)
	if strings.TrimSpace(string(calls)) != strings.Join(codeGraphInstallCmd, " ") {
		t.Fatalf("debió ejecutar el comando del manual una vez: %q", calls)
	}

	if _, ok := ensureCodeGraphMCP(ports.SettingsData{}, home, ""); !ok {
		t.Fatal("con CodeGraph activo el paso se informa")
	}
	if calls2, _ := os.ReadFile(log); string(calls2) != string(calls) {
		t.Fatalf("ya registrado: no debe volver a instalar: %q", calls2)
	}
}

func TestEnsureCodeGraphMCP_RespetaGrafoApagadoYListaExplicita(t *testing.T) {
	home := t.TempDir()
	log := fakeCodeGraph(t, home)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{}`), 0o644)
	for _, s := range []ports.SettingsData{{CodeGraphDisabled: true}, {CodeGraphProviders: []string{"codebase-memory-mcp"}}} {
		if _, ok := ensureCodeGraphMCP(s, home, ""); ok {
			t.Fatalf("con %+v CodeGraph no está en uso", s)
		}
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("no debió ejecutar codegraph")
	}
}

func TestCodeGraphMCPState_ReconoceLasTresFormas(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	_ = os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[mcp_servers.codegraph]\ncommand = \"codegraph\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"mcp":{"servers":{"codegraph":{"command":["codegraph","serve","--mcp"]}}}}`), 0o644)
	state := codeGraphMCPState(home, "")
	if !state["codex"] || !state["opencode"] {
		t.Fatalf("estado = %v", state)
	}
	if _, present := state["claude"]; present {
		t.Fatal("sin ~/.claude.json, claude no está presente")
	}
}

// C-001 (ronda 3): una línea comentada en config.toml no es un registro.
func TestCodeGraphMCPState_CodexComentadoNoCuenta(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("# [mcp_servers.codegraph]\n[mcp_servers.gomemory]\ncommand = \"mem\"\n"), 0o644)
	if registered, present := codeGraphMCPState(home, "")["codex"]; !present || registered {
		t.Fatalf("codex con la sección comentada: presente=%v registrado=%v", present, registered)
	}
}

// S-001: un registro de Claude a nivel proyecto también cuenta.
func TestCodeGraphMCPState_ClaudeANivelProyecto(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{},"projects":{"`+root+`":{"mcpServers":{"codegraph":{"command":"codegraph"}}}}}`), 0o644)
	if !codeGraphMCPState(home, root)["claude"] {
		t.Fatal("registro en projects[raíz] no reconocido")
	}
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{}}`), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"codegraph":{"command":"codegraph"}}}`), 0o644)
	if !codeGraphMCPState(home, root)["claude"] {
		t.Fatal("registro en .mcp.json del proyecto no reconocido")
	}
}

// S-002: un binario llamado codegraph que no ofrece el instalador esperado
// no se ejecuta con --yes; queda el aviso con el comando manual.
func TestEnsureCodeGraphMCP_VerificaElInstaladorAntesDeEjecutar(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\necho 'otra herramienta'\n"
	_ = os.WriteFile(filepath.Join(bin, "codegraph"), []byte(script), 0o755)
	t.Setenv("PATH", bin)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{}}`), 0o644)
	step, ok := ensureCodeGraphMCP(ports.SettingsData{}, home, "")
	if !ok || step.Status != console.StepWarn || step.Manual != codeGraphInstallManual {
		t.Fatalf("paso = %+v", step)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "--yes") {
		t.Fatalf("no debió ejecutar install --yes: %q", calls)
	}
}

// Una entrada sin command ni url no es un servidor: en TOML basta una
// subtabla [mcp_servers.codegraph.tools.x] para crearla implícitamente.
func TestCodeGraphMCPState_EntradaSinServidorNoCuenta(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[mcp_servers.codegraph.tools.explore]\napproval = \"auto\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{"codegraph":{}}}`), 0o644)
	state := codeGraphMCPState(home, "")
	if state["codex"] || state["claude"] {
		t.Fatalf("entradas sin command/url no son registros: %v", state)
	}
}
