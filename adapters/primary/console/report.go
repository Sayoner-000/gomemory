package console

import (
	"os"
)

// StepStatus es el resultado de un paso del ciclo de vida.
type StepStatus int

const (
	StepOK StepStatus = iota
	// StepWarn: el paso no se completó, pero los siguientes siguen (FR-015).
	StepWarn
	// StepFail: el paso aborta la operación (por ejemplo, un checksum).
	StepFail
	// StepSkip: el paso no aplica en este entorno (por ejemplo, un proveedor
	// opcional no instalado); no cuenta como aviso.
	StepSkip
)

// State traduce el resultado al estado visual compartido con StepLines.
func (s StepStatus) State() StepState {
	switch s {
	case StepWarn:
		return StateWarn
	case StepFail:
		return StateFail
	case StepSkip:
		return StateSkip
	}
	return StateDone
}

// StepResult es una línea del resumen. Manual es el comando para completar a
// mano un paso con ⚠.
// Meta es un dato breve del resultado (conteos, duración) que se muestra
// atenuado y, en los paneles, alineado a la derecha.
type StepResult struct {
	Name, Detail, Manual, Meta string
	Status                     StepStatus
}

func renderStep(r StepResult, styled bool) string {
	icon := stateGlyph(r.Status.State())
	color := Theme(os.Getenv).StateColor(r.Status.State())
	if styled {
		icon = foreground(color) + icon + "\x1b[0m"
	}
	line := icon + " " + r.Name
	if r.Detail != "" {
		line += ": " + r.Detail
	}
	if r.Meta != "" {
		line += " · " + r.Meta
	}
	if r.Manual != "" {
		line += " → " + r.Manual
	}
	return line
}
