package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

var errUnavailable = errors.New("proveedor no disponible (fake de test)")

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsertMemory_RedactsPrivateBlocks(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Learning,
		Title:   "token <private>sk-abc123</private> guardado",
		Content: "usa la key <private>sk-abc123</private> para autenticar",
	}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	mems, err := ListMemories(db, "proj", 10)
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	if len(mems) != 1 || mems[0].ID != id {
		t.Fatalf("expected 1 memory with id %d, got %+v", id, mems)
	}
	if strings.Contains(mems[0].Title, "sk-abc123") || strings.Contains(mems[0].Content, "sk-abc123") {
		t.Fatalf("private content leaked into stored memory: %+v", mems[0])
	}
	if strings.Contains(mems[0].Title, "<private>") || strings.Contains(mems[0].Content, "<private>") {
		t.Fatalf("private tags not stripped: %+v", mems[0])
	}
}

func TestSecondsSinceLastSave(t *testing.T) {
	db := openTestDB(t)

	// Proyecto sin memorias: no hay guardado real.
	if _, exists, err := SecondsSinceLastSave(db, "proj"); err != nil || exists {
		t.Fatalf("proyecto vacío debía dar exists=false, got exists=%v err=%v", exists, err)
	}

	// Solo un checkpoint reciente: no cuenta como guardado real.
	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Checkpoint, Content: "actividad"}); err != nil {
		t.Fatalf("insert checkpoint: %v", err)
	}
	if _, exists, err := SecondsSinceLastSave(db, "proj"); err != nil || exists {
		t.Fatalf("solo checkpoint debía dar exists=false, got exists=%v err=%v", exists, err)
	}

	// Un guardado real retrasado 20 min: existe y el reloj lo refleja.
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Decision, Content: "elegimos X"})
	if err != nil {
		t.Fatalf("insert decision: %v", err)
	}
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now','-5 hours','-20 minutes') WHERE id = ?`, id); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	secs, exists, err := SecondsSinceLastSave(db, "proj")
	if err != nil || !exists {
		t.Fatalf("con guardado real debía dar exists=true, got exists=%v err=%v", exists, err)
	}
	if secs < 900 {
		t.Fatalf("esperaba > 900s desde el guardado real, got %d", secs)
	}

	// Un checkpoint reciente NO debe reiniciar el reloj del guardado real.
	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Checkpoint, Content: "más actividad"}); err != nil {
		t.Fatalf("insert checkpoint 2: %v", err)
	}
	secs2, exists2, err := SecondsSinceLastSave(db, "proj")
	if err != nil || !exists2 {
		t.Fatalf("checkpoint reciente no debía borrar el guardado real, got exists=%v err=%v", exists2, err)
	}
	if secs2 < 900 {
		t.Fatalf("el checkpoint reciente reinició el reloj indebidamente: %d", secs2)
	}
}

// ─── Historia 1 (feature 010): anotación de impacto al guardar ───────────

// fakeImpactProvider implementa ports.CodeGraphProvider solo para ejercitar
// InsertMemory: los métodos que no usa esa ruta (Snapshot/MaybeRefresh) son
// no-op, ImpactFor responde según un mapa fijo de filepath→anotación.
type fakeImpactProvider struct {
	byFile map[string]domain.CodeImpactAnnotation
	called []string
}

func (f *fakeImpactProvider) Name() string                          { return "fake" }
func (f *fakeImpactProvider) Snapshot() domain.CodeProviderSnapshot { return domain.CodeProviderSnapshot{} }
func (f *fakeImpactProvider) MaybeRefresh()                         {}
func (f *fakeImpactProvider) ImpactFor(filepath string) (domain.CodeImpactAnnotation, bool) {
	f.called = append(f.called, filepath)
	ann, ok := f.byFile[filepath]
	return ann, ok
}

var _ ports.CodeGraphProvider = (*fakeImpactProvider)(nil)

func TestInsertMemory_AnnotatesImpact_WhenHotspot(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeImpactProvider{byFile: map[string]domain.CodeImpactAnnotation{
		"adapters/secondary/persistence/memory.go": {Hotspot: true, Symbol: "InsertMemory", FanIn: 12},
	}}
	SetCodeImpactProvider(fake)
	t.Cleanup(func() { SetCodeImpactProvider(nil) })

	id, err := InsertMemory(db, &domain.Memory{
		Project:  "proj",
		Type:     domain.Bugfix,
		Content:  "arreglé el choke point",
		Filepath: "adapters/secondary/persistence/memory.go",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 1 || mems[0].ID != id {
		t.Fatalf("esperaba 1 memoria con id %d, hubo %+v", id, mems)
	}
	if !strings.Contains(mems[0].Content, "arreglé el choke point") {
		t.Fatalf("el contenido original se perdió: %q", mems[0].Content)
	}
	if !strings.Contains(mems[0].Content, "InsertMemory") || !strings.Contains(mems[0].Content, "12") {
		t.Fatalf("esperaba una anotación de impacto mencionando el símbolo y el fan-in, contenido: %q", mems[0].Content)
	}
}

func TestInsertMemory_NoAnnotation_WhenNoProviderConfigured(t *testing.T) {
	db := openTestDB(t)
	SetCodeImpactProvider(nil) // default explícito, por si un test previo dejó algo seteado

	id, err := InsertMemory(db, &domain.Memory{
		Project:  "proj",
		Type:     domain.Bugfix,
		Content:  "contenido sin cambios",
		Filepath: "cualquier/archivo.go",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 1 || mems[0].ID != id {
		t.Fatalf("esperaba 1 memoria con id %d", id)
	}
	if mems[0].Content != "contenido sin cambios" {
		t.Fatalf("sin proveedor configurado, el contenido no debería tocarse: %q", mems[0].Content)
	}
}

func TestInsertMemory_NoAnnotation_WhenProviderHasNoMatch(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeImpactProvider{byFile: map[string]domain.CodeImpactAnnotation{}}
	SetCodeImpactProvider(fake)
	t.Cleanup(func() { SetCodeImpactProvider(nil) })

	id, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Learning, Content: "sin match",
		Filepath: "no/es/hotspot.go",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 1 || mems[0].ID != id || mems[0].Content != "sin match" {
		t.Fatalf("sin match, el contenido no debería tocarse: %+v", mems)
	}
}

func TestInsertMemory_NoImpactLookup_WhenFilepathEmpty(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeImpactProvider{byFile: map[string]domain.CodeImpactAnnotation{}}
	SetCodeImpactProvider(fake)
	t.Cleanup(func() { SetCodeImpactProvider(nil) })

	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Learning, Content: "sin filepath"}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	if len(fake.called) != 0 {
		t.Fatalf("sin filepath, ImpactFor no debería llamarse: %v", fake.called)
	}
}

// ─── Historia 2 (feature 010): export gomemory→ADR en InsertMemory ───────

type fakeADRSyncProvider struct {
	name        string
	getContent  string
	getErr      error
	updateErr   error
	lastUpdated string
	getCalls    int
	updateCalls int
}

func (f *fakeADRSyncProvider) Name() string { return f.name }
func (f *fakeADRSyncProvider) GetDocument(ctx context.Context) (string, error) {
	f.getCalls++
	return f.getContent, f.getErr
}
func (f *fakeADRSyncProvider) UpdateDocument(ctx context.Context, content string) error {
	f.updateCalls++
	f.lastUpdated = content
	return f.updateErr
}

var _ ports.ADRSyncProvider = (*fakeADRSyncProvider)(nil)

func setADRSyncForTest(t *testing.T, db *sql.DB, provider ports.ADRSyncProvider) {
	t.Helper()
	SetADRSync(provider, NewADRSyncRepository(db))
	t.Cleanup(func() { SetADRSync(nil, nil) })
}

func TestInsertMemory_ExportsArchitectureToADR(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeADRSyncProvider{name: "fake-provider", getContent: ""}
	setADRSyncForTest(t, db, fake)

	settingsAdrSyncEnabled = true
	t.Cleanup(func() { settingsAdrSyncEnabled = false })

	id, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Architecture, Title: "Usar SQLite WAL", Content: "concurrencia de escritura",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	if fake.updateCalls != 1 {
		t.Fatalf("esperaba 1 llamada a UpdateDocument, hubo %d", fake.updateCalls)
	}
	if !strings.Contains(fake.lastUpdated, "## ARCHITECTURE") || !strings.Contains(fake.lastUpdated, "Usar SQLite WAL") {
		t.Fatalf("el documento actualizado no contiene el bloque esperado: %q", fake.lastUpdated)
	}

	rec, err := GetADRSyncByMemory(db, "proj", id)
	if err != nil || rec == nil {
		t.Fatalf("esperaba un ADRSyncRecord para la memoria: %v / %v", rec, err)
	}
	if rec.Origin != domain.SyncOriginGomemory || rec.Status != domain.SyncStatusOK || rec.Section != domain.ADRSectionArchitecture {
		t.Fatalf("registro inesperado: %+v", rec)
	}
}

func TestInsertMemory_NoExport_WhenAdrSyncDisabled(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeADRSyncProvider{name: "fake"}
	setADRSyncForTest(t, db, fake)
	// settingsAdrSyncEnabled se queda en false (default) — no se activa.

	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "x"})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	if fake.updateCalls != 0 {
		t.Fatalf("con adr_sync desactivado, no debería llamarse UpdateDocument (hubo %d)", fake.updateCalls)
	}
	if rec, _ := GetADRSyncByMemory(db, "proj", id); rec != nil {
		t.Fatalf("no debería crearse ADRSyncRecord con la capacidad apagada: %+v", rec)
	}
}

func TestInsertMemory_NoExport_ForLearningType(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeADRSyncProvider{name: "fake"}
	setADRSyncForTest(t, db, fake)
	settingsAdrSyncEnabled = true
	t.Cleanup(func() { settingsAdrSyncEnabled = false })

	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Learning, Content: "x"}); err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	if fake.updateCalls != 0 {
		t.Fatalf("type=learning no debería exportarse a ADR (hubo %d llamadas)", fake.updateCalls)
	}
}

func TestInsertMemory_ExportUpdatesExistingBlock_NotDuplicate(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeADRSyncProvider{name: "fake-provider"}
	setADRSyncForTest(t, db, fake)
	settingsAdrSyncEnabled = true
	t.Cleanup(func() { settingsAdrSyncEnabled = false })

	m := &domain.Memory{Project: "proj", Type: domain.Decision, TopicKey: "adr-topic", Title: "Preferir simplicidad", Content: "v1"}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("primer insert: %v", err)
	}
	fake.getContent = fake.lastUpdated // simula que el proveedor ya persistió lo exportado

	// Actualización de la misma memoria (mismo topic_key ⇒ dedup/upsert).
	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Decision, TopicKey: "adr-topic", Title: "Preferir simplicidad", Content: "v2"}); err != nil {
		t.Fatalf("segundo insert: %v", err)
	}

	if fake.updateCalls != 2 {
		t.Fatalf("esperaba 2 llamadas a UpdateDocument (una por guardado), hubo %d", fake.updateCalls)
	}
	recs, err := GetADRSyncByMemory(db, "proj", id)
	if err != nil || recs == nil {
		t.Fatalf("esperaba seguir teniendo un único registro para la memoria %d: %v / %v", id, recs, err)
	}
	if !strings.Contains(fake.lastUpdated, "v2") || strings.Contains(fake.lastUpdated, "v1") {
		t.Fatalf("el bloque debería haberse actualizado, no duplicado: %q", fake.lastUpdated)
	}
}

func TestInsertMemory_ExportPending_WhenProviderUnavailable(t *testing.T) {
	db := openTestDB(t)
	fake := &fakeADRSyncProvider{name: "fake", getErr: errUnavailable}
	setADRSyncForTest(t, db, fake)
	settingsAdrSyncEnabled = true
	t.Cleanup(func() { settingsAdrSyncEnabled = false })

	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "x"})
	if err != nil {
		t.Fatalf("el guardado NO debe fallar aunque el proveedor esté caído: %v", err)
	}
	if fake.updateCalls != 0 {
		t.Fatalf("sin poder leer el documento, no debería intentar escribirlo (hubo %d)", fake.updateCalls)
	}
	rec, err := GetADRSyncByMemory(db, "proj", id)
	if err != nil || rec == nil {
		t.Fatalf("esperaba un registro en estado pending: %v / %v", rec, err)
	}
	if rec.Status != domain.SyncStatusPending {
		t.Fatalf("status = %q, se esperaba pending", rec.Status)
	}
}

func TestGetMemoryByID(t *testing.T) {
	db := openTestDB(t)
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Learning, Title: "t", Content: "c"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := GetMemoryByID(db, "proj", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Content != "c" || got.Title != "t" {
		t.Fatalf("memoria inesperada: %+v", got)
	}
}

func TestGetMemoryByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := GetMemoryByID(db, "proj", 999)
	if err != nil {
		t.Fatalf("no debería error, solo nil: %v", err)
	}
	if got != nil {
		t.Fatalf("esperaba nil, hubo %+v", got)
	}
}

func TestGetMemoryByID_WrongProject(t *testing.T) {
	db := openTestDB(t)
	id, _ := InsertMemory(db, &domain.Memory{Project: "proj-a", Type: domain.Learning, Content: "c"})
	got, err := GetMemoryByID(db, "proj-b", id)
	if err != nil {
		t.Fatalf("no debería error: %v", err)
	}
	if got != nil {
		t.Fatalf("no debería devolver una memoria de otro proyecto: %+v", got)
	}
}

func TestUpdateMemoryContent(t *testing.T) {
	db := openTestDB(t)
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Title: "original", Content: "v1"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := UpdateMemoryContent(db, "proj", id, "actualizado", "v2"); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := GetMemoryByID(db, "proj", id)
	if err != nil || got == nil {
		t.Fatalf("get after update: %v / %v", got, err)
	}
	if got.Title != "actualizado" || got.Content != "v2" {
		t.Fatalf("no se actualizó: %+v", got)
	}
}

func TestUpdateMemoryContent_RedactsSecrets(t *testing.T) {
	db := openTestDB(t)
	id, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "v1"})

	if err := UpdateMemoryContent(db, "proj", id, "t", "token <private>sk-abc123</private>"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := GetMemoryByID(db, "proj", id)
	if strings.Contains(got.Content, "sk-abc123") || strings.Contains(got.Content, "<private>") {
		t.Fatalf("no se redactó: %q", got.Content)
	}
}

func TestUpdateMemoryContent_NotFound(t *testing.T) {
	db := openTestDB(t)
	if err := UpdateMemoryContent(db, "proj", 999, "t", "c"); err == nil {
		t.Fatal("esperaba error al actualizar un id inexistente")
	}
}

func TestInsertMemory_EmptyAfterRedactionFails(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Learning,
		Content: "<private>todo el contenido es secreto</private>",
	}
	if _, err := InsertMemory(db, m); err == nil {
		t.Fatal("expected error when content is empty after redaction, got nil")
	}
}

func TestInsertMemory_NoPrivateBlocksUnaffected(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Decision,
		Title:   "decisión normal",
		Content: "contenido sin secretos",
	}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 1 || mems[0].ID != id || mems[0].Content != "contenido sin secretos" {
		t.Fatalf("unexpected content: %+v", mems)
	}
}

func TestDeleteMemory(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{Project: "proj", Type: domain.Learning, Content: "borrar esto"}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	deleted, err := DeleteMemory(db, "proj", id)
	if err != nil {
		t.Fatalf("delete memory: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 0 {
		t.Fatalf("expected memory to be gone, got %+v", mems)
	}

	deletedAgain, err := DeleteMemory(db, "proj", id)
	if err != nil {
		t.Fatalf("delete memory again: %v", err)
	}
	if deletedAgain {
		t.Fatal("expected deleted=false on second delete")
	}
}

func TestDeleteMemory_ScopedToProject(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{Project: "proj-a", Type: domain.Learning, Content: "de proj-a"}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	deleted, err := DeleteMemory(db, "proj-b", id)
	if err != nil {
		t.Fatalf("delete memory: %v", err)
	}
	if deleted {
		t.Fatal("delete should not cross project scope")
	}

	mems, _ := ListMemories(db, "proj-a", 10)
	if len(mems) != 1 {
		t.Fatalf("memory should still exist in proj-a, got %+v", mems)
	}
}
