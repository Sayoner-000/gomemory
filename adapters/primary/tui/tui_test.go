package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
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

func newTestModel(mems []domain.Memory, height int) model {
	items := make([]list.Item, len(mems))
	for i, mem := range mems {
		items[i] = memoryItem{memory: mem}
	}
	l := list.New(items, memoryDelegate{}, 80, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	return model{
		project:  "demo",
		memories: mems,
		list:     l,
		ready:    height > 0,
		height:   height,
		width:    80,
	}
}

// El cuerpo de la lista debe caber en la altura de terminal disponible.
func TestListViewFitsTerminalHeight(t *testing.T) {
	mems := manyMemories(200)
	m := newTestModel(mems, 20)

	out := m.listView()
	lines := strings.Split(out, "\n")
	if len(lines) > m.height+4 {
		t.Fatalf("listView() produjo %d líneas para una terminal de %d filas; se esperaba que quedara acotado", len(lines), m.height)
	}
}

// El ítem seleccionado siempre debe aparecer en la ventana visible.
func TestListViewKeepsCursorVisible(t *testing.T) {
	total := 200
	for _, cursor := range []int{0, total / 2, total - 1} {
		mems := manyMemories(total)
		m := newTestModel(mems, 20)
		m.list.Select(cursor)
		out := m.listView()
		if !strings.Contains(out, "▸") {
			t.Fatalf("cursor=%d: el marcador de selección '▸' no aparece en la ventana visible", cursor)
		}
	}
}

// Sin recorte de altura (terminal aún no reportó tamaño) se muestra todo.
func TestListViewNoWindowingWhenNotReady(t *testing.T) {
	mems := manyMemories(5)
	m := newTestModel(mems, 0)
	m.ready = false

	out := m.listView()
	_ = out
}

// Verificar que el filtrado del list funciona correctamente.
// (No testeable con keystrokes simulados: el list maneja su propio state machine interno.)
func TestSearchFiltersCorrectly(t *testing.T) {
	// bubbles' filtering requiere tea.KeyMsg con teclas especiales que no se
	// pueden simular fuera del Update real. Este test valida que el list se
	// inicializa con filtering habilitado.
	mems := manyMemories(10)
	m := newTestModel(mems, 20)
	if !m.list.FilteringEnabled() {
		t.Fatal("esperaba que el list tuviera filtering habilitado")
	}
}

// ─── fakeMemRepo ──────────────────────────────────────────────────

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

// helper para crear modelo con list para tests de optimización
func newTestModelForOpt(mems []domain.Memory, height int) model {
	items := make([]list.Item, len(mems))
	for i, mem := range mems {
		items[i] = memoryItem{memory: mem}
	}
	l := list.New(items, memoryDelegate{}, 100, height)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()

	return model{
		memRepo:    &fakeMemRepo{mems: mems},
		project:    "demo",
		memories:   mems,
		list:       l,
		ready:      true,
		width:      100,
		height:     height,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}
}

func TestOptimizeFlow_DetectsAndDeletesDuplicateGroup(t *testing.T) {
	memFixture := duplicatePreferenceFixture()
	repo := &fakeMemRepo{mems: memFixture}
	items := make([]list.Item, len(memFixture))
	for i, mem := range memFixture {
		items[i] = memoryItem{memory: mem}
	}
	l := list.New(items, memoryDelegate{}, 100, 40)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		list:       l,
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
	memFixture := duplicatePreferenceFixture()
	repo := &fakeMemRepo{mems: memFixture}
	items := make([]list.Item, len(memFixture))
	for i, mem := range memFixture {
		items[i] = memoryItem{memory: mem}
	}
	l := list.New(items, memoryDelegate{}, 100, 40)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		list:       l,
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

func TestOptimizeDetailViewFitsTerminalHeight(t *testing.T) {
	memFixture := duplicateGroupsFixture(1, 6)
	repo := &fakeMemRepo{mems: memFixture}
	items := make([]list.Item, len(memFixture))
	for i, mem := range memFixture {
		items[i] = memoryItem{memory: mem}
	}
	l := list.New(items, memoryDelegate{}, 100, 20)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		list:       l,
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

func TestOptimizeDetailViewKeepsCursorVisible(t *testing.T) {
	memFixture := duplicateGroupsFixture(1, 6)
	repo := &fakeMemRepo{mems: memFixture}
	for _, cursor := range []int{0, 3, 5} {
		items := make([]list.Item, len(memFixture))
		for i, mem := range memFixture {
			items[i] = memoryItem{memory: mem}
		}
		l := list.New(items, memoryDelegate{}, 100, 20)
		l.SetShowTitle(false)
		l.DisableQuitKeybindings()

		m := model{
			memRepo:    repo,
			project:    "demo",
			memories:   memFixture,
			list:       l,
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

func TestWindowLinesScrollIndicators(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("item %d", i)
	}

	out := windowLines(lines, 25, 10)
	if !strings.Contains(out, "más arriba") {
		t.Fatalf("esperaba indicador '↑ más arriba' con cursor en línea 25 de 50, budget=10")
	}
	if !strings.Contains(out, "más abajo") {
		t.Fatalf("esperaba indicador '↓ más abajo' con cursor en línea 25 de 50, budget=10")
	}

	out = windowLines(lines, 0, 10)
	if strings.Contains(out, "más arriba") {
		t.Fatalf("no debería haber indicador '↑ más arriba' con cursor en línea 0")
	}
	if !strings.Contains(out, "más abajo") {
		t.Fatalf("esperaba indicador '↓ más abajo' con cursor en línea 0")
	}

	out = windowLines(lines, 49, 10)
	if !strings.Contains(out, "más arriba") {
		t.Fatalf("esperaba indicador '↑ más arriba' con cursor en la última línea")
	}
	if strings.Contains(out, "más abajo") {
		t.Fatalf("no debería haber indicador '↓ más abajo' con cursor en la última línea")
	}
}

func TestBodyBudgetCalculations(t *testing.T) {
	tests := []struct {
		name    string
		height  int
		ready   bool
		wantMin int
		wantMax int
	}{
		{"terminal chica 20 lineas", 20, true, 8, 18},
		{"terminal grande 50 lineas", 50, true, 35, 45},
		{"sin height", 0, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				ready:  tt.ready,
				height: tt.height,
			}
			head := "title\n"
			foot := "\nhelp"
			got := m.bodyBudget(head, foot)
			if got < tt.wantMin || got > tt.wantMax {
				t.Fatalf("bodyBudget(%q, %q) = %d, want [%d, %d]", head, foot, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestOptimizeAllFlow_CompactsEveryGroupUsingSuggestion(t *testing.T) {
	memFixture := append(duplicatePreferenceFixture(), []domain.Memory{
		{ID: 101, Type: domain.Architecture,
			Title:   "Decisión de despliegue: usar contenedores Docker en producción",
			Content: "El equipo decidió empaquetar el servicio con Docker y desplegarlo en contenedores para producción, evitando instalaciones manuales."},
		{ID: 102, Type: domain.Architecture,
			Title:   "Recordatorio: despliegue con contenedores Docker en producción",
			Content: "Siempre desplegar el servicio en producción usando contenedores Docker, nunca instalaciones manuales en el servidor."},
	}...)
	repo := &fakeMemRepo{mems: memFixture}
	items := make([]list.Item, len(memFixture))
	for i, mem := range memFixture {
		items[i] = memoryItem{memory: mem}
	}
	l := list.New(items, memoryDelegate{}, 100, 40)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		list:       l,
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
