package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mem/domain"
)

func manyMemories(n int) []domain.Memory {
	mems := make([]domain.Memory, n)
	for i := 0; i < n; i++ {
		mems[i] = domain.Memory{
			ID:      int64(i + 1),
			Type:    domain.Learning,
			Title:   "memoria de prueba",
			Content: "contenido de prueba",
		}
	}
	return mems
}

// El cuerpo de la lista debe recortarse a la altura de terminal disponible:
// antes de este fix, listView() escribía todos los items sin ventana, lo que
// hacía que listas largas se solaparan/desbordaran en terminales chicas.
func TestListViewFitsTerminalHeight(t *testing.T) {
	m := model{
		project:  "demo",
		memories: manyMemories(200),
		cursor:   0,
		ready:    true,
		height:   20,
	}

	out := m.listView()
	lines := strings.Split(out, "\n")
	if len(lines) > m.height+2 { // +2 de margen por el padding de appStyle
		t.Fatalf("listView() produjo %d líneas para una terminal de %d filas; se esperaba que quedara acotado", len(lines), m.height)
	}
}

// El ítem seleccionado siempre debe aparecer en la ventana visible, sin
// importar en qué punto de una lista larga esté el cursor (arriba, medio,
// abajo) — antes no había ventana, así que el cursor podía quedar fuera de
// lo que se veía en pantalla.
func TestListViewKeepsCursorVisible(t *testing.T) {
	total := 200
	for _, cursor := range []int{0, total / 2, total - 1} {
		m := model{
			project:  "demo",
			memories: manyMemories(total),
			cursor:   cursor,
			ready:    true,
			height:   20,
		}
		out := m.listView()
		if !strings.Contains(out, "▸") {
			t.Fatalf("cursor=%d: el marcador de selección '▸' no aparece en la ventana visible", cursor)
		}
	}
}

// Sin recorte de altura (terminal aún no reportó tamaño, o el contenido cabe
// entero) el comportamiento debe ser idéntico al de antes: todo visible.
func TestListViewNoWindowingWhenNotReady(t *testing.T) {
	m := model{
		project:  "demo",
		memories: manyMemories(5),
		cursor:   0,
		ready:    false,
		height:   0,
	}
	out := m.listView()
	if strings.Contains(out, "más arriba") || strings.Contains(out, "más abajo") {
		t.Fatalf("no debería mostrar indicadores de scroll cuando el tamaño de terminal aún no se conoce")
	}
}

// Regresión: navegar con "down" mientras se filtra por búsqueda no debía
// dejar avanzar el cursor más allá de los resultados filtrados (antes
// comparaba contra len(m.memories) en vez de len(visibleMemories())), lo que
// hacía que nada quedara resaltado y "enter" no abriera ningún detalle.
func TestSearchCursorStaysWithinFilteredResults(t *testing.T) {
	mems := manyMemories(10)
	mems[3].Title = "único match"
	mems[3].Content = "único match"
	m := model{
		project:   "demo",
		memories:  mems,
		searching: true,
		search:    "único",
		cursor:    0,
	}

	filtered := m.visibleMemories()
	if len(filtered) != 1 {
		t.Fatalf("se esperaba 1 resultado filtrado, hubo %d", len(filtered))
	}

	for i := 0; i < 5; i++ {
		mm, _ := m.updateList(tea.KeyMsg{Type: tea.KeyDown})
		m = mm.(model)
	}

	if m.cursor > len(filtered)-1 {
		t.Fatalf("el cursor (%d) quedó fuera de los %d resultados filtrados", m.cursor, len(filtered))
	}
}
