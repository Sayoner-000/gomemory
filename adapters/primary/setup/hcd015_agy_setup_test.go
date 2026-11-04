package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

var agyRef = AgentRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}

func readAgyJSON(t *testing.T, home, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(agyConfigDir(home), name))
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func writeAgyFile(t *testing.T, home, name, content string) {
	t.Helper()
	dir := agyConfigDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func agyBackups(t *testing.T, home string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(agyConfigDir(home), "*.bak-*"))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestHCD015InstallAgy(t *testing.T) {
	home := t.TempDir()
	writeAgyFile(t, home, "mcp_config.json", `{"mcpServers":{"ajeno":{"command":"otro"}}}`)
	writeAgyFile(t, home, "hooks.json", `{"ajeno":{"Stop":[{"type":"command","command":"otro"}]}}`)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

	changed, err := InstallAgyGlobal(home, agyRef, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 {
		t.Fatalf("archivos cambiados: %v", changed)
	}
	if len(agyBackups(t, home)) != 2 {
		t.Errorf("copias previas: %v", agyBackups(t, home))
	}
	servers := readAgyJSON(t, home, "mcp_config.json")["mcpServers"].(map[string]any)
	if servers["ajeno"] == nil || servers["gomemory"] == nil {
		t.Errorf("servidores: %v", servers)
	}
	hooks := readAgyJSON(t, home, "hooks.json")
	if hooks["ajeno"] == nil {
		t.Error("se perdió el hook ajeno")
	}
	group, _ := json.Marshal(hooks[agyHookGroup])
	for _, want := range []string{"mem hook agy SessionStart", "mem hook agy PreInvocation", "mem hook agy PreToolUse", "mem hook agy PostToolUse", "mem hook agy Stop"} {
		if !strings.Contains(string(group), want) {
			t.Errorf("falta %s en %s", want, group)
		}
	}
	if strings.Contains(string(group), "jq") || strings.Contains(string(group), ".claude") {
		t.Errorf("el grupo depende de jq o de Claude: %s", group)
	}

	again, err := InstallAgyGlobal(home, agyRef, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 || len(agyBackups(t, home)) != 2 {
		t.Errorf("no es idempotente: %v, %v", again, agyBackups(t, home))
	}

	writeAgyFile(t, home, "skills.json", `{roto`)
	if _, err := InstallAgyGlobal(home, agyRef, now); err == nil {
		t.Error("un JSON inválido debe rechazarse")
	}
	if b, _ := os.ReadFile(filepath.Join(agyConfigDir(home), "skills.json")); string(b) != `{roto` {
		t.Errorf("se tocó un archivo inválido: %s", b)
	}
}

func TestHCD015MigrateManualGroup(t *testing.T) {
	home := t.TempDir()
	manual := `{"gomemory-lifecycle":{"enabled":true,"Stop":[{"type":"command","command":"PAYLOAD=$(cat); jq -r . ~/.local/bin/mem hook turn-end"}]}}`
	writeAgyFile(t, home, "hooks.json", manual)

	if _, err := InstallAgyGlobal(home, agyRef, time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	hooks := readAgyJSON(t, home, "hooks.json")
	if len(hooks) != 1 {
		t.Errorf("grupos duplicados: %v", hooks)
	}
	rendered, _ := json.Marshal(hooks)
	if strings.Contains(string(rendered), "jq") || strings.Count(string(rendered), "mem hook agy Stop") != 1 {
		t.Errorf("no se reemplazó el grupo manual: %s", rendered)
	}
	backups := agyBackups(t, home)
	if len(backups) == 0 {
		t.Fatal("falta la copia previa")
	}
	found := false
	for _, path := range backups {
		if b, _ := os.ReadFile(path); string(b) == manual {
			found = true
		}
	}
	if !found {
		t.Error("la copia previa no conserva el grupo manual")
	}
}

func agySkillPaths(t *testing.T, home string) []string {
	t.Helper()
	var paths []string
	for _, entry := range readAgyJSON(t, home, "skills.json")["entries"].([]any) {
		paths = append(paths, entry.(map[string]any)["path"].(string))
	}
	return paths
}

func TestHCD016InstallDoesNotAddSkillsEntry(t *testing.T) {
	home := t.TempDir()
	if _, err := InstallAgyGlobal(home, agyRef, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agyConfigDir(home), "skills.json")); !os.IsNotExist(err) {
		t.Fatalf("la carpeta global de agy no necesita skills.json: %v", err)
	}
}

func TestHCD016InstallRepairsTildeSkillPaths(t *testing.T) {
	home := t.TempDir()
	absolute := filepath.Join(home, ".agents", "skills")
	quoted, err := json.Marshal(absolute)
	if err != nil {
		t.Fatal(err)
	}
	writeAgyFile(t, home, "skills.json", `{"entries":[
		{"path":"~/.claude/skills","exclude":["synced"]},
		{"path":"~/.agents/skills"},
		{"path":`+string(quoted)+`},
		{"path":"/opt/skills"}]}`)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	if _, err := InstallAgyGlobal(home, agyRef, now); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(agySkillPaths(t, home), "|")
	want := filepath.Join(home, ".claude", "skills") + "|" + absolute + "|/opt/skills"
	if got != want {
		t.Fatalf("rutas de skills:\n got %s\nwant %s", got, want)
	}
	entry := readAgyJSON(t, home, "skills.json")["entries"].([]any)[0].(map[string]any)
	if entry["exclude"] == nil {
		t.Error("se perdieron las exclusiones de la entrada reparada")
	}
	if changed, err := InstallAgyGlobal(home, agyRef, now.Add(time.Hour)); err != nil || len(changed) != 0 {
		t.Errorf("no es idempotente: %v, %v", changed, err)
	}
}

func TestHCD016AgySkillTargetDoesNotDependOnClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	const body = "---\nname: hcd\ndescription: fixture\n---\ncuerpo\n"

	if written, err := InstallAgentSkill("hcd", body); err != nil || len(written) != 0 {
		t.Fatalf("sin agy no se escribe nada: %v, %v", written, err)
	}
	if err := os.MkdirAll(agyConfigDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	written, err := InstallAgentSkill("hcd", body)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(agyConfigDir(home), "skills", "hcd", "SKILL.md")
	if len(written) != 1 || written[0] != want {
		t.Fatalf("destinos: %v", written)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Errorf("no debe crear la carpeta de Claude: %v", err)
	}
}

func TestHCD016AgySkillsAreManagedForUninstall(t *testing.T) {
	found := false
	for _, cell := range domain.CellsForActivity(domain.ActivityUninstallGlobal) {
		if cell.Agent == "agy" && cell.Kind == domain.KindNativeWrapper {
			found = strings.HasSuffix(filepath.Join(cell.Path...), filepath.Join(".gemini", "config", "skills"))
		}
	}
	if !found {
		t.Error("la retirada global no cubre las skills de agy")
	}
}

func TestInspectAgyEntry_FlexibleVersion(t *testing.T) {
	home := t.TempDir()
	hooks := `{"gomemory-lifecycle":{"SessionStart":[{"command":"mem hook agy SessionStart"}],"PreInvocation":[{"command":"mem hook agy PreInvocation"}]}}`
	writeAgyFile(t, home, "hooks.json", hooks)

	ch := InspectAgyEntry(home, "1.4.12")
	if ch.State != domain.StateOK {
		t.Fatalf("1.4.12 debe ser StateOK, obtenido: %v", ch.State)
	}

	chPatch := InspectAgyEntry(home, "1.4.13")
	if chPatch.State != domain.StateOK {
		t.Fatalf("1.4.13 debe ser StateOK, obtenido: %v (%s)", chPatch.State, chPatch.Detail)
	}

	chMajor := InspectAgyEntry(home, "2.0.0")
	if chMajor.State != domain.StateOutdated {
		t.Fatalf("2.0.0 debe ser StateOutdated, obtenido: %v", chMajor.State)
	}
}

// HCD-020: InstallAgyGlobal solo gestiona su entrada; codegraph y
// codebase-memory-mcp los registran sus instaladores y deben sobrevivir.
func TestInstallAgyGlobal_ConservaMCPAjenos(t *testing.T) {
	home := t.TempDir()
	writeAgyFile(t, home, "mcp_config.json", `{"mcpServers":{"codegraph":{"command":"/bin/codegraph","args":["serve","--mcp"]},"codebase-memory-mcp":{"command":"/bin/codebase-memory-mcp","args":[]}},"otraClave":true}`)
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	if _, err := InstallAgyGlobal(home, agyRef, now); err != nil {
		t.Fatal(err)
	}
	doc := readAgyJSON(t, home, "mcp_config.json")
	servers, _ := doc["mcpServers"].(map[string]any)
	for _, name := range []string{"gomemory", "codegraph", "codebase-memory-mcp"} {
		if _, ok := servers[name]; !ok {
			t.Fatalf("falta %s tras instalar: %v", name, servers)
		}
	}
	if cg, _ := servers["codegraph"].(map[string]any); cg["command"] != "/bin/codegraph" {
		t.Fatalf("codegraph alterado: %v", cg)
	}
	if doc["otraClave"] != true {
		t.Fatalf("clave ajena perdida: %v", doc)
	}
	changed, err := InstallAgyGlobal(home, agyRef, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range changed {
		if strings.HasSuffix(path, "mcp_config.json") {
			t.Fatalf("segunda instalación reescribió mcp_config.json: %v", changed)
		}
	}
}
