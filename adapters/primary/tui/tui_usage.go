package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mem/application/usecases"
	"mem/domain"
)

// buildUsageReport resuelve el ámbito de la sesión (activa, o la más
// reciente con registros, o vacío) y arma el domain.UsageReport de la
// sección [1] — exactamente el mismo criterio y la misma
// usecases.BuildUsageReport que usa `mem usage` en la línea de comandos, para
// que ambos coincidan cifra por cifra (feature 020, SC-006).
func (m model) buildUsageReport() (domain.UsageReport, string) {
	if m.usageRepo == nil {
		return domain.UsageReport{Project: m.project}, "empty"
	}

	scope, sessionID := "empty", ""
	if m.sessionRepo != nil {
		if active, _ := m.sessionRepo.Active(m.project); active != nil {
			scope, sessionID = "session", active.ID
		}
	}
	if scope == "empty" {
		if sessions, err := m.usageRepo.Sessions(m.project, 1); err == nil && len(sessions) > 0 {
			scope, sessionID = "session", sessions[0]
		}
	}
	if scope == "empty" {
		return domain.UsageReport{Project: m.project}, "empty"
	}

	windowTokens := 0
	if m.settingsRepo != nil {
		windowTokens = m.settingsRepo.Read(m.root).UsageWindowTokens
	}

	report, err := usecases.BuildUsageReport(m.usageRepo, m.project, sessionID, windowTokens)
	if err != nil {
		return domain.UsageReport{Project: m.project}, "empty"
	}
	return report, scope
}

// updateUsageFocus mueve el foco de teclado entre los dos inputs del
// snapshot (mismo patrón que updateFocus para la pantalla de guardar).
func (m *model) updateUsageFocus() {
	m.usageTaskInput.Blur()
	m.usageBudgetInput.Blur()
	switch m.usageFocus {
	case 0:
		m.usageTaskInput.Focus()
	case 1:
		m.usageBudgetInput.Focus()
	}
}

func (m model) updateUsage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenList
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "tab", "shift+tab":
		if msg.String() == "tab" {
			m.usageFocus = (m.usageFocus + 1) % 2
		} else {
			m.usageFocus = (m.usageFocus - 1 + 2) % 2
		}
		m.updateUsageFocus()
		return m, nil

	case "enter":
		return m.computeUsageSnapshot()
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	m.usageTaskInput, cmd = m.usageTaskInput.Update(msg)
	cmds = append(cmds, cmd)
	m.usageBudgetInput, cmd = m.usageBudgetInput.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// computeUsageSnapshot valida la entrada (FR-021) y dispara el mismo proceso
// de optimización de contexto que `mem pack build` (FR-002 heredado de la
// spec 017): recuperación, deduplicación, clasificación por prioridad y
// compresión. Nunca persiste nada (FR-025) y el resultado no sobrevive a
// salir de la pantalla (FR-023, lo limpia la tecla `u` de updateList).
func (m model) computeUsageSnapshot() (tea.Model, tea.Cmd) {
	m.usageSnapshotErr = ""
	m.usageSnapshot = nil

	task := strings.TrimSpace(m.usageTaskInput.Value())
	if task == "" {
		m.usageSnapshotErr = "La tarea no puede estar vacía"
		return m, nil
	}
	budget, err := strconv.Atoi(strings.TrimSpace(m.usageBudgetInput.Value()))
	if err != nil || budget <= 0 {
		m.usageSnapshotErr = "El presupuesto debe ser un número entero positivo"
		return m, nil
	}
	if m.compressor == nil || m.tokenCounter == nil {
		m.usageSnapshotErr = "El snapshot no está disponible en este proyecto"
		return m, nil
	}

	pack, err := usecases.BuildContextPack(m.memRepo, m.compressor, m.tokenCounter, m.specKitReader, usecases.ContextRequest{
		Task:      task,
		Project:   m.project,
		MaxTokens: budget,
		Root:      m.root,
	})
	if err != nil {
		if err == domain.ErrCriticalContextOverflow {
			m.usageSnapshotErr = fmt.Sprintf("el contenido crítico para esta tarea excede el presupuesto de %d tokens — sube el presupuesto o acota la tarea", budget)
		} else {
			m.usageSnapshotErr = "no se pudo calcular el snapshot: " + err.Error()
		}
		return m, nil
	}
	m.usageSnapshot = &pack
	return m, nil
}

// usageSnapshotBlock renderiza domain.ContextStats en texto plano. No
// reutiliza cli.FormatContextStats: ese formateador vive en
// adapters/primary/cli, que ya importa este paquete (tui) para lanzar la
// interfaz interactiva — importarlo de vuelta crearía un ciclo entre dos
// adaptadores primarios. El motor SÍ se reutiliza sin duplicar
// (usecases.BuildContextPack); solo el texto final se repite, y es un bloque
// corto de siete líneas.
func usageSnapshotBlock(stats domain.ContextStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tokens antes:        %d\n", stats.RawTokens)
	fmt.Fprintf(&b, "Tokens después:      %d\n", stats.FinalTokens)
	fmt.Fprintf(&b, "Reducción:           %.2f%%\n", stats.CompressionRatio*100)
	fmt.Fprintf(&b, "Críticos / relevantes / opcionales:  %d / %d / %d\n", stats.ItemsCritical, stats.ItemsRelevant, stats.ItemsOptional)
	if stats.ItemsDuplicate > 0 {
		fmt.Fprintf(&b, "Duplicados removidos: %d\n", stats.ItemsDuplicate)
	}
	if stats.ItemsDiscarded > 0 {
		fmt.Fprintf(&b, "Descartados:         %d\n", stats.ItemsDiscarded)
	}
	return b.String()
}

// usageView renderiza la pantalla `u`: sección [1] (reporte de la sesión
// actual, mismos números que `mem usage`) y sección [2] (snapshot puntual,
// efímero).
func (m model) usageView() string {
	var b strings.Builder

	b.WriteString(backHint())
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("[1] Sesión actual"))
	b.WriteString("\n")
	b.WriteString(usecases.FormatUsageReport(m.usageReport, m.usageScope))

	b.WriteString(subtitleStyle.Render("[2] Snapshot puntual (no se conserva entre visitas)"))
	b.WriteString("\n")
	b.WriteString(formLabel.Render("Tarea:") + " " + m.usageTaskInput.View())
	b.WriteString("\n")
	b.WriteString(formLabel.Render("Presupuesto (tokens):") + " " + m.usageBudgetInput.View())
	b.WriteString("\n")

	switch {
	case m.usageSnapshotErr != "":
		b.WriteString(errorStyle.Render("✕ " + m.usageSnapshotErr))
		b.WriteString("\n\n")
	case m.usageSnapshot != nil:
		b.WriteString(usageSnapshotBlock(m.usageSnapshot.Stats))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  tab cambiar campo  ·  enter calcular  ·  esc volver"))
	return appStyle.Render(b.String())
}
