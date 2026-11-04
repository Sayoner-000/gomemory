package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		t.Skip("el ejecutable simulado es un script de shell")
	}
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
	if strings.TrimSpace(string(calls)) != strings.Join(codeGraphInstallArgs([]string{"claude"}), " ") {
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
	// La clave va serializada: una ruta de Windows lleva "\\", que sin escapar no es JSON válido.
	key, _ := json.Marshal(root)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{},"projects":{`+string(key)+`:{"mcpServers":{"codegraph":{"command":"codegraph"}}}}}`), 0o644)
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
	if runtime.GOOS == "windows" {
		t.Skip("el ejecutable simulado es un script de shell")
	}
	home := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\necho 'otra herramienta'\n"
	_ = os.WriteFile(filepath.Join(bin, "codegraph"), []byte(script), 0o755)
	t.Setenv("PATH", bin)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{}}`), 0o644)
	step, ok := ensureCodeGraphMCP(ports.SettingsData{}, home, "")
	if !ok || step.Status != console.StepWarn || step.Manual != codeGraphManual([]string{"claude"}) {
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

// HCD-020 G1: agy registra sus MCP en ~/.gemini/config/mcp_config.json.
func TestCodeGraphMCP_Agy(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".gemini", "config")
	_ = os.MkdirAll(dir, 0o755)
	cfg := filepath.Join(dir, "mcp_config.json")
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"gomemory":{"command":"mem","args":["mcp"]}}}`), 0o644)
	state := codeGraphMCPState(home, "")
	if registered, present := state["agy"]; !present || registered {
		t.Fatalf("agy sin codegraph: presente=%v registrado=%v", present, registered)
	}
	if missing := codeGraphMissing(state); len(missing) != 1 || missing[0] != "agy" {
		t.Fatalf("faltantes = %v", missing)
	}
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"codegraph":{"command":"codegraph","args":["serve","--mcp"]}}}`), 0o644)
	state = codeGraphMCPState(home, "")
	if !state["agy"] || len(codeGraphMissing(state)) != 0 {
		t.Fatalf("agy con codegraph: estado = %v", state)
	}
	if got := codeGraphRegistered(state); len(got) != 1 || got[0] != "agy" {
		t.Fatalf("registrados = %v", got)
	}
}

// HCD-020 G1: si solo falta en agy, ensureCodeGraphMCP lanza el instalador.
func TestEnsureCodeGraphMCP_SoloFaltaAgy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el ejecutable simulado es un script de shell")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".gemini", "config")
	_ = os.MkdirAll(dir, 0o755)
	cfg := filepath.Join(dir, "mcp_config.json")
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"gomemory":{"command":"mem"}}}`), 0o644)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{"codegraph":{"command":"codegraph"}}}`), 0o644)
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	script := "#!/bin/sh\nif [ \"$2\" = --help ]; then echo 'Install codegraph MCP server --target --location'; exit 0; fi\necho \"$@\" >> " + log + "\nprintf '{\"mcpServers\":{\"gomemory\":{\"command\":\"mem\"},\"codegraph\":{\"command\":\"codegraph\"}}}' > " + cfg + "\n"
	if err := os.WriteFile(filepath.Join(bin, "codegraph"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	step, ok := ensureCodeGraphMCP(ports.SettingsData{}, home, "")
	if !ok || step.Status != console.StepOK || step.Detail != "activo en claude, agy" {
		t.Fatalf("paso = %+v ok=%v", step, ok)
	}
	if calls, _ := os.ReadFile(log); strings.TrimSpace(string(calls)) != strings.Join(codeGraphInstallArgs([]string{"agy"}), " ") {
		t.Fatalf("debió ejecutar el instalador una vez: %q", calls)
	}
}

// HCD-020 G2: gomemory no registra codebase-memory-mcp en agy; solo avisa.
func TestDoctorCodebaseMemoryMCP_Agy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el ejecutable simulado es un script de shell")
	}
	home, bin := t.TempDir(), t.TempDir()
	_ = os.WriteFile(filepath.Join(bin, "codebase-memory-mcp"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin)
	if _, warn := agyCodebaseMemoryWarning(home); warn {
		t.Fatal("sin configuración de agy no hay nada que avisar")
	}
	dir := filepath.Join(home, ".gemini", "config")
	_ = os.MkdirAll(dir, 0o755)
	cfg := filepath.Join(dir, "mcp_config.json")
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"gomemory":{"command":"mem"}}}`), 0o644)
	msg, warn := agyCodebaseMemoryWarning(home)
	if !warn || !strings.Contains(msg, "codebase-memory-mcp install --clients=antigravity") || !strings.Contains(msg, "GEMINI.md") {
		t.Fatalf("aviso = %q warn=%v", msg, warn)
	}
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{"codebase-memory-mcp":{"command":"codebase-memory-mcp"}}}`), 0o644)
	if _, warn := agyCodebaseMemoryWarning(home); warn {
		t.Fatal("registrado: sin aviso")
	}
	t.Setenv("PATH", t.TempDir())
	_ = os.WriteFile(cfg, []byte(`{"mcpServers":{}}`), 0o644)
	if _, warn := agyCodebaseMemoryWarning(home); warn {
		t.Fatal("sin binario en PATH: cbm no está en uso, sin aviso")
	}
}

// HCD-020 Q1: --target auto también conecta Gemini CLI y crea
// ~/.gemini/GEMINI.md; gomemory nombra solo los agentes faltantes.
func TestEnsureCodeGraphMCP_TargetsExplicitos(t *testing.T) {
	if got := strings.Join(codeGraphInstallArgs([]string{"claude", "agy"}), " "); got != "install --target claude,antigravity --location global --yes" {
		t.Fatalf("argumentos = %q", got)
	}
	if got := codeGraphManual([]string{"agy"}); got != "codegraph install --target antigravity --location global --yes" {
		t.Fatalf("manual = %q", got)
	}
	for _, args := range [][]string{codeGraphInstallArgs([]string{"codex", "opencode"})} {
		if strings.Contains(strings.Join(args, " "), "auto") {
			t.Fatalf("no debe usar --target auto: %v", args)
		}
	}
}
