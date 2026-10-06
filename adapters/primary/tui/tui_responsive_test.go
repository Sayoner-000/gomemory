package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"mem/domain"
)

func TestListResponsive_RespetaAnchoYAlto(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {60, 18}, {80, 24}, {120, 40}, {200, 60}} {
		for _, state := range []string{"normal", "buscar", "eliminar"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				mems := manyMemories(200)
				mems[0].Type = domain.Architecture
				mems[0].Title = strings.Repeat("漢字 título 👩‍💻 ", 25)
				mems[0].Content = strings.Repeat("contenido largo ", 40)
				m := newTestModel(mems, size[1])
				m.project = strings.Repeat("proyecto", 20)
				updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				m = updated.(model)
				m.filtering = state == "buscar"
				m.deleteConfirm = state == "eliminar"
				m.deleteTarget = mems[0]
				out := m.renderView()
				if out != m.listView() {
					t.Fatal("la lista se volvió a recortar después de su ventaneo propio")
				}
				if lipgloss.Width(out) > size[0] || lipgloss.Height(out) > size[1] {
					t.Fatalf("vista %dx%d supera terminal %dx%d", lipgloss.Width(out), lipgloss.Height(out), size[0], size[1])
				}
			})
		}
	}
}

func TestViewportResponsive_ConservaSeleccionYPie(t *testing.T) {
	m := newTestModel(nil, 12)
	m.width = 40
	m.screen = screenConfig
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("opción %d", i)
	}
	lines[30] = "▸ opción seleccionada"
	content := strings.Join(lines, "\n") + "\n" + footerBoundary + strings.Repeat("─", 40) + "\nesc volver"
	got := m.fitTerminalView(content)
	if !strings.Contains(got, "opción seleccionada") || !strings.Contains(got, "esc volver") || lipgloss.Height(got) > m.height {
		t.Fatalf("selección o pie fuera de la ventana: %q", got)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	next := updated.(model)
	if next.viewScroll == 0 {
		t.Fatal("pgdown no desplaza la pantalla")
	}
	updated, _ = next.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if updated.(model).viewScroll != 0 {
		t.Fatal("pgup no vuelve al inicio")
	}
}

func TestViewportResponsive_PieExplicitoSinConfundirContenido(t *testing.T) {
	m := newTestModel(nil, 12)
	m.width = 40
	m.screen = screenConfig
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("contenido %d esc falso", i)
	}
	lines[30] = "▸ selección"
	lines[32] = strings.Repeat("─", 40)
	content := strings.Join(lines, "\n") + "\n" + footerBoundary + "s confirmar · n cancelar"
	got := m.fitTerminalView(content)
	if !strings.Contains(got, "▸ selección") || !strings.Contains(got, "s confirmar") || !strings.Contains(got, "n cancelar") {
		t.Fatalf("confundió contenido con el pie: %q", got)
	}
	if strings.Contains(got, "contenido 39 esc falso") {
		t.Fatal("una fila de contenido quedó fijada como parte del pie")
	}
	if strings.Contains(got, footerBoundary) || lipgloss.Height(got) > m.height {
		t.Fatalf("marca interna o desborde en vista: %q", got)
	}
	short := m.fitTerminalView("contenido\n" + renderFooter("esc volver"))
	if strings.Contains(short, footerBoundary) {
		t.Fatal("marca filtrada en pantalla corta")
	}
}
