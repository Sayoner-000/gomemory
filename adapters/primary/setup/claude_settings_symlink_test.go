package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// H-01 (ADM 2026-10-22): un .claude/settings.json del proyecto que es un
// symlink no debe llevar las escrituras de gomemory a su destino, que puede
// estar fuera del repositorio (o ser la configuración global de Claude).
func symlinkedProjectSettings(t *testing.T) (home, root, external string, before []byte) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	home, root = t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	external = filepath.Join(t.TempDir(), "externo.json")
	before = []byte(`{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook turn-end"}]}]}}`)
	if err := os.WriteFile(external, before, 0o644); err != nil {
		t.Fatal(err)
	}
	writeClaudeSettingsFile(t, home, string(before))
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, ".claude", "settings.json")); err != nil {
		t.Fatal(err)
	}
	return home, root, external, before
}

func assertUntouched(t *testing.T, external string, before []byte) {
	t.Helper()
	if got, _ := os.ReadFile(external); string(got) != string(before) {
		t.Errorf("el destino externo del symlink cambió:\n%s", got)
	}
}

func TestClaudeSettings_SymlinkNoSeSigue(t *testing.T) {
	for name, write := range map[string]func(home, root string) error{
		"dedup": func(home, root string) error { _, err := DedupClaudeProjectHooks(home, root); return err },
		"hooks": func(_, root string) error {
			return writeClaudeHooksSkipping(root, AgentRef{HookCommand: "mem"}, nil)
		},
		"permisos":        func(_, root string) error { return writeClaudePermissions(root) },
		"quitar-permisos": func(_, root string) error { _, err := RemoveClaudePermissions(root); return err },
	} {
		t.Run(name, func(t *testing.T) {
			home, root, external, before := symlinkedProjectSettings(t)
			if err := write(home, root); err == nil {
				t.Error("debe rechazar el symlink con un error")
			}
			assertUntouched(t, external, before)
		})
	}
}

func TestClaudeSettings_DirectorioEnlazadoNoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	root, external := t.TempDir(), t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	if err := os.Symlink(external, filepath.Join(root, ".claude")); err != nil {
		t.Fatal(err)
	}
	if err := writeClaudeHooksSkipping(root, AgentRef{HookCommand: "mem"}, nil); err == nil {
		t.Error("debe rechazar un .claude enlazado")
	}
	if entries, _ := os.ReadDir(external); len(entries) != 0 {
		t.Errorf("no debe escribir en el destino del enlace: %v", entries)
	}
}

// En el HOME el enlace se sigue: ~/.claude enlazado desde un repositorio de
// dotfiles es la configuración que la persona eligió.
func TestClaudeSettings_HomeEnlazadoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	home, dotfiles := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.Symlink(dotfiles, filepath.Join(home, ".claude")); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeHooksGlobal(home, AgentRef{HookCommand: "mem"}); err != nil {
		t.Fatalf("el ámbito global debe seguir el enlace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dotfiles, "settings.json")); err != nil {
		t.Errorf("los hooks globales deben quedar en el destino de dotfiles: %v", err)
	}
}

// H-02 (ADM 2026-10-22, 2.ª ronda): reemplazar el archivo no amplía sus
// permisos; uno nuevo queda 0644.
func TestWriteConfigFile_ConservaElModo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permisos de Unix")
	}
	dir := t.TempDir()
	privado := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(privado, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfigFile(privado, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(privado); info.Mode().Perm() != 0o600 {
		t.Errorf("un 0600 pasó a %o", info.Mode().Perm())
	}
	nuevo := filepath.Join(dir, "nuevo.json")
	if err := WriteConfigFile(nuevo, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(nuevo); info.Mode().Perm() != 0o644 {
		t.Errorf("un archivo nuevo quedó %o, want 644", info.Mode().Perm())
	}
}

// H-01 (ADM 2026-10-22, 2.ª ronda): .mcp.json y los MCP de proyecto de otros
// agentes reciben la misma protección que settings.json.
func TestProjectConfigPath_RechazaEnlaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	root, external := t.TempDir(), t.TempDir()
	if err := os.Symlink(filepath.Join(external, "x.json"), filepath.Join(root, ".mcp.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, ".cursor")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range [][]string{{".mcp.json"}, {".cursor", "mcp.json"}} {
		if _, err := ProjectConfigPath(root, rel...); err == nil {
			t.Errorf("%v: debe rechazar el enlace", rel)
		}
	}
	if p, err := ProjectConfigPath(root, ".windsurf", "mcp_config.json"); err != nil || p != filepath.Join(root, ".windsurf", "mcp_config.json") {
		t.Errorf("una ruta sin enlaces debe aceptarse: %q %v", p, err)
	}
}

// Un ~/.claude/settings.json que es en sí mismo un enlace a dotfiles se
// actualiza en su destino y sigue siendo un enlace: reemplazarlo con rename
// rompía la configuración de la persona.
func TestClaudeSettings_ArchivoGlobalEnlazadoConservaElEnlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	home, dotfiles := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target := filepath.Join(dotfiles, "settings.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".claude", "settings.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeHooksGlobal(home, AgentRef{HookCommand: "mem"}); err != nil {
		t.Fatalf("WriteClaudeHooksGlobal: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("el enlace de dotfiles debe conservarse: %v", err)
	}
	if data, _ := os.ReadFile(target); !strings.Contains(string(data), "mem hook") {
		t.Errorf("los hooks deben quedar en el destino del enlace:\n%s", data)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o600 {
		t.Errorf("el modo del destino cambió a %o", info.Mode().Perm())
	}
}

// Un .claude/plugins enlazado no recibe el plugin ni pierde contenido: la
// limpieza de artefactos legados (RemoveAll de scripts/) borraría fuera.
func TestInstallClaudeCode_PluginsEnlazadoNoSeToca(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	root, external := t.TempDir(), t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	keep := filepath.Join(external, "gomemory", "scripts", "ajeno.sh")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, ".claude", "plugins")); err != nil {
		t.Fatal(err)
	}
	if err := InstallClaudeCode(root, AgentRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}); err == nil {
		t.Error("debe rechazar el enlace")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("el contenido externo debe conservarse: %v", err)
	}
}

func TestWriteOpenCodeMCP_EnlaceNoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(external, []byte(`{"mcp":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "opencode.json")); err != nil {
		t.Fatal(err)
	}
	if err := WriteOpenCodeMCP(root, AgentRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}); err == nil {
		t.Error("debe rechazar el enlace")
	}
	if got, _ := os.ReadFile(external); string(got) != `{"mcp":{}}` {
		t.Errorf("el destino externo cambió: %s", got)
	}
}

// El opencode.json global enlazado desde dotfiles se actualiza en su destino
// y sigue siendo un enlace.
func TestOpenCodeGlobal_EnlaceDeDotfilesSeConserva(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	home, dotfiles := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target := filepath.Join(dotfiles, "opencode.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := registerOpenCodeGlobal(AgentRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}); err != nil {
		t.Fatalf("registerOpenCodeGlobal: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("el enlace de dotfiles debe conservarse")
	}
	if data, _ := os.ReadFile(target); !strings.Contains(string(data), "gomemory") {
		t.Errorf("el registro debe quedar en el destino del enlace:\n%s", data)
	}
}
