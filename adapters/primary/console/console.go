// Package console es la consola guiada del ciclo de vida de gomemory
// (feature 034): selección múltiple, selección única, confirmación,
// confirmación escrita y el informe de pasos ✓/⚠/✗, al estilo de skills.sh.
//
// Hay dos implementaciones de UI con la misma información: una rica sobre
// bubbletea/lipgloss y una en texto plano para terminales sin color, tontas o
// estrechas (FR-027). Sin TTY no se instancia ninguna: quien llama usa los
// valores recomendados o guardados, igual que con --yes (FR-022).
package console

import (
	"errors"
	"os"

	"github.com/charmbracelet/x/term"
)

// ErrCanceled indica que la persona canceló (Esc, Ctrl+C o fin de entrada).
// Cancelar nunca es un error de la operación: quien llama sale sin cambios.
var ErrCanceled = errors.New("cancelado por la persona")

// Mode es cómo puede interactuar la consola con la persona.
type Mode int

const (
	// ModeNonInteractive: nunca se lee la entrada (sin TTY, --yes, CI, hooks).
	ModeNonInteractive Mode = iota
	// ModePlain: se pregunta en texto plano, sin estilos.
	ModePlain
	// ModeRich: se pregunta con la interfaz de terminal completa.
	ModeRich
)

// minRichWidth es el ancho mínimo para la interfaz rica (FR-027).
const minRichWidth = 60

// Env describe la terminal. Es un valor para poder probar DetectMode sin TTY.
type Env struct {
	StdinTTY, StdoutTTY bool
	// Width es el ancho en columnas; 0 si no se pudo medir.
	Width  int
	Getenv func(string) string
}

// DetectEnv mide la terminal del proceso actual.
func DetectEnv() Env {
	width := 0
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil {
		width = w
	}
	return Env{
		StdinTTY:  term.IsTerminal(os.Stdin.Fd()),
		StdoutTTY: term.IsTerminal(os.Stdout.Fd()),
		Width:     width,
		Getenv:    os.Getenv,
	}
}

// DetectMode decide el modo. yes corresponde a --yes/-y.
func DetectMode(e Env, yes bool) Mode {
	if yes || !e.StdinTTY || !e.StdoutTTY {
		return ModeNonInteractive
	}
	getenv := e.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	if getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" || e.Width < minRichWidth {
		return ModePlain
	}
	return ModeRich
}

// Option es una opción de una selección.
type Option struct {
	Value, Label, Hint string
	// Checked marca la opción de salida en una selección múltiple.
	Checked bool
	// Recommended es la opción por defecto de una selección única.
	Recommended bool
}

// UI es lo que los comandos del ciclo de vida preguntan a la persona.
type UI interface {
	MultiSelect(title string, opts []Option) ([]string, error)
	Select(title string, opts []Option) (string, error)
	Confirm(title string, def bool) (bool, error)
	// TypedConfirm exige escribir word literalmente (FR-016).
	TypedConfirm(title, word string) (bool, error)
}

// New devuelve la UI del modo dado sobre la terminal del proceso, o nil en
// ModeNonInteractive.
func New(mode Mode) UI {
	switch mode {
	case ModeRich:
		return NewRich(os.Stdin, os.Stdout)
	case ModePlain:
		return NewPlain(os.Stdin, os.Stdout)
	default:
		return nil
	}
}

func recommendedIndex(opts []Option) int {
	for i, o := range opts {
		if o.Recommended {
			return i
		}
	}
	return 0
}
