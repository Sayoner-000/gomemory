package console

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// La UI rica se dibuja en línea, sin pantalla alternativa: al terminar queda
// en el historial de la terminal lo que se eligió, como en skills.sh.

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Theme(os.Getenv).Primary)).Bold(true)
	faintStyle  = lipgloss.NewStyle().Faint(true)
	recStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

type richUI struct {
	in  io.Reader
	out io.Writer
}

// NewRich crea la UI con la interfaz de terminal completa.
func NewRich(in io.Reader, out io.Writer) UI {
	return &richUI{in: in, out: out}
}

func (r *richUI) run(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m, tea.WithInput(r.in), tea.WithOutput(r.out)).Run()
}

func (r *richUI) MultiSelect(title string, opts []Option) ([]string, error) {
	final, err := r.run(newMultiSelectModel(title, opts))
	if err != nil {
		return nil, err
	}
	m := final.(multiSelectModel)
	if m.canceled {
		return nil, ErrCanceled
	}
	return m.values(), nil
}

func (r *richUI) Select(title string, opts []Option) (string, error) {
	final, err := r.run(newSelectModel(title, opts))
	if err != nil {
		return "", err
	}
	m := final.(selectModel)
	if m.canceled {
		return "", ErrCanceled
	}
	return m.value(), nil
}

func (r *richUI) Confirm(title string, def bool) (bool, error) {
	final, err := r.run(confirmModel{title: title, answer: def})
	if err != nil {
		return false, err
	}
	m := final.(confirmModel)
	if m.canceled {
		return false, ErrCanceled
	}
	return m.answer, nil
}

func (r *richUI) TypedConfirm(title, word string) (bool, error) {
	ti := textinput.New()
	ti.Placeholder = word
	ti.CharLimit = 64
	final, err := r.run(typedModel{title: title, word: word, input: ti})
	if err != nil {
		return false, err
	}
	m := final.(typedModel)
	if m.canceled {
		return false, ErrCanceled
	}
	return m.input.Value() == word, nil
}

func isCancel(k string) bool { return k == "esc" || k == "ctrl+c" }

// --- selección múltiple ---

type multiSelectModel struct {
	title    string
	opts     []Option
	checked  []bool
	cursor   int
	done     bool
	canceled bool
}

func newMultiSelectModel(title string, opts []Option) multiSelectModel {
	checked := make([]bool, len(opts))
	for i, o := range opts {
		checked[i] = o.Checked
	}
	return multiSelectModel{title: title, opts: opts, checked: checked}
}

func (m multiSelectModel) Init() tea.Cmd { return nil }

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch s := k.String(); {
	case isCancel(s):
		m.canceled = true
		return m, tea.Quit
	case s == "up" || s == "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case s == "down" || s == "j":
		if m.cursor < len(m.opts)-1 {
			m.cursor++
		}
	case s == "space" || s == "x":
		m.checked[m.cursor] = !m.checked[m.cursor]
	case s == "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m multiSelectModel) values() []string {
	var out []string
	for i, c := range m.checked {
		if c {
			out = append(out, m.opts[i].Value)
		}
	}
	return out
}

func (m multiSelectModel) View() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("◆ "+m.title) + "\n")
	if m.done || m.canceled {
		labels := make([]string, 0, len(m.opts))
		for i, c := range m.checked {
			if c {
				labels = append(labels, m.opts[i].Label)
			}
		}
		b.WriteString(faintStyle.Render("  "+strings.Join(labels, ", ")) + "\n")
		return tea.NewView(b.String())
	}
	for i, o := range m.opts {
		box := "◻"
		if m.checked[i] {
			box = "◼"
		}
		line := fmt.Sprintf("%s %s%s", box, o.Label, hintSuffix(o.Hint))
		if i == m.cursor {
			line = cursorStyle.Render("› " + line)
		} else {
			line = "  " + line
		}
		b.WriteString("  " + line + "\n")
	}
	b.WriteString(faintStyle.Render("  ↑/↓ mover · espacio marcar · enter confirmar · esc cancelar") + "\n")
	return tea.NewView(b.String())
}

// --- selección única ---

type selectModel struct {
	title    string
	opts     []Option
	cursor   int
	done     bool
	canceled bool
}

func newSelectModel(title string, opts []Option) selectModel {
	return selectModel{title: title, opts: opts, cursor: recommendedIndex(opts)}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch s := k.String(); {
	case isCancel(s):
		m.canceled = true
		return m, tea.Quit
	case s == "up" || s == "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case s == "down" || s == "j":
		if m.cursor < len(m.opts)-1 {
			m.cursor++
		}
	case s == "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m selectModel) value() string { return m.opts[m.cursor].Value }

func (m selectModel) View() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("◆ "+m.title) + "\n")
	if m.done || m.canceled {
		b.WriteString(faintStyle.Render("  "+m.opts[m.cursor].Label) + "\n")
		return tea.NewView(b.String())
	}
	for i, o := range m.opts {
		line := o.Label
		if o.Recommended {
			line += " " + recStyle.Render("(Recomendado)")
		}
		line += hintSuffix(o.Hint)
		if i == m.cursor {
			line = cursorStyle.Render("● ") + line
		} else {
			line = "○ " + line
		}
		b.WriteString("  " + line + "\n")
	}
	b.WriteString(faintStyle.Render("  ↑/↓ mover · enter elegir · esc cancelar") + "\n")
	return tea.NewView(b.String())
}

// --- confirmación ---

type confirmModel struct {
	title    string
	answer   bool
	done     bool
	canceled bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch s := k.String(); {
	case isCancel(s):
		m.canceled = true
		return m, tea.Quit
	case s == "s" || s == "y":
		m.answer, m.done = true, true
		return m, tea.Quit
	case s == "n":
		m.answer, m.done = false, true
		return m, tea.Quit
	case s == "left" || s == "right" || s == "tab":
		m.answer = !m.answer
	case s == "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m confirmModel) View() tea.View {
	yes, no := "Sí", "No"
	if m.answer {
		yes = cursorStyle.Render("› Sí")
	} else {
		no = cursorStyle.Render("› No")
	}
	return tea.NewView(titleStyle.Render("◆ "+m.title) + "\n  " + yes + "  /  " + no + "\n")
}

// --- confirmación escrita ---

type typedModel struct {
	title    string
	word     string
	input    textinput.Model
	done     bool
	canceled bool
}

func (m typedModel) Init() tea.Cmd { return m.input.Focus() }

func (m typedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch s := k.String(); {
		case isCancel(s):
			m.canceled = true
			return m, tea.Quit
		case s == "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m typedModel) View() tea.View {
	return tea.NewView(titleStyle.Render("◆ "+m.title) + "\n  Escribe " + m.word + " para confirmar: " +
		m.input.View() + "\n")
}
