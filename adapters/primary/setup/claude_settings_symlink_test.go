package setup

import (
	"os"
	"path/filepath"
	"runtime"
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
