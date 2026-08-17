package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

// TestActivationInspect_ClaudeUserInstructionsMiraElSubdirectorioCorrecto
// cubre un bug real encontrado en T055 (verificación en vivo contra $HOME):
// el archivo de instrucciones de usuario de claude vive en
// home/.claude/CLAUDE.md (ver globalTargets en atomic_plan_global.go), no en
// home/CLAUDE.md directo. El inspector reportaba "ok" por coincidencia
// cuando home/AGENTS.md también existía en la versión vigente — sin mirar
// realmente el archivo que la instalación escribe.
func TestActivationInspect_ClaudeUserInstructionsMiraElSubdirectorioCorrecto(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	// Archivo señuelo en la raíz de HOME, en la versión vigente: si el
	// inspector lo mirara (el bug), reportaría "ok" sin haber comprobado
	// nunca el archivo real de claude.
	if err := os.WriteFile(filepath.Join(home, "AGENTS.md"),
		[]byte(domain.ProtocolVersionMarker+"\n## Memoria Persistente\nseñuelo\n"), 0644); err != nil {
		t.Fatalf("write señuelo: %v", err)
	}

	// El archivo REAL de claude, en una versión vieja.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir ~/.claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"),
		[]byte("<!-- gomemory-protocol-v4 -->\n## Memoria Persistente\ncontenido viejo\n"), 0644); err != nil {
		t.Fatalf("write ~/.claude/CLAUDE.md: %v", err)
	}

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Agent == "claude" && c.Kind == domain.KindInstructions && c.Scope == domain.ScopeUser {
			if c.State != domain.StateOutdated {
				t.Errorf("debía detectar ~/.claude/CLAUDE.md en v4 (outdated), got %v (detail=%q) — "+
					"si dice ok, está mirando el señuelo en ~/AGENTS.md en vez del archivo real", c.State, c.Detail)
			}
			if !strings.HasPrefix(c.Detail, "CLAUDE.md:") {
				t.Errorf("debe reportar CLAUDE.md (el archivo real bajo .claude/), no el señuelo AGENTS.md, got %q", c.Detail)
			}
			return
		}
	}
	t.Fatal("no se encontró el canal instructions de claude/user")
}

// TestActivationInspect_OpenCodeEntryComprubaElPluginRealNoElHookDeClaude
// cubre un segundo bug real encontrado junto al anterior (T055): el canal
// plan_entry de opencode se evaluaba con inspectClaudeHook — que busca
// PostToolUse:EnterPlanMode en .claude/settings.json — un mecanismo que
// pertenece exclusivamente a Claude Code. Contra el $HOME real, esto daba
// "ok" en ámbito usuario SOLO porque el hook de plan-entered de CLAUDE existe
// en ese mismo archivo compartido, sin comprobar en ningún momento si el
// plugin de OpenCode (~/.config/opencode/plugins/gomemory.ts) existe.
func TestActivationInspect_OpenCodeEntryComprubaElPluginRealNoElHookDeClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	// Registrar el hook de CLAUDE (plan-entered) en .claude/settings.json,
	// SIN instalar el plugin de OpenCode. Si el bug sigue presente, el canal
	// de opencode reportaría "ok" de todas formas.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settings := `{"hooks":{"PostToolUse":[{"matcher":"EnterPlanMode","hooks":[{"type":"command","command":"mem hook plan-entered"}]}]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Agent == "opencode" && c.Kind == domain.KindPlanEntry && c.Scope == domain.ScopeUser {
			if c.State == domain.StateOK {
				t.Fatalf("plan_entry de opencode no debe ser ok sin el plugin real instalado — "+
					"está mirando el hook de claude en vez de ~/.config/opencode/plugins/gomemory.ts, detail=%q", c.Detail)
			}
			return
		}
	}
	t.Fatal("no se encontró el canal plan_entry de opencode/user")
}

// TestActivationInspect_OpenCodeEntryOKConElPluginInstalado es el camino
// positivo del test anterior: con el plugin real presente, sí debe ser ok.
func TestActivationInspect_OpenCodeEntryOKConElPluginInstalado(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	pluginDir := filepath.Join(home, ".config", "opencode", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "gomemory.ts"), []byte("// plugin"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Agent == "opencode" && c.Kind == domain.KindPlanEntry && c.Scope == domain.ScopeUser {
			if c.State != domain.StateOK {
				t.Errorf("con el plugin real presente, plan_entry de opencode debe ser ok, got %v (%q)", c.State, c.Detail)
			}
			return
		}
	}
	t.Fatal("no se encontró el canal plan_entry de opencode/user")
}

func TestActivationInspect_OutdatedInstructions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	claudeMD := "# notas\n\n<!-- gomemory-protocol-v6 -->\n## Memoria Persistente (`mem`) — Protocolo Activo\ncontenido viejo\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(claudeMD), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	found := false
	for _, c := range channels {
		if c.Agent == "claude" && c.Kind == domain.KindInstructions && c.Scope == domain.ScopeProject {
			found = true
			if c.State != domain.StateOutdated {
				t.Errorf("instrucciones con marcador v6 debían quedar outdated, got %v (%v)", c.State, c.Detail)
			}
		}
	}
	if !found {
		t.Fatal("no se encontró el canal instructions de claude/project")
	}
}

func TestActivationInspect_MissingWhenNothingInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Agent == "claude" && c.Kind == domain.KindPlanGuard && c.Scope == domain.ScopeProject {
			if c.State != domain.StateMissing {
				t.Errorf("sin instalación, plan_guard de claude/project debía ser missing, got %v", c.State)
			}
			return
		}
	}
	t.Fatal("no se encontró el canal plan_guard de claude/project")
}

func TestActivationInspect_OKWhenGuardRegistered(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settings := `{"hooks":{"PreToolUse":[{"matcher":"ExitPlanMode","hooks":[{"type":"command","command":"mem hook plan-guard"}]}]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Agent == "claude" && c.Kind == domain.KindPlanGuard && c.Scope == domain.ScopeProject {
			if c.State != domain.StateOK {
				t.Errorf("con el hook registrado, plan_guard debía ser ok, got %v", c.State)
			}
			return
		}
	}
	t.Fatal("no se encontró el canal plan_guard de claude/project")
}

func TestActivationInspect_DuplicatedWhenTwoEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settings := `{"hooks":{"PreToolUse":[
		{"matcher":"ExitPlanMode","hooks":[{"type":"command","command":"mem hook plan-guard"}]},
		{"matcher":"ExitPlanMode","hooks":[{"type":"command","command":"mem hook plan-guard"}]}
	]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Agent == "claude" && c.Kind == domain.KindPlanGuard && c.Scope == domain.ScopeProject {
			if c.State != domain.StateDuplicated {
				t.Errorf("con dos entradas registradas, plan_guard debía ser duplicated, got %v", c.State)
			}
			return
		}
	}
	t.Fatal("no se encontró el canal plan_guard de claude/project")
}

// TestActivationInspect_OpenCodeGuardEsNotApplicableConMotivo cubre FR-017/
// FR-019/T049: opencode no declara AgentLevelGuard, así que su canal
// plan_guard debe aparecer como degradación DECLARADA (not_applicable con
// motivo), nunca omitido en silencio ni disfrazado de otro estado.
func TestActivationInspect_OpenCodeGuardEsNotApplicableConMotivo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	found := false
	for _, c := range channels {
		if c.Agent == "opencode" && c.Kind == domain.KindPlanGuard {
			found = true
			if c.State != domain.StateNotApplicable {
				t.Errorf("plan_guard de opencode debe ser not_applicable, got %v", c.State)
			}
			if c.Detail == "" {
				t.Error("la degradación debe llevar un motivo, no puede estar vacía")
			}
		}
	}
	if !found {
		t.Fatal("opencode debe tener un canal plan_guard declarado (aunque sea not_applicable) — omitirlo sería ocultar la degradación")
	}
}

// TestActivationInspect_CodegraphAusenteSeOmite cubre INV-4: sin el brazo
// extensor instalado, sus canales se omiten del reporte, sin avisos.
func TestActivationInspect_CodegraphAusenteSeOmite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	inspector := NewActivationInspector()
	channels := inspector.Inspect(root)

	for _, c := range channels {
		if c.Arm == domain.ArmCodegraph {
			t.Fatalf("sin el brazo extensor instalado, no debe aparecer ningún canal codegraph: %+v", c)
		}
	}
}
