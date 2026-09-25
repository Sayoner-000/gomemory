package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

// T026 — contrato de las herramientas MCP de compresión (feature 033),
// interrogando el servidor real con la conexión abierta.
func TestMCPPackTools(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mem-pack-tools")
	build := exec.Command("go", "build", "-o", bin, "./infrastructure")
	build.Dir = repoRootContract(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar binario: %v\n%s", err, out)
	}
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".memory"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, ".memory", "settings.json"), []byte(`{"context_compression_level":"structural"}`), 0o600)

	logText := strings.Repeat("2026-09-25 10:00:00 INFO petición servida en 12ms\n", 200) + "2026-09-25 10:00:01 ERROR fallo definitivo\n"
	arg := func(v any) string { b, _ := json.Marshal(v); return string(b) }

	// Fase 1: compress sin level (cliente antiguo) y con level=max.
	r := llamarMCP(t, bin, dir,
		[2]string{"pack_compress", arg(map[string]any{"text": "a   b\n\n\na   b"})},
		[2]string{"pack_compress", arg(map[string]any{"text": logText, "level": "max"})},
		[2]string{"pack_retrieve", arg(map[string]any{"ref": "deadbeef0000"})},
	)
	if r[0].err || strings.Count(r[0].text, "a b") != 1 {
		t.Errorf("pack_compress sin level debe seguir funcionando: %+v", r[0])
	}
	if r[1].err || !strings.Contains(r[1].text, "⟦mem⟧") || !strings.Contains(r[1].text, "ERROR fallo definitivo") {
		t.Fatalf("pack_compress level=max debe dejar marcadores y conservar el ERROR: %+v", r[1])
	}
	if !r[2].err {
		t.Error("una ref desconocida debe devolver un error MCP")
	}

	// Fase 2: la ref emitida se recupera byte a byte (otro proceso, mismo store).
	ref := regexp.MustCompile(`ref=([0-9a-f]{12,16})`).FindStringSubmatch(r[1].text)
	if ref == nil {
		t.Fatal("no se encontró la ref en la salida")
	}
	r2 := llamarMCP(t, bin, dir, [2]string{"pack_retrieve", arg(map[string]any{"ref": ref[1]})})
	if r2[0].err || r2[0].text != logText {
		t.Errorf("pack_retrieve debe devolver el original exacto (len %d frente a %d)", len(r2[0].text), len(logText))
	}

	auto := false
	for _, name := range domain.MCPAutoApprovableToolsFor(false) {
		if name == domain.ToolPackRetrieve {
			auto = true
		}
	}
	if !auto {
		t.Error("pack_retrieve solo lee: debe ser auto-aprobable")
	}
}

type respuestaMCP struct {
	text string
	err  bool
}

func llamarMCP(t *testing.T, bin, dir string, calls ...[2]string) []respuestaMCP {
	t.Helper()
	cmd := exec.Command(bin, "mcp")
	cmd.Dir = dir
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	w := func(s string) { _, _ = stdin.Write([]byte(s + "\n")) }
	w(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	w(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	for i, c := range calls {
		w(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, i+10, c[0], c[1]))
	}
	out := make([]respuestaMCP, len(calls))
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 8<<20), 8<<20)
		for got := 0; got < len(calls) && sc.Scan(); {
			var m struct {
				ID     int `json:"id"`
				Result struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
					IsError bool `json:"isError"`
				} `json:"result"`
				Error json.RawMessage `json:"error"`
			}
			if json.Unmarshal(sc.Bytes(), &m) != nil || m.ID < 10 {
				continue
			}
			var r respuestaMCP
			for _, c := range m.Result.Content {
				r.text += c.Text
			}
			r.err = m.Result.IsError || len(m.Error) > 0
			out[m.ID-10] = r
			got++
		}
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("el servidor MCP no respondió a tiempo")
	}
	return out
}
