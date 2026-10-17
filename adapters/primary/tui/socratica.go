package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// SocraticaDeps son las dependencias de las pantallas Socratica (feature 039,
// US5). Todo cambio de estado pasa por Flow: la TUI nunca reduce eventos.
type SocraticaDeps struct {
	Flow     *usecases.SocraticaFlow
	Launcher ports.AgentLauncher
}

// socraticaState agrupa el estado de las tres pantallas. Update solo guarda
// lo que devuelven los tea.Cmd del caso de uso.
type socraticaState struct {
	deps    SocraticaDeps
	pending []usecases.SocraticaResult
	history []domain.RefinementHistory
	cursor  int
	current usecases.SocraticaResult
	// entry es la fila del historial abierta en el detalle, si la hay: el
	// flujo puede haberse purgado y el historial sigue sirviéndola (I6).
	entry *domain.RefinementHistory
	busy  string
	err   string
	input textinput.Model
	// inputMode: "" sin caja, "prompt", "edit" o "custom".
	inputMode       string
	confirmResend   bool
	scroll          int
	choosingHarness bool
	harnessCursor   int
	harnesses       []ports.SocraticHarness
	harness         string
	switchFlow      *domain.RefinementFlow
}

func newSocraticaState(deps SocraticaDeps) socraticaState {
	in := textinput.New()
	in.CharLimit = 4000
	in.SetWidth(60)
	return socraticaState{deps: deps, input: in}
}

type socraticaLoadedMsg struct {
	pending []usecases.SocraticaResult
	history []domain.RefinementHistory
	err     error
}

type socraticaHarnessesMsg struct{ choices []ports.SocraticHarness }

type socraticaFlowMsg struct {
	id     string
	result usecases.SocraticaResult
	err    error
	// fromGet marca una relectura: tras un error se relee el flujo una vez.
	fromGet bool
}

// socTrunc acota una fila con elipsis; nunca la deja ajustarse.
func socTrunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(s, n, "…")
}

type socraticaExecDoneMsg struct {
	id     string
	result usecases.SocraticaResult
	err    error
}

type socraticaAssistDoneMsg struct{ err error }

// ─── Comandos: único camino hacia el caso de uso ────────────────────

func (m model) socLoad() tea.Cmd {
	flow, project := m.soc.deps.Flow, m.project
	return func() tea.Msg {
		ctx := context.Background()
		pending, err := flow.Pending(ctx, project)
		if err != nil {
			return socraticaLoadedMsg{err: err}
		}
		history, err := flow.History(ctx, project, "")
		return socraticaLoadedMsg{pending: pending, history: history, err: err}
	}
}

func (m model) socStart(client, prompt string) tea.Cmd {
	flow, project := m.soc.deps.Flow, m.project
	return func() tea.Msg {
		ctx := context.Background()
		r, err := flow.Start(ctx, project, client, domain.ModeRefine, prompt)
		if err != nil {
			return socraticaFlowMsg{err: err}
		}
		if r.Flow.Phase == domain.PhaseCaptured || r.Flow.Phase == domain.PhaseRewriting {
			resumed, err := flow.Resume(ctx, r.Flow.ID, usecases.OriginLocal)
			return socraticaFlowMsg{id: r.Flow.ID, result: resumed, err: err}
		}
		return socraticaFlowMsg{id: r.Flow.ID, result: r}
	}
}

func (m model) socApply(id string, ev domain.Event) tea.Cmd {
	flow := m.soc.deps.Flow
	return func() tea.Msg {
		r, err := flow.Apply(context.Background(), id, ev, usecases.OriginLocal)
		return socraticaFlowMsg{id: id, result: r, err: err}
	}
}

func (m model) socGet(id string) tea.Cmd {
	flow := m.soc.deps.Flow
	return func() tea.Msg {
		r, err := flow.Get(context.Background(), id)
		return socraticaFlowMsg{id: id, result: r, err: err, fromGet: true}
	}
}

// socraticaExecCommand cede la terminal al agente mientras Execute corre: el
// lanzador arranca el proceso, onStarted escribe el ack y Wait lo espera.
type socraticaExecCommand struct {
	flow     *usecases.SocraticaFlow
	launcher ports.AgentLauncher
	id       string
	client   string
	result   usecases.SocraticaResult
	assisted bool
}

func (c *socraticaExecCommand) Run() error {
	if c.assisted {
		return c.flow.Assist(context.Background(), c.id, c.launcher)
	}
	r, err := c.flow.Execute(context.Background(), c.id, c.client, c.launcher)
	c.result = r
	return err
}
func (c *socraticaExecCommand) SetStdin(io.Reader)  {}
func (c *socraticaExecCommand) SetStdout(io.Writer) {}
func (c *socraticaExecCommand) SetStderr(io.Writer) {}

func (m model) socExecCommand(f domain.RefinementFlow) *socraticaExecCommand {
	return &socraticaExecCommand{flow: m.soc.deps.Flow, launcher: m.soc.deps.Launcher, id: f.ID, client: f.Client}
}

func (m model) socExecute(f domain.RefinementFlow) tea.Cmd {
	c := m.socExecCommand(f)
	return tea.Exec(c, func(err error) tea.Msg { return socraticaExecDoneMsg{id: c.id, result: c.result, err: err} })
}

func (m model) socAssist(f domain.RefinementFlow) tea.Cmd {
	c := m.socExecCommand(f)
	c.assisted = true
	return tea.Exec(c, func(err error) tea.Msg { return socraticaAssistDoneMsg{err: err} })
}

// ─── Update ─────────────────────────────────────────────────────────

func (m model) openSocratica() (tea.Model, tea.Cmd) {
	if m.soc.deps.Flow == nil {
		m.statusMsg = "Socratica no está disponible en este proyecto"
		m.statusTimer = 40
		return m, nil
	}
	m.screen = screenSocratica
	m.soc.err, m.soc.busy, m.soc.inputMode = "", "", ""
	return m, m.socLoad()
}

func (m model) generatorName() string {
	if m.soc.harness != "" {
		return m.soc.harness
	}
	return m.soc.current.Flow.Client
}

func (m model) handleSocraticaMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case socraticaAssistDoneMsg:
		m.soc.busy, m.soc.err = "", ""
		m.screen = screenSocratica
		m.statusMsg = "Sesión asistida terminada; revisa los pendientes"
		if msg.err != nil {
			m.soc.err = m.soc.deps.Flow.ExplainFailure(msg.err.Error()).Message
		}
		m.statusTimer = 40
		return m, m.socLoad(), true
	case socraticaHarnessesMsg:
		if m.soc.choosingHarness {
			m.soc.harnesses = msg.choices
			if m.soc.switchFlow != nil {
				for i := range m.soc.harnesses {
					if m.soc.harnesses[i].ID == m.soc.switchFlow.Client {
						m.soc.harnesses[i].Available = false
						m.soc.harnesses[i].Reason = "falló en este flujo; elige otro"
					}
				}
			}
			m.soc.busy = ""
		}
		return m, nil, true
	case socraticaLoadedMsg:
		m.soc.pending, m.soc.history = msg.pending, msg.history
		if msg.err != nil {
			m.soc.err = m.soc.deps.Flow.ExplainFailure(msg.err.Error()).Message
		}
		m.soc.cursor = min(m.soc.cursor, max(0, len(m.soc.pending)+len(m.soc.history)-1))
		return m, nil, true
	case socraticaFlowMsg:
		m.soc.busy = ""
		if msg.err != nil {
			// Un historial purgado no es un error para el detalle (I6).
			if msg.fromGet && m.soc.entry != nil {
				return m, nil, true
			}
			m.soc.err = msg.err.Error()
			// Tras un error, el resultado puede ser el flujo previo al fallo:
			// se relee para mostrar el estado real (p. ej. failed con el
			// original intacto y sus tres salidas, FR-016).
			if msg.id != "" && !msg.fromGet {
				return m, m.socGet(msg.id), true
			}
			return m, nil, true
		}
		m.soc.current = msg.result
		m.soc.harness = msg.result.Flow.Client
		if msg.result.Flow.Phase == domain.PhaseFailed {
			m.soc.err = ""
		}
		m.soc.confirmResend = false
		switch msg.result.Flow.Phase {
		case domain.PhaseQuestions:
			m.screen = screenSocraticaQuestions
		case domain.PhaseCancelled:
			m.statusMsg = "Cancelado: se conserva tu original"
			m.statusTimer = 40
			m.screen = screenSocratica
			return m, m.socLoad(), true
		case "":
			m.screen = screenSocratica
			return m, m.socLoad(), true
		default:
			m.screen = screenSocraticaDetail
		}
		return m, nil, true
	case socraticaExecDoneMsg:
		m.soc.busy = ""
		m.soc.err = ""
		if msg.err != nil {
			m.soc.err = socraticaExecError(msg.err)
		}
		m.screen = screenSocraticaDetail
		return m, m.socGet(msg.id), true
	}
	return m, nil, false
}

func socraticaExecError(err error) string {
	switch {
	case errors.Is(err, usecases.ErrDispatchUnconfirmed):
		return "el agente arrancó pero no se pudo confirmar el envío; no reintentes"
	case errors.Is(err, usecases.ErrDispatchUncertain):
		return "puede que ya se haya enviado: decide marcar, reenviar o cancelar"
	case errors.Is(err, ports.ErrLaunchFailed):
		return "no se pudo arrancar el agente; el flujo queda aprobado sin confirmar"
	}
	return err.Error()
}

func (m model) socRows() int { return len(m.soc.pending) + len(m.soc.history) }

func (m model) updateSocratica(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.soc.choosingHarness {
		if key != "esc" && len(m.soc.harnesses) == 0 {
			return m, nil
		}
		switch key {
		case "esc":
			m.soc.choosingHarness = false
			m.soc.switchFlow = nil
			m.soc.err = ""
			m.soc.busy = ""
		case "up", "k":
			m.soc.harnessCursor = max(0, m.soc.harnessCursor-1)
		case "down", "j":
			m.soc.harnessCursor = min(len(m.soc.harnesses)-1, m.soc.harnessCursor+1)
		case "enter":
			if len(m.soc.harnesses) == 0 {
				return m, nil
			}
			h := m.soc.harnesses[m.soc.harnessCursor]
			if !h.Available {
				m.soc.err = h.Label + ": " + h.Reason
				return m, nil
			}
			m.soc.harness = h.ID
			m.soc.choosingHarness = false
			m.soc.err = ""
			if m.soc.switchFlow != nil {
				old := *m.soc.switchFlow
				m.soc.switchFlow = nil
				m.soc.busy = "Generando preguntas con " + h.ID + "…"
				flow := m.soc.deps.Flow
				return m, func() tea.Msg {
					r, err := flow.ChangeHarness(context.Background(), old.ID, h.ID)
					if err != nil {
						return socraticaFlowMsg{id: old.ID, err: err}
					}
					id := r.Flow.ID
					r, err = flow.Resume(context.Background(), id, usecases.OriginLocal)
					return socraticaFlowMsg{id: id, result: r, err: err}
				}
			}
			m.soc.inputMode = "prompt"
			m.soc.input.SetValue("")
			m.soc.input.Placeholder = "tu prompt"
			return m, m.soc.input.Focus()
		}
		return m, nil
	}
	if m.soc.inputMode != "" {
		return m.updateSocraticaInput(msg)
	}
	switch m.screen {
	case screenSocratica:
		switch key {
		case "esc":
			m.screen = screenList
		case "up", "k":
			m.soc.cursor = max(0, m.soc.cursor-1)
		case "down", "j":
			m.soc.cursor = min(max(0, m.socRows()-1), m.soc.cursor+1)
		case "enter":
			return m.openSocraticaRow()
		case "r":
			m.soc.switchFlow = nil
			m.soc.harnesses = nil
			m.soc.harnessCursor = 0
			m.soc.harness = ""
			m.soc.err = ""
			m.soc.choosingHarness = true
			m.soc.busy = "Detectando harnesses…"
			flow := m.soc.deps.Flow
			return m, func() tea.Msg { return socraticaHarnessesMsg{choices: flow.Harnesses()} }
		case "i":
			return m.toggleSocraticaInterception()
		case "p":
			return m.toggleSocraticaPrivate()
		}
	case screenSocraticaQuestions:
		return m.updateSocraticaQuestions(key)
	case screenSocraticaDetail:
		return m.updateSocraticaDetail(key)
	}
	return m, nil
}

func (m model) openSocraticaRow() (tea.Model, tea.Cmd) {
	m.soc.entry = nil
	m.soc.err = ""
	m.soc.harness = ""
	if m.soc.cursor < len(m.soc.pending) {
		r := m.soc.pending[m.soc.cursor]
		m.soc.current = r
		if r.Flow.Phase == domain.PhaseQuestions && !r.Flow.Uncertain && !r.Flow.InFlight {
			m.screen = screenSocraticaQuestions
			return m, nil
		}
		m.screen = screenSocraticaDetail
		return m, nil
	}
	i := m.soc.cursor - len(m.soc.pending)
	if i < 0 || i >= len(m.soc.history) {
		return m, nil
	}
	h := m.soc.history[i]
	m.soc.entry = &h
	m.soc.current = usecases.SocraticaResult{}
	m.screen = screenSocraticaDetail
	// El flujo puede seguir vivo (p. ej. dispatched): se consulta para saber
	// si ofrecer Reenviar; si se purgó, el detalle sale solo del historial.
	return m, m.socGet(h.FlowID)
}

func (m model) updateSocraticaInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.soc.inputMode = ""
		m.soc.input.Blur()
		return m, nil
	case "enter":
		text := m.soc.input.Value()
		mode := m.soc.inputMode
		m.soc.inputMode = ""
		m.soc.input.Blur()
		if strings.TrimSpace(text) == "" {
			return m, nil
		}
		f := m.soc.current.Flow
		switch mode {
		case "prompt":
			gen := m.generatorName()
			// Acuse inmediato: se pinta antes de que corra el generador (FR-017).
			m.soc.busy = "Generando preguntas con " + gen + "…"
			return m, m.socStart(gen, text)
		case "edit":
			return m, m.socApply(f.ID, domain.Event{Kind: "edit", Proposed: text})
		case "custom":
			ev := domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: len(f.Answers), Custom: text}}
			if len(f.Answers) == len(f.Questions)-1 {
				m.soc.busy = "Reescribiendo con " + m.generatorName() + "…"
			}
			return m, m.socApply(f.ID, ev)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.soc.input, cmd = m.soc.input.Update(msg)
	return m, cmd
}

func (m model) updateSocraticaQuestions(key string) (tea.Model, tea.Cmd) {
	f := m.soc.current.Flow
	if key == "esc" {
		m.screen = screenSocratica
		return m, m.socLoad()
	}
	i := len(f.Answers)
	if f.Phase != domain.PhaseQuestions || i >= len(f.Questions) || m.soc.busy != "" {
		return m, nil
	}
	last := i == len(f.Questions)-1
	var ev domain.Event
	switch key {
	case "S":
		ev = domain.Event{Kind: "skip_rest"}
		last = true
	case "s":
		ev = domain.Event{Kind: "skip"}
	case "o":
		m.soc.inputMode = "custom"
		m.soc.input.SetValue("")
		m.soc.input.Placeholder = "tu respuesta"
		return m, m.soc.input.Focus()
	default:
		var n int
		if _, err := fmt.Sscanf(key, "%d", &n); err != nil || n < 1 || n > len(f.Questions[i].Options) {
			return m, nil
		}
		ev = domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: i, Choice: f.Questions[i].Options[n-1]}}
	}
	if last {
		m.soc.busy = "Reescribiendo con " + m.generatorName() + "…"
	}
	return m, m.socApply(f.ID, ev)
}

// socraticaActions son las acciones que admite el estado del flujo, alineadas
// con el next de contracts/mcp-tool.md (I11).
func socraticaActions(f domain.RefinementFlow, hasFlow bool) (label string, keys string) {
	switch {
	case !hasFlow:
		return "", ""
	case f.InFlight:
		return "Envío en curso", ""
	case f.Uncertain:
		return "Puede que ya se haya enviado", "m Marcar enviado · r Reenviar · x Cancelar"
	}
	switch f.Phase {
	case domain.PhaseReview, domain.PhaseApproved:
		return "", "e Ejecutar · d Editar · c Copiar · x Cancelar"
	case domain.PhaseFailed:
		return "No se pudo generar", "r Reintentar · o Enviar mi original · x Cancelar"
	case domain.PhaseDispatched:
		return "Ya enviado", "r Reenviar"
	}
	return "", ""
}

func (m model) updateSocraticaDetail(key string) (tea.Model, tea.Cmd) {
	if key == "esc" {
		m.screen = screenSocratica
		m.soc.confirmResend = false
		return m, m.socLoad()
	}
	f := m.soc.current.Flow
	if f.ID == "" || m.soc.busy != "" || f.InFlight {
		return m, nil
	}
	if f.Uncertain {
		kind := map[string]string{"m": "mark_dispatched", "r": "force_resend", "x": "cancel"}[key]
		if kind == "" {
			return m, nil
		}
		return m, m.socApply(f.ID, domain.Event{Kind: kind, DispatchID: f.DispatchID})
	}
	switch f.Phase {
	case domain.PhaseReview, domain.PhaseApproved:
		switch key {
		case "e":
			m.soc.busy = "Ejecutando en " + f.Client + "…"
			return m, m.socExecute(f)
		case "d":
			m.soc.inputMode = "edit"
			m.soc.input.SetValue(domain.StripTerminalControls(f.ProposedPrompt))
			return m, m.soc.input.Focus()
		case "c":
			m.statusMsg = "Propuesta copiada al portapapeles"
			m.statusTimer = 40
			return m, tea.SetClipboard(domain.StripTerminalControls(domain.ProtectText(f.ProposedPrompt)))
		case "x":
			return m, m.socApply(f.ID, domain.Event{Kind: "cancel"})
		}
	case domain.PhaseFailed:
		switch key {
		case "a":
			m.soc.busy = "Abriendo Socratica asistida en " + f.Client + "…"
			return m, m.socAssist(f)
		case "h":
			m.soc.switchFlow = &f
			m.soc.harnesses = nil
			m.soc.harnessCursor = 0
			m.soc.choosingHarness = true
			m.soc.busy = "Detectando harnesses…"
			flow := m.soc.deps.Flow
			return m, func() tea.Msg { return socraticaHarnessesMsg{choices: flow.Harnesses()} }
		case "r":
			if info := m.soc.deps.Flow.ExplainFailure(f.LastError); !info.Retryable {
				m.statusMsg = "a continúa asistido · h cambia de harness"
				m.statusTimer = 40
				return m, nil
			}
			m.soc.busy = "Generando con " + m.generatorName() + "…"
			return m, m.socApply(f.ID, domain.Event{Kind: "retry"})
		case "o":
			m.soc.busy = "Ejecutando en " + f.Client + "…"
			return m, m.socExecute(f)
		case "x":
			return m, m.socApply(f.ID, domain.Event{Kind: "cancel"})
		}
	case domain.PhaseDispatched:
		// Reenviar es una autorización nueva: exige una segunda pulsación (I10).
		switch {
		case key == "r" && !m.soc.confirmResend:
			m.soc.confirmResend = true
		case key == "y" && m.soc.confirmResend:
			m.soc.confirmResend = false
			return m, m.socApply(f.ID, domain.Event{Kind: "force_resend", DispatchID: f.DispatchID})
		default:
			m.soc.confirmResend = false
		}
	}
	return m, nil
}

func (m model) toggleSocraticaInterception() (tea.Model, tea.Cmd) {
	client := "claude"
	if m.soc.cursor < len(m.soc.pending) {
		client = m.soc.pending[m.soc.cursor].Flow.Client
	} else if i := m.soc.cursor - len(m.soc.pending); i >= 0 && i < len(m.soc.history) {
		client = m.soc.history[i].Client
	}
	s := m.settingsRepo.Read(m.root)
	on := !s.Interception[client]
	// FR-019: sin evidencia de aceptación no se activa desde la TUI.
	if on && !domain.CapabilitiesForSocratica(client).Interception {
		m.statusMsg = "Intercepción no soportada en " + client + " · para probarla: mem socratica on --client " + client + " --unverified"
		m.statusTimer = 40
		return m, nil
	}
	if s.Interception == nil {
		s.Interception = map[string]bool{}
	}
	s.Interception[client] = on
	if !on && s.InterceptionUnverified != nil {
		s.InterceptionUnverified[client] = false
	}
	return m.writeSocraticaSettings(s, fmt.Sprintf("Intercepción en %s: %t", client, on))
}

func (m model) toggleSocraticaPrivate() (tea.Model, tea.Cmd) {
	s := m.settingsRepo.Read(m.root)
	s.Private = !s.Private
	return m.writeSocraticaSettings(s, fmt.Sprintf("Modo privado: %t", s.Private))
}

func (m model) writeSocraticaSettings(s ports.SettingsData, ok string) (tea.Model, tea.Cmd) {
	if _, err := domain.ValidateSocraticaSettings(s.SocraticaSettings); err != nil {
		m.soc.err = err.Error()
		return m, nil
	}
	if err := m.settingsRepo.Write(m.root, s); err != nil {
		m.soc.err = err.Error()
		return m, nil
	}
	// El caso de uso de este proceso usa ya los ajustes nuevos.
	m.soc.deps.Flow.Settings = s.SocraticaSettings
	m.statusMsg = ok
	m.statusTimer = 40
	return m, nil
}

// ─── View (solo lee el modelo) ──────────────────────────────────────

func (m model) socTitle(width int) string {
	label := "SOCRATICA"
	if client := m.generatorName(); client != "" && !m.soc.choosingHarness {
		label += " · " + client
	}
	title := lipgloss.NewStyle().Foreground(socCurrent).Bold(true).Underline(true).Render(socTrunc(label, width))
	rest := max(0, width-lipgloss.Width(title)-1)
	return title + " " + lipgloss.NewStyle().Foreground(socLabel).Render(strings.Repeat("─", rest))
}

var socraticaStages = []string{"Tu prompt", "Preguntas", "Reescritura", "Revisión"}

func socraticaStage(p domain.SocraticaPhase) int {
	switch p {
	case domain.PhaseCaptured:
		return 0
	case domain.PhaseQuestions:
		return 1
	case domain.PhaseRewriting:
		return 2
	}
	return 3
}

func socraticaTracker(p domain.SocraticaPhase, width int) string {
	current := socraticaStage(p)
	parts := make([]string, len(socraticaStages))
	for i, s := range socraticaStages {
		style := lipgloss.NewStyle().Foreground(socUpcoming())
		switch {
		case i < current:
			style = lipgloss.NewStyle().Foreground(socDone)
		case i == current:
			style = lipgloss.NewStyle().Foreground(socCurrent).Bold(true)
		}
		parts[i] = style.Render(s)
	}
	return ansi.Truncate(strings.Join(parts, lipgloss.NewStyle().Foreground(socLabel).Render(" │ ")), width, "…")
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "ahora"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// firstLine is the one-line summary of a stored prompt, safe to paint: the
// flow keeps its exact bytes, only what is shown loses control sequences.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return domain.StripTerminalControls(line)
}

func (m model) socFooter(help string, width int) string {
	var b strings.Builder
	if m.soc.busy != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(socCurrent).Render(socTrunc(m.soc.busy, width)) + "\n")
	}
	if m.soc.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(socWarn).Render(socTrunc(domain.StripTerminalControls(m.soc.err), width)) + "\n")
	}
	if status := m.statusLine(); status != "" {
		b.WriteString(socTrunc(status, width) + "\n")
	}
	if m.soc.inputMode != "" {
		b.WriteString(formLabel.Render("› ") + m.soc.input.View() + "\n")
		help = "enter confirmar · esc cancelar"
	}
	b.WriteString(helpStyle.Render(socTrunc(help, width)))
	return footerBoundary + b.String()
}

func (m model) socraticaListView() string {
	width := max(1, m.width-2)
	var b strings.Builder
	b.WriteString(m.socTitle(width) + "\n")
	if m.soc.choosingHarness {
		b.WriteString(lipgloss.NewStyle().Foreground(socFg).Bold(true).Render(socTrunc("Elige el harness para este flujo", width)) + "\n\n")
		b.WriteString(socTrunc("La misma elección refina y ejecuta tras tu aprobación.", width) + "\n\n")
		for i, h := range m.soc.harnesses {
			cursor, state := "  ", "disponible"
			if i == m.soc.harnessCursor {
				cursor = "▸ "
			}
			if !h.Available {
				state = h.Reason
			}
			b.WriteString(socTrunc(cursor+h.Label+" · "+state, width) + "\n")
		}
		if len(m.soc.harnesses) == 0 && m.soc.busy == "" {
			b.WriteString("No hay harnesses disponibles.\n")
		}
		return b.String() + m.socFooter("↑↓ elegir · enter confirmar · esc cancelar", width)
	}
	settings := m.settingsRepo.Read(m.root).SocraticaSettings
	var states []string
	for _, c := range []string{"claude", "codex", "opencode"} {
		state := "desactivada"
		switch {
		case settings.Interception[c] && domain.CapabilitiesForSocratica(c).Interception:
			state = "activa"
		case settings.Interception[c] && settings.InterceptionUnverified[c]:
			state = "sin verificar"
		}
		states = append(states, c+" "+state)
	}
	private := "no"
	if settings.Private {
		private = "sí"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(socLabel).Render(socTrunc("Intercepción: "+strings.Join(states, " · ")+" · privado: "+private, width)) + "\n\n")

	row := 0
	cursor := func() string {
		defer func() { row++ }()
		if row == m.soc.cursor {
			return "▸ "
		}
		return "  "
	}
	if len(m.soc.pending) > 0 {
		b.WriteString(socraticaTracker(m.soc.pending[min(m.soc.cursor, len(m.soc.pending)-1)].Flow.Phase, width) + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(socLabel).Render("Pendientes") + "\n")
		for _, r := range m.soc.pending {
			state := string(r.Flow.Phase)
			switch {
			case r.Flow.InFlight:
				state = "envío en curso"
			case r.Flow.Uncertain:
				state = "puede que enviado"
			}
			line := fmt.Sprintf("%s · %s · hace %s · %s", r.Flow.Client, state, ago(r.Flow.UpdatedAt), firstLine(r.Flow.OriginalPrompt))
			b.WriteString(socTrunc(cursor()+line, width) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(socLabel).Render("Historial") + "\n")
	if len(m.soc.history) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(socUpcoming()).Render("  Sin historial todavía · r refinar un prompt") + "\n")
	}
	for _, h := range m.soc.history {
		line := fmt.Sprintf("%s · %s · %s · %d→%d palabras · %s", h.CompletedAt.Local().Format("2006-01-02 15:04"), h.Client, h.Mode, h.WordsBefore, h.WordsAfter, firstLine(h.OriginalPrompt))
		b.WriteString(socTrunc(cursor()+line, width) + "\n")
	}
	return b.String() + m.socFooter("↑↓ · enter detalle · r refinar · i intercepción · p privado · esc volver", width)
}

func (m model) socraticaQuestionsView() string {
	width := max(1, m.width-2)
	f := m.soc.current.Flow
	var b strings.Builder
	b.WriteString(m.socTitle(width) + "\n" + socraticaTracker(f.Phase, width) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(socLabel).Render(fmt.Sprintf("%d/%d respondidas", len(f.Answers), len(f.Questions))) + "\n")
	frame := lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(socLabel).Padding(0, 1).Width(max(10, min(width, 100)))
	for i, q := range f.Questions {
		head := lipgloss.NewStyle().Foreground(socFg).Bold(true).Render(fmt.Sprintf("%d. %s", i+1, domain.StripTerminalControls(q.Text)))
		if i < len(f.Answers) {
			head = lipgloss.NewStyle().Foreground(socDone).Render("✓ ") + head
		}
		var chips []string
		for n, o := range q.Options {
			chips = append(chips, lipgloss.NewStyle().Background(socControl).Foreground(socFg).Padding(0, 1).Render(fmt.Sprintf("%d %s", n+1, domain.StripTerminalControls(o))))
		}
		inner := max(1, frame.GetWidth()-frame.GetHorizontalFrameSize())
		body := socTrunc(head, inner) + "\n" + ansi.Truncate(strings.Join(chips, " "), inner, "…")
		b.WriteString(frame.Render(body) + "\n")
	}
	return b.String() + m.socFooter("número elegir · o Otra · s Omitir · S Omitir el resto · esc volver", width)
}

func (m model) socraticaCard(title, text string, accent bool, width int) string {
	border := socLabel
	if accent {
		border = socCurrent
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Width(max(8, width))
	inner := max(1, width-style.GetHorizontalFrameSize())
	lines := strings.Split(ansi.Hardwrap(ansi.Wrap(domain.StripTerminalControls(domain.ProtectText(text)), inner, ""), inner, false), "\n")
	return style.Render(lipgloss.NewStyle().Foreground(socLabel).Render(title) + "\n" + lipgloss.NewStyle().Foreground(socFg).Render(strings.Join(lines, "\n")))
}

func socraticaSynthesis(s *domain.ScrutinySynthesis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", s.Restatement)
	for _, a := range s.Assumptions {
		kind := "hecho"
		if a.Kind == "hypothesis" {
			kind = "hipótesis"
		}
		fmt.Fprintf(&b, "· [%s] %s\n", kind, a.Text)
	}
	// Orden fijo: View debe ser determinista.
	for _, kv := range [][2]string{{"A favor", s.StrongestSupport}, {"Objeción más fuerte", s.StrongestObjection}} {
		if kv[1] != "" {
			fmt.Fprintf(&b, "%s: %s\n", kv[0], kv[1])
		}
	}
	for _, kv := range []struct {
		label string
		list  []string
	}{{"Alternativas", s.Alternatives}, {"Evidencia faltante", s.MissingEvidence}, {"Criterios", s.DecisionCriteria}} {
		if len(kv.list) > 0 {
			fmt.Fprintf(&b, "%s: %s\n", kv.label, strings.Join(kv.list, "; "))
		}
	}
	for _, h := range s.Hypotheses {
		fmt.Fprintf(&b, "Hipótesis: %s\n", strings.TrimPrefix(h, "Hipótesis: "))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) socraticaDetailView() string {
	if m.soc.choosingHarness {
		return m.socraticaListView()
	}
	width := max(1, m.width-2)
	f := m.soc.current.Flow
	hasFlow := f.ID != ""
	original, proposed, mode := f.OriginalPrompt, f.ProposedPrompt, f.Mode
	scrutiny := f.Scrutiny
	if m.soc.entry != nil && (!hasFlow || f.OriginalPrompt == "") {
		original, proposed, mode, scrutiny = m.soc.entry.OriginalPrompt, m.soc.entry.ProposedPrompt, m.soc.entry.Mode, m.soc.entry.Scrutiny
	}
	var b strings.Builder
	b.WriteString(m.socTitle(width) + "\n")
	label, keys := socraticaActions(f, hasFlow)
	if f.Phase == domain.PhaseFailed {
		info := m.soc.deps.Flow.ExplainFailure(f.LastError)
		keys = "a Continuar asistido · h Cambiar harness · o Enviar original · x Cancelar"
		if info.Retryable {
			keys += " · r Reintentar"
		}
		label = ansi.Hardwrap(ansi.Wrap(info.Message, width, ""), width, false)
	}
	if label != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(socWarn).Bold(true).Render(label) + "\n")
	}
	if m.soc.confirmResend {
		keys = "y confirmar reenvío (autorización nueva) · cualquier otra tecla cancela"
	}
	if keys != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(socFg).Background(socControl).Render(ansi.Hardwrap(ansi.Wrap(" "+keys+" ", width, ""), width, false)) + "\n")
	}
	b.WriteString("\n")
	if width < 60 {
		b.WriteString(lipgloss.NewStyle().Foreground(socWarn).Render(socTrunc("Ventana demasiado estrecha: original conservado", width)) + "\n")
		line := strings.ReplaceAll(domain.StripTerminalControls(domain.ProtectText(original)), "\n", " ")
		runes := []rune(line)
		start := min(m.soc.scroll, max(0, len(runes)-1))
		b.WriteString(socTrunc(string(runes[start:]), width) + "\n")
		return b.String() + m.socFooter("esc volver", width)
	}
	right := proposed
	rightTitle := "Propuesta"
	if mode == domain.ModeScrutiny && scrutiny != nil {
		right, rightTitle = socraticaSynthesis(scrutiny), "Síntesis"
	}
	if width >= 100 {
		half := (width - 1) / 2
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.socraticaCard("Original", original, false, half), " ", m.socraticaCard(rightTitle, right, true, half)) + "\n")
	} else {
		b.WriteString(m.socraticaCard("Original", original, false, width) + "\n" + m.socraticaCard(rightTitle, right, true, width) + "\n")
	}
	summary := fmt.Sprintf("Palabras: %d → %d · adiciones: %d", domain.WordCount(original), domain.WordCount(proposed), len(f.Additions))
	b.WriteString(lipgloss.NewStyle().Foreground(socLabel).Render(socTrunc(summary, width)) + "\n")
	return b.String() + m.socFooter("esc volver", width)
}
