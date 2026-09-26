package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BinRef describe cómo referenciar el binario `mem` en archivos de config de
// agentes, de forma portable entre máquinas, sistemas operativos y agentes
// (Claude, OpenCode, Cursor, Windsurf, Cline, Codex).
//
// Nunca se escribe una ruta absoluta de la máquina de instalación: esa era la
// causa de que la config se rompiera al viajar entre Linux, macOS y Windows.
//
// La referencia universal y agnóstica de agente es el binario por nombre en el
// PATH ("mem"), que garantiza el instalador de consola (install.sh / install.ps1).
// El campo HookCommand solo difiere en el fallback por-proyecto de Claude Code,
// que sabe expandir ${CLAUDE_PROJECT_DIR} en runtime; el resto de agentes
// dependen del PATH.
type BinRef struct {
	// MCPCommand es el valor del campo "command" para configs MCP de cualquier
	// agente. Siempre "mem" (PATH): agnóstico de agente y de SO.
	MCPCommand string
	// MCPArgs son los argumentos del server MCP. Sin --root absoluto: el server
	// resuelve el proyecto desde el cwd con FindRoot(), lo que mantiene la
	// config portable entre checkouts.
	MCPArgs []string
	// HookCommand es el prefijo para comandos de hooks de Claude Code (se le
	// concatena " hook <evento>").
	HookCommand string
	// Global indica si `mem` está en el PATH (instalación universal).
	Global bool
}

// binRefFor decide la referencia portable al binario para un proyecto dado.
//
//  1. Si `mem` está en el PATH → todo se referencia por nombre ("mem"). Es el
//     camino de la instalación universal y funciona para todos los agentes.
//  2. Si no, como fallback de desarrollo con binario por-proyecto, los hooks de
//     Claude usan "${CLAUDE_PROJECT_DIR}/mem" (Claude lo expande en runtime).
//     Para MCP se mantiene "mem" porque otros agentes no expanden esa variable;
//     lo correcto es instalar `mem` en el PATH.
func binRefFor(target string) BinRef {
	binName := memBinaryName()

	base := BinRef{
		MCPCommand: "mem",
		MCPArgs:    []string{"mcp"},
		Global:     true,
	}

	if _, err := exec.LookPath(binName); err == nil {
		base.HookCommand = "mem"
		return base
	}

	if _, err := os.Stat(filepath.Join(target, binName)); err == nil {
		base.Global = false
		base.HookCommand = "${CLAUDE_PROJECT_DIR}/" + binName
		return base
	}

	base.HookCommand = "mem"
	return base
}

func memBinaryName() string {
	if runtime.GOOS == "windows" {
		return "mem.exe"
	}
	return "mem"
}

// resolveGlobalBinary es la única regla para decidir si hay un binario global
// (R1 de la feature 034): `mem` se resuelve por el PATH y no es el mismo
// archivo que la copia del proyecto. Install, update, la retirada de copias y
// doctor la comparten para no discrepar sobre qué binario manda.
func resolveGlobalBinary(projectRoot string) (string, bool) {
	path, err := exec.LookPath(memBinaryName())
	if err != nil {
		return "", false
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	global, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	if local, err := os.Stat(filepath.Join(projectRoot, memBinaryName())); err == nil && os.SameFile(global, local) {
		return "", false
	}
	return path, true
}
