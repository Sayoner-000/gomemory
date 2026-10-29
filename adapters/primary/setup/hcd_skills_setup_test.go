package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallHCDSkillsOnlyExistingAgentsAndIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, root := range []string{".codex", ".config/opencode"} {
		if e := os.MkdirAll(filepath.Join(home, root), 0755); e != nil {
			t.Fatal(e)
		}
	}
	bodies, wrappers := map[string]string{}, map[string]string{}
	names := []string{"hcd", "hcd-define", "hcd-clarify", "hcd-approve", "hcd-slice", "hcd-tasks", "hcd-implement", "hcd-verify", "hcd-analyze", "hcd-close"}
	for _, name := range names {
		data, e := os.ReadFile(filepath.Join("../../../infrastructure/templates/hcd", name, "SKILL.md"))
		if e != nil {
			t.Fatal(e)
		}
		bodies[name] = string(data)
		data, e = os.ReadFile(filepath.Join("../../../infrastructure/templates/hcd/commands", name+".md"))
		if e != nil {
			t.Fatal(e)
		}
		wrappers[name+".md"] = string(data)
	}
	written, e := InstallHCDSkills(bodies, wrappers)
	if e != nil || len(written) != len(names)*3 {
		t.Fatalf("written=%d e=%v", len(written), e)
	}
	if _, e = os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(e) {
		t.Fatalf("created absent agent: %v", e)
	}
	again, e := InstallHCDSkills(bodies, wrappers)
	if e != nil || len(again) != 0 {
		t.Fatalf("rewrote equal contents=%v %v", again, e)
	}
}

// Un comando de OpenCode o un SKILL.md que es un enlace simbólico no se escribe
// a través del enlace: el archivo apuntado queda intacto y la instalación falla
// con un error que nombra el enlace.
func TestInstallHCDSkillsRefusesSymlinkDestinations(t *testing.T) {
	for _, dest := range []string{".config/opencode/commands/hcd.md", ".codex/skills/hcd/SKILL.md"} {
		t.Run(dest, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			for _, root := range []string{".codex", ".config/opencode"} {
				if e := os.MkdirAll(filepath.Join(home, root), 0755); e != nil {
					t.Fatal(e)
				}
			}
			target := filepath.Join(t.TempDir(), "ajeno.md")
			if e := os.WriteFile(target, []byte("contenido ajeno\n"), 0644); e != nil {
				t.Fatal(e)
			}
			link := filepath.Join(home, dest)
			if e := os.MkdirAll(filepath.Dir(link), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.Symlink(target, link); e != nil {
				t.Fatal(e)
			}
			_, e := InstallHCDSkills(map[string]string{"hcd": "cuerpo\n"}, map[string]string{"hcd.md": "comando\n"})
			if e == nil {
				t.Fatal("se esperaba error por el enlace simbólico")
			}
			if data, _ := os.ReadFile(target); string(data) != "contenido ajeno\n" {
				t.Fatalf("destino del enlace modificado: %q", data)
			}
		})
	}
}
