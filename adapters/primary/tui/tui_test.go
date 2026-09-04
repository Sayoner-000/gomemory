package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"mem/application/ports"
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
	fi := textinput.New()
	fi.Placeholder = "buscar..."
	fi.SetWidth(40)

	return model{
		project:     "demo",
		memories:    mems,
		filtered:    mems,
		filterInput: fi,
		ready:       height > 0,
		height:      height,
		width:       80,
		dupConfirm:  textinput.New(),
		dupExclude:  make(map[int64]bool),
	}
}

// La lista debe caber en la altura de terminal disponible.
func TestListFitsTerminalHeight(t *testing.T) {
	mems := manyMemories(200)
	m := newTestModel(mems, 30)

	out := m.listView()
	lines := strings.Split(out, "\n")
	if len(lines) > m.height+4 {
		t.Fatalf("listView() produjo %d líneas para una terminal de %d filas; se esperaba que quedara acotado", len(lines), m.height)
	}
}

// Cada fila debe mostrar tipo y título de la memoria.
func TestListShowsTypeAndTitle(t *testing.T) {
	mems := manyMemories(3)
	mems[0].Title = "memoria de prueba"
	m := newTestModel(mems, 20)

	out := m.listView()
	if !strings.Contains(out, "Aprendizaje") || !strings.Contains(out, "memoria de prueba") {
		t.Fatal("la lista no muestra el tipo (Aprendizaje) y el título de la memoria")
	}
}

// Verificar que el filtrado por texto funciona correctamente.
func TestFilterWorksCorrectly(t *testing.T) {
	mems := manyMemories(10)
	mems[3].Title = "único match"
	mems[3].Content = "único match"
	m := newTestModel(mems, 20)

	// Activar filtro
	mm, _ := m.updateList(keyMsg("/"))
	m = mm.(model)
	if !m.filtering {
		t.Fatal("esperaba que filtering sea true tras presionar /")
	}

	// Escribir texto de filtro
	for _, ch := range "único" {
		mm, _ = m.updateList(keyMsg(string(ch)))
		m = mm.(model)
	}

	if len(m.filtered) != 1 {
		t.Fatalf("se esperaba 1 resultado filtrado, hubo %d", len(m.filtered))
	}
}

// Verificar que Esc cancela el filtro y restaura las memorias originales.
func TestFilterEscapeRestores(t *testing.T) {
	mems := manyMemories(10)
	m := newTestModel(mems, 20)

	mm, _ := m.updateList(keyMsg("/"))
	m = mm.(model)
	for _, ch := range "algo" {
		mm, _ = m.updateList(keyMsg(string(ch)))
		m = mm.(model)
	}

	mm, _ = m.updateList(keyMsg("esc"))
	m = mm.(model)

	if m.filtering {
		t.Fatal("esperaba que filtering sea false tras Esc")
	}
	if len(m.filtered) != len(mems) {
		t.Fatalf("esperaba %d memorias restauradas, hubo %d", len(mems), len(m.filtered))
	}
}

func TestList_EliminaMemoriaFiltradaTrasConfirmar(t *testing.T) {
	mems := []domain.Memory{
		{ID: 1, Type: domain.Preference, Title: "Preferencia de idioma", Content: "español"},
		{ID: 2, Type: domain.Learning, Title: "Aprendizaje", Content: "otro"},
	}
	repo := &fakeMemRepo{mems: mems}
	m := newTestModel(mems, 20)
	m.memRepo = repo
	m.filterInput.SetValue("preferencia")
	m.applyFilter()

	next, _ := m.updateList(keyMsg("d"))
	m = next.(model)
	if !m.deleteConfirm || m.deleteTarget.ID != 1 {
		t.Fatalf("d debe pedir confirmación para la memoria filtrada, target=%d confirm=%t", m.deleteTarget.ID, m.deleteConfirm)
	}

	next, _ = m.updateList(keyMsg("s"))
	m = next.(model)
	if m.deleteConfirm || len(repo.deleted) != 1 || repo.deleted[0] != 1 {
		t.Fatalf("la confirmación debe eliminar solo la memoria elegida: %+v", repo.deleted)
	}
	if len(m.memories) != 1 || len(m.filtered) != 0 || m.listCursor != 0 {
		t.Fatalf("la lista filtrada debe quedar sincronizada tras borrar el último resultado: memorias=%d filtradas=%d cursor=%d", len(m.memories), len(m.filtered), m.listCursor)
	}
	if !strings.Contains(m.statusMsg, "Memoria 1 eliminada") {
		t.Fatalf("faltó el mensaje de éxito: %q", m.statusMsg)
	}
}

func TestList_CancelaEliminacionIndividual(t *testing.T) {
	mems := manyMemories(1)
	repo := &fakeMemRepo{mems: mems}
	m := newTestModel(mems, 20)
	m.memRepo = repo

	next, _ := m.updateList(keyMsg("d"))
	m = next.(model)
	next, _ = m.updateList(keyMsg("n"))
	m = next.(model)

	if m.deleteConfirm || len(repo.deleted) != 0 || len(m.memories) != 1 {
		t.Fatalf("cancelar no debe borrar la memoria: confirm=%t deleted=%v memorias=%d", m.deleteConfirm, repo.deleted, len(m.memories))
	}
	if m.statusMsg != "Eliminación cancelada" {
		t.Fatalf("mensaje de cancelación inesperado: %q", m.statusMsg)
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

func TestOptimizeFlow_DetectsAndDeletesDuplicateGroup(t *testing.T) {
	memFixture := duplicatePreferenceFixture()
	repo := &fakeMemRepo{mems: memFixture}

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		filtered:   memFixture,
		ready:      true,
		width:      100,
		height:     40,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(keyMsg("o"))
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

	mm, _ = m.updateOptimize(keyMsg("enter"))
	m = mm.(model)
	if m.screen != screenOptimizeDetail {
		t.Fatalf("esperaba screenOptimizeDetail, quedó en %v", m.screen)
	}

	mm, _ = m.updateOptimizeDetail(keyMsg("c"))
	m = mm.(model)
	if m.screen != screenOptimizeConfirm {
		t.Fatalf("esperaba screenOptimizeConfirm, quedó en %v", m.screen)
	}

	mm, _ = m.updateOptimizeConfirm(keyMsg("s"))
	m = mm.(model)
	mm, _ = m.updateOptimizeConfirm(keyMsg("i"))
	m = mm.(model)
	if got := m.dupConfirm.Value(); got != "si" {
		t.Fatalf(`esperaba dupConfirm="si", obtuve %q`, got)
	}

	keepID := m.dupGroups[m.dupGroupIdx].Memories[m.dupKeepIdx].ID
	mm, _ = m.updateOptimizeConfirm(keyMsg("enter"))
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

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		filtered:   memFixture,
		ready:      true,
		width:      100,
		height:     40,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(keyMsg("o"))
	m = mm.(model)
	mm, _ = m.updateOptimize(keyMsg("enter"))
	m = mm.(model)

	nonKeepIdx := 1 - m.dupKeepIdx
	m.dupMemberCursor = nonKeepIdx
	mm, _ = m.updateOptimizeDetail(keyMsg("space"))
	m = mm.(model)

	if m.deletionCandidates(m.dupGroups[m.dupGroupIdx]) != 0 {
		t.Fatalf("tras excluir la única no-canónica, no debía quedar nada para borrar")
	}

	mm, _ = m.updateOptimizeDetail(keyMsg("c"))
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

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		filtered:   memFixture,
		ready:      true,
		width:      100,
		height:     20,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(keyMsg("o"))
	m = mm.(model)
	mm, _ = m.updateOptimize(keyMsg("enter"))
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
		m := model{
			memRepo:    repo,
			project:    "demo",
			memories:   memFixture,
			filtered:   memFixture,
			ready:      true,
			width:      100,
			height:     20,
			dupConfirm: textinput.New(),
			dupExclude: make(map[int64]bool),
		}

		mm, _ := m.updateList(keyMsg("o"))
		m = mm.(model)
		mm, _ = m.updateOptimize(keyMsg("enter"))
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

func TestPasteMsgReachesFocusedInput(t *testing.T) {
	m := newTestModel(nil, 20)
	m.screen = screenSave
	m.saveContent = textinput.New()
	m.saveContent.Focus()

	next, _ := m.Update(tea.PasteMsg{Content: "texto pegado agnóstico"})
	got := next.(model).saveContent.Value()
	if got != "texto pegado agnóstico" {
		t.Fatalf("el mensaje de pegado no llegó al input activo: %q", got)
	}
}

// El pegado debe llegar a la caja de la pantalla activa, no a la que quedó
// enfocada de arrastre. saveContent se enfoca al construir el modelo y nunca se
// desenfoca al salir de la pantalla de guardar: si el destino se eligiera por
// Focused() recorriendo el modelo, se tragaría el pegado de todas las demás.
func TestPasteMsgLlegaALaCajaDeLaPantallaActiva(t *testing.T) {
	casos := []struct {
		nombre  string
		prepara func(m *model)
		valor   func(m model) string
	}{
		{
			nombre: "import",
			prepara: func(m *model) {
				m.screen = screenImport
				m.importPath = textinput.New()
				m.importPath.Focus()
			},
			valor: func(m model) string { return m.importPath.Value() },
		},
		{
			nombre: "ajuste de configuración",
			prepara: func(m *model) {
				m.screen = screenEditSetting
				m.editSettingInput = textinput.New()
				m.editSettingInput.Focus()
			},
			valor: func(m model) string { return m.editSettingInput.Value() },
		},
		{
			nombre: "ruta de documento",
			prepara: func(m *model) {
				m.screen = screenDocs
				m.docPendiente = docActionImportar
				m.docPath = textinput.New()
				m.docPath.Focus()
			},
			valor: func(m model) string { return m.docPath.Value() },
		},
		{
			nombre: "confirmación de mantenimiento",
			prepara: func(m *model) {
				m.screen = screenMaintenanceConfirm
				m.maintConfirm = textinput.New()
				m.maintConfirm.Focus()
			},
			valor: func(m model) string { return m.maintConfirm.Value() },
		},
		{
			nombre: "filtro de la lista",
			prepara: func(m *model) {
				m.screen = screenList
				m.filtering = true
				m.filterInput.Focus()
			},
			valor: func(m model) string { return m.filterInput.Value() },
		},
		{
			nombre: "tarea de uso",
			prepara: func(m *model) {
				m.screen = screenUsage
				m.usageTaskInput = textinput.New()
				m.usageBudgetInput = textinput.New()
				m.usageFocus = 0
				m.updateUsageFocus()
			},
			valor: func(m model) string { return m.usageTaskInput.Value() },
		},
	}

	const pegado = "/ruta/pegada/desde/el/portapapeles"

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m := newTestModel(nil, 20)
			// Foco de arrastre: así queda el modelo tras arrancar o tras
			// visitar la pantalla de guardar.
			m.saveContent = textinput.New()
			m.saveContent.Focus()
			c.prepara(&m)

			next, _ := m.Update(tea.PasteMsg{Content: pegado})
			got := next.(model)
			if v := c.valor(got); v != pegado {
				t.Fatalf("la caja activa no recibió el pegado: %q", v)
			}
			if got.saveContent.Value() != "" {
				t.Fatalf("el pegado se filtró a saveContent: %q", got.saveContent.Value())
			}
		})
	}
}

// Entrar a la pantalla de guardar debe dejar UNA sola caja enfocada. Enfocar
// contenido sin tocar saveFocus dejaba dos activas si se había tabulado antes,
// y el teclado escribía en las dos a la vez.
func TestPantallaGuardarEnfocaUnaSolaCaja(t *testing.T) {
	m := newTestModel(nil, 20)
	m.saveTitle = textinput.New()
	m.saveType = textinput.New()
	m.saveContent = textinput.New()
	m.saveFilepath = textinput.New()
	m.saveFocus = 0
	m.updateFocus() // el usuario había tabulado hasta Título

	next, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	got := next.(model)

	if got.screen != screenSave {
		t.Fatalf("no se entró a la pantalla de guardar: %v", got.screen)
	}
	enfocadas := 0
	for _, in := range []textinput.Model{got.saveTitle, got.saveType, got.saveContent, got.saveFilepath} {
		if in.Focused() {
			enfocadas++
		}
	}
	if enfocadas != 1 {
		t.Fatalf("se esperaba exactamente una caja enfocada, hay %d", enfocadas)
	}
	if !got.saveContent.Focused() || got.saveFocus != 2 {
		t.Fatalf("el foco debía quedar en contenido (saveFocus=%d)", got.saveFocus)
	}
}

func TestDetailViewScrollsLongMemory(t *testing.T) {
	contentLines := make([]string, 60)
	for i := range contentLines {
		contentLines[i] = fmt.Sprintf("línea larga %02d", i)
	}
	m := newTestModel(nil, 18)
	m.screen = screenDetail
	m.selected = domain.Memory{Title: "larga", Type: domain.Learning, Content: strings.Join(contentLines, "\n")}

	first := ansi.Strip(m.detailView())
	if !strings.Contains(first, "línea larga 00") || strings.Contains(first, "línea larga 59") {
		t.Fatalf("la vista inicial no está acotada al comienzo:\n%s", first)
	}
	next, _ := m.updateDetail(keyMsg("G"))
	m = next.(model)
	last := ansi.Strip(m.detailView())
	if !strings.Contains(last, "línea larga 59") || !strings.Contains(last, "más arriba") {
		t.Fatalf("end no desplazó el detalle hasta el final:\n%s", last)
	}
}

func TestCopyableTextIncludesFullMemoryDespiteScroll(t *testing.T) {
	m := newTestModel(nil, 12)
	m.screen = screenDetail
	m.detailScroll = 40
	m.selected = domain.Memory{
		Title:   "memoria completa",
		Type:    domain.Decision,
		Content: "primera línea\n" + strings.Repeat("intermedia\n", 40) + "última línea",
	}

	got := m.copyableText()
	if !strings.Contains(got, "primera línea") || !strings.Contains(got, "última línea") {
		t.Fatalf("copiar debe incluir el contenido completo, no solo la ventana visible:\n%s", got)
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

	m := model{
		memRepo:    repo,
		project:    "demo",
		memories:   memFixture,
		filtered:   memFixture,
		ready:      true,
		width:      100,
		height:     40,
		dupConfirm: textinput.New(),
		dupExclude: make(map[int64]bool),
	}

	mm, _ := m.updateList(keyMsg("o"))
	m = mm.(model)
	if len(m.dupGroups) != 2 {
		t.Fatalf("esperaba 2 grupos de duplicados, obtuve %d", len(m.dupGroups))
	}

	mm, _ = m.updateOptimize(keyMsg("a"))
	m = mm.(model)
	if m.screen != screenOptimizeAllConfirm {
		t.Fatalf("esperaba screenOptimizeAllConfirm, quedó en %v", m.screen)
	}

	wantDeleted := m.totalDeletionCandidates()
	keepIDs := map[int64]bool{}
	for _, g := range m.dupGroups {
		keepIDs[g.SuggestedKeepID] = true
	}

	mm, _ = m.updateOptimizeAllConfirm(keyMsg("s"))
	m = mm.(model)
	mm, _ = m.updateOptimizeAllConfirm(keyMsg("i"))
	m = mm.(model)

	mm, _ = m.updateOptimizeAllConfirm(keyMsg("enter"))
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

// --- Feature 013: interruptor de planificación atómica ---

// tuiSettingsStub guarda el estado en memoria para poder ejercer el interruptor
// sin tocar disco.
type tuiSettingsStub struct{ data *ports.SettingsData }

func (s tuiSettingsStub) Read(string) ports.SettingsData              { return *s.data }
func (s tuiSettingsStub) Write(_ string, d ports.SettingsData) error  { *s.data = d; return nil }
func (s tuiSettingsStub) ApplyAutoApprove(string, ports.SettingsData) {}

// TestConfigScreen_MuestraInterruptorDePlanificacionAtomica cubre FR-033: el
// estado del ajuste debe ser consultable desde la pantalla de configuración.
func TestConfigScreen_MuestraInterruptorDePlanificacionAtomica(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		width:        100,
		height:       40,
		ready:        true,
	}

	// bubbletea v2 / lipgloss v2 ya no degradan el color en Render(): el
	// texto sale siempre con los códigos ANSI del estilo aplicado (la
	// degradación por perfil de terminal ocurre en el renderer real del
	// Program, no en la cadena). Se despoja el ANSI para comparar el
	// contenido semántico, igual que veía la pantalla en v1 bajo test.
	view := ansi.Strip(m.renderView())
	if !strings.Contains(view, "Planificación atómica") {
		t.Errorf("la pantalla de configuración debe mostrar el interruptor; vista:\n%s", view)
	}
	if !strings.Contains(view, "Planificación atómica: ON") {
		t.Error("con el ajuste ausente la funcionalidad debe verse como ON (opt-out)")
	}

	data.AtomicPlanDisabled = true
	if view := ansi.Strip(m.renderView()); !strings.Contains(view, "Planificación atómica: OFF") {
		t.Error("con atomic_plan_disabled=true debe verse como OFF")
	}
}

// TestConfigScreen_ToggleDePlanificacionAtomicaPersiste verifica que la opción
// escribe el ajuste, no solo lo pinta.
func TestConfigScreen_ToggleDePlanificacionAtomicaPersiste(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		configCursor: configRowAtomicPlan,
		width:        100,
		height:       40,
	}

	updated, _ := m.updateConfig(keyMsg("enter"))
	if !data.AtomicPlanDisabled {
		t.Error("al confirmar sobre el interruptor debe quedar desactivada")
	}

	m2 := updated.(model)
	m2.configCursor = configRowAtomicPlan
	if _, _ = m2.updateConfig(keyMsg("enter")); data.AtomicPlanDisabled {
		t.Error("al confirmar de nuevo debe reactivarse")
	}
}

// TestConfigScreen_TogglePlanGuardPersiste cubre T063 (feature 019, Historia
// 1): el interruptor de la exigencia de forma del plan en la pantalla de
// configuración, mismo patrón que el de planificación atómica.
func TestConfigScreen_TogglePlanGuardPersiste(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		configCursor: configRowPlanGuard,
		width:        100,
		height:       40,
	}

	updated, _ := m.updateConfig(keyMsg("enter"))
	if !data.PlanGuardDisabled {
		t.Error("al confirmar sobre el interruptor debe quedar desactivada")
	}

	m2 := updated.(model)
	m2.configCursor = configRowPlanGuard
	if _, _ = m2.updateConfig(keyMsg("enter")); data.PlanGuardDisabled {
		t.Error("al confirmar de nuevo debe reactivarse")
	}
}

// --- Feature 016: reindexado del grafo externo desde la TUI (US2) ---

// fakeCodeIndexer implementa ports.CodeGraphProvider + ports.CodeGraphIndexer
// (proveedor que SÍ soporta el reindexado explícito).
type fakeCodeIndexer struct {
	nodes, edges int
	indexErr     error
	indexCalls   int
}

func (f *fakeCodeIndexer) Name() string { return "fake-indexer" }
func (f *fakeCodeIndexer) Snapshot() domain.CodeProviderSnapshot {
	return domain.CodeProviderSnapshot{}
}
func (f *fakeCodeIndexer) MaybeRefresh() {}
func (f *fakeCodeIndexer) ImpactFor(string) (domain.CodeImpactAnnotation, bool) {
	return domain.CodeImpactAnnotation{}, false
}
func (f *fakeCodeIndexer) IndexRepository(context.Context, string) (int, int, error) {
	f.indexCalls++
	return f.nodes, f.edges, f.indexErr
}

// fakeCodeProviderNoIndexer implementa SOLO ports.CodeGraphProvider (sin
// soporte de reindexado explícito) — su aserción a ports.CodeGraphIndexer
// debe fallar de forma segura (ok=false, sin panic).
type fakeCodeProviderNoIndexer struct{}

func (f fakeCodeProviderNoIndexer) Name() string { return "fake-no-indexer" }
func (f fakeCodeProviderNoIndexer) Snapshot() domain.CodeProviderSnapshot {
	return domain.CodeProviderSnapshot{}
}
func (f fakeCodeProviderNoIndexer) MaybeRefresh() {}
func (f fakeCodeProviderNoIndexer) ImpactFor(string) (domain.CodeImpactAnnotation, bool) {
	return domain.CodeImpactAnnotation{}, false
}

func newConfigTestModel(cp ports.CodeGraphProvider, data *ports.SettingsData) model {
	return model{
		screen:           screenConfig,
		settingsRepo:     tuiSettingsStub{data: data},
		codeProvider:     cp,
		editSettingInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
	}
}

func TestConfigView_ReindexLabel_SegunSoporteDeInterfaz(t *testing.T) {
	data := &ports.SettingsData{}

	withSupport := newConfigTestModel(&fakeCodeIndexer{}, data)
	if view := withSupport.configView(); !strings.Contains(view, "Reindexar grafo externo (codebase-memory-mcp)") {
		t.Errorf("esperaba label con soporte de interfaz; vista:\n%s", view)
	}

	withoutSupport := newConfigTestModel(fakeCodeProviderNoIndexer{}, data)
	if view := withoutSupport.configView(); !strings.Contains(view, "Reindexar grafo externo: no disponible") {
		t.Errorf("esperaba label sin soporte de interfaz; vista:\n%s", view)
	}
}

func TestUpdateConfig_ReindexRow_SinSoporte_NoDisparaCmd(t *testing.T) {
	data := &ports.SettingsData{}
	m := newConfigTestModel(fakeCodeProviderNoIndexer{}, data)
	m.configCursor = configRowReindexGraph

	updated, cmd := m.updateConfig(keyMsg("enter"))
	if cmd != nil {
		t.Fatal("sin soporte de interfaz no debería disparar ningún tea.Cmd")
	}
	m2 := updated.(model)
	if m2.statusMsg != "codebase-memory-mcp no disponible" {
		t.Fatalf("statusMsg = %q", m2.statusMsg)
	}
}

func TestUpdateConfig_ReindexRow_ConSoporte_DisparaCmd(t *testing.T) {
	data := &ports.SettingsData{}
	m := newConfigTestModel(&fakeCodeIndexer{nodes: 10, edges: 20}, data)
	m.configCursor = configRowReindexGraph

	updated, cmd := m.updateConfig(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("con soporte de interfaz debería disparar un tea.Cmd")
	}
	m2 := updated.(model)
	if !m2.reindexInProgress {
		t.Fatal("esperaba reindexInProgress=true de inmediato, antes de que el Cmd resuelva")
	}
}

func TestUpdateConfig_ReindexRow_YaEnCurso_NoDisparaSegundoCmd(t *testing.T) {
	data := &ports.SettingsData{}
	m := newConfigTestModel(&fakeCodeIndexer{}, data)
	m.configCursor = configRowReindexGraph
	m.reindexInProgress = true

	updated, cmd := m.updateConfig(keyMsg("enter"))
	if cmd != nil {
		t.Fatal("con un reindexado ya en curso no debería dispararse un segundo tea.Cmd")
	}
	m2 := updated.(model)
	if !strings.Contains(m2.statusMsg, "ya hay") && !strings.Contains(m2.statusMsg, "Ya hay") {
		t.Fatalf("esperaba un statusMsg que deje claro que ya hay uno en curso, obtuve %q", m2.statusMsg)
	}
}

func TestReindexExternalGraphCmd_PropagaResultado(t *testing.T) {
	indexer := &fakeCodeIndexer{nodes: 42, edges: 84}
	m := newConfigTestModel(indexer, &ports.SettingsData{})

	msg := m.reindexExternalGraphCmd()()
	done, ok := msg.(externalReindexDoneMsg)
	if !ok {
		t.Fatalf("esperaba externalReindexDoneMsg, obtuve %T", msg)
	}
	if done.nodes != 42 || done.edges != 84 || done.err != nil {
		t.Fatalf("resultado inesperado: %+v", done)
	}
	if indexer.indexCalls != 1 {
		t.Fatalf("esperaba exactamente 1 llamada a IndexRepository, hubo %d", indexer.indexCalls)
	}
}

func TestUpdate_ExternalReindexDoneMsg_Exito(t *testing.T) {
	m := newConfigTestModel(&fakeCodeIndexer{}, &ports.SettingsData{})
	m.reindexInProgress = true

	updated, _ := m.Update(externalReindexDoneMsg{nodes: 5, edges: 9})
	m2 := updated.(model)
	if m2.reindexInProgress {
		t.Fatal("esperaba reindexInProgress=false tras el resultado")
	}
	if !strings.Contains(m2.statusMsg, "5 nodos") || !strings.Contains(m2.statusMsg, "9 aristas") {
		t.Fatalf("statusMsg no refleja los conteos: %q", m2.statusMsg)
	}
}

func TestUpdate_ExternalReindexDoneMsg_NoInstalado(t *testing.T) {
	m := newConfigTestModel(fakeCodeProviderNoIndexer{}, &ports.SettingsData{})
	m.reindexInProgress = true

	updated, _ := m.Update(externalReindexDoneMsg{err: ports.ErrIndexerNotInstalled})
	m2 := updated.(model)
	if m2.reindexInProgress {
		t.Fatal("esperaba reindexInProgress=false tras el resultado")
	}
	if m2.statusMsg != "codebase-memory-mcp no disponible" {
		t.Fatalf("statusMsg = %q", m2.statusMsg)
	}
}

// --- Feature 016: editar huella de contexto desde la TUI (US3) ---

func TestUpdateConfig_EditBudgetRow_PrecargaValorActual(t *testing.T) {
	data := &ports.SettingsData{Budget: 12345}
	m := newConfigTestModel(nil, data)
	m.configCursor = configRowEditBudget

	updated, _ := m.updateConfig(keyMsg("enter"))
	m2 := updated.(model)
	if m2.screen != screenEditSetting {
		t.Fatalf("esperaba screenEditSetting, quedó en %v", m2.screen)
	}
	if got := m2.editSettingInput.Value(); got != strconv.Itoa(12345) {
		t.Fatalf("esperaba input precargado con %q, obtuve %q", "12345", got)
	}
	if !m2.editSettingInput.Focused() {
		t.Fatal("esperaba el input enfocado")
	}
}

func typeInto(m model, s string) model {
	for _, ch := range s {
		updated, _ := m.updateEditSetting(keyMsg(string(ch)))
		m = updated.(model)
	}
	return m
}

func TestUpdateEditSetting_GuardaValorValido(t *testing.T) {
	data := &ports.SettingsData{}
	m := newConfigTestModel(nil, data)
	m.screen = screenEditSetting
	m.editSettingField = editFieldBudget
	m.editSettingInput.SetValue("")
	m.editSettingInput.Focus()

	m = typeInto(m, "999")
	updated, _ := m.updateEditSetting(keyMsg("enter"))
	m2 := updated.(model)

	if m2.screen != screenConfig {
		t.Fatalf("esperaba volver a screenConfig, quedó en %v", m2.screen)
	}
	if data.Budget != 999 {
		t.Fatalf("esperaba Budget=999 persistido, obtuve %d", data.Budget)
	}
	if m2.editSettingErr != "" {
		t.Fatalf("no esperaba error, obtuve %q", m2.editSettingErr)
	}
}

func TestUpdateEditSetting_RechazaNoNumerico(t *testing.T) {
	for _, raw := range []string{"", "abc", "3.5"} {
		t.Run(raw, func(t *testing.T) {
			data := &ports.SettingsData{Budget: 111}
			m := newConfigTestModel(nil, data)
			m.screen = screenEditSetting
			m.editSettingField = editFieldBudget
			m.editSettingInput.SetValue("")
			m.editSettingInput.Focus()

			m = typeInto(m, raw)
			updated, _ := m.updateEditSetting(keyMsg("enter"))
			m2 := updated.(model)

			if m2.screen != screenEditSetting {
				t.Fatalf("valor inválido %q no debería avanzar la pantalla, quedó en %v", raw, m2.screen)
			}
			if m2.editSettingErr == "" {
				t.Fatalf("esperaba un mensaje de error para %q", raw)
			}
			if data.Budget != 111 {
				t.Fatalf("no debería haberse guardado nada, Budget quedó en %d", data.Budget)
			}
		})
	}
}

func TestUpdateEditSetting_PermiteCeroYNegativo(t *testing.T) {
	for _, raw := range []string{"0", "-5"} {
		t.Run(raw, func(t *testing.T) {
			data := &ports.SettingsData{}
			m := newConfigTestModel(nil, data)
			m.screen = screenEditSetting
			m.editSettingField = editFieldDedupDays
			m.editSettingInput.SetValue("")
			m.editSettingInput.Focus()

			m = typeInto(m, raw)
			updated, _ := m.updateEditSetting(keyMsg("enter"))
			m2 := updated.(model)

			want, _ := strconv.Atoi(raw)
			if data.DedupWindowDays != want {
				t.Fatalf("esperaba DedupWindowDays=%d, obtuve %d", want, data.DedupWindowDays)
			}
			if m2.screen != screenConfig {
				t.Fatalf("esperaba volver a screenConfig, quedó en %v", m2.screen)
			}
		})
	}
}

func TestUpdateEditSetting_Esc_CancelaSinGuardar(t *testing.T) {
	data := &ports.SettingsData{CompactThreshold: 777}
	m := newConfigTestModel(nil, data)
	m.screen = screenEditSetting
	m.editSettingField = editFieldCompactThreshold
	m.editSettingInput.SetValue("")
	m.editSettingInput.Focus()

	m = typeInto(m, "1")
	updated, _ := m.updateEditSetting(keyMsg("esc"))
	m2 := updated.(model)

	if m2.screen != screenConfig {
		t.Fatalf("esperaba volver a screenConfig, quedó en %v", m2.screen)
	}
	if data.CompactThreshold != 777 {
		t.Fatalf("Esc no debería guardar nada, CompactThreshold quedó en %d", data.CompactThreshold)
	}
}

// --- Feature 027: interruptor del módulo Octopus AAR ---

// El módulo nace APAGADO y la fila lo dice. Es la única presencia visible de la
// funcionalidad con el interruptor en off, y es intencional: es el interruptor.
func TestConfigScreen_OctopusNaceApagado(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		width:        100,
		height:       40,
		ready:        true,
	}

	vista := m.configView()

	if !strings.Contains(ansi.Strip(vista), "Octopus AAR: OFF") {
		t.Errorf("la fila debe mostrar el módulo apagado por defecto:\n%s", vista)
	}
}

// Alternar la fila persiste el ajuste y recalcula la lista de auto-aprobación:
// registrar las tools sin pre-aprobarlas las dejaría pidiendo permiso una a una,
// y apagarlas dejaría nombres muertos en la lista.
func TestConfigScreen_OctopusAlternaYPersiste(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		configCursor: configRowOctopus,
		width:        100,
		height:       40,
		ready:        true,
	}

	updated, _ := m.updateConfig(keyMsg("enter"))
	if !data.OctopusEnabled {
		t.Fatal("al confirmar sobre el interruptor el módulo debe encenderse")
	}

	m2 := updated.(model)
	if !strings.Contains(m2.statusMsg, "Octopus AAR activado") {
		t.Errorf("statusMsg = %q, debería confirmar la activación", m2.statusMsg)
	}
	if !strings.Contains(ansi.Strip(m2.configView()), "Octopus AAR: ON") {
		t.Error("la vista debería reflejar el nuevo estado")
	}

	aprobables := map[string]bool{}
	for _, tool := range data.AutoApproveTools {
		aprobables[tool] = true
	}
	for _, tool := range domain.MCPOctopusTools {
		if !aprobables[tool] {
			t.Errorf("al encender el módulo, %q debería quedar pre-aprobada", tool)
		}
	}

	// Y de vuelta: apagar no debe dejar nombres muertos pre-aprobados.
	m2.configCursor = configRowOctopus
	if _, _ = m2.updateConfig(keyMsg("enter")); data.OctopusEnabled {
		t.Fatal("al confirmar de nuevo el módulo debe apagarse")
	}
	for _, tool := range data.AutoApproveTools {
		for _, oct := range domain.MCPOctopusTools {
			if tool == oct {
				t.Errorf("con el módulo apagado %q no debería seguir pre-aprobada", tool)
			}
		}
	}
}
