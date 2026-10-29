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
	Start(context.Context, usecases.HCDStartInput) (domain.Unit, error)
}

// StartSpecification is the creation boundary shared by the interactive form
// and adapter contract tests. It fixes provenance and exposes no approval action.
func StartSpecification(ctx context.Context, flow HCDStarter, input usecases.HCDStartInput) (domain.Unit, error) {
	if flow == nil {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	input.Client = "tui"
	return flow.Start(ctx, input)
}

type specificationState struct {
	flow                   HCDStarter
	cursor, kind, focus    int
	editing, busy, created bool
	title, implementer     textinput.Model
	message                string
}
type specificationStartedMsg struct {
	unit domain.Unit
	err  error
}

var specificationKinds = []domain.Kind{domain.KindFeature, domain.KindEvolution, domain.KindFix, domain.KindTechnical}

func newSpecificationState(flow HCDStarter) specificationState {
	title, implementer := textinput.New(), textinput.New()
	title.CharLimit, implementer.CharLimit = 200, 100
	title.SetWidth(50)
	implementer.SetWidth(50)
	return specificationState{flow: flow, title: title, implementer: implementer}
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
		case "up", "down", "j", "k":
			m.spec.cursor = 1 - m.spec.cursor
		case "enter":
			if m.spec.cursor == 1 {
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
			m.spec.implementer.Blur()
			return m, m.spec.title.Focus()
		}
		m.spec.title.Blur()
		return m, m.spec.implementer.Focus()
	case "left":
		m.spec.kind = (m.spec.kind + len(specificationKinds) - 1) % len(specificationKinds)
	case "right":
		m.spec.kind = (m.spec.kind + 1) % len(specificationKinds)
	case "enter":
		if strings.TrimSpace(m.spec.title.Value()) == "" || strings.TrimSpace(m.spec.implementer.Value()) == "" {
			m.spec.message = "Completa título e implementador"
			return m, nil
		}
		m.spec.busy = true
		flow := m.spec.flow
		input := usecases.HCDStartInput{Title: m.spec.title.Value(), Implementer: m.spec.implementer.Value(), Kind: specificationKinds[m.spec.kind], Client: "tui"}
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
			m.spec.implementer, cmd = m.spec.implementer.Update(key)
		}
		return m, cmd
	}
	return m, nil
}
func (m model) specificationView() string {
	b := "Especificación\n\n" + specificationGuide + "\n\n"
	for i, option := range []string{"HCD Playbook — recomendado", "Spec Kit — legado"} {
		prefix := "  "
		if m.spec.cursor == i {
			prefix = "▸ "
		}
		b += prefix + option + "\n"
	}
	if m.spec.editing {
		b += "\nTipo: " + string(specificationKinds[m.spec.kind]) + " (←/→)\nTítulo: " + m.spec.title.View() + "\nImplementador: " + m.spec.implementer.View() + "\nTab cambia campo · Enter crea borrador\n"
	}
	if m.spec.busy {
		b += "\nCreando unidad…\n"
	}
	return b + "\n" + m.spec.message + "\n\nesc volver"
}
