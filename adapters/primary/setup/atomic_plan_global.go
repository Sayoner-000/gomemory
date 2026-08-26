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
		// AGENTS.md es el archivo que Codex lee siempre, el mismo estándar que
		// usa OpenCode. Sin envoltorio nativo: Codex no expone un formato de
		// comandos o habilidades propio que gomemory pueda escribir, y el
		// método de descomposición le llega igual por el bloque de protocolo.
		instructions: "AGENTS.md",
	},
}

// InstallAtomicPlanGlobal escribe, en el ámbito de usuario, el bloque de
// protocolo (que lleva el disparador de modo plan) y el envoltorio nativo del
// método, para los agentes que tienen configuración de nivel usuario.
//
// Con esto, habilitar la funcionalidad una sola vez cubre todos los proyectos
// presentes y futuros (feature 013, FR-024). Un proyecto puede seguir instalando
// su propia versión, que prevalece: los agentes mezclan su configuración de
// usuario con la del proyecto, y el bloque es el MISMO bloque versionado, así
// que el del proyecto sustituye conceptualmente al global en vez de sumarse.
//
// Solo toca el bloque marcado de gomemory: composeAgentFile preserva íntegro
// cualquier otro contenido que la persona tenga en su archivo de instrucciones.
//
// compose recibe el contenido actual del archivo de instrucciones y devuelve el
// contenido con el bloque de protocolo al día, más si hubo cambios. Se inyecta
// desde el paquete cli, que es dueño de los marcadores de versión: cli ya
// importa setup, así que la dependencia no puede ir en el otro sentido.
//
// Devuelve la lista de rutas efectivamente escritas, para poder informarlas.
func InstallAtomicPlanGlobal(method string, compose func(existing string) (string, bool)) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var written []string
	for _, t := range globalTargets {
		base := filepath.Join(append([]string{home}, t.dir...)...)
		// Solo se escribe si el agente ya tiene su directorio de configuración:
		// crear ~/.claude o ~/.config/opencode para un agente que la persona no
		// usa sería ensuciar su HOME sin motivo.
		if info, err := os.Stat(base); err != nil || !info.IsDir() {
			continue
		}

		p, err := writeGlobalInstructions(filepath.Join(base, t.instructions), compose)
		if err != nil {
			return written, err
		}
		if p != "" {
			written = append(written, p)
		}

		if strings.TrimSpace(method) == "" || len(t.wrapper) == 0 {
			continue
		}
		dest := filepath.Join(append([]string{base}, t.wrapper...)...)
		p, err = writeGlobalWrapper(dest, t.frontmatter, method)
		if err != nil {
			return written, err
		}
		if p != "" {
			written = append(written, p)
		}
	}
	return written, nil
}

// writeGlobalInstructions inserta o actualiza el bloque de protocolo en el
// archivo de instrucciones de usuario del agente, sin tocar nada más.
// Devuelve la ruta si hubo cambios, o "" si ya estaba al día (idempotencia).
func writeGlobalInstructions(path string, compose func(existing string) (string, bool)) (string, error) {
	previo, _ := os.ReadFile(path)

	out, changed := compose(string(previo))
	if !changed {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func writeGlobalWrapper(path, frontmatter, method string) (string, error) {
	content := []byte(frontmatter + strings.TrimSpace(method) + "\n")
	if previo, err := os.ReadFile(path); err == nil && string(previo) == string(content) {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
