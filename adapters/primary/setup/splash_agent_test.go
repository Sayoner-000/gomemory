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

const splashDePrueba = "---\nname: splash\ndescription: revisor\ntools: Read, Grep, Glob\nomitClaudeMd: true\n---\nRevisa.\n"

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
