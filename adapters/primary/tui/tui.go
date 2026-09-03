package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	screenOptimize
	screenOptimizeDetail
	screenOptimizeConfirm
	screenOptimizeAllConfirm
	screenEditSetting
	// screenUsage se añade AL FINAL, por la convención de este enum (feature
	// 020): benchmark de tokens por sesión (mem usage) + snapshot puntual,
	// absorbe la spec 017.
	screenUsage
	// screenDocs (feature 021): administración de documentos fijados — ver,
	// exportar, importar y restaurar las reglas de trabajo y la constitución.
	screenDocs
)

// editSettingField identifica cuál de los tres ajustes de huella de contexto
// está editando screenEditSetting (feature 016).
type editSettingField int

const (
	editFieldBudget editSettingField = iota
	editFieldCompactThreshold
	editFieldDedupDays
)

const gcDefaultOlderThanDays = 90

var maintenanceOptions = []string{"Purgar", "Compactar", "Garbage Collection", "Consolidar (topic_key + actividad duplicada)"}

// backHint es el encabezado corto que usan las pantallas de detalle para
// indicar cómo volver, reemplazando el bloque repetido en detailView y
// optimizeDetailView.
func backHint() string {
	return backHintStyle.Render("  ← esc para volver  ·  ↑/↓ scroll  ·  ctrl+y copiar")
}

// statusLine devuelve el mensaje de estado con timer activo (statusMsg +
// statusTimer), o "" si no hay nada que mostrar. Centraliza el bloque
// repetido en listView, maintenanceView, configView y optimizeView.
func (m model) statusLine() string {
	if m.statusTimer <= 0 {
		return ""
	}
	return statusLineStyle.Render("  " + m.statusMsg)
}

// ─── List rows ────────────────────────────────────────────────────

// memoryDisplayTitle da un título de respaldo a memorias sin título (por
// ejemplo checkpoints antiguos), para no dejar la primera línea de la fila en
// blanco.
func memoryDisplayTitle(m domain.Memory) string {
	if strings.TrimSpace(m.Title) != "" {
		return m.Title
	}
	return "(sin título)"
}

// listRowLines arma las 2 líneas de una memoria en la lista principal: icono
// + tipo + título en la primera, una vista previa del contenido en la
// segunda. Sigue el patrón ya usado en optimizeDetailView (fragmentos
// renderizados por separado con JoinHorizontal y luego un único Render de
// fondo/selección envolviendo el bloque) para que el resaltado de selección
// no pise los colores por tipo.
//
// width es el ancho interior disponible (ya descontados marco y padding);
// título y contenido se truncan en función de él para no desbordar el marco
// en terminales angostas — mismo espíritu que el tableColumns(width) que
// reemplaza esta función.
func listRowLines(m domain.Memory, selected bool, width int) []string {
	icon := lipgloss.NewStyle().Foreground(typeColor(string(m.Type))).Bold(true).
		Render(typeIcon(string(m.Type)) + " " + typeLabel(string(m.Type)))

	titleBudget := width - lipgloss.Width(icon) - 4
	if titleBudget < 15 {
		titleBudget = 15
	}
	contentBudget := width - 4
	if contentBudget < 15 {
		contentBudget = 15
	}

	title := lipgloss.NewStyle().Bold(true).Render(truncate(memoryDisplayTitle(m), titleBudget))
	preview := lipgloss.NewStyle().Foreground(faint).Render(truncate(strings.ReplaceAll(m.Content, "\n", " "), contentBudget))

	prefix := "  "
	style := itemNormal
	if selected {
		prefix = "▸ "
		style = itemSelected
	}

	line1 := lipgloss.JoinHorizontal(lipgloss.Top, icon, "  ", title)
	block := style.Render(prefix + line1 + "\n    " + preview)
	return strings.Split(block, "\n")
}

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
	err      error

	// Filtro
	filterInput textinput.Model
	filtering   bool
	filtered    []domain.Memory // memorias tras el filtro activo
	listCursor  int             // índice seleccionado dentro de filtered

	selected     domain.Memory
	detailScroll int
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

	stats        ports.StorageStats
	maintCursor  int
	maintAction  string // "purge", "gc" o "consolidate" (feature 020, T057)
	maintConfirm textinput.Model
	maintErr     string
	// consolidationPreview cachea la previsualización calculada al elegir
	// "Consolidar" en la pantalla de mantenimiento, para no recalcularla al
	// renderizar la confirmación.
	consolidationPreview usecases.ConsolidationReport

	configCursor int
	importPath   textinput.Model
	importErr    string

	// Documentos fijados (feature 021). docTemplates lo inyecta quien construye
	// la TUI: el contenido por defecto vive embebido en el binario, y esta capa
	// no debe leerlo por su cuenta.
	docIndex        int
	docPath         textinput.Model
	docErr          string
	docVista        string
	docPendiente    docAction
	docConfirmReset bool
	docTemplates    map[string]string

	// Reindexado del grafo externo (feature 016, US2).
	reindexInProgress bool

	// Edición de huella de contexto (feature 016, US3): una sola pantalla
	// (screenEditSetting) reutilizada por los 3 ajustes, parametrizada por
	// editSettingField — mismo molde de "un solo input a la vez" que screenImport.
	editSettingField editSettingField
	editSettingInput textinput.Model
	editSettingErr   string

	// Optimizar memorias (detección de duplicados) — ver detect_duplicates.go.
	dupGroups       []usecases.DuplicateGroup
	dupCursor       int             // cursor sobre la lista de grupos (screenOptimize)
	dupGroupIdx     int             // índice del grupo actualmente en detalle
	dupKeepIdx      int             // índice DENTRO del grupo marcado como canónico
	dupMemberCursor int             // cursor sobre miembros del grupo (screenOptimizeDetail)
	dupExclude      map[int64]bool  // IDs del grupo que el usuario excluyó del borrado
	dupConfirm      textinput.Model // input "si" para confirmar el borrado del grupo
	dupErr          string

	// Uso (feature 020): pantalla `screenUsage`, absorbe la spec 017.
	// sessionRepo/usageRepo/tokenCounter/compressor/specKitReader son
	// opcionales (nil-safe) — sin ellos la pantalla degrada mostrando lo que
	// puede. schemaTokens/schemaOperations del reporte quedan SIEMPRE en cero
	// aquí: medirlos exige levantar el servidor MCP real (measurePublishedSchemas
	// vive en adapters/primary/cli, que ya importa este paquete para lanzar la
	// TUI — importarlo de vuelta crearía un ciclo entre dos adaptadores
	// primarios). La sección [1] sigue coincidiendo cifra por cifra con
	// `mem usage` en todo lo demás: ambos llaman a la misma
	// usecases.BuildUsageReport con los mismos argumentos.
	sessionRepo   ports.SessionRepository
	usageRepo     ports.UsageRepository
	tokenCounter  ports.TokenCounter
	compressor    ports.Compressor
	specKitReader ports.SpecKitReader

	usageReport domain.UsageReport
	usageScope  string // "session" | "all" | "empty" — vocabulario de contracts/usage-report.md

	// Snapshot puntual (sección [2], FR-018..FR-025): efímero, no persiste
	// entre visitas a la pantalla (FR-023).
	usageTaskInput   textinput.Model
	usageBudgetInput textinput.Model
	usageFocus       int // 0=tarea, 1=presupuesto
	usageSnapshot    *domain.ContextPack
	usageSnapshotErr string

	width  int
	height int
	ready  bool
}

// UsageDeps son las dependencias de la pantalla de uso (feature 020),
// agrupadas aparte para no seguir alargando la firma posicional de Run — son
// opcionales en su totalidad: con el valor cero, la pantalla degrada
// mostrando lo que puede (nil-safe, mismo criterio que el resto del modelo).
type UsageDeps struct {
	SessionRepo   ports.SessionRepository
	UsageRepo     ports.UsageRepository
	TokenCounter  ports.TokenCounter
	Compressor    ports.Compressor
	SpecKitReader ports.SpecKitReader
}

func Run(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, settingsRepo ports.SettingsRepository, maintenanceRepo ports.MaintenanceRepository, codeProvider ports.CodeGraphProvider, root, project string, usageDeps UsageDeps) error {
	p := tea.NewProgram(initialModel(memRepo, relRepo, settingsRepo, maintenanceRepo, codeProvider, root, project, usageDeps))
	_, err := p.Run()
	return err
}

func initialModel(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, settingsRepo ports.SettingsRepository, maintenanceRepo ports.MaintenanceRepository, codeProvider ports.CodeGraphProvider, root, project string, usageDeps UsageDeps) model {
	mems, _ := memRepo.List(project, 200)

	ti := textinput.New()
	ti.Placeholder = "Título (opcional)"
	ti.CharLimit = 120
	ti.SetWidth(50)

	ty := textinput.New()
	ty.Placeholder = "learning, decision, architecture, bugfix, pattern, discovery, preference"
	ty.CharLimit = 20
	ty.SetWidth(50)
	ty.SetValue("learning")

	tc := textinput.New()
	tc.Placeholder = "¿Qué aprendiste o decidiste?"
	tc.CharLimit = 500
	tc.SetWidth(50)
	tc.Focus()

	tf := textinput.New()
	tf.Placeholder = "Archivo relacionado (opcional)"
	tf.CharLimit = 200
	tf.SetWidth(50)

	mc := textinput.New()
	mc.Placeholder = "nombre del proyecto"
	mc.CharLimit = 200
	mc.SetWidth(50)

	ip := textinput.New()
	ip.Placeholder = "ruta al archivo .json a importar"
	ip.CharLimit = 400
	ip.SetWidth(50)

	dp := textinput.New()
	dp.Placeholder = "/ruta/al/documento.md"
	dp.CharLimit = 400
	dp.SetWidth(60)

	dc := textinput.New()
	dc.Placeholder = `escribe "si"`
	dc.CharLimit = 10
	dc.SetWidth(20)

	fi := textinput.New()
	fi.Placeholder = "buscar..."
	fi.CharLimit = 80
	fi.SetWidth(40)

	es := textinput.New()
	es.Placeholder = "valor entero"
	es.CharLimit = 20
	es.SetWidth(20)

	ut := textinput.New()
	ut.Placeholder = "descripción de la tarea"
	ut.CharLimit = 200
	ut.SetWidth(50)

	ub := textinput.New()
	ub.Placeholder = "presupuesto en tokens, p. ej. 4000"
	ub.CharLimit = 10
	ub.SetWidth(20)

	settings := settingsRepo.Read(root)

	var stats ports.StorageStats
	if maintenanceRepo != nil {
		stats, _ = maintenanceRepo.Stats(project)
	}

	return model{
		memRepo:          memRepo,
		relRepo:          relRepo,
		settingsRepo:     settingsRepo,
		maintenanceRepo:  maintenanceRepo,
		codeProvider:     codeProvider,
		root:             root,
		project:          project,
		screen:           screenList,
		memories:         mems,
		filtered:         mems,
		filterInput:      fi,
		autoApprove:      settings.AutoApprove,
		saveTitle:        ti,
		saveType:         ty,
		saveContent:      tc,
		saveFilepath:     tf,
		stats:            stats,
		maintConfirm:     mc,
		importPath:       ip,
		docPath:          dp,
		docTemplates:     DocTemplates,
		editSettingInput: es,
		dupConfirm:       dc,
		dupExclude:       make(map[int64]bool),

		sessionRepo:      usageDeps.SessionRepo,
		usageRepo:        usageDeps.UsageRepo,
		tokenCounter:     usageDeps.TokenCounter,
		compressor:       usageDeps.Compressor,
		specKitReader:    usageDeps.SpecKitReader,
		usageTaskInput:   ut,
		usageBudgetInput: ub,
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

	case externalReindexDoneMsg:
		m.reindexInProgress = false
		switch {
		case msg.err == nil:
			m.statusMsg = fmt.Sprintf("Grafo externo reindexado: %d nodos, %d aristas", msg.nodes, msg.edges)
		case errors.Is(msg.err, ports.ErrIndexerNotInstalled):
			m.statusMsg = "codebase-memory-mcp no disponible"
		default:
			m.statusMsg = fmt.Sprintf("⚠️ grafo externo: %v", msg.err)
		}
		m.statusTimer = 80
		return m, nil

	case tea.KeyMsg:
		if m.statusTimer > 0 {
			m.statusTimer--
		}
		if msg.String() == "ctrl+y" {
			content := m.copyableText()
			if content == "" {
				return m, nil
			}
			m.statusMsg = "Contenido copiado al portapapeles"
			m.statusTimer = 40
			return m, tea.SetClipboard(content)
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
		if m.screen == screenDocs {
			return m.updateDocs(msg)
		}
		if m.screen == screenImport {
			return m.updateImport(msg)
		}
		if m.screen == screenOptimize {
			return m.updateOptimize(msg)
		}
		if m.screen == screenOptimizeDetail {
			return m.updateOptimizeDetail(msg)
		}
		if m.screen == screenOptimizeConfirm {
			return m.updateOptimizeConfirm(msg)
		}
		if m.screen == screenOptimizeAllConfirm {
			return m.updateOptimizeAllConfirm(msg)
		}
		if m.screen == screenEditSetting {
			return m.updateEditSetting(msg)
		}
		if m.screen == screenUsage {
			return m.updateUsage(msg)
		}
		return m.updateList(msg)
	}
	if next, cmd, ok := m.updateFocusedInput(msg); ok {
		return next, cmd
	}

	return m, nil
}

// updateFocusedInput entrega al input activo los mensajes que no son de
// teclado: el pegado bracketed del terminal (tea.PasteMsg) y la respuesta
// asíncrona del portapapeles que dispara ctrl+v.
//
// El destino se decide por PANTALLA, igual que el enrutado de teclas. Elegirlo
// por Focused() recorriendo todo el modelo era incorrecto: saveContent se
// enfoca al construir el modelo y nunca se desenfoca al salir de la pantalla
// de guardar, así que se tragaba el pegado de todas las demás cajas (ruta de
// import, ajustes de IA, documentos), que quedaban sin poder pegar.
//
// Dentro de una pantalla con varios inputs se le pasa el mensaje a todos: el
// textinput ignora lo que recibe mientras está desenfocado.
func (m model) updateFocusedInput(msg tea.Msg) (model, tea.Cmd, bool) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch m.screen {
	case screenList:
		if !m.filtering {
			return m, nil, false
		}
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)
		m.applyFilter()

	case screenSave:
		m.saveTitle, cmd = m.saveTitle.Update(msg)
		cmds = append(cmds, cmd)
		m.saveType, cmd = m.saveType.Update(msg)
		cmds = append(cmds, cmd)
		m.saveContent, cmd = m.saveContent.Update(msg)
		cmds = append(cmds, cmd)
		m.saveFilepath, cmd = m.saveFilepath.Update(msg)
		cmds = append(cmds, cmd)

	case screenMaintenanceConfirm:
		m.maintConfirm, cmd = m.maintConfirm.Update(msg)
		cmds = append(cmds, cmd)

	case screenImport:
		m.importPath, cmd = m.importPath.Update(msg)
		cmds = append(cmds, cmd)

	case screenDocs:
		// La caja de ruta solo existe mientras hay una acción pendiente
		// (exportar/importar); fuera de eso la pantalla no tiene entrada.
		if m.docPendiente == docActionNinguna {
			return m, nil, false
		}
		m.docPath, cmd = m.docPath.Update(msg)
		cmds = append(cmds, cmd)

	case screenEditSetting:
		m.editSettingInput, cmd = m.editSettingInput.Update(msg)
		cmds = append(cmds, cmd)

	case screenOptimizeConfirm, screenOptimizeAllConfirm:
		m.dupConfirm, cmd = m.dupConfirm.Update(msg)
		cmds = append(cmds, cmd)

	case screenUsage:
		m.usageTaskInput, cmd = m.usageTaskInput.Update(msg)
		cmds = append(cmds, cmd)
		m.usageBudgetInput, cmd = m.usageBudgetInput.Update(msg)
		cmds = append(cmds, cmd)

	default:
		return m, nil, false
	}

	return m, tea.Batch(cmds...), true
}

// ─── List screen ───────────────────────────────────────────────────

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Si el filtro está activo, las teclas van al textinput
	if m.filtering {
		switch msg.String() {
		case "esc":
			m.filtering = false
			m.filterInput.Blur()
			m.filterInput.SetValue("")
			m.applyFilter()
			return m, nil
		case "enter":
			m.filtering = false
			m.filterInput.Blur()
			m.applyFilter()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		m.filterInput.SetValue("")
		return m, textinput.Blink

	case "j", "down":
		if m.listCursor < len(m.filtered)-1 {
			m.listCursor++
		}

	case "k", "up":
		if m.listCursor > 0 {
			m.listCursor--
		}

	case "enter":
		if m.listCursor >= 0 && m.listCursor < len(m.filtered) {
			m.selected = m.filtered[m.listCursor]
			m.detailScroll = 0
			m.screen = screenDetail
		}

	case "s":
		if m.ready {
			m.screen = screenSave
			// updateFocus, y no Focus() suelto: deja saveFocus y el foco
			// real de las cuatro cajas de acuerdo. Enfocar solo contenido
			// dejaba dos cajas activas a la vez si se volvía a entrar tras
			// haber tabulado, y el teclado escribía en ambas.
			m.saveFocus = 2
			m.updateFocus()
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

	case "o":
		if m.ready {
			m.screen = screenOptimize
			m.dupCursor = 0
			m.dupErr = ""
			m.dupGroups, _ = usecases.DetectProjectDuplicates(m.memRepo, m.project)
		}

	case "u":
		if m.ready {
			m.screen = screenUsage
			m.usageReport, m.usageScope = m.buildUsageReport()
			m.usageSnapshot = nil
			m.usageSnapshotErr = ""
			m.usageTaskInput.SetValue("")
			m.usageBudgetInput.SetValue("")
			m.usageFocus = 0
			m.updateUsageFocus()
		}
	}

	return m, nil
}

// applyFilter filtra las memorias según el texto del input y reacota
// listCursor al nuevo rango — se llama tanto al escribir en el filtro como
// tras cualquier operación que cambie m.memories (guardar, purgar, importar,
// optimizar), para que la lista nunca quede desincronizada.
func (m *model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.filtered = m.memories
	} else {
		m.filtered = nil
		for _, mem := range m.memories {
			if strings.Contains(strings.ToLower(mem.Title), query) ||
				strings.Contains(strings.ToLower(mem.Content), query) ||
				strings.Contains(strings.ToLower(string(mem.Type)), query) {
				m.filtered = append(m.filtered, mem)
			}
		}
	}
	if m.listCursor >= len(m.filtered) {
		m.listCursor = len(m.filtered) - 1
	}
	if m.listCursor < 0 {
		m.listCursor = 0
	}
}

// ─── Detail screen ─────────────────────────────────────────────────

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.screen = screenList
	case "j", "down":
		if m.detailScroll < m.detailLastLine() {
			m.detailScroll++
		}
	case "k", "up":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	case "pgdown", "ctrl+f":
		m.detailScroll += max(1, m.detailBodyBudget()-2)
		if m.detailScroll > m.detailLastLine() {
			m.detailScroll = m.detailLastLine()
		}
	case "pgup", "ctrl+b":
		m.detailScroll -= max(1, m.detailBodyBudget()-2)
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
	case "home", "g":
		m.detailScroll = 0
	case "end", "G":
		m.detailScroll = m.detailLastLine()
	}
	return m, nil
}

func (m model) detailContentLines() []string {
	width := m.width - 14
	if width < 20 {
		width = 20
	}
	return strings.Split(ansi.Wrap(m.selected.Content, width, ""), "\n")
}

func (m model) detailBodyBudget() int {
	budget := m.height - 13
	if budget < 3 {
		return 3
	}
	return budget
}

func (m model) detailLastLine() int {
	return max(0, len(m.detailContentLines())-1)
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

		case 3: // Consolidar (feature 020, fase B): previsualiza primero,
			// como exige FR-027 — la operación es irreversible.
			preview, err := usecases.ConsolidateMemories(m.memRepo, m.project, false)
			if err != nil {
				m.statusMsg = fmt.Sprintf("Error al previsualizar consolidación: %v", err)
				m.statusTimer = 30
				return m, nil
			}
			if len(preview.Groups) == 0 {
				m.statusMsg = "No hay memorias consolidables (ningún grupo por topic_key ni por actividad duplicada)"
				m.statusTimer = 30
				return m, nil
			}
			m.consolidationPreview = preview
			m.maintAction = "consolidate"
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

		if m.maintAction == "consolidate" {
			report, err := usecases.ConsolidateMemories(m.memRepo, m.project, true)
			if err != nil {
				m.statusMsg = fmt.Sprintf("Error al consolidar: %v", err)
			} else {
				m.statusMsg = fmt.Sprintf("Consolidación: %d grupo(s), %d fila(s) fundida(s)", len(report.Groups), report.DeletedCount)
				m.memories, _ = m.memRepo.List(m.project, 200)
				m.applyFilter()
				m.stats, _ = m.maintenanceRepo.Stats(m.project)
			}
			m.statusTimer = 30
			m.maintErr = ""
			m.screen = screenMaintenance
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
			m.applyFilter()
			m.stats, _ = m.maintenanceRepo.Stats(m.project)
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

// configRowAtomicPlan es la fila del interruptor de planificación atómica
// (feature 013). Nombrada en vez de un literal porque el test la referencia y
// un reordenamiento del menú debe romper la compilación, no el comportamiento
// en silencio.
const configRowAtomicPlan = 6

// configRowReindexGraph es la fila que dispara el reindexado del grafo
// externo (feature 016, US2) — equivalente en la TUI a `mem index`. Las filas
// nuevas se agregan siempre al FINAL del menú, nunca insertadas en medio, para
// no invalidar configRowAtomicPlan/configOptions ya referenciadas por nombre
// en los tests existentes.
const configRowReindexGraph = configRowAtomicPlan + 1

// configRowEditBudget/EditCompactThreshold/EditDedupDays son las 3 filas que
// abren screenEditSetting para editar la huella de contexto (feature 016, US3).
const (
	configRowEditBudget           = configRowReindexGraph + 1
	configRowEditCompactThreshold = configRowEditBudget + 1
	configRowEditDedupDays        = configRowEditCompactThreshold + 1
)

// configRowPlanGuard es la fila del interruptor de la exigencia de forma del
// plan (feature 019, Historia 1): PlanGuardDisabled. Añadida al final, como
// exige la convención de configRowReindexGraph/configRowAtomicPlan.
const configRowPlanGuard = configRowEditDedupDays + 1

// configRowDocsBase es la primera de las filas de documentos fijados (feature
// 021). Se GENERAN recorriendo domain.PinnedDocs en vez de enumerarse: añadir un
// documento al catálogo no debe exigir tocar la TUI. Van al final del menú, como
// exige la convención de configRowReindexGraph.
const configRowDocsBase = configRowPlanGuard + 1

// configRowOctopus es la fila del interruptor del módulo Octopus AAR (feature
// 027): el enrutador adaptativo que decide inline contra delegar. Va al FINAL
// del menú, después de las filas de documentos, como exige la convención de
// configRowReindexGraph: nunca insertada en medio.
//
// A diferencia del resto de interruptores, este es OPT-IN: nace apagado y
// enciende una capacidad completa, no refina uno de los flujos existentes.
var configRowOctopus = configRowDocsBase + len(domain.PinnedDocs)

// configOptions es el número de filas del menú de configuración.
var configOptions = configRowOctopus + 1

// externalReindexDoneMsg es el mensaje de resultado del primer tea.Cmd
// asíncrono real de esta TUI (feature 016, US2): IndexRepository puede tardar
// minutos, así que corre en background vía reindexExternalGraphCmd() y
// reporta su resultado por este mensaje en vez de bloquear Update().
type externalReindexDoneMsg struct {
	nodes, edges int
	err          error
}

// reindexExternalGraphCmd dispara el reindexado bloqueante del proveedor
// externo fuera del bucle de eventos de Bubble Tea. Si m.codeProvider no
// soporta ports.CodeGraphIndexer, resuelve de inmediato con el sentinel de
// "no instalado" — misma semántica que el CLI (indexExternalGraph).
func (m model) reindexExternalGraphCmd() tea.Cmd {
	return func() tea.Msg {
		indexer, ok := m.codeProvider.(ports.CodeGraphIndexer)
		if !ok {
			return externalReindexDoneMsg{err: ports.ErrIndexerNotInstalled}
		}
		nodes, edges, err := indexer.IndexRepository(context.Background(), "full")
		return externalReindexDoneMsg{nodes: nodes, edges: edges, err: err}
	}
}

// atomicPlanScope indica si la planificación atómica también está habilitada en
// el ámbito de usuario, para que la persona vea desde dónde le llega la
// funcionalidad (feature 013, FR-033). Vacío si solo aplica a este proyecto.
func atomicPlanScope() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	globales := []string{
		filepath.Join(home, ".claude", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(home, ".config", "opencode", "commands", "atomic-decomposition.md"),
	}
	for _, p := range globales {
		if _, err := os.Stat(p); err == nil {
			return "  (también en ámbito global)"
		}
	}
	return ""
}

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
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

		case 4: // Toggle sinapsis automática
			s := m.settingsRepo.Read(m.root)
			s.SynapseDisabled = !s.SynapseDisabled
			m.settingsRepo.Write(m.root, s)
			if s.SynapseDisabled {
				m.statusMsg = "Sinapsis desactivada (ahorra 1-3 queries por save)"
			} else {
				m.statusMsg = "Sinapsis activada (relaciona memorias de la misma sesión)"
			}
			m.statusTimer = 40

		case 5: // Toggle brazo extensor spec-kit
			s := m.settingsRepo.Read(m.root)
			s.SpeckitContextDisabled = !s.SpeckitContextDisabled
			m.settingsRepo.Write(m.root, s)
			if s.SpeckitContextDisabled {
				m.statusMsg = "Brazo extensor spec-kit desactivado"
			} else {
				m.statusMsg = "Brazo extensor spec-kit activado"
			}
			m.statusTimer = 40

		case configRowAtomicPlan: // Toggle planificación atómica en modo plan
			s := m.settingsRepo.Read(m.root)
			s.AtomicPlanDisabled = !s.AtomicPlanDisabled
			m.settingsRepo.Write(m.root, s)
			if s.AtomicPlanDisabled {
				m.statusMsg = "Planificación atómica desactivada en este proyecto"
			} else {
				m.statusMsg = "Planificación atómica activada (el agente la carga al entrar en modo plan)"
			}
			m.statusTimer = 40

		case configRowPlanGuard: // Toggle exigencia de forma del plan (feature 019)
			s := m.settingsRepo.Read(m.root)
			s.PlanGuardDisabled = !s.PlanGuardDisabled
			m.settingsRepo.Write(m.root, s)
			if s.PlanGuardDisabled {
				m.statusMsg = "Exigencia de forma del plan desactivada (todo plan se permite)"
			} else {
				m.statusMsg = "Exigencia de forma del plan activada (un plan sin árbol se devuelve)"
			}
			m.statusTimer = 40

		case configRowOctopus: // Toggle del módulo Octopus AAR (feature 027)
			s := m.settingsRepo.Read(m.root)
			s.OctopusEnabled = !s.OctopusEnabled
			// La lista de auto-aprobación se recalcula con el módulo ya en su
			// nuevo estado: si no, encender Octopus registraría sus tools pero
			// las dejaría pidiendo permiso una por una, y apagarlo dejaría
			// nombres muertos pre-aprobados.
			s.AutoApproveTools = domain.MCPAutoApprovableToolsFor(s.OctopusEnabled)
			m.settingsRepo.Write(m.root, s)
			// Persistir SettingsData no actualiza por sí mismo las configuraciones
			// de los clientes MCP ya instalados. Igual que los toggles vecinos,
			// aplicar la lista evita que el estado efectivo quede desfasado.
			m.settingsRepo.ApplyAutoApprove(m.root, s)
			if s.OctopusEnabled {
				m.statusMsg = "Octopus AAR activado (enrutador adaptativo: decide inline o delegar)"
			} else {
				m.statusMsg = "Octopus AAR desactivado (sin huella: ni tools MCP ni telemetría)"
			}
			m.statusTimer = 40

		case configRowReindexGraph: // Reindexar grafo externo (feature 016, US2)
			if _, ok := m.codeProvider.(ports.CodeGraphIndexer); !ok {
				m.statusMsg = "codebase-memory-mcp no disponible"
				m.statusTimer = 40
			} else if m.reindexInProgress {
				m.statusMsg = "Ya hay un reindexado del grafo externo en curso"
				m.statusTimer = 40
			} else {
				m.reindexInProgress = true
				m.statusMsg = "🔗 Indexando grafo externo... (puede tardar, no bloquea la TUI)"
				m.statusTimer = 999
				cmd = m.reindexExternalGraphCmd()
			}

		case configRowEditBudget:
			s := m.settingsRepo.Read(m.root)
			m.editSettingField = editFieldBudget
			m.editSettingInput.SetValue(strconv.Itoa(s.Budget))
			m.editSettingInput.Focus()
			m.editSettingErr = ""
			m.screen = screenEditSetting

		case configRowEditCompactThreshold:
			s := m.settingsRepo.Read(m.root)
			m.editSettingField = editFieldCompactThreshold
			m.editSettingInput.SetValue(strconv.Itoa(s.CompactThreshold))
			m.editSettingInput.Focus()
			m.editSettingErr = ""
			m.screen = screenEditSetting

		case configRowEditDedupDays:
			s := m.settingsRepo.Read(m.root)
			m.editSettingField = editFieldDedupDays
			m.editSettingInput.SetValue(strconv.Itoa(s.DedupWindowDays))
			m.editSettingInput.Focus()
			m.editSettingErr = ""
			m.screen = screenEditSetting

		default:
			// Filas de documentos fijados (feature 021): se resuelven por
			// posición dentro del catálogo, no con un case por documento.
			// Añadir uno nuevo al catálogo no debe tocar este switch.
			if i := m.configCursor - configRowDocsBase; i >= 0 && i < len(domain.PinnedDocs) {
				m.docIndex = i
				m.docVista = ""
				m.docErr = ""
				m.docPendiente = docActionNinguna
				m.docConfirmReset = false
				m.screen = screenDocs
			}
		}
	}
	return m, cmd
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

// ─── Edit setting screen (feature 016, US3) ──────────────────────────

// editSettingLabel devuelve el nombre visible del ajuste que edita
// screenEditSetting, usado en el título y en el mensaje de confirmación.
func editSettingLabel(f editSettingField) string {
	switch f {
	case editFieldBudget:
		return "Presupuesto get_context"
	case editFieldCompactThreshold:
		return "Umbral recordatorio compactación"
	case editFieldDedupDays:
		return "Ventana dedup por identidad"
	default:
		return ""
	}
}

// updateEditSetting maneja screenEditSetting: Esc vuelve a Configuración sin
// guardar; Enter valida con strconv.Atoi (vacío/no numérico/decimal → error,
// permanece en la pantalla) y, si es válido, persiste el entero TAL CUAL
// (positivo, cero o negativo — la normalización 0→default/negativo→opt-out ya
// vive en ReadSettings, no aquí) vía settingsRepo.Write.
func (m model) updateEditSetting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenConfig
		m.editSettingErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		raw := strings.TrimSpace(m.editSettingInput.Value())
		val, err := strconv.Atoi(raw)
		if err != nil {
			m.editSettingErr = "Debe ser un número entero"
			return m, nil
		}
		s := m.settingsRepo.Read(m.root)
		switch m.editSettingField {
		case editFieldBudget:
			s.Budget = val
		case editFieldCompactThreshold:
			s.CompactThreshold = val
		case editFieldDedupDays:
			s.DedupWindowDays = val
		}
		m.settingsRepo.Write(m.root, s)
		m.statusMsg = fmt.Sprintf("%s actualizado: %d", editSettingLabel(m.editSettingField), val)
		m.statusTimer = 40
		m.editSettingErr = ""
		m.screen = screenConfig
		return m, nil
	}

	var cmd tea.Cmd
	m.editSettingInput, cmd = m.editSettingInput.Update(msg)
	return m, cmd
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
		m.applyFilter()
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

// ─── Optimizar memorias (detección de duplicados) ───────────────────

// updateOptimize maneja la lista de grupos candidatos a duplicado
// (usecases.DetectProjectDuplicates, calculada al entrar a la pantalla vía
// la tecla "o" en updateList).
func (m model) updateOptimize(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenList
		m.dupErr = ""

	case "j", "down":
		if m.dupCursor < len(m.dupGroups)-1 {
			m.dupCursor++
		}

	case "k", "up":
		if m.dupCursor > 0 {
			m.dupCursor--
		}

	case "enter":
		if len(m.dupGroups) == 0 || m.dupCursor >= len(m.dupGroups) {
			return m, nil
		}
		m.dupGroupIdx = m.dupCursor
		m.dupMemberCursor = 0
		m.dupExclude = make(map[int64]bool)
		group := m.dupGroups[m.dupGroupIdx]
		for i, mem := range group.Memories {
			if mem.ID == group.SuggestedKeepID {
				m.dupKeepIdx = i
			}
		}
		m.screen = screenOptimizeDetail

	case "a":
		if len(m.dupGroups) == 0 {
			return m, nil
		}
		m.dupErr = ""
		m.dupConfirm.SetValue("")
		m.dupConfirm.Focus()
		m.screen = screenOptimizeAllConfirm
	}
	return m, nil
}

// updateOptimizeDetail muestra el contenido completo de cada memoria del
// grupo y deja elegir cuál se conserva (dupKeepIdx) y cuáles, además de la
// canónica, se excluyen del borrado (dupExclude) — por si el grupo junta una
// memoria que en realidad es un tema distinto y conviene conservar las dos.
func (m model) updateOptimizeDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	group := m.dupGroups[m.dupGroupIdx]

	switch msg.String() {
	case "esc":
		m.screen = screenOptimize

	case "j", "down":
		if m.dupMemberCursor < len(group.Memories)-1 {
			m.dupMemberCursor++
		}

	case "k", "up":
		if m.dupMemberCursor > 0 {
			m.dupMemberCursor--
		}

	case "enter":
		m.dupKeepIdx = m.dupMemberCursor

	case "space":
		id := group.Memories[m.dupMemberCursor].ID
		if m.dupMemberCursor != m.dupKeepIdx {
			m.dupExclude[id] = !m.dupExclude[id]
		}

	case "c":
		if m.deletionCandidates(group) == 0 {
			m.dupErr = "No hay nada para borrar: todo el grupo quedó excluido"
			return m, nil
		}
		m.dupErr = ""
		m.dupConfirm.SetValue("")
		m.dupConfirm.Focus()
		m.screen = screenOptimizeConfirm
	}
	return m, nil
}

// deletionCandidates cuenta cuántas memorias del grupo se borrarían: todas
// menos la canónica (dupKeepIdx) y las excluidas explícitamente por el
// usuario (dupExclude).
func (m model) deletionCandidates(group usecases.DuplicateGroup) int {
	n := 0
	for i, mem := range group.Memories {
		if i == m.dupKeepIdx || m.dupExclude[mem.ID] {
			continue
		}
		n++
	}
	return n
}

func (m model) updateOptimizeConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenOptimizeDetail
		m.dupErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		typed := strings.TrimSpace(m.dupConfirm.Value())
		if !strings.EqualFold(typed, "si") {
			m.dupErr = `Escribe "si" para confirmar. No se eliminó nada.`
			return m, nil
		}

		group := m.dupGroups[m.dupGroupIdx]
		deleted := 0
		for i, mem := range group.Memories {
			if i == m.dupKeepIdx || m.dupExclude[mem.ID] {
				continue
			}
			if ok, err := m.memRepo.Delete(m.project, mem.ID); err == nil && ok {
				deleted++
			}
		}

		m.statusMsg = fmt.Sprintf("Grupo optimizado: %d memoria(s) eliminada(s), se conservó #%d", deleted, group.Memories[m.dupKeepIdx].ID)
		m.statusTimer = 60
		m.memories, _ = m.memRepo.List(m.project, 200)
		m.applyFilter()
		if m.maintenanceRepo != nil {
			m.stats, _ = m.maintenanceRepo.Stats(m.project)
		}

		m.dupGroups = append(m.dupGroups[:m.dupGroupIdx], m.dupGroups[m.dupGroupIdx+1:]...)
		if m.dupCursor >= len(m.dupGroups) && m.dupCursor > 0 {
			m.dupCursor--
		}
		m.dupErr = ""
		m.screen = screenOptimize
		return m, nil
	}

	var cmd tea.Cmd
	m.dupConfirm, cmd = m.dupConfirm.Update(msg)
	return m, cmd
}

// totalDeletionCandidates cuenta cuántas memorias se borrarían en total si se
// compactan TODOS los grupos de una vez, usando la sugerencia automática
// (SuggestedKeepID) como canónica de cada grupo — sin revisión manual grupo
// por grupo.
func (m model) totalDeletionCandidates() int {
	n := 0
	for _, g := range m.dupGroups {
		for _, mem := range g.Memories {
			if mem.ID != g.SuggestedKeepID {
				n++
			}
		}
	}
	return n
}

// updateOptimizeAllConfirm aplica la compactación masiva: borra, en todos los
// grupos detectados, todo lo que no sea la memoria sugerida como canónica.
// Es la vía rápida para mantenimiento cuando hay muchos grupos y revisar uno
// por uno no es práctico.
func (m model) updateOptimizeAllConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenOptimize
		m.dupErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		typed := strings.TrimSpace(m.dupConfirm.Value())
		if !strings.EqualFold(typed, "si") {
			m.dupErr = `Escribe "si" para confirmar. No se eliminó nada.`
			return m, nil
		}

		deleted := 0
		for _, g := range m.dupGroups {
			for _, mem := range g.Memories {
				if mem.ID == g.SuggestedKeepID {
					continue
				}
				if ok, err := m.memRepo.Delete(m.project, mem.ID); err == nil && ok {
					deleted++
				}
			}
		}

		m.statusMsg = fmt.Sprintf("Compactación completa: %d memoria(s) eliminada(s) en %d grupo(s)", deleted, len(m.dupGroups))
		m.statusTimer = 60
		m.memories, _ = m.memRepo.List(m.project, 200)
		m.applyFilter()
		if m.maintenanceRepo != nil {
			m.stats, _ = m.maintenanceRepo.Stats(m.project)
		}

		m.dupGroups = nil
		m.dupCursor = 0
		m.dupErr = ""
		m.screen = screenOptimize
		return m, nil
	}

	var cmd tea.Cmd
	m.dupConfirm, cmd = m.dupConfirm.Update(msg)
	return m, cmd
}

func (m model) optimizeAllConfirmView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Compactar todas las memorias"))
	b.WriteString("\n")
	b.WriteString(dangerStyle.Render(
		fmt.Sprintf("Se aplicará la sugerencia automática en los %d grupo(s) detectados: se eliminarán %d memoria(s) en total.",
			len(m.dupGroups), m.totalDeletionCandidates()),
	))
	b.WriteString("\n\n")
	b.WriteString(formLabel.Render(`Escribe "si" para confirmar:`))
	b.WriteString("\n")
	b.WriteString(m.dupConfirm.View())
	b.WriteString("\n\n")

	if m.dupErr != "" {
		b.WriteString(errorStyle.Render("✕ " + m.dupErr))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  enter confirmar  ·  esc cancelar"))
	return appStyle.Render(b.String())
}

func (m model) optimizeView() string {
	var head strings.Builder

	head.WriteString(titleStyle.Render("Optimizar memorias"))
	head.WriteString("\n")
	head.WriteString(subtitleStyle.Render(fmt.Sprintf("%s · candidatos a duplicado por similitud de contenido", m.project)))
	head.WriteString("\n\n")

	var foot strings.Builder
	foot.WriteString("\n")
	if status := m.statusLine(); status != "" {
		foot.WriteString(status)
		foot.WriteString("\n")
	}
	foot.WriteString(helpStyle.Render("  ↑↓ navegar  ·  enter revisar grupo  ·  a compactar todas  ·  esc volver"))

	var bodyLines []string
	if len(m.dupGroups) == 0 {
		bodyLines = append(bodyLines, lipgloss.NewStyle().Foreground(faint).Render("  No se detectaron duplicados."))
	} else {
		for i, g := range m.dupGroups {
			label := fmt.Sprintf("[%s] %d memorias — %s", typeLabel(string(g.Type)), len(g.Memories), truncate(groupPreview(g), 60))
			if i == m.dupCursor {
				bodyLines = append(bodyLines, itemSelected.Render("▸ "+label))
			} else {
				bodyLines = append(bodyLines, itemNormal.Render("  "+label))
			}
		}
	}

	return appStyle.Render(head.String() + windowLines(bodyLines, m.dupCursor, m.bodyBudget(head.String(), foot.String())) + foot.String())
}

// groupPreview arma un resumen corto de los títulos del grupo para la lista,
// para distinguir grupos sin tener que entrar a cada uno.
func groupPreview(g usecases.DuplicateGroup) string {
	titles := make([]string, 0, len(g.Memories))
	for _, mem := range g.Memories {
		if mem.Title != "" {
			titles = append(titles, mem.Title)
		}
	}
	return strings.Join(titles, " / ")
}

func (m model) optimizeDetailView() string {
	group := m.dupGroups[m.dupGroupIdx]

	var head strings.Builder
	head.WriteString(backHint())
	head.WriteString("\n\n")
	head.WriteString(subtitleStyle.Render(fmt.Sprintf("%s · %d memorias en este grupo", typeLabel(string(group.Type)), len(group.Memories))))
	head.WriteString("\n\n")

	var foot strings.Builder
	if m.dupErr != "" {
		foot.WriteString(errorStyle.Render("✕ " + m.dupErr))
		foot.WriteString("\n")
	}
	foot.WriteString(helpStyle.Render("  ↑↓ elegir memoria  ·  enter marcar canónica  ·  space conservar/borrar  ·  c confirmar borrado  ·  esc volver"))

	// El cuerpo (una caja bordeada por memoria) se arma como líneas
	// independientes para poder recortarlo a la altura visible, igual que en
	// listView — si no, con varias memorias de contenido largo el grupo no
	// cabe en la terminal y la caja seleccionada queda fuera de pantalla sin
	// forma de desplazarse hasta ella.
	var bodyLines []string
	cursorLine := 0
	for i, mem := range group.Memories {
		tag := "        "
		switch {
		case i == m.dupKeepIdx:
			tag = lipgloss.NewStyle().Foreground(green).Bold(true).Render("[CANÓNICA]")
		case m.dupExclude[mem.ID]:
			tag = lipgloss.NewStyle().Foreground(yellow).Render("[conservar]")
		default:
			tag = lipgloss.NewStyle().Foreground(red).Render("[se borra]")
		}

		header := fmt.Sprintf("#%d — %s", mem.ID, mem.CreatedAt)
		body := lipgloss.JoinVertical(lipgloss.Top,
			lipgloss.NewStyle().Bold(true).Foreground(highlight).Render(mem.Title),
			lipgloss.NewStyle().Foreground(faint).Render(header),
			"",
			truncate(mem.Content, 300),
		)

		border := detailBorder
		if i == m.dupMemberCursor {
			border = border.BorderForeground(highlight)
		} else {
			border = border.BorderForeground(faint)
		}

		block := strings.Split(tag+"\n"+border.Render(body), "\n")
		if i == m.dupMemberCursor {
			cursorLine = len(bodyLines) + len(block)/2
		}
		bodyLines = append(bodyLines, block...)
		bodyLines = append(bodyLines, "")
	}

	return appStyle.Render(head.String() + windowLines(bodyLines, cursorLine, m.bodyBudget(head.String(), foot.String())) + foot.String())
}

func (m model) optimizeConfirmView() string {
	group := m.dupGroups[m.dupGroupIdx]
	var b strings.Builder

	b.WriteString(titleStyle.Render("Confirmar optimización"))
	b.WriteString("\n")
	b.WriteString(dangerStyle.Render(
		fmt.Sprintf("Se eliminarán %d memoria(s) de este grupo; se conserva #%d.",
			m.deletionCandidates(group), group.Memories[m.dupKeepIdx].ID),
	))
	b.WriteString("\n\n")
	b.WriteString(formLabel.Render(`Escribe "si" para confirmar:`))
	b.WriteString("\n")
	b.WriteString(m.dupConfirm.View())
	b.WriteString("\n\n")

	if m.dupErr != "" {
		b.WriteString(errorStyle.Render("✕ " + m.dupErr))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  enter confirmar  ·  esc cancelar"))
	return appStyle.Render(b.String())
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
	m.applyFilter()
	m.saveTitle.SetValue("")
	m.saveType.SetValue("learning")
	m.saveContent.SetValue("")
	m.saveFilepath.SetValue("")
	m.saveFocus = 2
	m.updateFocus()
	return m, nil
}

// ─── View ──────────────────────────────────────────────────────────

// View satisface tea.Model. bubbletea v2 cambió la firma de string a
// tea.View (tea.WithAltScreen() ya no existe como ProgramOption; el alt
// screen ahora se declara por-view aquí).
func (m model) View() tea.View {
	v := tea.NewView(m.renderView())
	v.AltScreen = true
	return v
}

func (m model) renderView() string {
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
	case screenDocs:
		return m.docsView()
	case screenImport:
		return m.importView()
	case screenOptimize:
		return m.optimizeView()
	case screenOptimizeDetail:
		return m.optimizeDetailView()
	case screenOptimizeConfirm:
		return m.optimizeConfirmView()
	case screenOptimizeAllConfirm:
		return m.optimizeAllConfirmView()
	case screenEditSetting:
		return m.editSettingView()
	case screenUsage:
		return m.usageView()
	}
	return ""
}

func (m model) copyableText() string {
	switch m.screen {
	case screenDetail:
		mem := m.selected
		var b strings.Builder
		fmt.Fprintf(&b, "%s\nTipo: %s\nFecha: %s\n\n%s", memoryDisplayTitle(mem), mem.Type, mem.CreatedAt, mem.Content)
		if mem.Filepath != "" {
			fmt.Fprintf(&b, "\n\nArchivo: %s", mem.Filepath)
		}
		if mem.SessionID != "" {
			fmt.Fprintf(&b, "\nSesión: %s", mem.SessionID)
		}
		return b.String()
	case screenDocs:
		if m.docVista != "" {
			return m.docVista
		}
	}
	return strings.TrimSpace(ansi.Strip(m.renderView()))
}

func (m model) listView() string {
	// Header con título y info
	sizeInfo := ""
	if m.maintenanceRepo != nil {
		sizeInfo = fmt.Sprintf(" · %s en disco", humanize.Bytes(uint64(m.stats.FileSizeBytes)))
	}
	header := titleStyle.Render(fmt.Sprintf("%s · %d memorias%s", m.project, len(m.filtered), sizeInfo))

	// Input de filtro (visible solo cuando se está buscando)
	filterBar := ""
	if m.filtering {
		filterBar = formLabel.Render("buscar:") + " " + m.filterInput.View()
	} else {
		filterBar = lipgloss.NewStyle().Foreground(faint).Render("  / buscar")
	}

	// Footer
	footer := helpStyle.Render("  ↑↓ navegar  ·  / buscar  ·  enter detalle  ·  ctrl+y copiar  ·  s guardar  ·  c config  ·  m mantenimiento  ·  o optimizar  ·  u uso  ·  q salir")
	if status := m.statusLine(); status != "" {
		footer = status + "\n" + footer
	}

	// Cuerpo: lista compacta (2 líneas por memoria) enmarcada, o mensaje de
	// estado vacío si no hay filas que mostrar.
	head := header + "\n" + filterBar + "\n"
	var inner string
	switch {
	case len(m.filtered) > 0:
		bodyLines, cursorLine := m.listBodyLines()
		inner = windowLines(bodyLines, cursorLine, m.listBodyBudget(head, footer))
	case strings.TrimSpace(m.filterInput.Value()) != "":
		inner = itemNormal.Foreground(faint).Render(fmt.Sprintf("Sin resultados para «%s»", m.filterInput.Value()))
	default:
		inner = itemNormal.Foreground(faint).Render("Sin memorias en este proyecto")
	}
	body := listBorder.Render(inner)

	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Top, header, filterBar, "", body, footer))
}

// listBodyLines arma el cuerpo de la lista principal como líneas
// independientes (2 por memoria: tipo+título, y vista previa del contenido),
// listas para recortarse a la altura visible con windowLines — mismo patrón
// que optimizeView/optimizeDetailView.
func (m model) listBodyLines() ([]string, int) {
	// appStyle (4) + borde de listBorder (2) + su padding horizontal (2).
	innerWidth := m.width - 8
	if innerWidth < 30 {
		innerWidth = 30
	}

	var lines []string
	cursorLine := 0
	for i, mem := range m.filtered {
		if i == m.listCursor {
			cursorLine = len(lines)
		}
		lines = append(lines, listRowLines(mem, i == m.listCursor, innerWidth)...)
		lines = append(lines, "")
	}
	if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines, cursorLine
}

// listBodyBudget es bodyBudget descontando además las 2 líneas del marco
// (listBorder) que envuelve la lista.
func (m model) listBodyBudget(head, foot string) int {
	b := m.bodyBudget(head, foot) - 2
	if b < 0 {
		return 0
	}
	return b
}

// bodyBudget calcula cuántas líneas quedan disponibles para el cuerpo,
// descontando el padding vertical de appStyle y lo que ya ocupan el
// encabezado y el pie. Usado por las pantallas de optimización.
func (m model) bodyBudget(head, foot string) int {
	if !m.ready || m.height <= 0 {
		return 0
	}
	const appStyleVerticalPadding = 2
	used := appStyleVerticalPadding + strings.Count(head, "\n") + strings.Count(foot, "\n") + 3
	budget := m.height - used
	if budget < 3 {
		return 0
	}
	return budget
}

// windowLines recorta líneas a `budget` de alto, centrando la ventana en
// `cursorLine`. Usado por las pantallas de optimización.
func windowLines(lines []string, cursorLine, budget int) string {
	if budget <= 0 || len(lines) <= budget {
		return strings.Join(lines, "\n")
	}

	inner := budget - 2
	if inner < 1 {
		inner = 1
	}
	if cursorLine < 0 {
		cursorLine = 0
	}

	offset := cursorLine - inner/2
	maxOffset := len(lines) - inner
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + inner
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	if offset > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(faint).Render(fmt.Sprintf("  ↑ %d más arriba", offset)))
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(lines[offset:end], "\n"))
	if hidden := len(lines) - end; hidden > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(lipgloss.NewStyle().Foreground(faint).Render(fmt.Sprintf("  ↓ %d más abajo", hidden)))
	}
	return b.String()
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
	if status := m.statusLine(); status != "" {
		b.WriteString(status)
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("  ↑↓ navegar  ·  enter seleccionar  ·  esc volver"))
	return appStyle.Render(b.String())
}

func (m model) maintenanceConfirmView() string {
	var b strings.Builder

	actionLabel := "Purgar"
	switch m.maintAction {
	case "gc":
		actionLabel = "Garbage Collection"
	case "consolidate":
		actionLabel = "Consolidar"
	}

	b.WriteString(titleStyle.Render(actionLabel))
	b.WriteString("\n")
	if m.maintAction == "consolidate" {
		b.WriteString(dangerStyle.Render(fmt.Sprintf(
			"Esto fundirá %d grupo(s) en su fila más reciente y eliminará %d fila(s) redundante(s) del proyecto %q. Ningún contenido se pierde: se fusiona antes de eliminar.",
			len(m.consolidationPreview.Groups), m.consolidationPreview.DeletedCount, m.project,
		)))
	} else {
		b.WriteString(dangerStyle.Render(
			fmt.Sprintf("Esto eliminará memorias del proyecto %q permanentemente.", m.project),
		))
	}
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
	b.WriteString(sectionHeaderStyle.Render("  Grafo de código externo"))
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

	// Huella de contexto (feature 008): resumen de solo lectura; editable
	// desde el menú de abajo (feature 016, US3), sin salir de la TUI.
	b.WriteString(sectionHeaderStyle.Render("  Huella de contexto"))
	b.WriteString("\n")
	budgetLabel := fmt.Sprintf("%d caracteres", s.Budget)
	if s.Budget < 0 {
		budgetLabel = "sin límite"
	}
	threshLabel := fmt.Sprintf("%d caracteres", s.CompactThreshold)
	if s.CompactThreshold <= 0 {
		threshLabel = "desactivado"
	}
	dedupLabel := fmt.Sprintf("%d días", s.DedupWindowDays)
	if s.DedupWindowDays <= 0 {
		dedupLabel = "desactivado"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("    Presupuesto get_context: "+budgetLabel) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("    Umbral recordatorio compactación: "+threshLabel) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("    Ventana dedup por identidad: "+dedupLabel) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("    (editable desde el menú de abajo)") + "\n\n")

	// Label de reindexado condicionado a si el proveedor soporta la interfaz
	// (feature 016, US2) — si soporta la interfaz pero el binario no está
	// instalado, eso se resuelve en runtime vía ErrIndexerNotInstalled, misma
	// UX que el CLI (indexExternalGraph).
	reindexLabel := "Reindexar grafo externo: no disponible"
	if _, ok := m.codeProvider.(ports.CodeGraphIndexer); ok {
		reindexLabel = "Reindexar grafo externo (codebase-memory-mcp)"
	}
	if m.reindexInProgress {
		reindexLabel += " (en curso...)"
	}

	// Menú de acciones. Las filas nuevas se agregan SIEMPRE al final — ver
	// nota en configRowReindexGraph/configRowAtomicPlan.
	rows := []string{
		"Grafo de código externo: " + onOff(!s.CodeGraphDisabled),
		"Auto-approve MCP: " + onOff(s.AutoApprove),
		"Exportar memorias",
		"Importar memorias",
		"Sinapsis automática: " + onOff(!s.SynapseDisabled),
		"Brazo extensor spec-kit: " + onOff(!s.SpeckitContextDisabled),
		"Planificación atómica: " + onOff(!s.AtomicPlanDisabled) + atomicPlanScope(),
		reindexLabel,
		"Editar presupuesto get_context: " + budgetLabel,
		"Editar umbral recordatorio compactación: " + threshLabel,
		"Editar ventana dedup por identidad: " + dedupLabel,
		"Exigencia de forma del plan: " + onOff(!s.PlanGuardDisabled),
	}
	// Documentos fijados: una fila por entrada del catálogo, con su estado a la
	// vista para saber de un vistazo si el contenido es el del equipo o el que
	// trae la herramienta.
	for _, d := range domain.PinnedDocs {
		rows = append(rows, fmt.Sprintf("Actualizar %s: %s", d.Label, m.docEstado(d).State))
	}
	rows = append(rows, "Octopus AAR: "+onOff(s.OctopusEnabled))
	for i, label := range rows {
		if i == m.configCursor {
			b.WriteString(itemSelected.Render("▸ " + label))
		} else {
			b.WriteString(itemNormal.Render("  " + label))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if status := m.statusLine(); status != "" {
		b.WriteString(status)
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("  ↑↓ navegar  ·  enter activar/ejecutar  ·  ctrl+y copiar  ·  esc volver"))
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

// editSettingView es el molde de importView() (un solo input a la vez)
// reutilizado para los 3 ajustes de huella de contexto (feature 016, US3).
func (m model) editSettingView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Editar: " + editSettingLabel(m.editSettingField)))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Entero: positivo, 0 (valor por defecto) o negativo (desactiva el límite)"))
	b.WriteString("\n\n")
	b.WriteString(formLabel.Render("Nuevo valor:"))
	b.WriteString("\n")
	b.WriteString(m.editSettingInput.View())
	b.WriteString("\n\n")

	if m.editSettingErr != "" {
		b.WriteString(errorStyle.Render("✕ " + m.editSettingErr))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  enter guardar  ·  esc cancelar"))
	return appStyle.Render(b.String())
}

func (m model) detailView() string {
	mem := m.selected
	var b strings.Builder
	content := windowLines(m.detailContentLines(), m.detailScroll, m.detailBodyBudget())
	sessionID := mem.SessionID
	if len(sessionID) > 8 {
		sessionID = sessionID[:8]
	}

	b.WriteString(backHint())
	b.WriteString("\n\n")
	b.WriteString(detailBorder.Render(
		lipgloss.JoinVertical(lipgloss.Top,
			lipgloss.NewStyle().Bold(true).Foreground(highlight).Render(mem.Title),
			"",
			typeTag(string(mem.Type))+"  "+lipgloss.NewStyle().Foreground(faint).Render(mem.CreatedAt),
			"",
			content,
			func() string {
				if mem.Filepath != "" {
					return "\n" + lipgloss.NewStyle().Foreground(faint).Italic(true).Render("📁 "+mem.Filepath)
				}
				return ""
			}(),
			func() string {
				if mem.SessionID != "" {
					return "\n" + lipgloss.NewStyle().Foreground(faint).Render("Sesión: "+sessionID)
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
