package console

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"mem/version"

	"github.com/charmbracelet/x/ansi"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Flow muestra el avance real de un comando. En una terminal animada dibuja
// un panel que se redibuja en sitio (checklist, spinner y barra de estado);
// en cualquier otra salida escribe una línea por evento, sin secuencias de
// control, para que logs y pipes conserven el historial.
type Flow struct {
	mu           sync.Mutex
	out          io.Writer
	env          Env
	layout       Layout
	title, next  string
	rows         []StepRow
	results      []StepResult
	planned      bool
	started      time.Time
	live         bool
	drawn, frame int
	ticker       chan struct{}
	tickerDone   chan struct{}
	lastProgress string
}

func NewFlow(out io.Writer, e Env, title string) *Flow {
	f := &Flow{out: out, env: e, layout: NewLayout(e), title: title, started: time.Now(), live: animatedTerminal(e)}
	if f.live {
		f.ticker, f.tickerDone = make(chan struct{}), make(chan struct{})
		go f.animate()
		return f
	}
	f.line("┌ " + f.layout.Header(title))
	f.line("│")
	return f
}

// line escribe texto ajustado al ancho; las continuaciones conservan la guía
// │ del flujo para que un paso largo no se confunda con el siguiente.
func (f *Flow) line(text string) {
	width := f.layout.Width()
	wrap := func(t string, w int) []string {
		return strings.Split(ansi.Hardwrap(ansi.Wrap(t, w, ""), w, false), "\n")
	}
	lines := wrap(text, width)
	indent := ""
	switch {
	case strings.HasPrefix(text, "│"):
		indent = "│    "
	case strings.HasPrefix(text, "  "):
		indent = "    "
	}
	if indent != "" && len(lines) > 1 {
		rest := strings.TrimSpace(strings.Join(lines[1:], " "))
		lines = lines[:1]
		for _, cont := range wrap(rest, width-ansi.StringWidth(indent)) {
			lines = append(lines, indent+cont)
		}
	}
	_, _ = fmt.Fprintln(f.out, strings.Join(lines, "\n"))
}

// Plan declara de antemano los pasos para mostrar pendientes y avance n de N.
func (f *Flow) Plan(labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.planned = true
	for _, label := range labels {
		f.rows = append(f.rows, StepRow{Label: label, State: StatePending})
	}
}

// Next fija la acción sugerida al terminar.
func (f *Flow) Next(hint string) {
	f.mu.Lock()
	f.next = hint
	f.mu.Unlock()
}

func (f *Flow) animate() {
	defer close(f.tickerDone)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		f.mu.Lock()
		f.draw(spinnerFrames[f.frame%len(spinnerFrames)])
		f.frame++
		f.mu.Unlock()
		select {
		case <-f.ticker:
			return
		case <-t.C:
		}
	}
}

// Stop detiene la animación sin dibujar el cierre.
func (f *Flow) Stop() {
	if f.ticker != nil {
		close(f.ticker)
		<-f.tickerDone
		f.ticker = nil
	}
}

// running devuelve la fila en curso, o -1.
func (f *Flow) running() int {
	for i, row := range f.rows {
		if row.State == StateRunning {
			return i
		}
	}
	return -1
}

func (f *Flow) find(label string) int {
	for i, row := range f.rows {
		if row.Label == label && (row.State == StatePending || row.State == StateRunning) {
			return i
		}
	}
	return -1
}

// Progress marca un paso planificado como en curso; cualquier otro texto se
// muestra como detalle del paso actual.
func (f *Flow) Progress(label string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.live {
		// Un paso planificado ya queda visible al terminar; repetirlo aquí
		// solo duplica líneas en logs y pipes.
		if f.lastProgress != label && f.find(label) < 0 {
			f.line("│  " + f.layout.Muted(label))
		}
		f.lastProgress = label
		return
	}
	if i := f.find(label); i >= 0 {
		f.rows[i].State = StateRunning
		return
	}
	if i := f.running(); i >= 0 {
		f.rows[i].Detail = label
		return
	}
	for i := range f.rows {
		if f.rows[i].State == StatePending {
			f.rows[i].State, f.rows[i].Detail = StateRunning, label
			return
		}
	}
	f.rows = append(f.rows, StepRow{Label: label, State: StateRunning})
}

// Log añade una línea a la cola visible del paso en curso (solo en vivo).
func (f *Flow) Log(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i := f.running(); i >= 0 && strings.TrimSpace(text) != "" {
		logs := append(f.rows[i].Logs, strings.TrimSpace(text))
		f.rows[i].Logs = logs[max(0, len(logs)-maxStepLogs):]
	}
}

func (f *Flow) Done(step StepResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, step)
	if !f.live {
		f.line("│  " + renderStep(step, styledTerminal(f.env)))
		return
	}
	detail, _, _ := strings.Cut(step.Detail, "\n")
	row := StepRow{Label: step.Name, State: step.Status.State(), Detail: detail, Meta: step.Meta}
	i := f.find(step.Name)
	if i < 0 && !f.planned {
		i = f.running()
	}
	if i >= 0 {
		f.rows[i] = row
	} else {
		f.rows = append(f.rows, row)
	}
}

func (f *Flow) count(s StepStatus) int {
	n := 0
	for _, r := range f.results {
		if r.Status == s {
			n++
		}
	}
	return n
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func (f *Flow) tally() string {
	parts := []string{strconv.Itoa(f.count(StepOK)) + " ok"}
	if n := f.count(StepWarn); n > 0 {
		parts = append(parts, plural(n, "aviso", "avisos"))
	}
	if n := f.count(StepFail); n > 0 {
		parts = append(parts, plural(n, "error", "errores"))
	}
	if n := f.count(StepSkip); n > 0 {
		parts = append(parts, plural(n, "omitido", "omitidos"))
	}
	return strings.Join(parts, " · ")
}

func (f *Flow) finished() int {
	n := 0
	for _, row := range f.rows {
		if row.State != StatePending && row.State != StateRunning {
			n++
		}
	}
	return n
}

func elapsed(since time.Time) string {
	d := time.Since(since)
	if d < 10*time.Second {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

func (f *Flow) meter() string {
	if !f.planned {
		return ""
	}
	return f.layout.Meter(f.finished(), len(f.rows), 12)
}

// render compone el panel del modo en vivo; mientras corre añade la barra de
// estado, que en el cierre se sustituye por el resumen final.
func (f *Flow) render(frame string) string {
	l := f.layout
	meta := plural(f.finished(), "paso", "pasos")
	if f.planned {
		meta = strconv.Itoa(f.finished()) + " de " + strconv.Itoa(len(f.rows))
	}
	took := ""
	if !f.started.IsZero() {
		took = elapsed(f.started)
	}
	panel := l.Panel(f.title, meta, took, l.StepLines(f.rows, frame, l.panelWidth()-4))
	if frame == "" {
		return panel
	}
	brand := l.bold("◆ goMemory", l.palette.Primary) + " " + l.Muted(version.Version)
	return panel + l.StatusBar(brand, f.meter(), f.tally()) + "\n"
}

func (f *Flow) draw(frame string) {
	block := f.render(frame)
	prefix := "\r"
	if f.drawn > 0 {
		prefix += "\x1b[" + strconv.Itoa(f.drawn) + "A"
	}
	_, _ = fmt.Fprint(f.out, prefix+"\x1b[J"+block)
	f.drawn = strings.Count(block, "\n")
}

// End dibuja el estado final, el detalle completo de avisos y errores (que el
// panel recorta) y la siguiente acción sugerida.
func (f *Flow) End(status string) {
	f.Stop()
	f.mu.Lock()
	defer f.mu.Unlock()
	l := f.layout
	word := l.color("✓ Listo", l.palette.Secondary)
	switch {
	case status == "fail" || f.count(StepFail) > 0:
		word = l.color("✗ Interrumpido", l.palette.ErrorColor())
	case status == "warn" || f.count(StepWarn) > 0:
		word = l.color("⚠ Completado con avisos", l.palette.WarningColor())
	}
	next := ""
	if f.next != "" {
		next = l.Muted("siguiente →") + " " + f.next
	}
	if !f.live {
		f.line("│")
		f.line("└ " + l.StatusBar(word, f.tally()))
		if next != "" {
			f.line("  " + next)
		}
		return
	}
	if !f.planned {
		// Sin plan, la fila en curso solo describía la espera; al cerrar ya
		// no corresponde a ningún paso.
		kept := f.rows[:0]
		for _, row := range f.rows {
			if row.State != StateRunning {
				kept = append(kept, row)
			}
		}
		f.rows = kept
	}
	f.draw("")
	pending := false
	for _, r := range f.results {
		if r.Status == StepWarn || r.Status == StepFail || r.Manual != "" {
			if !pending {
				_, _ = fmt.Fprintln(f.out)
				pending = true
			}
			f.line("  " + renderStep(r, true))
		}
	}
	_, _ = fmt.Fprintln(f.out)
	f.line("  " + l.StatusBar(word, f.meter(), f.tally(), next))
}

// PrintSummary cierra un comando cuyos pasos ya se informaron en línea (por
// ejemplo, porque un subproceso escribe en la misma salida). En terminal
// estilizada dibuja el panel de pasos y el estado final; en cualquier otra
// salida conserva el bloque "Resumen:" del contrato FR-024.
func PrintSummary(out io.Writer, e Env, title string, results []StepResult, next string) {
	if !styledTerminal(e) {
		_, _ = fmt.Fprintln(out, "\nResumen:")
		for _, r := range results {
			_, _ = fmt.Fprintln(out, "  "+renderStep(r, false))
		}
		return
	}
	f := &Flow{out: out, env: e, layout: NewLayout(e), title: title, next: next, live: true, results: results}
	for _, r := range results {
		detail, _, _ := strings.Cut(r.Detail, "\n")
		f.rows = append(f.rows, StepRow{Label: r.Name, State: r.Status.State(), Detail: detail, Meta: r.Meta})
	}
	_, _ = fmt.Fprintln(out)
	f.End("")
}
