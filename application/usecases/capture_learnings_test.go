package usecases

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mem/domain"
)

type fakeCaptureMemories struct {
	inserted []domain.Memory
	err      error
	calls    int
}

func (f *fakeCaptureMemories) Insert(m *domain.Memory) (int64, error) {
	f.calls++
	if f.err != nil {
		return 0, f.err
	}
	f.inserted = append(f.inserted, *m)
	return int64(len(f.inserted)), nil
}
func (f *fakeCaptureMemories) Get(project string, id int64) (*domain.Memory, error) { return nil, nil }
func (f *fakeCaptureMemories) UpdateContent(project string, id int64, title, content string) error {
	return nil
}
func (f *fakeCaptureMemories) List(project string, limit int) ([]domain.Memory, error) {
	return nil, nil
}
func (f *fakeCaptureMemories) ListAll(project string) ([]domain.Memory, error) { return nil, nil }
func (f *fakeCaptureMemories) ImportMemory(m *domain.Memory) (int64, error)    { return 0, nil }
func (f *fakeCaptureMemories) Search(project, query string, limit int) ([]domain.Memory, error) {
	return nil, nil
}
func (f *fakeCaptureMemories) Delete(project string, id int64) (bool, error) { return false, nil }
func (f *fakeCaptureMemories) SecondsSinceLastSave(project string) (int64, bool, error) {
	return 0, false, nil
}

const learningsExampleText = `Listo.

## Aprendizajes clave
1. El caché se invalida al escribir la configuración del proyecto entero
2. Los hooks de Codex exigen JSON en el evento SubagentStop siempre
`

func TestCaptureLearnings_NItemsProducenNMemoriasConProcedencia(t *testing.T) {
	mems := &fakeCaptureMemories{}
	sessions := &fakeCompactionSessions{active: &domain.Session{ID: "s1", Project: "proj"}}

	n, err := CaptureLearnings(mems, sessions, "proj", learningsExampleText)
	if err != nil {
		t.Fatalf("CaptureLearnings: %v", err)
	}
	if n != 2 {
		t.Fatalf("esperaba 2 ítems capturados, got %d", n)
	}
	if len(mems.inserted) != 2 {
		t.Fatalf("esperaba 2 inserciones, got %d", len(mems.inserted))
	}
	for _, m := range mems.inserted {
		if m.Type != domain.Learning {
			t.Errorf("tipo inesperado: %v", m.Type)
		}
		if !strings.Contains(m.Content, "Procedencia: subagente") {
			t.Errorf("debía indicar procedencia: %q", m.Content)
		}
		if m.SessionID != "s1" {
			t.Errorf("debía asociarse a la sesión activa: %+v", m)
		}
		if !strings.HasPrefix(m.TopicKey, "passive:") {
			t.Errorf("TopicKey debía empezar con 'passive:': %q", m.TopicKey)
		}
	}
}

func TestCaptureLearnings_MismoItemNormalizadoMismoTopico(t *testing.T) {
	mems := &fakeCaptureMemories{}
	sessions := &fakeCompactionSessions{}

	textoA := "## Aprendizajes\n1. El caché se invalida al escribir la configuración del proyecto\n"
	textoB := "## Aprendizajes\n1. EL CACHÉ SE INVALIDA AL ESCRIBIR LA CONFIGURACIÓN DEL PROYECTO.\n"

	if _, err := CaptureLearnings(mems, sessions, "proj", textoA); err != nil {
		t.Fatalf("CaptureLearnings A: %v", err)
	}
	if _, err := CaptureLearnings(mems, sessions, "proj", textoB); err != nil {
		t.Fatalf("CaptureLearnings B: %v", err)
	}
	if len(mems.inserted) != 2 {
		t.Fatalf("esperaba 2 llamadas a Insert (el dedup real ocurre en InsertMemory, no aquí), got %d", len(mems.inserted))
	}
	if mems.inserted[0].TopicKey != mems.inserted[1].TopicKey {
		t.Errorf("mismo ítem normalizado debía producir el mismo TopicKey: %q vs %q",
			mems.inserted[0].TopicKey, mems.inserted[1].TopicKey)
	}
}

func TestCaptureLearnings_SinSeccionNoInsertaNada(t *testing.T) {
	mems := &fakeCaptureMemories{}
	sessions := &fakeCompactionSessions{}

	n, err := CaptureLearnings(mems, sessions, "proj", "Texto normal sin aprendizajes.")
	if err != nil {
		t.Fatalf("CaptureLearnings: %v", err)
	}
	if n != 0 || len(mems.inserted) != 0 {
		t.Errorf("sin sección de aprendizajes no debía insertar nada: n=%d inserted=%d", n, len(mems.inserted))
	}
}

func TestCaptureLearnings_TopeDe10Items(t *testing.T) {
	mems := &fakeCaptureMemories{}
	sessions := &fakeCompactionSessions{}

	var b strings.Builder
	b.WriteString("## Aprendizajes\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "%d. Ítem número %d con longitud suficiente para pasar el filtro de validez\n", i+1, i+1)
	}

	n, err := CaptureLearnings(mems, sessions, "proj", b.String())
	if err != nil {
		t.Fatalf("CaptureLearnings: %v", err)
	}
	if n != 10 {
		t.Fatalf("esperaba el tope de 10 ítems, got %d", n)
	}
}

func TestCaptureLearnings_ActiveErrorDoesNotInsert(t *testing.T) {
	want := errors.New("session unavailable")
	mems := &fakeCaptureMemories{}
	n, err := CaptureLearnings(mems, &fakeCompactionSessions{err: want}, "proj", learningsExampleText)
	if !errors.Is(err, want) || n != 0 || mems.calls != 0 {
		t.Fatalf("n=%d err=%v calls=%d", n, err, mems.calls)
	}
}
func TestCaptureLearnings_InsertErrorStopsCapture(t *testing.T) {
	want := errors.New("insert unavailable")
	mems := &fakeCaptureMemories{err: want}
	n, err := CaptureLearnings(mems, &fakeCompactionSessions{}, "proj", learningsExampleText)
	if !errors.Is(err, want) || n != 0 || mems.calls != 1 {
		t.Fatalf("n=%d err=%v calls=%d", n, err, mems.calls)
	}
}
