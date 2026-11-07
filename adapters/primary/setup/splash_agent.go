package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// splashAgentTarget dice dónde acepta un agente la definición de un subagente
// propio, relativo al HOME (HCD-047).
type splashAgentTarget struct {
	agent string
	// dir es la raíz de configuración de usuario del agente.
	dir []string
	// file es el archivo del agente splash, relativo a dir.
	file []string
}

// splashAgentTargets son los runtimes con definiciones de agente en archivo.
// agy define sus tipos con define_subagent y Codex con spawn_agent(fork_turns:
// none): ninguno de los dos lee un archivo de agente (sondeos de HCD-047).
var splashAgentTargets = []splashAgentTarget{
	{agent: "claude", dir: []string{".claude"}, file: []string{"agents", "splash.md"}},
	// OpenCode 2.x: ~/.config/opencode/agents, el mismo directorio donde otras
	// herramientas ya instalan sus subagentes (verificado en 2.2.5).
	{agent: "opencode", dir: []string{".config", "opencode"}, file: []string{"agents", "splash.md"}},
}

// InstallSplashAgent escribe la definición del subagente splash de cada agente
// cuyo cuerpo llega en bodies, con las mismas garantías que las habilidades:
// solo donde el agente ya tiene su directorio, solo si el contenido difiere y
// nunca a través de un enlace. Devuelve las rutas escritas.
func InstallSplashAgent(bodies map[string]string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var written []string
	for _, t := range splashAgentTargets {
		body := bodies[t.agent]
		if strings.TrimSpace(body) == "" {
			continue
		}
		base := filepath.Join(append([]string{home}, t.dir...)...)
		if info, err := os.Stat(base); err != nil || !info.IsDir() {
			continue
		}
		dest := filepath.Join(append([]string{base}, t.file...)...)
		if previo, err := os.ReadFile(dest); err == nil && string(previo) == body {
			continue
		}
		if err := writeManagedFile(dest, []byte(body)); err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}
