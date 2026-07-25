package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
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

// ─── fakeMemRepo — implementación mínima de ports.MemoryRepository para
// probar el flujo de optimizar memorias sin tocar una DB real. ───────

type fakeMemRepo struct {
	mems    []domain.Memory
	deleted []int64
}

func (f *fakeMemRepo) Insert(m *domain.Memory) (int64, error) {
	f.mems = append(f.mems, *m)
	return m.ID, nil
}

func (f *fakeMemRepo) Get(project string, id int64) (*domain.Memory, error) {
	for _, m := range f.mems {
		if m.ID == id {
			mm := m
			return &mm, nil
		}
	}
	return nil, nil
}

func (f *fakeMemRepo) UpdateContent(project string, id int64, title, content string) error {
	return nil
}

func (f *fakeMemRepo) List(project string, limit int) ([]domain.Memory, error) {
	return f.mems, nil
}

func (f *fakeMemRepo) ListAll(project string) ([]domain.Memory, error) {
	return f.mems, nil
}

func (f *fakeMemRepo) ImportMemory(m *domain.Memory) (int64, error) {
	return f.Insert(m)
}

func (f *fakeMemRepo) Search(project, query string, limit int) ([]domain.Memory, error) {
	return f.mems, nil
}

func (f *fakeMemRepo) Delete(project string, id int64) (bool, error) {
	for i, m := range f.mems {
		if m.ID == id {
			f.mems = append(f.mems[:i], f.mems[i+1:]...)
			f.deleted = append(f.deleted, id)
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeMemRepo) SecondsSinceLastSave(project string) (int64, bool, error) {
	return 0, false, nil
}

// duplicatePreferenceFixture reproduce, en miniatura, el caso real que
// motivó esta feature: 2 memorias sobre el mismo tema con vocabulario muy
// solapado (deben agruparse) y 1 sin relación (no debe agruparse).
func duplicatePreferenceFixture() []domain.Memory {
	return []domain.Memory{
		{ID: 1, Type: domain.Preference,
			Title:   "Preferencia de idioma: español neutro con tuteo estándar",
			Content: "El usuario pide siempre español neutro con tuteo estándar, usar tú y tienes en todas las respuestas."},
		{ID: 2, Type: domain.Preference,
			Title:   "Recordatorio: siempre español neutro con tuteo estándar",
			Content: "Responder siempre en español neutro con tuteo estándar, con tú y tienes, en todas las respuestas del usuario."},
		{ID: 3, Type: domain.Architecture,
			Title:   "Arquitectura hexagonal del proyecto",
			Content: "El proyecto separa domain, ports, usecases y adapters siguiendo arquitectura hexagonal clásica."},
	}
}

func TestOptimizeFlow_DetectsAndDeletesDuplicateGroup(t *testing.T) {
	repo := &fakeMemRepo{mems: duplicatePreferenceFixture()}
	m := model{
		memRepo:    repo,
		project:    "demo",
		ready:      true,
		width:      100,
		height:     40,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m = mm.(model)
	if m.screen != screenOptimize {
		t.Fatalf("esperaba screenOptimize, quedó en %v", m.screen)
	}
	if len(m.dupGroups) != 1 {
		t.Fatalf("esperaba 1 grupo de duplicados, obtuve %d: %+v", len(m.dupGroups), m.dupGroups)
	}
	if len(m.dupGroups[0].Memories) != 2 {
		t.Fatalf("esperaba grupo de 2 memorias, obtuve %d", len(m.dupGroups[0].Memories))
	}

	mm, _ = m.updateOptimize(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	if m.screen != screenOptimizeDetail {
		t.Fatalf("esperaba screenOptimizeDetail, quedó en %v", m.screen)
	}

	mm, _ = m.updateOptimizeDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = mm.(model)
	if m.screen != screenOptimizeConfirm {
		t.Fatalf("esperaba screenOptimizeConfirm, quedó en %v", m.screen)
	}

	mm, _ = m.updateOptimizeConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = mm.(model)
	mm, _ = m.updateOptimizeConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = mm.(model)
	if got := m.dupConfirm.Value(); got != "si" {
		t.Fatalf(`esperaba dupConfirm="si", obtuve %q`, got)
	}

	keepID := m.dupGroups[m.dupGroupIdx].Memories[m.dupKeepIdx].ID
	mm, _ = m.updateOptimizeConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)

	if len(repo.deleted) != 1 {
		t.Fatalf("esperaba exactamente 1 Delete, hubo %d: %v", len(repo.deleted), repo.deleted)
	}
	if repo.deleted[0] == keepID {
		t.Fatalf("se borró la memoria canónica #%d, no debía tocarse", keepID)
	}
	if len(m.dupGroups) != 0 {
		t.Fatalf("esperaba dupGroups vacío tras optimizar, quedaron %d", len(m.dupGroups))
	}
	if m.screen != screenOptimize {
		t.Fatalf("esperaba volver a screenOptimize, quedó en %v", m.screen)
	}
}

func TestOptimizeDetail_SpaceExcludesFromDeletion(t *testing.T) {
	repo := &fakeMemRepo{mems: duplicatePreferenceFixture()}
	m := model{
		memRepo:    repo,
		project:    "demo",
		ready:      true,
		width:      100,
		height:     40,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m = mm.(model)
	mm, _ = m.updateOptimize(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)

	// Mover el cursor a la memoria que NO es la canónica y excluirla.
	nonKeepIdx := 1 - m.dupKeepIdx
	m.dupMemberCursor = nonKeepIdx
	mm, _ = m.updateOptimizeDetail(tea.KeyMsg{Type: tea.KeySpace})
	m = mm.(model)

	if m.deletionCandidates(m.dupGroups[m.dupGroupIdx]) != 0 {
		t.Fatalf("tras excluir la única no-canónica, no debía quedar nada para borrar")
	}

	mm, _ = m.updateOptimizeDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = mm.(model)
	if m.screen != screenOptimizeDetail {
		t.Fatalf("con 0 candidatos a borrar, 'c' no debía avanzar a confirmar")
	}
	if m.dupErr == "" {
		t.Fatalf("esperaba un mensaje de error explicando que no hay nada para borrar")
	}
}

// duplicateGroupsFixture arma varios grupos de duplicados independientes,
// cada uno con contenido largo, para forzar que optimizeDetailView() necesite
// más líneas que las que caben en una terminal chica.
func duplicateGroupsFixture(groups, perGroup int) []domain.Memory {
	longContent := strings.Repeat("contenido largo de prueba para forzar varias líneas de renderizado. ", 10)
	var mems []domain.Memory
	id := int64(1)
	for g := 0; g < groups; g++ {
		title := fmt.Sprintf("Preferencia de idioma: español neutro con tuteo grupo %d", g)
		for i := 0; i < perGroup; i++ {
			mems = append(mems, domain.Memory{
				ID:      id,
				Type:    domain.Preference,
				Title:   title,
				Content: longContent,
			})
			id++
		}
	}
	return mems
}

// Regresión: la pantalla de detalle de un grupo (varias cajas bordeadas, una
// por memoria) no aplicaba ninguna ventana de scroll, así que con memorias de
// contenido largo el grupo entero no cabía en una terminal chica y la caja
// seleccionada podía quedar fuera de lo visible sin forma de llegar a ella.
func TestOptimizeDetailViewFitsTerminalHeight(t *testing.T) {
	repo := &fakeMemRepo{mems: duplicateGroupsFixture(1, 6)}
	m := model{
		memRepo:    repo,
		project:    "demo",
		ready:      true,
		width:      100,
		height:     20,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m = mm.(model)
	mm, _ = m.updateOptimize(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)

	out := m.optimizeDetailView()
	lines := strings.Split(out, "\n")
	if len(lines) > m.height+2 {
		t.Fatalf("optimizeDetailView() produjo %d líneas para una terminal de %d filas; se esperaba que quedara acotado", len(lines), m.height)
	}
}

// El cursor sobre miembros del grupo (dupMemberCursor) debe permanecer
// visible sin importar en qué posición esté, incluso cuando el grupo no cabe
// entero en la terminal.
func TestOptimizeDetailViewKeepsCursorVisible(t *testing.T) {
	repo := &fakeMemRepo{mems: duplicateGroupsFixture(1, 6)}
	for _, cursor := range []int{0, 3, 5} {
		m := model{
			memRepo:    repo,
			project:    "demo",
			ready:      true,
			width:      100,
			height:     20,
			dupConfirm: textinput.New(),
			dupExclude: make(map[int64]bool),
		}

		mm, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		m = mm.(model)
		mm, _ = m.updateOptimize(tea.KeyMsg{Type: tea.KeyEnter})
		m = mm.(model)
		m.dupMemberCursor = cursor

		out := m.optimizeDetailView()
		if !strings.Contains(out, fmt.Sprintf("#%d", m.dupGroups[m.dupGroupIdx].Memories[cursor].ID)) {
			t.Fatalf("dupMemberCursor=%d: la memoria seleccionada no aparece en la ventana visible", cursor)
		}
	}
}

// Compactar todas (tecla "a" en screenOptimize) debe aplicar la sugerencia
// automática de cada grupo detectado y borrar todo lo que no sea la memoria
// canónica sugerida, en un solo paso, sin revisión grupo por grupo.
func TestOptimizeAllFlow_CompactsEveryGroupUsingSuggestion(t *testing.T) {
	// Dos clusters de duplicados sobre temas sin ningún vocabulario en común,
	// para que el detector los agrupe por separado (2 grupos), no juntos.
	mems := append(duplicatePreferenceFixture(), []domain.Memory{
		{ID: 101, Type: domain.Architecture,
			Title:   "Decisión de despliegue: usar contenedores Docker en producción",
			Content: "El equipo decidió empaquetar el servicio con Docker y desplegarlo en contenedores para producción, evitando instalaciones manuales."},
		{ID: 102, Type: domain.Architecture,
			Title:   "Recordatorio: despliegue con contenedores Docker en producción",
			Content: "Siempre desplegar el servicio en producción usando contenedores Docker, nunca instalaciones manuales en el servidor."},
	}...)
	repo := &fakeMemRepo{mems: mems}
	m := model{
		memRepo:    repo,
		project:    "demo",
		ready:      true,
		width:      100,
		height:     40,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m = mm.(model)
	if len(m.dupGroups) != 2 {
		t.Fatalf("esperaba 2 grupos de duplicados, obtuve %d", len(m.dupGroups))
	}

	mm, _ = m.updateOptimize(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = mm.(model)
	if m.screen != screenOptimizeAllConfirm {
		t.Fatalf("esperaba screenOptimizeAllConfirm, quedó en %v", m.screen)
	}

	wantDeleted := m.totalDeletionCandidates()
	keepIDs := map[int64]bool{}
	for _, g := range m.dupGroups {
		keepIDs[g.SuggestedKeepID] = true
	}

	mm, _ = m.updateOptimizeAllConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = mm.(model)
	mm, _ = m.updateOptimizeAllConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = mm.(model)

	mm, _ = m.updateOptimizeAllConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)

	if len(repo.deleted) != wantDeleted {
		t.Fatalf("esperaba %d Delete en total, hubo %d: %v", wantDeleted, len(repo.deleted), repo.deleted)
	}
	for _, id := range repo.deleted {
		if keepIDs[id] {
			t.Fatalf("se borró la memoria canónica #%d, no debía tocarse", id)
		}
	}
	if len(m.dupGroups) != 0 {
		t.Fatalf("esperaba dupGroups vacío tras compactar todas, quedaron %d", len(m.dupGroups))
	}
	if m.screen != screenOptimize {
		t.Fatalf("esperaba volver a screenOptimize, quedó en %v", m.screen)
	}
}
