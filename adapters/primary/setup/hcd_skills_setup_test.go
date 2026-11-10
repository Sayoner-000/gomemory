package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallHCDSkillsOnlyExistingAgentsAndIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
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
			t.Setenv("USERPROFILE", home)
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

func TestHCD057InstalledSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Presentar los 4 agentes soportados
	agentDirs := []string{
		".claude",
		".codex",
		filepath.Join(".config", "opencode"),
		filepath.Join(".gemini", "config"),
	}
	for _, dir := range agentDirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	bodies, wrappers := map[string]string{}, map[string]string{}
	names := []string{"hcd", "hcd-define", "hcd-clarify", "hcd-approve", "hcd-slice", "hcd-tasks", "hcd-implement", "hcd-verify", "hcd-analyze", "hcd-close"}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("../../../infrastructure/templates/hcd", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		bodies[name] = string(data)
		data, err = os.ReadFile(filepath.Join("../../../infrastructure/templates/hcd/commands", name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		wrappers[name+".md"] = string(data)
	}

	// 1. Instalación inicial: debe escribir a los 4 agentes (10 skills * 4 agentes = 40 archivos SKILL.md + 10 comandos OpenCode = 50)
	written, err := InstallHCDSkills(bodies, wrappers)
	if err != nil {
		t.Fatalf("InstallHCDSkills failed: %v", err)
	}
	expectedFiles := len(names)*4 + len(names)
	if len(written) != expectedFiles {
		t.Fatalf("expected %d written files, got %d (%v)", expectedFiles, len(written), written)
	}

	// 2. Verificar que el contenido instalado en cada agente es idéntico a las plantillas
	targets := []struct {
		agent string
		path  func(name string) string
	}{
		{"claude", func(name string) string { return filepath.Join(home, ".claude", "skills", name, "SKILL.md") }},
		{"codex", func(name string) string { return filepath.Join(home, ".codex", "skills", name, "SKILL.md") }},
		{"opencode", func(name string) string {
			return filepath.Join(home, ".config", "opencode", "skills", name, "SKILL.md")
		}},
		{"agy", func(name string) string { return filepath.Join(home, ".gemini", "config", "skills", name, "SKILL.md") }},
	}
	for _, tgt := range targets {
		for _, name := range names {
			dest := tgt.path(name)
			content, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("missing installed skill for %s: %s (%v)", tgt.agent, dest, err)
			}
			if string(content) != bodies[name] {
				t.Fatalf("content mismatch for %s in %s", name, tgt.agent)
			}
		}
	}
	for _, name := range names {
		cmdPath := filepath.Join(home, ".config", "opencode", "commands", name+".md")
		content, err := os.ReadFile(cmdPath)
		if err != nil {
			t.Fatalf("missing opencode command %s: %v", cmdPath, err)
		}
		if string(content) != wrappers[name+".md"] {
			t.Fatalf("wrapper mismatch for %s", name)
		}
	}

	// 3. Idempotencia: segunda llamada no escribe nada
	again, err := InstallHCDSkills(bodies, wrappers)
	if err != nil || len(again) != 0 {
		t.Fatalf("expected 0 written on replay, got %d (%v) err=%v", len(again), again, err)
	}

	// 4. Ausencia: en un nuevo home sin agentes, no se crean agentes ausentes
	emptyHome := t.TempDir()
	t.Setenv("HOME", emptyHome)
	t.Setenv("USERPROFILE", emptyHome)
	writtenEmpty, err := InstallHCDSkills(bodies, wrappers)
	if err != nil || len(writtenEmpty) != 0 {
		t.Fatalf("expected 0 written in empty home, got %d err=%v", len(writtenEmpty), err)
	}
	for _, dir := range agentDirs {
		if _, err := os.Stat(filepath.Join(emptyHome, dir)); !os.IsNotExist(err) {
			t.Fatalf("created absent agent dir: %s", dir)
		}
	}
}
