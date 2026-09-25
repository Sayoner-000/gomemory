package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helpers de las pruebas de la feature 033 (motor nativo de compresión). Todas
// corren contra el binario real con el store aislado (GOMEMORY_DATA_HOME), la
// regla de trabajo del proyecto para verificar qué escribe gomemory.

// proyectoCompresion crea un proyecto con settings.json dado.
func proyectoCompresion(t *testing.T, settings string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".memory"), 0o700); err != nil {
		t.Fatal(err)
	}
	escribirArchivo(t, filepath.Join(dir, ".memory", "settings.json"), settings)
	return dir
}

type memRun struct {
	stdout, stderr string
	code           int
}

func correrMem(t *testing.T, bin, dir string, env []string, stdin string, args ...string) memRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("mem %v: %v", args, err)
	}
	return memRun{out.String(), errb.String(), code}
}

// mcpLlamadas arranca `mem mcp` y ejecuta tools/call en orden. Mantiene stdin
// abierto hasta leer todas las respuestas: `mem mcp` termina al cerrarse stdin
// y un printf con tubería no llega a recibir nada (memoria del proyecto).
func mcpLlamadas(t *testing.T, bin, dir string, env []string, calls ...[2]string) []mcpRespuesta {
	t.Helper()
	cmd := exec.Command(bin, "mcp")
	cmd.Dir = dir
	cmd.Env = env
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	send := func(s string) { _, _ = stdin.Write([]byte(s + "\n")) }
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	for i, c := range calls {
		send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, i+10, c[0], c[1]))
	}
	res := make([]mcpRespuesta, len(calls))
	got := 0
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 8<<20), 8<<20)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for got < len(calls) && sc.Scan() {
			var m struct {
				ID     int `json:"id"`
				Result struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
					IsError bool `json:"isError"`
				} `json:"result"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(sc.Bytes(), &m) != nil || m.ID < 10 {
				continue
			}
			var r mcpRespuesta
			for _, c := range m.Result.Content {
				r.Text += c.Text
			}
			r.IsError = m.Result.IsError || m.Error != nil
			if m.Error != nil {
				r.Text = m.Error.Message
			}
			res[m.ID-10] = r
			got++
		}
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("el servidor MCP no respondió a tiempo")
	}
	return res
}

type mcpRespuesta struct {
	Text    string
	IsError bool
}
