package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type screen int

const (
	screenList screen = iota
	screenDetail
	screenSave
	screenMaintenance
	screenMaintenanceConfirm
	screenConfig
	screenImport
)

const gcDefaultOlderThanDays = 90

var maintenanceOptions = []string{"Purgar", "Compactar", "Garbage Collection"}

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
	case string(domain.Architecture):
		return lipgloss.Color("#A78BFA")
	case string(domain.Decision):
		return lipgloss.Color("#34D399")
	case string(domain.Bugfix):
		return lipgloss.Color("#F87171")
	case string(domain.Pattern):
		return lipgloss.Color("#60A5FA")
	case string(domain.Learning):
		return lipgloss.Color("#FBBF24")
	case string(domain.Discovery):
		return lipgloss.Color("#22D3EE")
	case string(domain.Preference):
		return lipgloss.Color("#F472B6")
	default:
		return lipgloss.Color("#52525B")
	}
}

func typeIcon(t string) string {
	switch t {
	case string(domain.Architecture):
		return "▲"
	case string(domain.Decision):
		return "◆"
	case string(domain.Bugfix):
		return "✕"
	case string(domain.Pattern):
		return "■"
	case string(domain.Learning):
		return "●"
	case string(domain.Discovery):
		return "◇"
	case string(domain.Preference):
		return "♥"
	default:
		return "●"
	}
}

func typeLabel(t string) string {
	switch t {
	case string(domain.Architecture):
		return "Arquitectura"
	case string(domain.Decision):
		return "Decisión"
	case string(domain.Bugfix):
		return "Bugfix"
	case string(domain.Pattern):
		return "Patrón"
	case string(domain.Learning):
		return "Aprendizaje"
	case string(domain.Discovery):
		return "Hallazgo"
	case string(domain.Preference):
		return "Preferencia"
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
	memRepo         ports.MemoryRepository
	relRepo         ports.RelationRepository
	settingsRepo    ports.SettingsRepository
	maintenanceRepo ports.MaintenanceRepository
	codeProvider    ports.CodeGraphProvider
	root            string
	project         string

	screen   screen
	memories []domain.Memory
	cursor   int
	err      error

	selected    domain.Memory
	searching   bool
	search      string
	autoApprove bool
	statusMsg   string
	statusTimer int

	saveTitle    textinput.Model
	saveType     textinput.Model
	saveContent  textinput.Model
	saveFilepath textinput.Model
	saveFocus    int
	saveErr      string
	saved        bool

	stats        ports.StorageStats
	maintCursor  int
	maintAction  string // "purge" o "gc"
	maintConfirm textinput.Model
	maintErr     string

	configCursor int
	importPath   textinput.Model
	importErr    string

	width  int
	height int
	ready  bool
}

func Run(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, settingsRepo ports.SettingsRepository, maintenanceRepo ports.MaintenanceRepository, codeProvider ports.CodeGraphProvider, root, project string) error {
	p := tea.NewProgram(initialModel(memRepo, relRepo, settingsRepo, maintenanceRepo, codeProvider, root, project), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, settingsRepo ports.SettingsRepository, maintenanceRepo ports.MaintenanceRepository, codeProvider ports.CodeGraphProvider, root, project string) model {
	mems, _ := memRepo.List(project, 200)

	ti := textinput.New()
	ti.Placeholder = "Título (opcional)"
	ti.CharLimit = 120
	ti.Width = 50

	ty := textinput.New()
	ty.Placeholder = "learning, decision, architecture, bugfix, pattern, discovery, preference"
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

	mc := textinput.New()
	mc.Placeholder = "nombre del proyecto"
	mc.CharLimit = 200
	mc.Width = 50

	ip := textinput.New()
	ip.Placeholder = "ruta al archivo .json a importar"
	ip.CharLimit = 400
	ip.Width = 50

	settings := settingsRepo.Read(root)

	var stats ports.StorageStats
	if maintenanceRepo != nil {
		stats, _ = maintenanceRepo.Stats(project)
	}

	return model{
		memRepo:         memRepo,
		relRepo:         relRepo,
		settingsRepo:    settingsRepo,
		maintenanceRepo: maintenanceRepo,
		codeProvider:    codeProvider,
		root:            root,
		project:         project,
		screen:          screenList,
		memories:        mems,
		autoApprove:     settings.AutoApprove,
		saveTitle:       ti,
		saveType:        ty,
		saveContent:     tc,
		saveFilepath:    tf,
		stats:           stats,
		maintConfirm:    mc,
		importPath:      ip,
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
		if m.screen == screenMaintenance {
			return m.updateMaintenance(msg)
		}
		if m.screen == screenMaintenanceConfirm {
			return m.updateMaintenanceConfirm(msg)
		}
		if m.screen == screenConfig {
			return m.updateConfig(msg)
		}
		if m.screen == screenImport {
			return m.updateImport(msg)
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
		settings := m.settingsRepo.Read(m.root)
		settings.AutoApprove = m.autoApprove
		m.settingsRepo.Write(m.root, settings)
		m.settingsRepo.ApplyAutoApprove(m.root, settings)
		if m.autoApprove {
			m.statusMsg = "Auto-approve activado ✓"
		} else {
			m.statusMsg = "Auto-approve desactivado"
		}
		m.statusTimer = 30

	case "m":
		if m.ready && m.maintenanceRepo != nil {
			m.screen = screenMaintenance
			m.maintCursor = 0
			m.maintErr = ""
			m.stats, _ = m.maintenanceRepo.Stats(m.project)
		}

	case "c":
		if m.ready {
			m.screen = screenConfig
			m.configCursor = 0
			m.statusMsg = ""
		}

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

func (m model) visibleMemories() []domain.Memory {
	if m.search == "" {
		return m.memories
	}
	q := strings.ToLower(m.search)
	var filtered []domain.Memory
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

// ─── Maintenance screen ─────────────────────────────────────────────

func (m model) updateMaintenance(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenList
		m.maintErr = ""

	case "j", "down":
		if m.maintCursor < len(maintenanceOptions)-1 {
			m.maintCursor++
		}

	case "k", "up":
		if m.maintCursor > 0 {
			m.maintCursor--
		}

	case "enter":
		switch m.maintCursor {
		case 0: // Purgar
			m.maintAction = "purge"
			m.maintConfirm.SetValue("")
			m.maintConfirm.Focus()
			m.maintErr = ""
			m.screen = screenMaintenanceConfirm

		case 1: // Compactar — no destructivo, se ejecuta directo (FR-006)
			before, after, err := m.maintenanceRepo.Compact()
			if err != nil {
				m.statusMsg = fmt.Sprintf("Error al compactar: %v", err)
			} else {
				m.statusMsg = fmt.Sprintf("Compactado: %s → %s", humanize.Bytes(uint64(before)), humanize.Bytes(uint64(after)))
				m.stats, _ = m.maintenanceRepo.Stats(m.project)
			}
			m.statusTimer = 30

		case 2: // Garbage Collection
			m.maintAction = "gc"
			m.maintConfirm.SetValue("")
			m.maintConfirm.Focus()
			m.maintErr = ""
			m.screen = screenMaintenanceConfirm
		}
	}
	return m, nil
}

func (m model) updateMaintenanceConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMaintenance
		m.maintErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		typed := strings.TrimSpace(m.maintConfirm.Value())
		if typed != m.project {
			m.maintErr = "El nombre no coincide. No se eliminó nada."
			return m, nil
		}

		filter := ports.PurgeFilter{Project: m.project}
		actionLabel := "Purga"
		if m.maintAction == "gc" {
			filter.OlderThanDays = gcDefaultOlderThanDays
			actionLabel = "Garbage collection"
		}

		deleted, err := m.maintenanceRepo.Purge(filter)
		if err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
		} else {
			m.statusMsg = fmt.Sprintf("%s: %d memoria(s) eliminada(s)", actionLabel, deleted)
			m.memories, _ = m.memRepo.List(m.project, 200)
			m.stats, _ = m.maintenanceRepo.Stats(m.project)
			m.cursor = 0
		}
		m.statusTimer = 30
		m.maintErr = ""
		m.screen = screenMaintenance
		return m, nil
	}

	var cmd tea.Cmd
	m.maintConfirm, cmd = m.maintConfirm.Update(msg)
	return m, cmd
}

// ─── Config screen ──────────────────────────────────────────────────

// configOptions es el número de filas del menú de configuración.
const configOptions = 4

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenList

	case "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		if m.configCursor < configOptions-1 {
			m.configCursor++
		}

	case "k", "up":
		if m.configCursor > 0 {
			m.configCursor--
		}

	case "enter", " ":
		switch m.configCursor {
		case 0: // Toggle grafo de código externo
			s := m.settingsRepo.Read(m.root)
			s.CodeGraphDisabled = !s.CodeGraphDisabled
			m.settingsRepo.Write(m.root, s)
			if s.CodeGraphDisabled {
				m.statusMsg = "Grafo externo desactivado (aplica en próximas sesiones)"
			} else {
				m.statusMsg = "Grafo externo activado (aplica en próximas sesiones)"
			}
			m.statusTimer = 40

		case 1: // Toggle auto-approve
			m.autoApprove = !m.autoApprove
			s := m.settingsRepo.Read(m.root)
			s.AutoApprove = m.autoApprove
			m.settingsRepo.Write(m.root, s)
			m.settingsRepo.ApplyAutoApprove(m.root, s)
			if m.autoApprove {
				m.statusMsg = "Auto-approve activado ✓"
			} else {
				m.statusMsg = "Auto-approve desactivado"
			}
			m.statusTimer = 30

		case 2: // Exportar memorias
			path, nMem, nRel, err := m.exportMemories()
			if err != nil {
				m.statusMsg = "Error al exportar: " + err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("Exportadas %d memorias y %d relaciones → %s", nMem, nRel, path)
			}
			m.statusTimer = 80

		case 3: // Importar memorias
			m.screen = screenImport
			m.importPath.SetValue("")
			m.importPath.Focus()
			m.importErr = ""
		}
	}
	return m, nil
}

// exportMemories vuelca las memorias + relaciones del proyecto a un JSON en la
// raíz del proyecto y devuelve la ruta y los conteos.
func (m model) exportMemories() (string, int, int, error) {
	bundle, err := usecases.ExportProject(m.memRepo, m.relRepo, m.project)
	if err != nil {
		return "", 0, 0, err
	}
	path := filepath.Join(m.root, fmt.Sprintf("gomemory-export-%s-%s.json", m.project, time.Now().Format("20060102")))
	f, err := os.Create(path)
	if err != nil {
		return "", 0, 0, err
	}
	defer f.Close()
	if err := usecases.EncodeBundle(f, bundle); err != nil {
		return "", 0, 0, err
	}
	return path, len(bundle.Memories), len(bundle.Relations), nil
}

// ─── Import screen ──────────────────────────────────────────────────

func (m model) updateImport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenConfig
		m.importErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		path := strings.TrimSpace(m.importPath.Value())
		if path == "" {
			m.importErr = "Indica una ruta de archivo"
			return m, nil
		}
		rep, err := m.importMemories(path)
		if err != nil {
			m.importErr = err.Error()
			return m, nil
		}
		m.memories, _ = m.memRepo.List(m.project, 200)
		m.statusMsg = fmt.Sprintf("Import: %d memorias nuevas (%d omitidas), %d relaciones nuevas (%d omitidas)",
			rep.MemoriesImported, rep.MemoriesSkipped, rep.RelationsImported, rep.RelationsSkipped)
		m.statusTimer = 80
		m.importErr = ""
		m.screen = screenConfig
		return m, nil
	}

	var cmd tea.Cmd
	m.importPath, cmd = m.importPath.Update(msg)
	return m, cmd
}

func (m model) importMemories(path string) (domain.ImportReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return domain.ImportReport{}, err
	}
	defer f.Close()
	bundle, err := usecases.DecodeBundle(f)
	if err != nil {
		return domain.ImportReport{}, err
	}
	return usecases.ImportBundle(m.memRepo, m.relRepo, m.project, bundle)
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

	mtype := domain.ValidMemoryType(strings.TrimSpace(m.saveType.Value()))
	mem := domain.Memory{
		Project:  m.project,
		Type:     mtype,
		Title:    strings.TrimSpace(m.saveTitle.Value()),
		Content:  content,
		Filepath: strings.TrimSpace(m.saveFilepath.Value()),
	}

	_, err := m.memRepo.Insert(&mem)
	if err != nil {
		m.saveErr = fmt.Sprintf("Error al guardar: %v", err)
		return m, nil
	}

	m.saveErr = ""
	m.saved = true
	m.screen = screenList
	m.memories, _ = m.memRepo.List(m.project, 200)
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
	case screenMaintenance:
		return m.maintenanceView()
	case screenMaintenanceConfirm:
		return m.maintenanceConfirmView()
	case screenConfig:
		return m.configView()
	case screenImport:
		return m.importView()
	}
	return ""
}

func (m model) listView() string {
	var b strings.Builder

	title := titleStyle.Render("gomemory")
	sizeInfo := ""
	if m.maintenanceRepo != nil {
		sizeInfo = " · " + humanize.Bytes(uint64(m.stats.FileSizeBytes)) + " en disco"
	}
	info := subtitleStyle.Render(fmt.Sprintf("%s · %d memorias%s", m.project, len(m.memories), sizeInfo))
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
		grouped := groupByType(visible)
		typeOrder := []string{"preference", "architecture", "decision", "pattern", "bugfix", "learning", "discovery"}
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

func (m model) maintenanceView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Mantenimiento de memoria"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf(
		"%s · %d/%d memorias (proyecto/total) · %s en disco",
		m.project, m.stats.ProjectMemoryCount, m.stats.TotalMemoryCount, humanize.Bytes(uint64(m.stats.FileSizeBytes)),
	)))
	b.WriteString("\n\n")

	for i, label := range maintenanceOptions {
		if i == m.maintCursor {
			b.WriteString(itemSelected.Render("▸ " + label))
		} else {
			b.WriteString(itemNormal.Render("  " + label))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.statusTimer > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(faint).Italic(true).Render("  " + m.statusMsg))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("  ↑↓ navegar  ·  enter seleccionar  ·  esc volver"))
	return appStyle.Render(b.String())
}

func (m model) maintenanceConfirmView() string {
	var b strings.Builder

	actionLabel := "Purgar"
	if m.maintAction == "gc" {
		actionLabel = "Garbage Collection"
	}

	b.WriteString(titleStyle.Render(actionLabel))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(red).Bold(true).Render(
		fmt.Sprintf("Esto eliminará memorias del proyecto %q permanentemente.", m.project),
	))
	b.WriteString("\n\n")
	b.WriteString(formLabel.Render("Escribe el nombre del proyecto para confirmar:"))
	b.WriteString("\n")
	b.WriteString(m.maintConfirm.View())
	b.WriteString("\n\n")

	if m.maintErr != "" {
		b.WriteString(errorStyle.Render("✕ " + m.maintErr))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  enter confirmar  ·  esc cancelar"))
	return appStyle.Render(b.String())
}

func onOff(v bool) string {
	if v {
		return lipgloss.NewStyle().Foreground(green).Render("ON")
	}
	return lipgloss.NewStyle().Foreground(faint).Render("OFF")
}

func (m model) configView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configuración"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(m.project))
	b.WriteString("\n\n")

	s := m.settingsRepo.Read(m.root)

	// Estado del grafo de código externo (solo lectura, desde el snapshot).
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("  Grafo de código externo"))
	b.WriteString("\n")
	var snap domain.CodeProviderSnapshot
	if m.codeProvider != nil {
		snap = m.codeProvider.Snapshot()
	}
	provState := lipgloss.NewStyle().Foreground(faint).Render("no disponible")
	if snap.Available {
		det := ""
		if snap.Architecture != nil {
			det = fmt.Sprintf(" · %d nodos, %d relaciones", snap.Architecture.TotalNodes, snap.Architecture.TotalEdges)
		}
		provState = lipgloss.NewStyle().Foreground(green).Render("disponible" + det)
	}
	b.WriteString("    Proveedor: " + provState + "\n")
	if !snap.CheckedAt.IsZero() {
		b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("    Última actualización: "+snap.CheckedAt.Format("2006-01-02 15:04:05")) + "\n")
	}
	bin := s.CodeGraphCommand
	if bin == "" {
		bin = "codebase-memory-mcp (PATH)"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("    Binario: "+bin) + "\n\n")

	// Menú de acciones.
	rows := []string{
		"Grafo de código externo: " + onOff(!s.CodeGraphDisabled),
		"Auto-approve MCP: " + onOff(s.AutoApprove),
		"Exportar memorias",
		"Importar memorias",
	}
	for i, label := range rows {
		if i == m.configCursor {
			b.WriteString(itemSelected.Render("▸ " + label))
		} else {
			b.WriteString(itemNormal.Render("  " + label))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.statusTimer > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(faint).Italic(true).Render("  " + m.statusMsg))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("  ↑↓ navegar  ·  enter activar/ejecutar  ·  esc volver"))
	return appStyle.Render(b.String())
}

func (m model) importView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Importar memorias"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Append con dedup por contenido · preserva timestamps · remapea al proyecto"))
	b.WriteString("\n\n")
	b.WriteString(formLabel.Render("Ruta del archivo .json a importar:"))
	b.WriteString("\n")
	b.WriteString(m.importPath.View())
	b.WriteString("\n\n")

	if m.importErr != "" {
		b.WriteString(errorStyle.Render("✕ " + m.importErr))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  enter importar  ·  esc volver"))
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
		"c config",
		"m mantenimiento",
		"/ buscar",
		"q salir",
	}
	return helpStyle.Render("  " + strings.Join(items, "  ·  "))
}

// ─── Helpers ───────────────────────────────────────────────────────

func groupByType(mems []domain.Memory) map[string][]domain.Memory {
	g := make(map[string][]domain.Memory)
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
