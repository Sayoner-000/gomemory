package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Este archivo cubre un hueco real encontrado al revisar la feature 013:
// get_context concatena memoryProtocolReminder a su salida, pero
// get_plan_context no lo hacía. Una sesión que solo entra en modo plan (nunca
// llama a get_context) se quedaba sin el recordatorio general de protocolo
// —guardar proactivamente, juez imparcial, privacidad, end_session— si el
// cliente MCP no renderiza initialize.instructions. Corregido en cmd_mcp.go.

// callGetPlanContext arranca el servidor MCP real en dir y llama a
// get_plan_context vía tools/call, devolviendo el texto de la respuesta.
func callGetPlanContext(t *testing.T, dir string) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "mem-reminder-test")
	build := exec.Command("go", "build", "-o", bin, "./infrastructure")
	build.Dir = repoRootContract(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar binario: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "mcp")
	cmd.Dir = dir
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

	peticiones := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"reminder-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_plan_context","arguments":{}}}`,
	}
	for _, line := range peticiones {
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
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil || msg.ID != 2 {
			continue
		}
		if len(msg.Result.Content) == 0 {
			return ""
		}
		return msg.Result.Content[0].Text
	}
	t.Fatal("el servidor MCP no respondió a tools/call")
	return ""
}

// marcaDeReminder es un fragmento único de memoryProtocolReminder (no aparece
// en el método de descomposición ni en el contexto del proyecto), así que su
// presencia/ausencia identifica sin ambigüedad si el recordatorio se incluyó.
const marcaDeReminder = "Memoria persistente activa (gomemory)"

// TestGetPlanContext_IncluyeRecordatorioDeProtocolo cubre el hueco: una sesión
// que solo planifica debe recibir el mismo recordatorio general que get_context,
// sin depender de que el cliente MCP muestre initialize.instructions.
func TestGetPlanContext_IncluyeRecordatorioDeProtocolo(t *testing.T) {
	dir := t.TempDir()

	got := callGetPlanContext(t, dir)

	if !strings.Contains(got, marcaDeReminder) {
		t.Errorf("get_plan_context no incluye el recordatorio de protocolo; respuesta:\n%.200s", got)
	}
	if !strings.HasPrefix(got, marcaDeReminder) {
		t.Error("el recordatorio debe ir AL INICIO de la respuesta, igual que en get_context")
	}
	if !strings.Contains(got, "Descomposición Atómica") {
		t.Error("el método debe seguir presente después del recordatorio")
	}
}

// TestGetPlanContext_ApagadoNoIncluyeRecordatorio protege FR-032: con la
// funcionalidad apagada, la salida debe ser estrictamente vacía. Añadir el
// recordatorio sobre una salida vacía contradiría al interruptor — el usuario
// que la apagó no debería ver NADA, ni siquiera un recordatorio.
func TestGetPlanContext_ApagadoNoIncluyeRecordatorio(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("crear .memory: %v", err)
	}
	settings := filepath.Join(memDir, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"atomic_plan_disabled":true}`), 0o644); err != nil {
		t.Fatalf("escribir settings: %v", err)
	}

	got := callGetPlanContext(t, dir)

	if got != "" {
		t.Errorf("con atomic_plan_disabled=true la salida debe ser vacía, se obtuvo:\n%.200s", got)
	}
}
