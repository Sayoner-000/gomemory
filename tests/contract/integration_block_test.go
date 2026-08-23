package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationBlock_GrafoYArbolSecuenciados verificaba la redacción del
// bloque de protocolo leyéndolo del CLAUDE.md/AGENTS.md que `mem install`
// escribía en el proyecto. La feature 021 retiró esa escritura: el bloque era
// una SEGUNDA copia del texto que el servidor MCP ya entrega en la respuesta
// initialize, y el archivo solo gastaba contexto y ensuciaba el repositorio.
//
// La cobertura NO se perdió. Las tres aserciones de redacción que hacía —que el
// párrafo del grafo no reclame el modo plan como mandato rival, que nombre el
// árbol de tareas atómicas, y que declare que lo explorado alimenta sus hojas—
// se comprueban sobre el canal que hoy es el único para ámbito de proyecto, en
// TestMCPInstructions_GrafoYArbolSecuenciados, justo debajo. Se añadió allí la
// tercera aserción, que solo vivía aquí.
//
// El texto es el mismo objeto en ambos casos: buildIntegrationBlock() alimenta
// tanto ServerOptions.Instructions como el ámbito de USUARIO que sigue
// escribiendo `setup-mcp --scope global`. La forma del bloque tiene además sus
// tests unitarios propios en cmd_install_protocol_test.go, intactos.

// TestMCPInstructions_GrafoYArbolSecuenciados cubre la misma redacción
// compuesta en el segundo canal de texto: las instrucciones que expone el
// servidor MCP (memoryProtocolReminder, cmd_hook.go / cmd_mcp.go).
func TestMCPInstructions_GrafoYArbolSecuenciados(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	// El primer prompt de la sesión emite el bootstrap + memoryProtocolReminder.
	res := runPlanEntered(t, bin, dir, "--emit=neutral") // fuerza a que exista .memory/
	_ = res
	cmd := exec.Command(bin, "hook", "user-prompt-submit")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook user-prompt-submit: %v\n%s", err, out)
	}

	content := string(out)
	if strings.Contains(content, "independientemente de la tarea: chat, plan, resumen") {
		t.Error("el párrafo del grafo en memoryProtocolReminder no debe reclamar el modo plan como mandato rival")
	}
	if !strings.Contains(content, "árbol de tareas atómicas") {
		t.Error("debe mencionar el árbol de tareas atómicas")
	}
	// Heredada del test del archivo instalado, retirado por la feature 021.
	if !strings.Contains(content, "alimenta") {
		t.Error("debe indicar que lo explorado con el grafo alimenta las hojas del árbol")
	}
}
