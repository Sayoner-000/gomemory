package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readOpenCodeConfig(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer opencode.json: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parsear opencode.json: %v", err)
	}
	return out
}

func openCodePermissions(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	cfg := readOpenCodeConfig(t, path)
	perm, ok := cfg["permission"].(map[string]interface{})
	if !ok {
		t.Fatalf("falta la clave 'permission' de primer nivel en %s; config: %v", path, cfg)
	}
	return perm
}

// TestOpenCodePermissions_PreApruebaGomemory cubre el hueco encontrado en D11:
// OpenCode no tenía NINGUNA gestión de permisos en gomemory. Sin ella, cada
// llamada del agente queda pidiendo aprobación y el protocolo no se aplica solo.
//
// El esquema es el real de OpenCode: clave `permission` de PRIMER NIVEL con
// comodines por servidor MCP. No es el `mcpServers[].autoApprove` de Claude Code
// — esa forma OpenCode la ignora por completo.
func TestOpenCodePermissions_PreApruebaGomemory(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "opencode.json")

	if err := writeOpenCodePermissions(cfgPath); err != nil {
		t.Fatalf("writeOpenCodePermissions: %v", err)
	}

	perm := openCodePermissions(t, cfgPath)
	if got := perm["gomemory_*"]; got != "allow" {
		t.Errorf(`permission["gomemory_*"] = %v, se esperaba "allow"`, got)
	}
}

// TestOpenCodePermissions_ForgetMemoryQuedaEnAsk protege la misma decisión de
// seguridad que ClaudeAutoAllowTools: forget_memory es irreversible. Un comodín
// plano la habría pre-aprobado de pasada, que es justo lo que hay que evitar.
func TestOpenCodePermissions_ForgetMemoryQuedaEnAsk(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "opencode.json")

	if err := writeOpenCodePermissions(cfgPath); err != nil {
		t.Fatalf("writeOpenCodePermissions: %v", err)
	}

	perm := openCodePermissions(t, cfgPath)
	if got := perm["gomemory_forget_memory"]; got != "ask" {
		t.Errorf(`permission["gomemory_forget_memory"] = %v, se esperaba "ask" (es irreversible)`, got)
	}
}

// TestOpenCodePermissions_NoUsaElEsquemaDeClaude es la prueba de regresión del
// error que el proyecto ya cometió una vez: escribir configuración con una forma
// que OpenCode ignora produce cero errores visibles y cero efecto. Ver el
// comentario de WriteOpenCodeMCP sobre el esquema legado.
func TestOpenCodePermissions_NoUsaElEsquemaDeClaude(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "opencode.json")

	if err := writeOpenCodePermissions(cfgPath); err != nil {
		t.Fatalf("writeOpenCodePermissions: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("leer opencode.json: %v", err)
	}
	if bs := string(data); strings.Contains(bs, "autoApprove") || strings.Contains(bs, "mcpServers") {
		t.Errorf("se escribió el esquema de Claude Code, que OpenCode ignora:\n%s", bs)
	}
}

// TestOpenCodePermissions_PreservaConfiguracionAjena verifica que no se pisa lo
// que la persona haya configurado por su cuenta, ni la entrada del servidor MCP.
func TestOpenCodePermissions_PreservaConfiguracionAjena(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "opencode.json")

	previo := `{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {"gomemory": {"type": "local", "command": ["mem", "mcp"], "enabled": true}},
  "permission": {"bash": "ask", "edit": "deny"},
  "theme": "tokyonight"
}`
	if err := os.WriteFile(cfgPath, []byte(previo), 0644); err != nil {
		t.Fatalf("escribir config previa: %v", err)
	}

	if err := writeOpenCodePermissions(cfgPath); err != nil {
		t.Fatalf("writeOpenCodePermissions: %v", err)
	}

	cfg := readOpenCodeConfig(t, cfgPath)
	if cfg["theme"] != "tokyonight" {
		t.Error("se perdió una clave ajena de primer nivel (theme)")
	}
	if _, ok := cfg["mcp"]; !ok {
		t.Error("se perdió la entrada del servidor MCP")
	}

	perm := openCodePermissions(t, cfgPath)
	if perm["bash"] != "ask" || perm["edit"] != "deny" {
		t.Errorf("se pisaron permisos configurados por la persona: %v", perm)
	}
	if perm["gomemory_*"] != "allow" {
		t.Error("no se añadió el permiso de gomemory sobre la configuración existente")
	}
}

// TestOpenCodePermissions_EsIdempotente cubre FR-029: reinstalar sobre un
// proyecto sin cambios no debe alterar el archivo.
func TestOpenCodePermissions_EsIdempotente(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "opencode.json")

	if err := writeOpenCodePermissions(cfgPath); err != nil {
		t.Fatalf("primera escritura: %v", err)
	}
	primera, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("leer tras la primera escritura: %v", err)
	}

	if err := writeOpenCodePermissions(cfgPath); err != nil {
		t.Fatalf("segunda escritura: %v", err)
	}
	segunda, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("leer tras la segunda escritura: %v", err)
	}

	if string(primera) != string(segunda) {
		t.Errorf("la segunda escritura modificó el archivo:\n--- antes ---\n%s\n--- después ---\n%s", primera, segunda)
	}
}

// El cableado de writeOpenCodePermissions dentro de InstallOpenCode e
// InstallOpenCodeGlobal NO se prueba aquí a propósito: installOpenCodePlugin
// escribe en ~/.config/opencode/plugins/ del usuario real (es su
// comportamiento de producción, no un defecto), y un test unitario no debe
// tocar el HOME de quien lo ejecuta. Esa verificación va contra el binario en
// un proyecto temporal, en el escenario 6b de quickstart.md.
