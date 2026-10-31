package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// globalTarget describe dónde vive, para un agente concreto, la configuración
// de nivel USUARIO: el archivo de instrucciones que ese agente carga en todos
// los proyectos, y el directorio de su formato propio de comandos/habilidades.
//
// Las rutas no son intercambiables entre agentes: Claude Code cuelga de
// ~/.claude y OpenCode de ~/.config/opencode. Por eso el ámbito global necesita
// esta tabla en vez de reutilizar las rutas relativas del ámbito de proyecto.
type globalTarget struct {
	agent string
	// dir es la raíz de configuración de usuario del agente, relativa al HOME.
	dir []string
	// instructions es el archivo de instrucciones que el agente lee siempre.
	// gomemory ya no escribe en él: solo retira los bloques heredados.
	instructions string
	// wrapper es la ruta del envoltorio nativo, relativa a dir. Vacío = el
	// agente no tiene un formato propio de comandos/habilidades que gomemory
	// sepa escribir; se instalan sus instrucciones y nada más. Inventar una
	// ruta produciría un archivo que ningún agente lee.
	wrapper []string
	// frontmatter que necesita ese envoltorio para ser descubierto.
	frontmatter string
}

var globalTargets = []globalTarget{
	{
		agent:        "claude",
		dir:          []string{".claude"},
		instructions: "CLAUDE.md",
		wrapper:      []string{"skills", "atomic-decomposition", "SKILL.md"},
		frontmatter: "---\n" +
			"name: atomic-decomposition\n" +
			"description: Descompone una solicitud grande, multi-paso o de resultado incierto en tareas atómicas verificables antes de planificar. Úsala en modo plan.\n" +
			"---\n\n",
	},
	{
		agent:        "opencode",
		dir:          []string{".config", "opencode"},
		instructions: "AGENTS.md",
		wrapper:      []string{"commands", "atomic-decomposition.md"},
		frontmatter: "---\n" +
			"description: Descompone el objetivo en tareas atómicas verificables antes de planificar\n" +
			"---\n\n",
	},
	{
		agent: "codex",
		dir:   []string{".codex"},
		// Sin envoltorio nativo: Codex no expone un formato de comandos propio
		// que gomemory pueda escribir, y el método le llega con get_plan_context.
		instructions: "AGENTS.md",
	},
}

// InstallAtomicPlanGlobal instala, en el ámbito de usuario, el envoltorio nativo
// del método para los agentes que ya tienen configuración de nivel usuario.
//
// No escribe en los archivos de instrucciones: protocolo y baseline llegan por
// MCP, hooks y contexto (HCD-009). Codex no tiene envoltorio y recibe el método
// con get_plan_context.
//
// Devuelve la lista de rutas efectivamente escritas, para poder informarlas.
func InstallAtomicPlanGlobal(method string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(method) == "" {
		return nil, nil
	}

	var written []string
	for _, t := range globalTargets {
		base := filepath.Join(append([]string{home}, t.dir...)...)
		// Solo se escribe si el agente ya tiene su directorio de configuración:
		// crear ~/.claude o ~/.config/opencode para un agente que la persona no
		// usa sería ensuciar su HOME sin motivo.
		if info, err := os.Stat(base); err != nil || !info.IsDir() || len(t.wrapper) == 0 {
			continue
		}
		dest := filepath.Join(append([]string{base}, t.wrapper...)...)
		p, err := writeGlobalWrapper(dest, t.frontmatter, method)
		if err != nil {
			return written, err
		}
		if p != "" {
			written = append(written, p)
		}
	}
	return written, nil
}

// GlobalInstructionPaths enumera los archivos de instrucciones de usuario donde
// versiones anteriores escribieron bloques de gomemory.
func GlobalInstructionPaths(home string) []string {
	paths := make([]string, 0, len(globalTargets))
	for _, t := range globalTargets {
		paths = append(paths, filepath.Join(append(append([]string{home}, t.dir...), t.instructions)...))
	}
	return paths
}

func writeGlobalWrapper(path, frontmatter, method string) (string, error) {
	content := []byte(frontmatter + strings.TrimSpace(method) + "\n")
	if previo, err := os.ReadFile(path); err == nil && string(previo) == string(content) {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := WriteConfigFile(GlobalConfigPath(path), content); err != nil {
		return "", err
	}
	return path, nil
}
