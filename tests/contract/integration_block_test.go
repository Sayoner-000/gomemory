package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationBlock_GrafoYArbolSecuenciados cubre research.md §10 (feature
// 019): el bloque de protocolo instalado ya no reclama el modo plan como
// mandato rival del grafo de código ("independientemente de la tarea: chat,
// plan, resumen") — los enuncia como pasos complementarios de una misma
// instrucción, y el grafo queda nombrado como el instrumento de exploración
// del plan (INV-5: nunca como una degradación del papel del grafo).
func TestIntegrationBlock_GrafoYArbolSecuenciados(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	target := t.TempDir()

	cmd := exec.Command(bin, "install", target)
	cmd.Dir = target
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mem install: %v\n%s", err, out)
	}

	var content string
	for _, fname := range []string{"CLAUDE.md", "AGENTS.md"} {
		data, err := os.ReadFile(filepath.Join(target, fname))
		if err == nil {
			content += string(data)
		}
	}
	if content == "" {
		t.Fatal("no se encontró ningún archivo de instrucciones instalado")
	}

	if strings.Contains(content, "independientemente de la tarea (chat, plan, resumen)") {
		t.Error("el párrafo del grafo no debe reclamar el modo plan como mandato rival")
	}
	if !strings.Contains(content, "árbol de tareas atómicas") {
		t.Error("debe mencionar el árbol de tareas atómicas como la forma de la salida del plan")
	}
	if !strings.Contains(content, "alimenta") {
		t.Error("debe indicar que lo explorado con el grafo alimenta las hojas del árbol")
	}
}

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
}
