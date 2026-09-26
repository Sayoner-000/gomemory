package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// FR-012: en ~/.claude.json solo se retira la entrada de gomemory.
func TestRemoveJSONServerEntry_ConservaLasAjenas(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".claude.json")
	writeFile(t, p, `{"mcpServers":{"otro":{"command":"otro"},"gomemory":{"command":"mem","args":["mcp"]}},"theme":"dark"}`)

	changed, err := removeJSONServerEntry(p, "mcpServers")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(readFile(t, p)), &got); err != nil {
		t.Fatal(err)
	}
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["gomemory"]; ok {
		t.Error("gomemory sigue registrado")
	}
	if _, ok := servers["otro"]; !ok || got["theme"] != "dark" {
		t.Errorf("se perdió configuración ajena: %v", got)
	}

	if changed, err := removeJSONServerEntry(p, "mcpServers"); changed || err != nil {
		t.Errorf("la segunda pasada debe ser idempotente: %v %v", changed, err)
	}
}

// Un archivo que no se puede interpretar no se toca: el paso queda en ⚠.
func TestRemoveJSONServerEntry_JSONInvalidoNoSeToca(t *testing.T) {
	p := filepath.Join(t.TempDir(), "opencode.json")
	writeFile(t, p, `{"mcp": {roto`)
	if _, err := removeJSONServerEntry(p, "mcp"); err == nil {
		t.Fatal("un JSON inválido debe devolver error")
	}
	if readFile(t, p) != `{"mcp": {roto` {
		t.Error("el archivo inválido no debe modificarse")
	}
}

// FR-012: en ~/.codex/config.toml se retiran el servidor y los hooks de
// gomemory; el resto del archivo (servidores y hooks ajenos) se conserva.
func TestRemoveCodexGomemory(t *testing.T) {
	base := "model = \"gpt-5\"\n\n[mcp_servers.otro]\ncommand = \"otro\"\n\n" +
		"[mcp_servers.gomemory]\ncommand = \"mem\"\nargs = [\"mcp\"]\n\n" +
		"[[hooks.SessionStart]]\n[[hooks.SessionStart.hooks]]\ntype = \"command\"\ncommand = \"otra-herramienta start\"\n"
	withHooks, _, err := ensureCodexGomemoryHooks([]byte(base), "mem")
	if err != nil {
		t.Fatal(err)
	}

	out, changed, err := removeCodexGomemory(withHooks)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	s := string(out)
	if strings.Contains(s, "gomemory") || strings.Contains(s, "mem hook") {
		t.Errorf("quedan restos de gomemory:\n%s", s)
	}
	var doc map[string]any
	if err := toml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("el resultado debe seguir siendo TOML válido: %v\n%s", err, s)
	}
	if doc["model"] != "gpt-5" || !strings.Contains(s, "[mcp_servers.otro]") || !strings.Contains(s, "otra-herramienta start") {
		t.Errorf("se perdió configuración ajena:\n%s", s)
	}

	if _, changed, _ := removeCodexGomemory(out); changed {
		t.Error("la segunda pasada debe ser idempotente")
	}
}

func TestRemoveCodexGomemory_TOMLInvalido(t *testing.T) {
	if _, _, err := removeCodexGomemory([]byte("[mcp_servers.gomemory\ncommand=")); err == nil {
		t.Fatal("un TOML inválido debe devolver error")
	}
}

// El bloque de protocolo de un archivo de instrucciones global se retira sin
// tocar lo que escribió la persona.
func TestRemoveProtocolBlockFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "CLAUDE.md")
	writeFile(t, p, "# Mis reglas\n\nNo tocar esto.\n\n"+integrationVersionMarker+"\n## Memoria Persistente\n\nprotocolo\n")
	changed, err := removeProtocolBlockFile(p)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got := readFile(t, p)
	if strings.Contains(got, "gomemory-protocol") || !strings.Contains(got, "No tocar esto.") {
		t.Errorf("resultado:\n%s", got)
	}

	// Instrucciones globales: baseline universal + protocolo, como las deja
	// composeAgentFile; se retiran ambos bloques.
	global := filepath.Join(t.TempDir(), "CLAUDE.md")
	compuesto, _ := composeAgentFile("# Personal\n\nMis notas.\n", embeddedTemplate("universal-agent-instructions.md"), buildIntegrationBlock())
	writeFile(t, global, compuesto)
	if _, err := removeProtocolBlockFile(global); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, global); strings.Contains(got, "gomemory") || !strings.Contains(got, "Mis notas.") {
		t.Errorf("instrucciones globales tras retirar:\n%s", got)
	}

	solo := filepath.Join(t.TempDir(), "AGENTS.md")
	writeFile(t, solo, integrationVersionMarker+"\n## Memoria Persistente\n\nprotocolo\n")
	if _, err := removeProtocolBlockFile(solo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(solo); !os.IsNotExist(err) {
		t.Error("un archivo que era solo el bloque de gomemory debe eliminarse")
	}
}

// Solo se retiran las habilidades y envoltorios que escribe gomemory; los
// ajenos en el mismo directorio se conservan.
func TestGlobalGeneratedArtifacts_SoloLosNuestros(t *testing.T) {
	home := t.TempDir()
	nuestro := filepath.Join(home, ".claude", "skills", "atomic-decomposition", "SKILL.md")
	ajeno := filepath.Join(home, ".claude", "skills", "mi-habilidad", "SKILL.md")
	writeFile(t, nuestro, "x")
	writeFile(t, ajeno, "y")
	writeFile(t, filepath.Join(home, ".codex", "skills", "adversarial-consensus-review", "SKILL.md"), "z")

	removed, errs := removeGlobalGeneratedArtifacts(home)
	if len(errs) != 0 {
		t.Fatalf("errores: %v", errs)
	}
	if len(removed) != 2 {
		t.Errorf("retirados = %v", removed)
	}
	if _, err := os.Stat(filepath.Dir(nuestro)); !os.IsNotExist(err) {
		t.Error("la habilidad de gomemory debe retirarse entera")
	}
	if _, err := os.Stat(ajeno); err != nil {
		t.Error("la habilidad ajena no debe tocarse")
	}
}
