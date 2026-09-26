package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installAgent es un agente que `mem install` sabe configurar (feature 034,
// US3). Incluye los de domain.KnownAgents (ciclo completo) y los que solo
// registran el servidor MCP; TestInstallAgents_CubrenElRegistro impide que la
// tabla se quede atrás del registro.
type installAgent struct {
	name, label string
	// bins y dirs son las señales de detección: el ejecutable en el PATH o el
	// directorio de configuración de usuario (relativo al HOME).
	bins []string
	dirs [][]string
	// optional: se ofrece pero nunca se marca solo (feature 021 los sacó de la
	// instalación automática).
	optional bool
}

var installAgents = []installAgent{
	{name: "claude", label: "Claude Code", bins: []string{"claude"}, dirs: [][]string{{".claude"}}},
	{name: "codex", label: "Codex", bins: []string{"codex"}, dirs: [][]string{{".codex"}}},
	{name: "opencode", label: "OpenCode", bins: []string{"opencode"}, dirs: [][]string{{".config", "opencode"}, {".opencode"}}},
	{name: "cursor", label: "Cursor", bins: []string{"cursor"}, dirs: [][]string{{".cursor"}}},
	{name: "windsurf", label: "Windsurf", bins: []string{"windsurf"}, dirs: [][]string{{".codeium", "windsurf"}}, optional: true},
	{name: "cline", label: "Cline", optional: true},
}

// legacyDefaultAgents son los agentes que install configuraba siempre antes de
// la feature 034. Se usan cuando no hay selección guardada ni se detecta
// ninguno: un entorno sin señales (CI, un HOME vacío) se comporta como antes.
var legacyDefaultAgents = []string{"claude", "opencode", "cursor", "codex"}

type agentChoice struct {
	installAgent
	detected bool
}

// checked indica si la opción sale marcada en la consola.
func (c agentChoice) checked() bool { return c.detected && !c.optional }

func detectAgents(home string, lookPath func(string) (string, error)) []agentChoice {
	out := make([]agentChoice, 0, len(installAgents))
	for _, a := range installAgents {
		c := agentChoice{installAgent: a}
		for _, b := range a.bins {
			if _, err := lookPath(b); err == nil {
				c.detected = true
			}
		}
		for _, d := range a.dirs {
			if info, err := os.Stat(filepath.Join(append([]string{home}, d...)...)); err == nil && info.IsDir() {
				c.detected = true
			}
		}
		out = append(out, c)
	}
	return out
}

// defaultAgents son los que se configuran sin selección explícita ni guardada.
func defaultAgents(choices []agentChoice) []string {
	var out []string
	for _, c := range choices {
		if c.checked() {
			out = append(out, c.name)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), legacyDefaultAgents...)
	}
	return out
}

// parseAgentList valida --agents contra la tabla (FR-022): un nombre
// desconocido es un error de uso antes de escribir nada.
func parseAgentList(s string) ([]string, error) {
	known := map[string]bool{}
	for _, a := range installAgents {
		known[a.name] = true
	}
	var out []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !known[f] {
			return nil, fmt.Errorf("agente desconocido %q (conocidos: %s)", f, strings.Join(agentNames(), ", "))
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--agents está vacío")
	}
	return out, nil
}

func agentNames() []string {
	out := make([]string, 0, len(installAgents))
	for _, a := range installAgents {
		out = append(out, a.name)
	}
	return out
}

func agentLabel(name string) string {
	for _, a := range installAgents {
		if a.name == name {
			return a.label
		}
	}
	return name
}
