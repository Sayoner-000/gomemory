package cli

import (
	"testing"

	"mem/domain"
)

// spyMemoryRepo registra si Insert fue invocado, sin persistir nada real —
// alcanza para verificar que un flag inválido no llega a guardar basura.
type spyMemoryRepo struct {
	inserted []domain.Memory
}

func (s *spyMemoryRepo) Insert(m *domain.Memory) (int64, error) {
	s.inserted = append(s.inserted, *m)
	return int64(len(s.inserted)), nil
}
func (s *spyMemoryRepo) Get(project string, id int64) (*domain.Memory, error) { return nil, nil }
func (s *spyMemoryRepo) UpdateContent(project string, id int64, title, content string) error {
	return nil
}
func (s *spyMemoryRepo) List(project string, limit int) ([]domain.Memory, error) { return nil, nil }
func (s *spyMemoryRepo) ListAll(project string) ([]domain.Memory, error)         { return nil, nil }
func (s *spyMemoryRepo) ImportMemory(m *domain.Memory) (int64, error)            { return 0, nil }
func (s *spyMemoryRepo) Search(project, query string, limit int) ([]domain.Memory, error) {
	return nil, nil
}
func (s *spyMemoryRepo) Delete(project string, id int64) (bool, error) { return false, nil }
func (s *spyMemoryRepo) SecondsSinceLastSave(project string) (int64, bool, error) {
	return 0, false, nil
}

type fakeProjectRepo struct{ root string }

func (f *fakeProjectRepo) FindRoot() (string, error)                           { return f.root, nil }
func (f *fakeProjectRepo) EnsureDir(root string) error                         { return nil }
func (f *fakeProjectRepo) MemDir() string                                      { return "" }
func (f *fakeProjectRepo) DbPath(root string) string                           { return "" }
func (f *fakeProjectRepo) Init(root string) error                              { return nil }
func (f *fakeProjectRepo) Key(root string) string                              { return "demo" }
func (f *fakeProjectRepo) MigrateLegacy(root string, force bool) (bool, error) { return false, nil }

type fakeSessionRepo struct{}

func (f *fakeSessionRepo) Start(project string) (*domain.Session, error)  { return nil, nil }
func (f *fakeSessionRepo) End(id, summary string) error                   { return nil }
func (f *fakeSessionRepo) Active(project string) (*domain.Session, error) { return nil, nil }
func (f *fakeSessionRepo) Recent(project string, limit int) ([]domain.Session, error) {
	return nil, nil
}
func (f *fakeSessionRepo) SetLastPrompt(project, prompt string) error { return nil }

// Regresión: `fs.Parse(args)` no revisaba el error de retorno, así que una
// flag inválida (p.ej. "--title" en vez de "-t", que es lo único soportado)
// no hacía fallar el comando — seguía adelante con los valores default y el
// texto de la flag no reconocida terminaba colándose como parte del
// contenido guardado. Ver README: `mem save` solo soporta flags de una letra
// (-t/-y/-f), que deben ir ANTES del texto.
func TestCmdSave_InvalidFlag_DoesNotInsertGarbage(t *testing.T) {
	repo := &spyMemoryRepo{}
	deps := &Deps{
		MemoryRepo:  repo,
		ProjectRepo: &fakeProjectRepo{root: "/tmp/demo"},
		SessionRepo: &fakeSessionRepo{},
	}

	CmdSave(deps, []string{"--title", "Prueba", "--type", "decision", "contenido de prueba"})

	if len(repo.inserted) != 0 {
		t.Fatalf("una flag inválida no debía llegar a Insert; se guardó %+v", repo.inserted)
	}
}

// Camino feliz: con las flags realmente soportadas (-t/-y/-f) antes del
// texto, el save debe llegar a Insert con los valores esperados.
func TestCmdSave_ValidFlags_InsertsExpectedMemory(t *testing.T) {
	repo := &spyMemoryRepo{}
	deps := &Deps{
		MemoryRepo:  repo,
		ProjectRepo: &fakeProjectRepo{root: "/tmp/demo"},
		SessionRepo: &fakeSessionRepo{},
	}

	CmdSave(deps, []string{"-t", "API REST", "-y", "decision", "Usamos Fiber para el enrutamiento"})

	if len(repo.inserted) != 1 {
		t.Fatalf("esperaba 1 memoria insertada, hubo %d", len(repo.inserted))
	}
	got := repo.inserted[0]
	if got.Title != "API REST" || got.Type != domain.Decision || got.Content != "Usamos Fiber para el enrutamiento" {
		t.Fatalf("memoria insertada no coincide con las flags dadas: %+v", got)
	}
}
