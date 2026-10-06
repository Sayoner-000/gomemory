package console

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// StepStatus es el resultado de un paso del ciclo de vida.
type StepStatus int

const (
	StepOK StepStatus = iota
	// StepWarn: el paso no se completó, pero los siguientes siguen (FR-015).
	StepWarn
	// StepFail: el paso aborta la operación (por ejemplo, un checksum).
	StepFail
)

// StepResult es una línea del resumen. Manual es el comando para completar a
// mano un paso con ⚠.
type StepResult struct {
	Name, Detail, Manual string
	Status               StepStatus
}

func renderStep(r StepResult, styled bool) string {
	icon := "✓"
	palette := Theme(os.Getenv)
	color := palette.Secondary
	switch r.Status {
	case StepWarn:
		icon, color = "⚠", palette.WarningColor()
	case StepFail:
		icon, color = "✗", palette.ErrorColor()
	}
	if styled {
		icon = foreground(color) + icon + "\x1b[0m"
	}
	line := icon + " " + r.Name
	if r.Detail != "" {
		line += ": " + r.Detail
	}
	if r.Manual != "" {
		line += " → " + r.Manual
	}
	return line
}

// RenderSummary devuelve una línea por paso (FR-024).
func RenderSummary(results []StepResult, styled bool) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(renderStep(r, styled))
		b.WriteByte('\n')
	}
	return b.String()
}

// Reporter informa de cada paso al terminar y acumula el resumen.
type Reporter struct {
	out     io.Writer
	styled  bool
	results []StepResult
}

func NewReporter(out io.Writer, styled bool) *Reporter {
	return &Reporter{out: out, styled: styled}
}

func (r *Reporter) Done(res StepResult) {
	r.results = append(r.results, res)
	line := "  " + renderStep(res, r.styled)
	if r.styled {
		line = ansi.Hardwrap(ansi.Wrap(line, max(1, DetectEnv().Width), ""), max(1, DetectEnv().Width), false)
	}
	_, _ = fmt.Fprintln(r.out, line)
}

func (r *Reporter) Results() []StepResult { return r.results }

func (r *Reporter) Warnings() int { return r.count(StepWarn) }

func (r *Reporter) Failures() int { return r.count(StepFail) }

func (r *Reporter) count(s StepStatus) int {
	n := 0
	for _, res := range r.results {
		if res.Status == s {
			n++
		}
	}
	return n
}
