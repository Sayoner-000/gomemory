package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FR-001: con un binario global, install no copia el binario al proyecto e
// informa de que usa el global.
func TestInstall_ConGlobalNoCopiaElBinario(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")

	res := s.Run("", p, "install", p)
	if res.ExitCode != 0 {
		t.Fatalf("install: code=%d\n%s\n%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, exeName("mem"))); !os.IsNotExist(err) {
		t.Fatalf("con global no debe existir %s/mem (err=%v)", p, err)
	}
	if !strings.Contains(res.Stdout, s.GlobalBin) || !strings.Contains(res.Stdout, "no se copia") {
		t.Errorf("install debe informar del global usado:\n%s", res.Stdout)
	}
}

// FR-003/FR-004a: install retira una copia de gomemory que ya estaba y lo
// cuenta en su salida.
func TestInstall_RetiraLaCopiaExistente(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p2")
	copyExecutable(t, buildLifecycleBinary(t), filepath.Join(p, exeName("mem")))

	res := s.Run("", p, "install", p)
	if res.ExitCode != 0 {
		t.Fatalf("install: code=%d\n%s\n%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, exeName("mem"))); !os.IsNotExist(err) {
		t.Fatal("la copia de gomemory debía retirarse")
	}
	if !strings.Contains(res.Stdout, "Se retiró") {
		t.Errorf("la retirada debe comunicarse:\n%s", res.Stdout)
	}
}

// FR-002: sin global, se mantiene la copia en el proyecto y se explica cómo
// pasar a una instalación global.
func TestInstall_SinGlobalMantieneLaCopia(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p3")
	standalone := filepath.Join(s.Root, "suelto", exeName("mem"))
	copyExecutable(t, s.GlobalBin, standalone)
	if err := os.Remove(s.GlobalBin); err != nil {
		t.Fatal(err)
	}

	res := s.Run(standalone, p, "install", p)
	if res.ExitCode != 0 {
		t.Fatalf("install: code=%d\n%s\n%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, exeName("mem"))); err != nil {
		t.Fatalf("sin global debe copiarse el binario al proyecto: %v", err)
	}
	if !strings.Contains(res.Stdout, "Sin `mem` global") {
		t.Errorf("debe explicar cómo pasar a global:\n%s", res.Stdout)
	}
}

// FR-021: solo se configura lo elegido con --agents; FR-023: la selección se
// guarda y una reinstalación sin flags la reutiliza.
func TestInstall_SoloLosAgentesElegidos(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	codexAntes := readAll(t, filepath.Join(s.Home, ".codex", "config.toml"))

	res := s.Run("", p, "install", "--agents", "claude", p)
	if res.ExitCode != 0 {
		t.Fatalf("install --agents claude: %d\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	if !strings.Contains(readAll(t, filepath.Join(p, ".mcp.json")), "gomemory") {
		t.Error("Claude Code debía configurarse (.mcp.json)")
	}
	for _, rel := range []string{"opencode.json", ".opencode", ".cursor"} {
		if _, err := os.Stat(filepath.Join(p, rel)); !os.IsNotExist(err) {
			t.Errorf("%s no debía escribirse con --agents claude", rel)
		}
	}
	if readAll(t, filepath.Join(s.Home, ".codex", "config.toml")) != codexAntes {
		t.Error("Codex no se eligió: su configuración global no debía cambiar")
	}

	var settings struct {
		Agents     []string `json:"agents"`
		AgentScope string   `json:"agent_scope"`
	}
	if err := json.Unmarshal([]byte(readAll(t, filepath.Join(p, ".memory", "settings.json"))), &settings); err != nil {
		t.Fatal(err)
	}
	if strings.Join(settings.Agents, ",") != "claude" || settings.AgentScope != "project" {
		t.Errorf("selección guardada = %+v", settings)
	}

	// Reinstalar sin flags (como hace mem update) reutiliza la selección.
	if r := s.Run("", p, "install", p); r.ExitCode != 0 {
		t.Fatalf("reinstall: %d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, "opencode.json")); !os.IsNotExist(err) {
		t.Error("la reinstalación debía reutilizar la selección guardada")
	}
}

// FR-020/FR-021: el alcance global instala la integración en el usuario y no
// añade otra configuración de Claude u OpenCode al proyecto.
func TestInstall_AlcanceGlobalNoDuplicaConfiguracionEnProyecto(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("global")
	r := s.Run("", p, "install", "--yes", "--agents", "claude,opencode", "--scope", "global", p)
	if r.ExitCode != 0 {
		t.Fatalf("install global: %d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	for _, rel := range []string{".mcp.json", ".claude/settings.json", "opencode.json"} {
		if _, err := os.Stat(filepath.Join(p, rel)); !os.IsNotExist(err) {
			t.Errorf("el alcance global escribió %s en el proyecto (err=%v)", rel, err)
		}
	}
}

func TestInstall_AlcanceGlobalRechazaAgenteSoloDeProyecto(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("cursor-global")
	r := s.Run("", p, "install", "--yes", "--agents", "cursor", "--scope", "global", p)
	if r.ExitCode != 2 || !strings.Contains(r.Stderr, "no admite --scope global") {
		t.Fatalf("agente sin scope global: code=%d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, ".memory")); !os.IsNotExist(err) {
		t.Fatalf("no se debía escribir en el proyecto: %v", err)
	}
}

func TestInstall_TerminaConResumenDePasos(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("resumen")
	r := s.Run("", p, "install", "--yes", "--agents", "claude", p)
	if r.ExitCode != 0 {
		t.Fatalf("install: %d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	i := strings.LastIndex(r.Stdout, "Resumen:")
	if i < 0 || !strings.Contains(r.Stdout[i:], "✓ Binario") || !strings.Contains(r.Stdout[i:], "✓ Integración Claude Code") {
		t.Fatalf("falta el resumen final por pasos:\n%s", r.Stdout)
	}
}

// FR-004a: en install, el aviso de retirada de la copia local forma parte del
// resumen final.
func TestInstall_ResumenIncluyeLaCopiaRetirada(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("retirada")
	copia := filepath.Join(p, exeName("mem"))
	copyExecutable(t, buildLifecycleBinary(t), copia)

	r := s.Run("", p, "install", "--yes", "--agents", "claude", p)
	if r.ExitCode != 0 {
		t.Fatalf("install: %d\n%s\n%s", r.ExitCode, r.Stdout, r.Stderr)
	}
	i := strings.LastIndex(r.Stdout, "Resumen:")
	if i < 0 {
		t.Fatalf("falta el resumen final:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stdout[i:], "✓ Binario: Se retiró "+copia) {
		t.Fatalf("el paso Binario del resumen debe anunciar la copia retirada:\n%s", r.Stdout[i:])
	}
}

// FR-022: un agente desconocido es un error de uso antes de escribir nada.
func TestInstall_AgenteDesconocido(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	res := s.Run("", p, "install", "--agents", "claude,vscode", p)
	if res.ExitCode != 2 {
		t.Fatalf("code=%d; quiero 2\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, ".memory")); !os.IsNotExist(err) {
		t.Error("no debía escribirse nada")
	}
}

// FR-022, SC-005: --yes y la ausencia de TTY nunca esperan entrada; sin
// selección guardada se usan los agentes detectados.
func TestInstall_NoInteractivoUsaLosDetectados(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	// El sandbox tiene ~/.codex (detectado) y ~/.claude.json, pero no ~/.claude
	// ni opencode ni cursor en el PATH.
	res := s.RunInput("", p, "", "install", "--yes", p)
	if res.ExitCode != 0 {
		t.Fatalf("install --yes: %d\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	var settings struct {
		Agents []string `json:"agents"`
	}
	_ = json.Unmarshal([]byte(readAll(t, filepath.Join(p, ".memory", "settings.json"))), &settings)
	if strings.Join(settings.Agents, ",") != "codex" {
		t.Errorf("agentes detectados guardados = %v; quiero [codex]", settings.Agents)
	}
}
