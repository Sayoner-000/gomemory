package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMCPMemoriasYContextoConBinarioInstrumentado(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("mcp-workflow")
	if res := s.Run("", project, "init"); res.ExitCode != 0 {
		t.Fatalf("init: %+v", res)
	}
	args := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	calls := [][2]string{
		{"start_session", `{}`},
		{"save_memory", args(map[string]any{"title": "Regla de prueba", "type": "decision", "content": "Conservar el contexto del proyecto"})},
		{"save_memory", args(map[string]any{"title": "Aprendizaje de prueba", "type": "learning", "content": "El contexto se obtiene desde MCP"})},
		{"list_memories", `{}`},
		{"search_memories", `{"query":"contexto"}`},
		{"get_memory", `{"id":1}`},
		{"get_context", `{}`},
		{"get_plan_context", `{}`},
		{"save_session_summary", `{"summary":"Resumen de prueba"}`},
		{"end_session", `{"summary":"Sesión de prueba completada"}`},
	}
	for _, call := range calls {
		result := callMCPMemory(t, s, project, call)
		if result.err || strings.TrimSpace(result.text) == "" {
			t.Errorf("tool %s: error=%t salida=%q", call[0], result.err, result.text)
		}
	}
}

func callMCPMemory(t *testing.T, s *lifecycleSandbox, project string, call [2]string) respuestaMCP {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.GlobalBin, "mcp")
	cmd.Dir = project
	cmd.Env = s.Env()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	request := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"coverage-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, call[0], call[1]),
	}, "\n") + "\n"
	if _, err := stdin.Write([]byte(request)); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var response struct {
			ID     int `json:"id"`
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &response) != nil || response.ID != 2 {
			continue
		}
		out := respuestaMCP{err: response.Result.IsError || len(response.Error) > 0}
		for _, item := range response.Result.Content {
			out.text += item.Text
		}
		_ = stdin.Close()
		_ = cmd.Wait() // El servidor termina al cerrar stdin; la respuesta ya llegó.
		return out
	}
	_ = stdin.Close()
	_ = cmd.Wait()
	t.Fatalf("mcp %s no respondió: scanner=%v stderr=%q", call[0], scanner.Err(), stderr.String())
	return respuestaMCP{}
}
