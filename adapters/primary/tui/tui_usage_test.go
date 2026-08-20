package tui

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// insertRawTopicDup inserta una fila directamente en SQL, evitando el upsert
// de findDuplicate — mismo helper que cmd_consolidate_test.go, duplicado
// aquí porque los paquetes cli y tui no comparten test helpers.
func insertRawTopicDup(t *testing.T, db *sql.DB, project, topicKey, content string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO memories (project, type, title, content, topic_key) VALUES (?, 'decision', 'rev', ?, ?)`,
		project, content, topicKey,
	); err != nil {
		t.Fatalf("insertRawTopicDup: %v", err)
	}
}

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// fakeUsageSessionRepo implementa ports.SessionRepository con una sesión
// activa fija (o ninguna), para probar la sección [1] sin tocar SQLite.
type fakeUsageSessionRepo struct {
	active *domain.Session
}

func (f fakeUsageSessionRepo) Start(project string) (*domain.Session, error) { return f.active, nil }
func (f fakeUsageSessionRepo) End(id, summary string) error                  { return nil }
func (f fakeUsageSessionRepo) Active(project string) (*domain.Session, error) {
	return f.active, nil
}
func (f fakeUsageSessionRepo) Recent(project string, limit int) ([]domain.Session, error) {
	return nil, nil
}
func (f fakeUsageSessionRepo) SetLastPrompt(project, prompt string) error { return nil }

// fakeUsageRepoStub implementa ports.UsageRepository en memoria.
type fakeUsageRepoStub struct {
	bySession map[string][]domain.UsageRecord
}

func (f *fakeUsageRepoStub) Record(rec domain.UsageRecord) error { return nil }
func (f *fakeUsageRepoStub) BySession(project, sessionID string) ([]domain.UsageRecord, error) {
	return f.bySession[sessionID], nil
}
func (f *fakeUsageRepoStub) Sessions(project string, limit int) ([]string, error) {
	var ids []string
	for id := range f.bySession {
		ids = append(ids, id)
	}
	return ids, nil
}
func (f *fakeUsageRepoStub) Totals(project string) ([]domain.UsageRecord, error) {
	var all []domain.UsageRecord
	for _, recs := range f.bySession {
		all = append(all, recs...)
	}
	return all, nil
}

func TestUpdateList_UKey_OpensUsageScreenAndMatchesFormatter(t *testing.T) {
	sessRepo := fakeUsageSessionRepo{active: &domain.Session{ID: "sess-1", Project: "proj"}}
	usageRepo := &fakeUsageRepoStub{bySession: map[string][]domain.UsageRecord{
		"sess-1": {
			{Project: "proj", SessionID: "sess-1", Operation: domain.OpBuildContext, Channel: "mcp", BaselineTokens: 6000, EmittedTokens: 3500},
			{Project: "proj", SessionID: "sess-1", Operation: domain.OpSaveMemory, Channel: "cli", BaselineTokens: 320, EmittedTokens: 310},
		},
	}}

	m := model{
		screen:           screenList,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		sessionRepo:      sessRepo,
		usageRepo:        usageRepo,
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
	}

	updated, _ := m.updateList(keyMsg("u"))
	got := updated.(model)

	if got.screen != screenUsage {
		t.Fatalf("la tecla u debe abrir screenUsage, got screen=%v", got.screen)
	}

	// La sección [1] debe coincidir cifra por cifra con lo que produciría
	// `mem usage` para la misma sesión (SC-006): misma llamada a
	// usecases.BuildUsageReport, mismos argumentos.
	want, err := usecases.BuildUsageReport(usageRepo, "proj", "sess-1", 0)
	if err != nil {
		t.Fatalf("BuildUsageReport de referencia: %v", err)
	}
	if got.usageReport.BaselineTokens != want.BaselineTokens || got.usageReport.EmittedTokens != want.EmittedTokens {
		t.Fatalf("usageReport no coincide: got=%+v want=%+v", got.usageReport, want)
	}
	if got.usageReport.Calls != 2 {
		t.Fatalf("Calls = %d, want 2", got.usageReport.Calls)
	}

	view := got.usageView()
	if !strings.Contains(view, "Sesión actual") {
		t.Fatalf("la vista debe incluir la sección [1], got:\n%s", view)
	}
	if !strings.Contains(view, "Snapshot puntual") {
		t.Fatalf("la vista debe incluir la sección [2], got:\n%s", view)
	}
	if !strings.Contains(view, "Aproximación neutral") && !strings.Contains(view, "aproximado") {
		t.Fatalf("la vista debe declarar el método de conteo, got:\n%s", view)
	}
}

func TestUsageView_NoActiveSession_ShowsEmptyReport(t *testing.T) {
	usageRepo := &fakeUsageRepoStub{bySession: map[string][]domain.UsageRecord{}}
	m := model{
		screen:           screenList,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		sessionRepo:      fakeUsageSessionRepo{active: nil},
		usageRepo:        usageRepo,
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
	}

	updated, _ := m.updateList(keyMsg("u"))
	got := updated.(model)

	if got.usageScope != "empty" {
		t.Fatalf("scope = %q, want empty", got.usageScope)
	}
	if got.usageReport.Calls != 0 {
		t.Fatalf("sin actividad, Calls debe ser 0, got %d", got.usageReport.Calls)
	}
	if !strings.Contains(got.usageView(), "Sin actividad registrada") {
		t.Fatalf("se esperaba el mensaje de sin actividad, got:\n%s", got.usageView())
	}
}

func TestUsageView_FitsWithinRealisticTerminalHeight(t *testing.T) {
	usageRepo := &fakeUsageRepoStub{bySession: map[string][]domain.UsageRecord{
		"sess-1": {
			{Project: "proj", SessionID: "sess-1", Operation: domain.OpBuildContext, Channel: "mcp", BaselineTokens: 6000, EmittedTokens: 3500},
			{Project: "proj", SessionID: "sess-1", Operation: domain.OpSearchMemories, Channel: "mcp", BaselineTokens: 1800, EmittedTokens: 1500},
			{Project: "proj", SessionID: "sess-1", Operation: domain.OpSaveMemory, Channel: "cli", BaselineTokens: 320, EmittedTokens: 310},
		},
	}}
	const terminalHeight = 30
	m := model{
		screen:           screenList,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		sessionRepo:      fakeUsageSessionRepo{active: &domain.Session{ID: "sess-1", Project: "proj"}},
		usageRepo:        usageRepo,
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           terminalHeight,
		ready:            true,
	}

	updated, _ := m.updateList(keyMsg("u"))
	got := updated.(model)

	view := got.usageView()
	lines := strings.Count(view, "\n") + 1
	if lines > terminalHeight {
		t.Fatalf("la pantalla de uso con datos realistas (3 registros) ocupa %d líneas, excede la altura de terminal %d", lines, terminalHeight)
	}
}

func TestUpdateUsage_EmptyTask_ValidationError(t *testing.T) {
	m := model{
		screen:           screenUsage,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
	}
	m.usageBudgetInput.SetValue("4000")

	updated, _ := m.updateUsage(keyMsg("enter"))
	got := updated.(model)

	if got.usageSnapshotErr == "" {
		t.Fatalf("con tarea vacía se esperaba un error de validación")
	}
	if got.usageSnapshot != nil {
		t.Fatalf("con tarea vacía no debe calcularse ningún snapshot")
	}
}

func TestUpdateUsage_InvalidBudget_ValidationError(t *testing.T) {
	m := model{
		screen:           screenUsage,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
	}
	m.usageTaskInput.SetValue("optimizar el flujo de guardado")
	m.usageBudgetInput.SetValue("no-es-un-numero")

	updated, _ := m.updateUsage(keyMsg("enter"))
	got := updated.(model)

	if got.usageSnapshotErr == "" {
		t.Fatalf("con presupuesto inválido se esperaba un error de validación")
	}
}

func TestUpdateUsage_Esc_ReturnsToListAndDoesNotPersistSnapshot(t *testing.T) {
	m := model{
		screen:           screenUsage,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
	}
	m.usageSnapshot = &domain.ContextPack{Stats: domain.ContextStats{RawTokens: 100, FinalTokens: 50}}

	updated, _ := m.updateUsage(keyMsg("esc"))
	got := updated.(model)

	if got.screen != screenList {
		t.Fatalf("esc debe volver a screenList, got %v", got.screen)
	}

	// Reentrar a la pantalla (como haría el usuario con la tecla u) debe
	// arrancar sin rastro del snapshot anterior (FR-023, SC-007).
	reentered, _ := got.updateList(keyMsg("u"))
	final := reentered.(model)
	if final.usageSnapshot != nil {
		t.Fatalf("el snapshot no debe sobrevivir a salir y volver a entrar a la pantalla")
	}
}

// fakeMaintenanceRepo implementa ports.MaintenanceRepository lo mínimo para
// que updateMaintenance no falle al pedir Stats().
type fakeMaintenanceRepo struct{}

func (f fakeMaintenanceRepo) Stats(project string) (ports.StorageStats, error) {
	return ports.StorageStats{}, nil
}
func (f fakeMaintenanceRepo) Purge(filter ports.PurgeFilter) (int64, error) { return 0, nil }
func (f fakeMaintenanceRepo) Compact() (int64, int64, error)             { return 0, 0, nil }

func TestUpdateMaintenance_ConsolidateRow_NoGroups_ShowsStatusWithoutLeavingScreen(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	m := model{
		screen:          screenMaintenance,
		project:         "proj",
		memRepo:         persistence.NewMemoryRepository(db),
		maintenanceRepo: fakeMaintenanceRepo{},
		maintCursor:     3, // "Consolidar"
		maintConfirm:    textinput.New(),
		ready:           true,
	}

	updated, _ := m.updateMaintenance(keyMsg("enter"))
	got := updated.(model)

	if got.screen != screenMaintenance {
		t.Fatalf("sin grupos consolidables no debe pasar a la confirmación, got screen=%v", got.screen)
	}
	if got.statusMsg == "" {
		t.Fatalf("se esperaba un mensaje de estado explicando que no hay nada consolidable")
	}
}

func TestUpdateMaintenance_ConsolidateRow_WithGroups_GoesToConfirmAndApplies(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	insertRawTopicDup(t, db, "proj", "same-topic", "v1")
	insertRawTopicDup(t, db, "proj", "same-topic", "v2")

	memRepo := persistence.NewMemoryRepository(db)
	m := model{
		screen:          screenMaintenance,
		project:         "proj",
		memRepo:         memRepo,
		maintenanceRepo: fakeMaintenanceRepo{},
		maintCursor:     3,
		maintConfirm:    textinput.New(),
		ready:           true,
	}

	updated, _ := m.updateMaintenance(keyMsg("enter"))
	afterSelect := updated.(model)
	if afterSelect.screen != screenMaintenanceConfirm {
		t.Fatalf("con grupos consolidables debe pedir confirmación, got screen=%v", afterSelect.screen)
	}
	if afterSelect.maintAction != "consolidate" {
		t.Fatalf("maintAction = %q, want consolidate", afterSelect.maintAction)
	}

	afterSelect.maintConfirm.SetValue("proj")
	updated2, _ := afterSelect.updateMaintenanceConfirm(keyMsg("enter"))
	final := updated2.(model)

	if final.screen != screenMaintenance {
		t.Fatalf("tras confirmar debe volver a screenMaintenance, got %v", final.screen)
	}
	mems, _ := memRepo.ListAll("proj")
	if len(mems) != 1 {
		t.Fatalf("tras aplicar debe quedar 1 fila, got %d", len(mems))
	}
}
