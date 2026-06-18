package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mem/store"
	"mem/types"
)

type screen int

const (
	screenList screen = iota
	screenDetail
	screenSave
)

// ─── Styles ───────────────────────────────────────────────────────

var (
	faint     = lipgloss.AdaptiveColor{Light: "#A1A1AA", Dark: "#52525B"}
	highlight = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	green     = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	red       = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	blue      = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"}
	yellow    = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}
	cyan      = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"}
	gray      = lipgloss.AdaptiveColor{Light: "#E4E4E7", Dark: "#27272A"}
	white     = lipgloss.AdaptiveColor{Light: "#18181B", Dark: "#FAFAFA"}
	bg        = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#18181B"}
)

func typeColor(t string) lipgloss.Color {
	switch t {
	case string(types.Architecture):
		return lipgloss.Color("#A78BFA")
	case string(types.Decision):
		return lipgloss.Color("#34D399")
	case string(types.Bugfix):
		return lipgloss.Color("#F87171")
	case string(types.Pattern):
		return lipgloss.Color("#60A5FA")
	case string(types.Learning):
		return lipgloss.Color("#FBBF24")
	case string(types.Discovery):
		return lipgloss.Color("#22D3EE")
	default:
		return lipgloss.Color("#52525B")
	}
}

func typeIcon(t string) string {
	switch t {
	case string(types.Architecture):
		return "▲"
	case string(types.Decision):
		return "◆"
	case string(types.Bugfix):
		return "✕"
	case string(types.Pattern):
		return "■"
	case string(types.Learning):
		return "●"
	case string(types.Discovery):
		return "◇"
	default:
		return "●"
	}
}

func typeLabel(t string) string {
	switch t {
	case string(types.Architecture):
		return "Arquitectura"
	case string(types.Decision):
		return "Decisión"
	case string(types.Bugfix):
		return "Bugfix"
	case string(types.Pattern):
		return "Patrón"
	case string(types.Learning):
		return "Aprendizaje"
	case string(types.Discovery):
		return "Hallazgo"
	default:
		return t
	}
}

var (
	appStyle = lipgloss.NewStyle().
		Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(highlight).
		MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(faint).
		Italic(true)

	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(faint).
				Padding(0, 1).
				MarginTop(1).
				MarginBottom(1)

	typeTag = func(t string) string {
		return lipgloss.NewStyle().
			Background(typeColor(t)).
			Foreground(white).
			Padding(0, 1).
			Bold(true).
			Render(typeIcon(t) + " " + typeLabel(t))
	}

	itemNormal = lipgloss.NewStyle().
			Padding(0, 2)

	itemSelected = lipgloss.NewStyle().
			Padding(0, 2).
			Background(gray).
			Foreground(white)

	itemContent = lipgloss.NewStyle().
			Foreground(faint).
			Padding(0, 2).
			MaxWidth(80)

	detailBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
		Foreground(faint).
		PaddingTop(1).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder())

	formStyle = lipgloss.NewStyle().
			MarginTop(1)

	formLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginRight(1)

	formInput = lipgloss.NewStyle().
			MarginBottom(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)
)

// ─── Model ─────────────────────────────────────────────────────────

type model struct {
	db      *sql.DB
	root    string
	project string

	screen   screen
	memories []types.Memory
	cursor   int
	err      error

	selected     types.Memory
	searching    bool
	search       string
	autoApprove  bool
	statusMsg    string
	statusTimer  int

	saveTitle    textinput.Model
	saveType     textinput.Model
	saveContent  textinput.Model
	saveFilepath textinput.Model
	saveFocus    int
	saveErr      string
	saved        bool

	width  int
	height int
	ready  bool
}

func Run(db *sql.DB, root, project string) error {
	p := tea.NewProgram(initialModel(db, root, project), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel(db *sql.DB, root, project string) model {
	mems, _ := store.ListMemories(db, project, 200)

	ti := textinput.New()
	ti.Placeholder = "Título (opcional)"
	ti.CharLimit = 120
	ti.Width = 50

	ty := textinput.New()
	ty.Placeholder = "learning, decision, architecture, bugfix, pattern, discovery"
	ty.CharLimit = 20
	ty.Width = 50
	ty.SetValue("learning")

	tc := textinput.New()
	tc.Placeholder = "¿Qué aprendiste o decidiste?"
	tc.CharLimit = 500
	tc.Width = 50
	tc.Focus()

	tf := textinput.New()
	tf.Placeholder = "Archivo relacionado (opcional)"
	tf.CharLimit = 200
	tf.Width = 50

	settings := store.ReadSettings(root)

	return model{
		db:           db,
		root:         root,
		project:      project,
		screen:       screenList,
		memories:     mems,
		autoApprove:  settings.AutoApprove,
		saveTitle:    ti,
		saveType:     ty,
		saveContent:  tc,
		saveFilepath: tf,
	}
}

// ─── Init ──────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// ─── Update ────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		if m.statusTimer > 0 {
			m.statusTimer--
		}
		if m.screen == screenSave {
			return m.updateSave(msg)
		}
		if m.screen == screenDetail {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}

	return m, nil
}

// ─── List screen ───────────────────────────────────────────────────

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.memories)-1 {
			m.cursor++
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "enter":
		// Filter memories for visible view
		visible := m.visibleMemories()
		if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
			m.selected = visible[m.cursor]
			m.screen = screenDetail
		}

	case "s":
		if m.ready {
			m.screen = screenSave
			m.saveContent.Focus()
		}

	case "a":
		m.autoApprove = !m.autoApprove
		settings := store.ReadSettings(m.root)
		settings.AutoApprove = m.autoApprove
		store.WriteSettings(m.root, settings)
		store.ApplyAutoApprove(m.root, settings)
		if m.autoApprove {
			m.statusMsg = "Auto-approve activado ✓"
		} else {
			m.statusMsg = "Auto-approve desactivado"
		}
		m.statusTimer = 30

	case "/":
		m.searching = !m.searching
		if !m.searching {
			m.search = ""
			m.cursor = 0
		}

	default:
		if m.searching {
			if msg.String() == "backspace" {
				if len(m.search) > 0 {
					m.search = m.search[:len(m.search)-1]
				}
			} else if msg.String() == "esc" {
				m.searching = false
				m.search = ""
			} else if len(msg.String()) == 1 {
				m.search += msg.String()
			}
			m.cursor = 0
		}
	}

	return m, nil
}

func (m model) visibleMemories() []types.Memory {
	if m.search == "" {
		return m.memories
	}
	q := strings.ToLower(m.search)
	var filtered []types.Memory
	for _, mem := range m.memories {
		if strings.Contains(strings.ToLower(mem.Title), q) ||
			strings.Contains(strings.ToLower(mem.Content), q) ||
			strings.Contains(strings.ToLower(string(mem.Type)), q) {
			filtered = append(filtered, mem)
		}
	}
	return filtered
}

// ─── Detail screen ─────────────────────────────────────────────────

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.screen = screenList
	}
	return m, nil
}

// ─── Save screen ───────────────────────────────────────────────────

func (m model) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenList
		m.saved = false
		m.saveErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "tab", "shift+tab", "up", "down":
		if msg.String() == "tab" || msg.String() == "down" {
			m.saveFocus = (m.saveFocus + 1) % 4
		} else {
			m.saveFocus = (m.saveFocus - 1 + 4) % 4
		}
		m.updateFocus()
		return m, nil

	case "enter":
		return m.saveAndReturn()
	}

	cmds := make([]tea.Cmd, 0, 4)
	var cmd tea.Cmd

	m.saveTitle, cmd = m.saveTitle.Update(msg)
	cmds = append(cmds, cmd)
	m.saveType, cmd = m.saveType.Update(msg)
	cmds = append(cmds, cmd)
	m.saveContent, cmd = m.saveContent.Update(msg)
	cmds = append(cmds, cmd)
	m.saveFilepath, cmd = m.saveFilepath.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) updateFocus() {
	m.saveTitle.Blur()
	m.saveType.Blur()
	m.saveContent.Blur()
	m.saveFilepath.Blur()

	switch m.saveFocus {
	case 0:
		m.saveTitle.Focus()
	case 1:
		m.saveType.Focus()
	case 2:
		m.saveContent.Focus()
	case 3:
		m.saveFilepath.Focus()
	}
}

func (m model) saveAndReturn() (tea.Model, tea.Cmd) {
	content := strings.TrimSpace(m.saveContent.Value())
	if content == "" {
		m.saveErr = "El contenido es obligatorio"
		return m, nil
	}

	mtype := types.ValidMemoryType(strings.TrimSpace(m.saveType.Value()))
	mem := types.Memory{
		Project:   m.project,
		Type:      mtype,
		Title:     strings.TrimSpace(m.saveTitle.Value()),
		Content:   content,
		Filepath:  strings.TrimSpace(m.saveFilepath.Value()),
	}

	_, err := store.InsertMemory(m.db, &mem)
	if err != nil {
		m.saveErr = fmt.Sprintf("Error al guardar: %v", err)
		return m, nil
	}

	m.saveErr = ""
	m.saved = true
	m.screen = screenList
	m.memories, _ = store.ListMemories(m.db, m.project, 200)
	m.saveTitle.SetValue("")
	m.saveType.SetValue("learning")
	m.saveContent.SetValue("")
	m.saveFilepath.SetValue("")
	m.saveFocus = 2
	m.saveContent.Focus()
	return m, nil
}

// ─── View ──────────────────────────────────────────────────────────

func (m model) View() string {
	if !m.ready {
		return ""
	}

	switch m.screen {
	case screenList:
		return m.listView()
	case screenDetail:
		return m.detailView()
	case screenSave:
		return m.saveView()
	}
	return ""
}

func (m model) listView() string {
	var b strings.Builder

	title := titleStyle.Render("gomemory")
	info := subtitleStyle.Render(fmt.Sprintf("%s · %d memorias", m.project, len(m.memories)))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", info))
	if m.autoApprove {
		aa := lipgloss.NewStyle().Foreground(green).Render("autoApprove")
		b.WriteString("  " + aa)
	}
	b.WriteString("\n")

	if m.searching {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(highlight).
			Render("🔍 " + m.search + "█"))
		b.WriteString("\n")
	}

	visible := m.visibleMemories()
	if len(visible) == 0 {
		b.WriteString("\n")
		if m.search != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("  Sin resultados para \"" + m.search + "\""))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("  Todavía no hay memorias.\n  Guarda la primera con: mem save \"aprendizaje\""))
		}
		b.WriteString("\n")
	} else {
		// Group by type
		grouped := groupByType(visible)
		typeOrder := []string{"architecture", "decision", "pattern", "bugfix", "learning", "discovery"}
		globalIdx := 0

		for _, t := range typeOrder {
			mems, ok := grouped[t]
			if !ok {
				continue
			}

			headerLabel := typeLabel(t)
			headerIcon := typeIcon(t)
			b.WriteString(groupHeaderStyle.Render(fmt.Sprintf("  %s %s  (%d)", headerIcon, headerLabel, len(mems))))
			b.WriteString("\n")

			for _, mem := range mems {
				content := truncate(mem.Content, 70)
				line := fmt.Sprintf("  %s", content)
				if mem.Title != "" {
					line = fmt.Sprintf("  %s — %s", mem.Title, content)
				}

				if globalIdx == m.cursor {
					tag := typeTag(string(mem.Type))
					b.WriteString(itemSelected.Render(
						lipgloss.JoinHorizontal(lipgloss.Top,
							lipgloss.NewStyle().Foreground(highlight).Render("▸"),
							" ",
							tag,
							" ",
							line,
						),
					))
				} else {
					b.WriteString(itemNormal.Render(
						lipgloss.JoinHorizontal(lipgloss.Top,
							"  ",
							lipgloss.NewStyle().Foreground(typeColor(string(mem.Type))).Render(typeIcon(string(mem.Type))),
							" ",
							line,
						),
					))
				}
				b.WriteString("\n")
				globalIdx++
			}
		}
	}

	b.WriteString("\n")
	if m.statusTimer > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(faint).Italic(true).Render("  " + m.statusMsg))
		b.WriteString("\n")
	}
	b.WriteString(m.helpView())
	return appStyle.Render(b.String())
}

func (m model) detailView() string {
	mem := m.selected
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("  ← esc para volver"))
	b.WriteString("\n\n")
	b.WriteString(detailBorder.Render(
		lipgloss.JoinVertical(lipgloss.Top,
			lipgloss.NewStyle().Bold(true).Foreground(highlight).Render(mem.Title),
			"",
			typeTag(string(mem.Type))+"  "+lipgloss.NewStyle().Foreground(faint).Render(mem.CreatedAt),
			"",
			mem.Content,
			func() string {
				if mem.Filepath != "" {
					return "\n" + lipgloss.NewStyle().Foreground(faint).Italic(true).Render("📁 "+mem.Filepath)
				}
				return ""
			}(),
			func() string {
				if mem.SessionID != "" {
					return "\n" + lipgloss.NewStyle().Foreground(faint).Render("Sesión: "+mem.SessionID[:8])
				}
				return ""
			}(),
		),
	))

	return appStyle.Render(b.String())
}

func (m model) saveView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Guardar aprendizaje"))
	b.WriteString("\n")

	b.WriteString(formStyle.Render(
		lipgloss.JoinVertical(lipgloss.Top,
			m.renderField("Título", &m.saveTitle),
			m.renderField("Tipo", &m.saveType),
			m.renderField("Contenido", &m.saveContent),
			m.renderField("Archivo", &m.saveFilepath),
		),
	))
	b.WriteString("\n")

	if m.saveErr != "" {
		b.WriteString(errorStyle.Render("✕ " + m.saveErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("  tab · siguiente campo    enter · guardar    esc · cancelar"))

	return appStyle.Render(b.String())
}

func (m model) renderField(label string, input *textinput.Model) string {
	style := formInput
	if m.saveFocus == 2 {
		style = formInput.MarginBottom(0)
	}
	return style.Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			formLabel.Render(label+":"),
			input.View(),
		),
	)
}

func (m model) helpView() string {
	items := []string{
		"↑↓ navegar",
		"enter detalle",
		"s guardar",
		"a autoApprove",
		"/ buscar",
		"q salir",
	}
	return helpStyle.Render("  " + strings.Join(items, "  ·  "))
}

// ─── Helpers ───────────────────────────────────────────────────────

func groupByType(mems []types.Memory) map[string][]types.Memory {
	g := make(map[string][]types.Memory)
	for _, m := range mems {
		t := string(m.Type)
		g[t] = append(g[t], m)
	}
	return g
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return "..."
	}
	return string(r[:n-3]) + "..."
}
