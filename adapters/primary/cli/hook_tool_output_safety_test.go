package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"mem/domain"
)

func claudeBashPayload(command string) []byte {
	b, _ := json.Marshal(map[string]any{
		"tool_name":     "Bash",
		"tool_input":    map[string]any{"command": command},
		"tool_response": map[string]any{"stdout": strings.Repeat("línea de salida larga ", 40), "stderr": ""},
	})
	return b
}

// T044 (feature 035, FR-013/FR-014): las lecturas exactas y los diagnósticos
// nunca se comprimen; la búsqueda por patrón sí (como listado).
func TestToolOutputExclusion_ComandosDeLecturaExacta(t *testing.T) {
	for _, cmd := range []string{
		"sed -n 1,80p adapters/x.go", "cat go.mod", "head -50 a.txt", "tail -n 20 log.txt",
		"git diff HEAD~1", "git show abc123", "mem doctor", "  cat con-espacios-iniciales.go",
	} {
		p := claudeBashPayload(cmd)
		if name, excluded := toolOutputExclusion("claude", p); !excluded || name == "" {
			t.Errorf("%q debía excluirse (got %q, %v)", cmd, name, excluded)
		}
		if out := rewriteToolOutput("claude", p, fakeCompress); out != nil {
			t.Errorf("%q: una lectura exacta no se reescribe, salió %s", cmd, out)
		}
	}
}

func TestToolOutputExclusion_NoExcluyeOtrosComandos(t *testing.T) {
	for _, cmd := range []string{"go test ./...", "awk 'NR<=200' a.go", "grep -rn func .", "catalog-tool run", "headless start"} {
		if _, excluded := toolOutputExclusion("claude", claudeBashPayload(cmd)); excluded {
			t.Errorf("%q no es una lectura exacta: no debe excluirse", cmd)
		}
	}
}

func TestToolOutputExclusion_HerramientasDeConsultaExacta(t *testing.T) {
	snippet, _ := json.Marshal(map[string]any{"tool_name": "mcp__codebase-memory-mcp__get_code_snippet",
		"tool_response": []any{map[string]any{"type": "text", "text": strings.Repeat("x", 500)}}})
	if out := rewriteToolOutput("claude", snippet, fakeCompress); out != nil {
		t.Errorf("get_code_snippet devuelve código exacto: no se reescribe")
	}
	grep, _ := json.Marshal(map[string]any{"tool_name": "Grep",
		"tool_response": map[string]any{"mode": "content", "content": strings.Repeat("a.go:1: x\n", 60)}})
	if _, excluded := toolOutputExclusion("claude", grep); excluded {
		t.Error("Grep sigue siendo comprimible (como listado)")
	}
}

func TestToolOutputExclusion_OpenCodeConComando(t *testing.T) {
	p, _ := json.Marshal(map[string]any{"tool": "bash", "output": strings.Repeat("x", 500), "command": "cat README.md"})
	if out := rewriteToolOutput("opencode", p, fakeCompress); out != nil {
		t.Errorf("OpenCode aplica las mismas exclusiones por comando: %s", out)
	}
	sin, _ := json.Marshal(map[string]any{"tool": "bash", "output": strings.Repeat("x", 500)})
	if out := rewriteToolOutput("opencode", sin, fakeCompress); out == nil {
		t.Error("sin comando conocido, OpenCode sigue comprimiendo como antes")
	}
}

func TestToolOutputMinTokens_Umbral(t *testing.T) {
	if domain.ToolOutputMinTokens != 2000 {
		t.Errorf("el umbral del hook de salidas debe ser 2 000 tokens, got %d", domain.ToolOutputMinTokens)
	}
}

// T046 (feature 035, FR-015): la nota de cómo recuperar lo omitido aparece una
// vez por conversación, no en cada salida comprimida.
func TestRetrieveHintOnce(t *testing.T) {
	root := t.TempDir()
	if got := retrieveHintOnce(root); got != RetrieveHint {
		t.Errorf("la primera salida comprimida lleva la nota")
	}
	if got := retrieveHintOnce(root); got != "" {
		t.Errorf("las siguientes no la repiten: %q", got)
	}
}
