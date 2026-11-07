package setup

import (
	"os"
	"path/filepath"
	"testing"
)

const splashOpenCodeDePrueba = "---\ndescription: revisor\nmode: subagent\npermission:\n  \"*\": deny\n  read: allow\n---\nRevisa.\n"

// HCD-047: en OpenCode el agente vive en ~/.config/opencode/agents/splash.md
// (formato verificado contra los agentes ya instalados en OpenCode 2.2.5).
func TestOpenCodeSplashAgent(t *testing.T) {
	home := homeFalso(t, filepath.Join(".config", "opencode"))
	dest := filepath.Join(home, ".config", "opencode", "agents", "splash.md")
	escritos, err := InstallSplashAgent(map[string]string{"opencode": splashOpenCodeDePrueba, "claude": splashDePrueba})
	if err != nil || len(escritos) != 1 || escritos[0] != dest {
		t.Fatalf("instalación = %v %v, quiero solo %s", escritos, err, dest)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != splashOpenCodeDePrueba {
		t.Fatalf("contenido = %q %v", got, err)
	}
	found := false
	for _, rel := range GlobalGeneratedArtifacts() {
		if filepath.Join(rel...) == filepath.Join(".config", "opencode", "agents", "splash.md") {
			found = true
		}
	}
	if !found {
		t.Fatal("mem uninstall no retiraría el agente splash de OpenCode")
	}
}

// HCD-051: Codex lee ~/.codex/agents/<nombre>.toml y agy 1.5.3
// ~/.gemini/config/agents/<nombre>/agent.md (sondeo real del 2026-11-07).
// mem setup instala splash donde cada agente ya tiene su configuración, no
// toca los ausentes, no reescribe lo que no cambió y mem uninstall lo retira.
func TestSplashCodexYAgy(t *testing.T) {
	const codex = "name = \"splash\"\nsandbox_mode = \"read-only\"\n"
	const agy = "---\nname: splash\nsubagent: true\ninheritMcp: false\n---\n\n# splash\n\nRevisa.\n"
	bodies := map[string]string{"codex": codex, "agy": agy}
	destinos := map[string][]string{
		"codex": {".codex", "agents", "splash.toml"},
		"agy":   {".gemini", "config", "agents", "splash", "agent.md"},
	}

	// Solo Codex instalado: agy no gana configuración.
	home := homeFalso(t, ".codex")
	escritos, err := InstallSplashAgent(bodies)
	codexDest := filepath.Join(append([]string{home}, destinos["codex"]...)...)
	if err != nil || len(escritos) != 1 || escritos[0] != codexDest {
		t.Fatalf("solo Codex = %v %v, quiero %s", escritos, err, codexDest)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini")); !os.IsNotExist(err) {
		t.Fatalf("no debe crear la configuración de agy: %v", err)
	}

	// Los dos instalados: escribe ambos y la segunda vez no reescribe nada.
	home = homeFalso(t, ".codex", ".gemini")
	escritos, err = InstallSplashAgent(bodies)
	if err != nil || len(escritos) != 2 {
		t.Fatalf("Codex y agy = %v %v", escritos, err)
	}
	for agente, rel := range destinos {
		dest := filepath.Join(append([]string{home}, rel...)...)
		if got, err := os.ReadFile(dest); err != nil || string(got) != bodies[agente] {
			t.Fatalf("%s: contenido = %q %v", agente, got, err)
		}
		// En agy el agente es un directorio: se retira entero.
		retirar := rel
		if rel[len(rel)-1] == "agent.md" {
			retirar = rel[:len(rel)-1]
		}
		found := false
		for _, a := range GlobalGeneratedArtifacts() {
			if filepath.Join(a...) == filepath.Join(retirar...) {
				found = true
			}
		}
		if !found {
			t.Errorf("mem uninstall no retiraría el agente splash de %s", agente)
		}
	}
	if again, err := InstallSplashAgent(bodies); err != nil || len(again) != 0 {
		t.Fatalf("reinstalar sin cambios no debe escribir: %v %v", again, err)
	}
}

const splashDePrueba ="---\nname: splash\ndescription: revisor\ntools: Read, Grep, Glob\nomitClaudeMd: true\n---\nRevisa.\n"

// HCD-047: mem setup instala el agente splash de Claude solo donde Claude ya
// tiene su directorio, sin reescribir lo que no cambió, y mem uninstall lo
// encuentra entre los artefactos generados.
func TestClaudeSplashAgent(t *testing.T) {
	home := homeFalso(t, ".claude")
	dest := filepath.Join(home, ".claude", "agents", "splash.md")

	if escritos, err := InstallSplashAgent(map[string]string{"claude": "  \n"}); err != nil || len(escritos) != 0 {
		t.Fatalf("un cuerpo vacío no debe escribir: %v %v", escritos, err)
	}
	escritos, err := InstallSplashAgent(map[string]string{"claude": splashDePrueba})
	if err != nil || len(escritos) != 1 || escritos[0] != dest {
		t.Fatalf("instalación = %v %v, quiero %s", escritos, err, dest)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != splashDePrueba {
		t.Fatalf("contenido = %q %v", got, err)
	}
	if again, err := InstallSplashAgent(map[string]string{"claude": splashDePrueba}); err != nil || len(again) != 0 {
		t.Fatalf("reinstalar sin cambios no debe escribir: %v %v", again, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); !os.IsNotExist(err) {
		t.Fatalf("no debe crear la configuración de un agente que no se usa: %v", err)
	}
	found := false
	for _, rel := range GlobalGeneratedArtifacts() {
		if filepath.Join(rel...) == filepath.Join(".claude", "agents", "splash.md") {
			found = true
		}
	}
	if !found {
		t.Fatal("mem uninstall no retiraría el agente splash de Claude")
	}
}
