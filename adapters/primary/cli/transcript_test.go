package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jsonlLines son fixtures que replican el formato real verificado contra un
// transcript de Claude Code (~/.claude/projects/.../*.jsonl): mensajes
// "user"/"assistant" con message.content, string plano para humanos y array
// de bloques text/tool_use para el asistente; los tool_result vuelven como
// entradas "user" con content array (no string).
func writeFixtureTranscript(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

const (
	userPlain = `{"type":"user","message":{"role":"user","content":%q}}`

	assistantEdit = `{"type":"assistant","message":{"role":"assistant","content":[` +
		`{"type":"tool_use","id":"toolu_1","name":"Edit","input":{"file_path":%q,"old_string":"a","new_string":"b"}}` +
		`]}}`

	assistantBash = `{"type":"assistant","message":{"role":"assistant","content":[` +
		`{"type":"tool_use","id":"toolu_2","name":"Bash","input":{"command":%q,"description":"run"}}` +
		`]}}`

	assistantText = `{"type":"assistant","message":{"role":"assistant","content":[` +
		`{"type":"text","text":"listo"}` +
		`]}}`

	userToolResult = `{"type":"user","message":{"role":"user","content":[` +
		`{"type":"tool_result","tool_use_id":"toolu_2","content":"ok"}` +
		`]}}`
)

func TestExtractLastTurnActivity_OnlyCapturesLatestTurn(t *testing.T) {
	lines := []string{
		fmtLine(userPlain, "arregla el bug en foo.go"),
		fmtLine(assistantEdit, "foo.go"), // turno viejo — no debe aparecer
		fmtLine(userPlain, "ahora corre los tests y actualiza bar.go"),
		fmtLine(assistantBash, "go test ./..."),
		userToolResult,
		fmtLine(assistantEdit, "bar.go"),
		assistantText,
	}
	path := writeFixtureTranscript(t, lines)

	activity := extractLastTurnActivity(path)

	if len(activity.Files) != 1 || activity.Files[0] != "bar.go" {
		t.Fatalf("expected only bar.go from the latest turn, got %v", activity.Files)
	}
	if len(activity.Commands) != 1 || activity.Commands[0] != "go test ./..." {
		t.Fatalf("expected the go test command, got %v", activity.Commands)
	}
}

func TestExtractLastTurnActivity_PureChatTurnIsEmpty(t *testing.T) {
	lines := []string{
		fmtLine(userPlain, "¿qué hace este archivo?"),
		assistantText,
	}
	path := writeFixtureTranscript(t, lines)

	activity := extractLastTurnActivity(path)

	if !activity.empty() {
		t.Fatalf("expected no activity for a pure-chat turn, got %+v", activity)
	}
}

func TestExtractLastTurnActivity_MissingFileReturnsEmpty(t *testing.T) {
	activity := extractLastTurnActivity(filepath.Join(t.TempDir(), "nope.jsonl"))
	if !activity.empty() {
		t.Fatalf("expected empty activity for missing file, got %+v", activity)
	}
}

func fmtLine(format string, args ...any) string {
	// fmt.Sprintf con %q ya deja el JSON válido porque los argumentos son
	// strings simples sin comillas embebidas en estos fixtures.
	return fmt.Sprintf(format, args...)
}
