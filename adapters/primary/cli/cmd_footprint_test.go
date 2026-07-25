package cli

import (
	"strings"
	"testing"

	"mem/domain"
)

// fakePrefMemRepo implementa ports.MemoryRepository con lo mínimo que
// computePreferenceReinforcement necesita (List); el resto son no-ops, no
// hace falta un mock completo para este caso de uso.
type fakePrefMemRepo struct {
	mems []domain.Memory
}

func (f *fakePrefMemRepo) Insert(m *domain.Memory) (int64, error) { return 0, nil }
func (f *fakePrefMemRepo) Get(project string, id int64) (*domain.Memory, error) {
	return nil, nil
}
func (f *fakePrefMemRepo) UpdateContent(project string, id int64, title, content string) error {
	return nil
}
func (f *fakePrefMemRepo) List(project string, limit int) ([]domain.Memory, error) {
	return f.mems, nil
}
func (f *fakePrefMemRepo) ListAll(project string) ([]domain.Memory, error) { return f.mems, nil }
func (f *fakePrefMemRepo) ImportMemory(m *domain.Memory) (int64, error)    { return 0, nil }
func (f *fakePrefMemRepo) Search(project, query string, limit int) ([]domain.Memory, error) {
	return nil, nil
}
func (f *fakePrefMemRepo) Delete(project string, id int64) (bool, error) { return false, nil }
func (f *fakePrefMemRepo) SecondsSinceLastSave(project string) (int64, bool, error) {
	return 0, false, nil
}

func TestFootprint_AcumulaYResetea(t *testing.T) {
	root := t.TempDir()
	if got := footprintRead(root); got != 0 {
		t.Fatalf("huella inicial debe ser 0, got %d", got)
	}
	footprintAdd(root, 100)
	footprintAdd(root, 50)
	if got := footprintRead(root); got != 150 {
		t.Fatalf("esperaba 150 acumulado, got %d", got)
	}
	footprintReset(root)
	if got := footprintRead(root); got != 0 {
		t.Fatalf("tras reset debe ser 0, got %d", got)
	}
}

func TestComputeCompactNudge_UmbralYDebounce(t *testing.T) {
	// Bajo el umbral: silencio.
	root := t.TempDir()
	footprintAdd(root, 100)
	if _, ok := computeCompactNudge(root, 48000); ok {
		t.Fatal("bajo el umbral no debe recordar")
	}

	// threshold<=0: desactivado aun con huella enorme.
	root2 := t.TempDir()
	footprintAdd(root2, 100000)
	if _, ok := computeCompactNudge(root2, 0); ok {
		t.Fatal("threshold<=0 debe desactivar el recordatorio")
	}

	// Sobre el umbral: recuerda una vez, con mensaje neutral (sin comando de cliente).
	root3 := t.TempDir()
	footprintAdd(root3, 60000)
	msg, ok := computeCompactNudge(root3, 48000)
	if !ok {
		t.Fatal("sobre el umbral debe recordar")
	}
	if strings.Contains(msg, "/compact") || strings.Contains(strings.ToLower(msg), "/clear") {
		t.Fatalf("el recordatorio NO debe nombrar un comando de cliente: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "compact") { // "compactar" en el texto neutral
		t.Fatalf("el recordatorio debería sugerir compactar el contexto: %q", msg)
	}

	// Segundo turno inmediato: debounce → silencio.
	if _, ok := computeCompactNudge(root3, 48000); ok {
		t.Fatal("debounce: no debe repetir de inmediato")
	}
}

func TestComputePreferenceReinforcement(t *testing.T) {
	pref := domain.Memory{ID: 1, Type: domain.Preference, Title: "Español neutro", Content: "Español neutro con tuteo estándar en todo texto."}

	// Bajo el step (threshold/3): silencio, aunque haya preferencias.
	root := t.TempDir()
	footprintAdd(root, 100)
	deps := &Deps{MemoryRepo: &fakePrefMemRepo{mems: []domain.Memory{pref}}}
	if _, ok := computePreferenceReinforcement(deps, root, "proj", 48000); ok {
		t.Fatal("bajo el step no debe reforzar")
	}

	// Sobre el step, sin memorias type=preference: silencio.
	root2 := t.TempDir()
	footprintAdd(root2, 20000) // > 48000/3 = 16000
	deps2 := &Deps{MemoryRepo: &fakePrefMemRepo{mems: []domain.Memory{{ID: 2, Type: domain.Bugfix, Title: "x", Content: "y"}}}}
	if _, ok := computePreferenceReinforcement(deps2, root2, "proj", 48000); ok {
		t.Fatal("sin preferencias guardadas no debe reforzar")
	}

	// Sobre el step, con preferencias: refuerza con el título y contenido REAL
	// (no un recordatorio genérico).
	root3 := t.TempDir()
	footprintAdd(root3, 20000)
	deps3 := &Deps{MemoryRepo: &fakePrefMemRepo{mems: []domain.Memory{pref}}}
	msg, ok := computePreferenceReinforcement(deps3, root3, "proj", 48000)
	if !ok {
		t.Fatal("sobre el step con preferencias debe reforzar")
	}
	if !strings.Contains(msg, pref.Title) || !strings.Contains(msg, pref.Content) {
		t.Fatalf("el refuerzo debe incluir título y contenido real de la preferencia: %q", msg)
	}

	// Debounce: segundo turno inmediato → silencio.
	if _, ok := computePreferenceReinforcement(deps3, root3, "proj", 48000); ok {
		t.Fatal("debounce: no debe repetir de inmediato")
	}

	// threshold<=0 cae al default (48000) y sigue funcionando.
	root4 := t.TempDir()
	footprintAdd(root4, 20000)
	deps4 := &Deps{MemoryRepo: &fakePrefMemRepo{mems: []domain.Memory{pref}}}
	if _, ok := computePreferenceReinforcement(deps4, root4, "proj", 0); !ok {
		t.Fatal("threshold<=0 debe caer al default y seguir reforzando")
	}
}
