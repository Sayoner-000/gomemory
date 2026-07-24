package usecases_test

import (
	"context"
	"errors"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// ─── fakes ─────────────────────────────────────────────────────────────

type fakeADRDocProvider struct {
	name    string
	content string
	getErr  error
}

func (f *fakeADRDocProvider) Name() string { return f.name }
func (f *fakeADRDocProvider) GetDocument(ctx context.Context) (string, error) {
	return f.content, f.getErr
}
func (f *fakeADRDocProvider) UpdateDocument(ctx context.Context, content string) error { return nil }

var _ ports.ADRSyncProvider = (*fakeADRDocProvider)(nil)

type fakeADRSyncRepo struct {
	byMemory map[int64]*domain.ADRSyncRecord
	byBlock  map[string]*domain.ADRSyncRecord
	nextID   int64
}

func newFakeADRSyncRepo() *fakeADRSyncRepo {
	return &fakeADRSyncRepo{byMemory: map[int64]*domain.ADRSyncRecord{}, byBlock: map[string]*domain.ADRSyncRecord{}}
}

func (f *fakeADRSyncRepo) Insert(r *domain.ADRSyncRecord) (int64, error) {
	f.nextID++
	cp := *r
	cp.ID = f.nextID
	if cp.MemoryID != nil {
		f.byMemory[*cp.MemoryID] = &cp
	}
	f.byBlock[cp.Provider+"|"+cp.BlockKey] = &cp
	return cp.ID, nil
}

func (f *fakeADRSyncRepo) GetByMemory(project string, memoryID int64) (*domain.ADRSyncRecord, error) {
	if r, ok := f.byMemory[memoryID]; ok {
		return r, nil
	}
	return nil, nil
}

func (f *fakeADRSyncRepo) GetByBlockKey(project, provider, blockKey string) (*domain.ADRSyncRecord, error) {
	if r, ok := f.byBlock[provider+"|"+blockKey]; ok {
		return r, nil
	}
	return nil, nil
}

func (f *fakeADRSyncRepo) UpdateStatus(id int64, status domain.SyncStatus, contentHash string) error {
	for _, r := range f.byMemory {
		if r.ID == id {
			r.Status, r.ContentHash = status, contentHash
		}
	}
	for _, r := range f.byBlock {
		if r.ID == id {
			r.Status, r.ContentHash = status, contentHash
		}
	}
	return nil
}

func (f *fakeADRSyncRepo) ListByProject(project string) ([]domain.ADRSyncRecord, error) {
	var out []domain.ADRSyncRecord
	for _, r := range f.byBlock {
		if r.Project == project {
			out = append(out, *r)
		}
	}
	return out, nil
}

var _ ports.ADRSyncRepository = (*fakeADRSyncRepo)(nil)

type fakeMemRepo struct {
	byID   map[int64]*domain.Memory
	nextID int64
}

func newFakeMemRepo() *fakeMemRepo { return &fakeMemRepo{byID: map[int64]*domain.Memory{}} }

func (f *fakeMemRepo) Insert(m *domain.Memory) (int64, error) { return f.ImportMemory(m) }
func (f *fakeMemRepo) Get(project string, id int64) (*domain.Memory, error) {
	m, ok := f.byID[id]
	if !ok || m.Project != project {
		return nil, nil
	}
	cp := *m
	return &cp, nil
}
func (f *fakeMemRepo) List(project string, limit int) ([]domain.Memory, error) { return nil, nil }
func (f *fakeMemRepo) ListAll(project string) ([]domain.Memory, error)         { return nil, nil }
func (f *fakeMemRepo) ImportMemory(m *domain.Memory) (int64, error) {
	f.nextID++
	cp := *m
	cp.ID = f.nextID
	f.byID[cp.ID] = &cp
	return cp.ID, nil
}
func (f *fakeMemRepo) Search(project, query string, limit int) ([]domain.Memory, error) { return nil, nil }
func (f *fakeMemRepo) Delete(project string, id int64) (bool, error)                    { return false, nil }
func (f *fakeMemRepo) SecondsSinceLastSave(project string) (int64, bool, error)          { return 0, false, nil }
func (f *fakeMemRepo) UpdateContent(project string, id int64, title, content string) error {
	m, ok := f.byID[id]
	if !ok {
		return errors.New("not found")
	}
	m.Title, m.Content = title, content
	return nil
}

var _ ports.MemoryRepository = (*fakeMemRepo)(nil)

// ─── tests ─────────────────────────────────────────────────────────────

const sampleDocForImport = `## ARCHITECTURE

<!-- gomemory:id=42 -->
### Ya exportada desde gomemory
no debe reimportarse

### Escrita directo en el proveedor
contenido nuevo sin marcador
`

func TestImportADRs_NewUnmarkedBlock_CreatesMemory(t *testing.T) {
	provider := &fakeADRDocProvider{name: "prov", content: sampleDocForImport}
	adrRepo := newFakeADRSyncRepo()
	memRepo := newFakeMemRepo()

	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("import: %v", err)
	}

	if len(memRepo.byID) != 1 {
		t.Fatalf("esperaba 1 memoria importada (el bloque marcado NO se reimporta), hubo %d", len(memRepo.byID))
	}
	var imported *domain.Memory
	for _, m := range memRepo.byID {
		imported = m
	}
	if imported.Type != domain.Architecture || imported.Title != "Escrita directo en el proveedor" {
		t.Fatalf("memoria importada inesperada: %+v", imported)
	}
	rec, _ := adrRepo.GetByMemory("proj", imported.ID)
	if rec == nil || rec.Origin != domain.SyncOriginProvider || rec.Status != domain.SyncStatusOK {
		t.Fatalf("registro inesperado: %+v", rec)
	}
}

func TestImportADRs_MarkedBlock_NeverReimported(t *testing.T) {
	provider := &fakeADRDocProvider{name: "prov", content: `## ARCHITECTURE

<!-- gomemory:id=7 -->
### Solo gomemory
contenido
`}
	adrRepo := newFakeADRSyncRepo()
	memRepo := newFakeMemRepo()

	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(memRepo.byID) != 0 {
		t.Fatalf("un bloque marcado (origen gomemory) nunca debería crear una memoria nueva, hubo %d", len(memRepo.byID))
	}
}

func TestImportADRs_UnchangedBlock_NoDuplicateNoUpdate(t *testing.T) {
	provider := &fakeADRDocProvider{name: "prov", content: sampleDocForImport}
	adrRepo := newFakeADRSyncRepo()
	memRepo := newFakeMemRepo()

	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("primer import: %v", err)
	}
	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("segundo import: %v", err)
	}

	if len(memRepo.byID) != 1 {
		t.Fatalf("un segundo import con el mismo contenido no debería duplicar memorias, hubo %d", len(memRepo.byID))
	}
}

func TestImportADRs_ProviderChanged_LocalUntouched_UpdatesMemory(t *testing.T) {
	provider := &fakeADRDocProvider{name: "prov", content: sampleDocForImport}
	adrRepo := newFakeADRSyncRepo()
	memRepo := newFakeMemRepo()

	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("primer import: %v", err)
	}

	provider.content = `## ARCHITECTURE

<!-- gomemory:id=42 -->
### Ya exportada desde gomemory
no debe reimportarse

### Escrita directo en el proveedor
CONTENIDO ACTUALIZADO por el proveedor
`
	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("segundo import: %v", err)
	}

	if len(memRepo.byID) != 1 {
		t.Fatalf("un cambio del lado del proveedor no debería duplicar la memoria, hubo %d", len(memRepo.byID))
	}
	var got *domain.Memory
	for _, m := range memRepo.byID {
		got = m
	}
	if got.Content != "CONTENIDO ACTUALIZADO por el proveedor" {
		t.Fatalf("la memoria no se actualizó con el cambio del proveedor: %q", got.Content)
	}
}

func TestImportADRs_LocalEdit_ConflictResolved_KeepsLocal(t *testing.T) {
	provider := &fakeADRDocProvider{name: "prov", content: sampleDocForImport}
	adrRepo := newFakeADRSyncRepo()
	memRepo := newFakeMemRepo()

	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("primer import: %v", err)
	}

	var importedID int64
	for id := range memRepo.byID {
		importedID = id
	}
	// Edición local (una persona editó la memoria importada en gomemory).
	memRepo.byID[importedID].Content = "edición local, no sincronizada aún"

	provider.content = `## ARCHITECTURE

<!-- gomemory:id=42 -->
### Ya exportada desde gomemory
no debe reimportarse

### Escrita directo en el proveedor
el proveedor TAMBIÉN cambió, al mismo tiempo
`
	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err != nil {
		t.Fatalf("segundo import: %v", err)
	}

	got := memRepo.byID[importedID]
	if got.Content != "edición local, no sincronizada aún" {
		t.Fatalf("la edición local no debería perderse silenciosamente: %q", got.Content)
	}
	rec, _ := adrRepo.GetByMemory("proj", importedID)
	if rec == nil || rec.Status != domain.SyncStatusConflictResolved {
		t.Fatalf("esperaba status=conflict_resolved, hubo %+v", rec)
	}
}

func TestImportADRs_ProviderUnavailable_ReturnsError(t *testing.T) {
	provider := &fakeADRDocProvider{name: "prov", getErr: errors.New("caído")}
	adrRepo := newFakeADRSyncRepo()
	memRepo := newFakeMemRepo()

	if err := usecases.ImportADRs(context.Background(), provider, adrRepo, memRepo, "proj"); err == nil {
		t.Fatal("esperaba error cuando el proveedor no está disponible (el llamador decide degradar en silencio)")
	}
	if len(memRepo.byID) != 0 {
		t.Fatal("no debería crear ninguna memoria si no se pudo leer el documento")
	}
}
