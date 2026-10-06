package console

import (
	"fmt"
	"io"

	"github.com/charmbracelet/x/ansi"
)

// Flow muestra resultados reales en línea y conserva el historial de pasos.
type Flow struct {
	out                io.Writer
	env                Env
	layout             Layout
	stop               func()
	lastProgress       string
	warnings, failures int
}

func NewFlow(out io.Writer, e Env, title string) *Flow {
	f := &Flow{out: out, env: e, layout: NewLayout(e)}
	f.line("┌ " + f.layout.Header(title))
	f.line("│")
	return f
}

func (f *Flow) line(text string) {
	_, _ = fmt.Fprintln(f.out, ansi.Hardwrap(ansi.Wrap(text, f.layout.Width(), ""), f.layout.Width(), false))
}

func (f *Flow) Stop() {
	if f.stop != nil {
		f.stop()
		f.stop = nil
	}
}

func (f *Flow) Progress(label string) {
	f.Stop()
	if animatedTerminal(f.env) {
		f.stop = startProgress(f.out, f.env, label)
		return
	}
	if f.lastProgress != label {
		f.line("│  " + f.layout.Muted(label))
	}
	f.lastProgress = label
}

func (f *Flow) Done(step StepResult) {
	f.Stop()
	if step.Status == StepWarn {
		f.warnings++
	}
	if step.Status == StepFail {
		f.failures++
	}
	styled := styledTerminal(f.env)
	f.line("│  " + renderStep(step, styled))
}

func (f *Flow) End(status string) {
	f.Stop()
	f.line("│")
	switch {
	case status == "fail" || f.failures > 0:
		f.line("└ " + f.layout.color("Instalación interrumpida", f.layout.palette.ErrorColor()))
	case status == "warn" || f.warnings > 0:
		f.line("└ " + f.layout.color("Completado con avisos · revisa los pasos ⚠", f.layout.palette.WarningColor()))
	default:
		f.line("└ " + f.layout.Section("goMemory listo · ejecuta mem tui"))
	}
}
