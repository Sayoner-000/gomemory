package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mem/adapters/primary/console"
)

// Selección de `mem install` (feature 034, US3): qué agentes, en qué alcance y
// dónde vive el binario. Se decide ANTES de escribir nada.

type installSelection struct {
	agents []string
	scope  string // "project" | "global"
	// binary: "global" | "project"; solo se pregunta si no hay global.
	binary string
}

func (s installSelection) has(agent string) bool {
	for _, a := range s.agents {
		if a == agent {
			return true
		}
	}
	return false
}

type installOptions struct {
	target    string
	yes       bool
	agents    []string
	scope     string
	events    bool
	help      bool
	agentsSet bool
}

// parseInstallArgs acepta los flags en cualquier posición, como uninstall:
// `mem install . --yes` y `mem install --yes .` son lo mismo.
func parseInstallArgs(args []string) (installOptions, error) {
	o := installOptions{target: "."}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func() (string, error) {
			if k, v, ok := strings.Cut(a, "="); ok && strings.HasPrefix(k, "--") {
				return v, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%w: %s necesita un valor", errUsage, a)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--help" || a == "-h":
			o.help = true
		case a == "--events":
			o.events, o.yes = true, true
		case a == "--yes" || a == "-y":
			o.yes = true
		case a == "--agents" || strings.HasPrefix(a, "--agents="):
			v, err := value()
			if err != nil {
				return o, err
			}
			o.agentsSet = true
			if v == "none" {
				o.agents = []string{}
				continue
			}
			if o.agents, err = parseAgentList(v); err != nil {
				return o, fmt.Errorf("%w: %v", errUsage, err)
			}
		case a == "--scope" || strings.HasPrefix(a, "--scope="):
			v, err := value()
			if err != nil {
				return o, err
			}
			if v != "project" && v != "global" {
				return o, fmt.Errorf("%w: --scope admite project o global", errUsage)
			}
			o.scope = v
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("%w: flag desconocido %s", errUsage, a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) > 0 {
		o.target = positional[0]
	}
	return o, nil
}

// resolveInstallSelection es la selección sin preguntar (FR-022, FR-023): los
// flags mandan; si no, la guardada; si no, los detectados.
func resolveInstallSelection(o installOptions, saved installSelection, choices []agentChoice) installSelection {
	sel := installSelection{agents: o.agents, scope: o.scope, binary: "global"}
	if len(sel.agents) == 0 && !o.agentsSet {
		sel.agents = saved.agents
	}
	// AgentScope se escribe junto con Agents. Un scope guardado distingue la
	// elección explícita de cero agentes de una instalación aún sin selección.
	if len(sel.agents) == 0 && saved.scope == "" && !o.agentsSet {
		sel.agents = defaultAgents(choices)
	}
	if sel.scope == "" {
		sel.scope = saved.scope
	}
	if sel.scope == "" {
		sel.scope = "project"
	}
	return sel
}

// askInstallSelection es la consola guiada (FR-020): agentes con los
// detectados (o los guardados) ya marcados → binario, solo sin global →
// alcance → resumen con confirmación.
func askInstallSelection(ui console.UI, choices []agentChoice, hasGlobal bool, saved installSelection) (installSelection, error) {
	marked := map[string]bool{}
	if saved.scope != "" {
		for _, a := range saved.agents {
			marked[a] = true
		}
	} else {
		for _, a := range defaultAgents(choices) {
			marked[a] = true
		}
	}
	opts := make([]console.Option, 0, len(choices))
	for _, c := range choices {
		hint := ""
		if c.detected {
			hint = "detectado"
		}
		opts = append(opts, console.Option{Value: c.name, Label: c.label, Hint: hint, Checked: marked[c.name]})
	}
	agents, err := ui.MultiSelect("¿Qué agentes quieres conectar?", opts)
	if err != nil {
		return installSelection{}, err
	}
	sel := installSelection{agents: agents, binary: "global"}

	if !hasGlobal {
		if sel.binary, err = ui.Select("¿Dónde instalar el binario?", []console.Option{
			{Value: "global", Label: "Global en " + displayPath(globalBinDir()), Hint: "una sola fuente de verdad; se actualiza con mem update", Recommended: true},
			{Value: "project", Label: "Copia en este proyecto", Hint: "solo si no puedes tocar el PATH"},
		}); err != nil {
			return installSelection{}, err
		}
	}

	scopeDefault := saved.scope
	if scopeDefault == "" {
		scopeDefault = "project"
	}
	if sel.scope, err = ui.Select("¿Alcance de la configuración?", []console.Option{
		{Value: "project", Label: "Este proyecto", Recommended: scopeDefault == "project"},
		{Value: "global", Label: "Global para mi usuario", Hint: "todos tus proyectos", Recommended: scopeDefault == "global"},
	}); err != nil {
		return installSelection{}, err
	}

	labels := make([]string, 0, len(sel.agents))
	for _, a := range sel.agents {
		labels = append(labels, agentLabel(a))
	}
	if len(labels) == 0 {
		labels = []string{"ningún agente"}
	}
	summary := fmt.Sprintf("Se configurará: %s · alcance %s · binario %s. ¿Continuar?",
		strings.Join(labels, ", "), sel.scope, sel.binary)
	ok, err := ui.Confirm(summary, true)
	if err != nil {
		return installSelection{}, err
	}
	if !ok {
		return installSelection{}, console.ErrCanceled
	}
	return sel, nil
}

// globalBinDir replica la elección del instalador de consola: GOMEMORY_BIN_DIR,
// /usr/local/bin si admite escritura, o ~/.local/bin.
func globalBinDir() string {
	if v := os.Getenv("GOMEMORY_BIN_DIR"); v != "" {
		return v
	}
	if checkReplaceable(filepath.Join("/usr/local/bin", memBinaryName())) == nil {
		return "/usr/local/bin"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

func displayPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// installGlobalBinary copia el binario en ejecución al directorio global y
// avisa si ese directorio no está en el PATH.
func installGlobalBinary(self string) error {
	dir := globalBinDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, memBinaryName())
	if sameFilePath(self, dest) {
		return nil
	}
	if err := copyFileMode(self, dest, 0o755); err != nil {
		return err
	}
	humanf("  ✅ Binario global instalado en %s\n", dest)
	if found, err := exec.LookPath(memBinaryName()); err != nil || !sameFilePath(found, dest) {
		humanf("  ⚠️  %s no está en el PATH: añádelo para usar la instalación global\n", dir)
	}
	return nil
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".mem-install-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// selectedWrapperAgents son los agentes elegidos que tienen envoltorios
// nativos por proyecto (habilidades de Claude Code, comandos de OpenCode).
func selectedWrapperAgents(sel installSelection) []string {
	var out []string
	for _, a := range []string{"claude", "opencode"} {
		if sel.has(a) {
			out = append(out, a)
		}
	}
	return out
}
