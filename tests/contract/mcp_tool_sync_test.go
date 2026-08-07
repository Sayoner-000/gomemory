package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// Este archivo existe por un bug real: la tool get_plan_context (feature 013) se
// registró en el servidor MCP pero quedó fuera del `select:` de ToolSearch que
// gomemory inyecta en Claude Code. Como las tools de un servidor MCP llegan
// DIFERIDAS (existen por nombre, sin esquema), la tool era literalmente
// ininvocable: el protocolo le ordenaba al agente llamarla y no podía.
//
// Ninguna prueba lo detectó porque cada lista estaba escrita a mano y nada las
// comparaba contra las tools reales. Estos tests cierran ese hueco.

// toolsRegistradasEnElServidor arranca el servidor MCP real y le pide tools/list.
// Es la única fuente de verdad que no puede mentir: si una tool no está aquí, el
// agente no la tiene, digan lo que digan las constantes.
func toolsRegistradasEnElServidor(t *testing.T) []string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "mem-tools-test")
	build := exec.Command("go", "build", "-o", bin, "./infrastructure")
	build.Dir = repoRootContract(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar binario: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "mcp")
	cmd.Dir = t.TempDir()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("arrancar servidor MCP: %v", err)
	}
	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	io := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"contract-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}
	for _, line := range io {
		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("escribir petición: %v", err)
		}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var msg struct {
			ID     int `json:"id"`
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil || msg.ID != 2 {
			continue
		}
		names := make([]string, 0, len(msg.Result.Tools))
		for _, tool := range msg.Result.Tools {
			names = append(names, tool.Name)
		}
		return names
	}
	t.Fatal("el servidor MCP no respondió a tools/list")
	return nil
}

func repoRootContract(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

func ordenado(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// TestDomainRefleja LasToolsRegistradas es el guardián principal: si alguien añade
// una tool al servidor y olvida declararla en domain, todo lo demás (bootstrap,
// permisos, protocolo) queda desactualizado en silencio. Aquí falla en voz alta.
func TestDomainReflejaLasToolsRegistradas(t *testing.T) {
	reales := ordenado(toolsRegistradasEnElServidor(t))
	declaradas := ordenado(domain.MCPAllTools())

	if strings.Join(reales, ",") == strings.Join(declaradas, ",") {
		return
	}

	enDomain := map[string]bool{}
	for _, d := range declaradas {
		enDomain[d] = true
	}
	enServidor := map[string]bool{}
	for _, r := range reales {
		enServidor[r] = true
	}
	for _, r := range reales {
		if !enDomain[r] {
			t.Errorf("la tool %q está registrada en el servidor pero NO en domain.MCPAllTools() — quedará fuera del bootstrap de ToolSearch y el agente no podrá invocarla", r)
		}
	}
	for _, d := range declaradas {
		if !enServidor[d] {
			t.Errorf("domain.MCPAllTools() declara %q pero el servidor NO la registra — el bootstrap pedirá una tool inexistente", d)
		}
	}
}

// TestBootstrapMaterializaTodasLasTools cubre la causa raíz exacta del bug: el
// `select:` debe nombrar TODAS las tools, o las que falten quedan diferidas.
func TestBootstrapMaterializaTodasLasTools(t *testing.T) {
	bootstrap := cli.MemoryToolBootstrap()

	for _, tool := range domain.MCPAllTools() {
		prefijada := "mcp__gomemory__" + tool
		if !strings.Contains(bootstrap, prefijada) {
			t.Errorf("el bootstrap de ToolSearch no incluye %q: esa tool quedará diferida y el agente no podrá invocarla", prefijada)
		}
	}
	if !strings.Contains(bootstrap, "select:") {
		t.Error("el bootstrap debe emitir un 'select:' para que ToolSearch lo reconozca")
	}
}

// TestAutoApproveNoDejaFueraNingunaToolSegura evita que un agente tipo Cursor o
// Windsurf pida permiso justo en la acción que debería ser automática.
func TestAutoApproveNoDejaFueraNingunaToolSegura(t *testing.T) {
	auto := map[string]bool{}
	for _, tool := range persistence.DefaultSettings().AutoApproveTools {
		auto[tool] = true
	}

	for _, tool := range domain.MCPAutoApprovableTools() {
		if !auto[tool] {
			t.Errorf("la tool segura %q no está en AutoApproveTools: los agentes con auto-approve pedirán permiso al usarla", tool)
		}
	}
	for _, destructiva := range domain.MCPDestructiveTools {
		if auto[destructiva] {
			t.Errorf("la tool destructiva %q NO debe pre-aprobarse", destructiva)
		}
	}
}

// TestClaudeAutoAllowCubreTodasLasSeguras y su contraparte destructiva.
func TestClaudeAutoAllowCubreTodasLasSeguras(t *testing.T) {
	allow := map[string]bool{}
	for _, tool := range setup.ClaudeAutoAllowTools {
		allow[tool] = true
	}

	for _, tool := range domain.MCPAutoApprovableTools() {
		if !allow["mcp__gomemory__"+tool] {
			t.Errorf("falta mcp__gomemory__%s en ClaudeAutoAllowTools", tool)
		}
	}
	for _, destructiva := range domain.MCPDestructiveTools {
		if allow["mcp__gomemory__"+destructiva] {
			t.Errorf("la tool destructiva %q NO debe pre-aprobarse en Claude Code", destructiva)
		}
	}
}
