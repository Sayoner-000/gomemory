package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"context"
	"mem/application/usecases"
	"mem/domain"
	"strings"
)

// HCDStarter deliberately exposes only creation: approvals belong to the agent ceremony.
type HCDStarter interface {
	Start(context.Context, usecases.HCDStartInput) (usecases.HCDView, error)
}

// StartSpecification is the creation boundary shared by the interactive form
// and adapter contract tests. It fixes provenance and exposes no approval action.
func StartSpecification(ctx context.Context, flow HCDStarter, input usecases.HCDStartInput) (domain.Unit, error) {
	if flow == nil {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	input.Client = "tui"
	view, err := flow.Start(ctx, input)
	return view.Unit, err
}

type specificationState struct {
	flow                   HCDStarter
	cursor, kind, focus    int
	editing, busy, created bool
	title, why             textinput.Model
	message                string
}
type specificationStartedMsg struct {
	unit domain.Unit
	err  error
}

var specificationKinds = []domain.Kind{domain.KindFeature, domain.KindEvolution, domain.KindFix, domain.KindTechnical}

// specificationOptions son los dos carriles de HCD y Spec Kit, que coexiste
// aparte. Un carril vacío no crea unidad.
var specificationOptions = []struct {
	label string
	lane  domain.Lane
}{
	{"HCD Vía rápida — trabajo ordinario, sin approve ni slice", domain.LaneFast},
	{"HCD ceremonia completa — decisiones interdependientes o riesgo alto", domain.LaneFull},
	{"Spec Kit — flujo aparte, sin cambios", ""},
}

func newSpecificationState(flow HCDStarter) specificationState {
	title, why := textinput.New(), textinput.New()
	title.CharLimit, why.CharLimit = 200, 200
	title.SetWidth(50)
	why.SetWidth(50)
	return specificationState{flow: flow, title: title, why: why}
}
func (m model) updateSpecification(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if key.String() == "esc" && !m.spec.busy {
		m.screen = screenList
		return m, nil
	}
	if m.spec.busy || m.spec.created {
		return m, nil
	}
	if !m.spec.editing {
		switch key.String() {
		case "up", "k":
			m.spec.cursor = (m.spec.cursor + len(specificationOptions) - 1) % len(specificationOptions)
		case "down", "j":
			m.spec.cursor = (m.spec.cursor + 1) % len(specificationOptions)
		case "enter":
			if specificationOptions[m.spec.cursor].lane == "" {
				m.spec.message = "Continúa con /speckit-specify o /socratica-hu."
				return m, nil
			}
			if m.spec.flow == nil {
				m.spec.message = "Motor HCD no disponible"
				return m, nil
			}
			m.spec.editing = true
			return m, m.spec.title.Focus()
		}
		return m, nil
	}
	switch key.String() {
	case "tab", "shift+tab":
		m.spec.focus = 1 - m.spec.focus
		if m.spec.focus == 0 {
			m.spec.why.Blur()
			return m, m.spec.title.Focus()
		}
		m.spec.title.Blur()
		return m, m.spec.why.Focus()
	case "left":
		m.spec.kind = (m.spec.kind + len(specificationKinds) - 1) % len(specificationKinds)
	case "right":
		m.spec.kind = (m.spec.kind + 1) % len(specificationKinds)
	case "enter":
		if strings.TrimSpace(m.spec.title.Value()) == "" || strings.TrimSpace(m.spec.why.Value()) == "" {
			m.spec.message = "Completa título y motivo"
			return m, nil
		}
		m.spec.busy = true
		flow := m.spec.flow
		input := usecases.HCDStartInput{Title: m.spec.title.Value(), Why: m.spec.why.Value(), Kind: string(specificationKinds[m.spec.kind]), Lane: string(specificationOptions[m.spec.cursor].lane)}
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), domain.DefaultQueryTimeout)
			defer cancel()
			u, err := StartSpecification(ctx, flow, input)
			return specificationStartedMsg{u, err}
		}
	default:
		var cmd tea.Cmd
		if m.spec.focus == 0 {
			m.spec.title, cmd = m.spec.title.Update(key)
		} else {
			m.spec.why, cmd = m.spec.why.Update(key)
		}
		return m, cmd
	}
	return m, nil
}
func (m model) specificationView() string {
	b := "Especificación\n\n" + specificationGuide + "\n\n"
	for i, option := range specificationOptions {
		prefix := "  "
		if m.spec.cursor == i {
			prefix = "▸ "
		}
		b += prefix + option.label + "\n"
	}
	if m.spec.editing {
		b += "\nTipo: " + string(specificationKinds[m.spec.kind]) + " (←/→)\nTítulo: " + m.spec.title.View() + "\nMotivo: " + m.spec.why.View() + "\nTab cambia campo · Enter crea la unidad\n"
	}
	if m.spec.busy {
		b += "\nCreando unidad…\n"
	}
	return b + "\n" + m.spec.message + "\n\nesc volver"
}
