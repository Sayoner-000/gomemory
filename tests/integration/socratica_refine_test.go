package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// socraticaSession arranca el servidor MCP real (stdio) sobre un proyecto
// inicializado, con HOME y datos aislados.
type socraticaSession struct {
	t       *testing.T
	ctx     context.Context
	session *mcp.ClientSession
}

func newSocraticaSession(t *testing.T, home string) *socraticaSession {
	t.Helper()
	bin := buildMemBinary(t)
	target := t.TempDir()
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	cmd := exec.Command(bin, "mcp", "--root", target)
	cmd.Env = append(os.Environ(), "HOME="+home, "GOMEMORY_DATA_HOME="+t.TempDir())
	client := mcp.NewClient(&mcp.Implementation{Name: "socratica-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return &socraticaSession{t: t, ctx: ctx, session: session}
}

func (s *socraticaSession) raw(args map[string]any) (map[string]any, bool) {
	s.t.Helper()
	res, err := s.session.CallTool(s.ctx, &mcp.CallToolParams{Name: "socratica_flow", Arguments: args})
	if err != nil {
		s.t.Fatal(err)
	}
	if res.IsError {
		return map[string]any{"error": toolResultText(res)}, false
	}
	var out map[string]any
	if err = json.Unmarshal([]byte(toolResultText(res)), &out); err != nil {
		s.t.Fatal(err)
	}
	return out, true
}
func (s *socraticaSession) call(args map[string]any) map[string]any {
	s.t.Helper()
	out, ok := s.raw(args)
	if !ok {
		s.t.Fatalf("%v: %v", args, out["error"])
	}
	return out
}

// snapshotSkills lista los archivos bajo los directorios de habilidades del HOME.
func snapshotSkills(t *testing.T, home string) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{".claude", ".codex", ".opencode", ".config/opencode"} {
		_ = filepath.Walk(filepath.Join(home, dir), func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				files = append(files, p)
			}
			return nil
		})
	}
	return files
}

// TestSocraticaFlow_RecorridoRefine (T023): recorrido refine completo por MCP.
func TestSocraticaFlow_RecorridoRefine(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{".claude/skills", ".codex/skills", ".opencode/skills"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotSkills(t, home)
	s := newSocraticaSession(t, home)

	// Con la intercepción apagada (valor por defecto), la invocación explícita funciona (FR-022).
	if got := s.call(map[string]any{"action": "settings", "client": "claude"}); got["interception"] != false {
		t.Fatal(got)
	}
	question := func(text string) map[string]any {
		return map[string]any{"text": text, "options": []string{"Equipo", "Cliente"}}
	}
	start := s.call(map[string]any{"action": "start", "client": "claude", "mode": "refine", "prompt": "envía el informe", "questions": []any{question("¿Para quién?"), question("¿Formato?")}})
	id := start["flow_id"]
	if start["phase"] != "questions" || len(start["questions"].([]any)) != 2 {
		t.Fatal(start)
	}
	s.call(map[string]any{"action": "answer", "flow_id": id, "answers": []any{map[string]any{"index": 0, "choice": "Cliente"}}})
	if got := s.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true}); got["phase"] != "rewriting" {
		t.Fatal(got)
	}
	review := s.call(map[string]any{"action": "rewrite", "flow_id": id, "proposed": "Envía el informe al cliente antes de [fecha].", "additions": []string{"destinatario"}})
	if review["phase"] != "review" || review["original"] != "envía el informe" {
		t.Fatal(review)
	}
	s.call(map[string]any{"action": "edit", "flow_id": id, "prompt": "Envía el informe trimestral al cliente antes de [fecha]."})
	approved := s.call(map[string]any{"action": "approve", "flow_id": id})
	if approved["approved_prompt"] != "Envía el informe trimestral al cliente antes de [fecha]." {
		t.Fatal(approved)
	}
	if got := s.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": approved["dispatch_id"], "ack_nonce": "n1"}); got["acked"] != true {
		t.Fatal(got)
	}
	if got := s.call(map[string]any{"action": "approve", "flow_id": id}); got["duplicate"] != true || got["already_dispatched"] != true {
		t.Fatal(got)
	}

	// Cancelar otro flujo devuelve el original literal.
	other := s.call(map[string]any{"action": "start", "client": "claude", "prompt": "  texto  original\n"})
	cancelled := s.call(map[string]any{"action": "cancel", "flow_id": other["flow_id"]})
	if cancelled["phase"] != "cancelled" || cancelled["original"] != "  texto  original\n" || cancelled["proposed"] != "" {
		t.Fatal(cancelled)
	}

	// Un fallo exige una acción explícita: approve no basta, send_original sí.
	failing := s.call(map[string]any{"action": "start", "client": "claude", "prompt": "original tras fallo"})
	fid := failing["flow_id"]
	if got := s.call(map[string]any{"action": "fail", "flow_id": fid, "error": "modelo sin respuesta"}); got["phase"] != "failed" || got["original"] != "original tras fallo" {
		t.Fatal(got)
	}
	if _, ok := s.raw(map[string]any{"action": "approve", "flow_id": fid}); ok {
		t.Fatal("approve desde failed sin send_original")
	}
	if got := s.call(map[string]any{"action": "send_original", "flow_id": fid}); got["approved_prompt"] != "original tras fallo" {
		t.Fatal(got)
	}
	// Start con un pendiente lo retoma sin crear otro.
	if got := s.call(map[string]any{"action": "start", "client": "claude", "prompt": "nuevo"}); got["flow_id"] != fid || got["recovered"] != true {
		t.Fatal(got)
	}

	// El payload del modelo con 5 preguntas se recorta a 3 (G4).
	five := []any{question("1"), question("2"), question("3"), question("4"), question("5")}
	if got := s.call(map[string]any{"action": "start", "client": "codex", "prompt": "otro", "questions": five}); len(got["questions"].([]any)) != 3 {
		t.Fatal(got)
	}

	if after := snapshotSkills(t, home); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("el recorrido escribió en los directorios de habilidades (FR-025): %v", after)
	}
}
