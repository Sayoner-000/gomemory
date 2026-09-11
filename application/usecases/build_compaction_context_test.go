package usecases

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mem/domain"
)

type fakeCompactionSessions struct {
	active *domain.Session
	err    error
}

func (f *fakeCompactionSessions) Active(project string) (*domain.Session, error) {
	return f.active, f.err
}
func (f *fakeCompactionSessions) Recent(project string, limit int) ([]domain.Session, error) {
	return nil, nil
}

type fakeCompactionMemories struct {
	mems map[string][]domain.Memory
	err  error
}

func (f *fakeCompactionMemories) ListBySession(project, sessionID string, limit int) ([]domain.Memory, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.mems[sessionID], nil
}

func TestBuildCompactionContext_SinSesionActivaDevuelveVacio(t *testing.T) {
	sessions := &fakeCompactionSessions{active: nil}
	mems := &fakeCompactionMemories{}
	out, err := BuildCompactionContext(sessions, mems, "proj", 24000)
	if err != nil {
		t.Fatalf("BuildCompactionContext: %v", err)
	}
	if out != "" {
		t.Errorf("sin sesión activa debía devolver cadena vacía, got %q", out)
	}
}

func TestBuildCompactionContext_SesionSinMemoriasMuestraNota(t *testing.T) {
	sessions := &fakeCompactionSessions{active: &domain.Session{ID: "s1", Project: "proj"}}
	mems := &fakeCompactionMemories{mems: map[string][]domain.Memory{}}
	out, err := BuildCompactionContext(sessions, mems, "proj", 24000)
	if err != nil {
		t.Fatalf("BuildCompactionContext: %v", err)
	}
	if !strings.Contains(out, domain.CompactionContextHeader) {
		t.Errorf("debía incluir el encabezado: %q", out)
	}
	if !strings.Contains(out, domain.CompactionEmptySessionNote) {
		t.Errorf("sesión sin memorias debía mostrar la nota explícita: %q", out)
	}
}

func TestBuildCompactionContext_OrdenYExtracto(t *testing.T) {
	sessions := &fakeCompactionSessions{active: &domain.Session{ID: "s1", Project: "proj", Summary: "resumen previo corto"}}
	largo := strings.Repeat("contenido bastante largo que debe recortarse a un extracto acotado de trescientos caracteres exactos para no inflar el contexto de compactación entregado tras compactar la conversación con el cliente, cualquiera que sea. ", 3)
	mems := &fakeCompactionMemories{mems: map[string][]domain.Memory{
		"s1": {
			{ID: 3, Type: domain.Discovery, Title: "hallazgo reciente", Content: "contenido corto"},
			{ID: 2, Type: domain.Bugfix, Title: "bug arreglado", Content: largo},
			{ID: 1, Type: domain.Decision, Title: "decisión tomada", Content: "contenido corto"},
		},
	}}
	out, err := BuildCompactionContext(sessions, mems, "proj", 24000)
	if err != nil {
		t.Fatalf("BuildCompactionContext: %v", err)
	}
	if !strings.Contains(out, "resumen previo corto") {
		t.Errorf("debía incluir el resumen previo de la sesión: %q", out)
	}
	// Decisiones y bugfixes primero: "decisión tomada" y "bug arreglado" antes
	// que "hallazgo reciente", aunque este último sea el más reciente (id 3).
	posDecision := strings.Index(out, "decisión tomada")
	posBug := strings.Index(out, "bug arreglado")
	posHallazgo := strings.Index(out, "hallazgo reciente")
	if posDecision < 0 || posBug < 0 || posHallazgo < 0 {
		t.Fatalf("faltan entradas en la salida: %q", out)
	}
	if posDecision >= posHallazgo || posBug >= posHallazgo {
		t.Errorf("decisiones y bugfixes deben ir antes que el resto: %q", out)
	}
	for _, id := range []int64{1, 2, 3} {
		ptr := fmt.Sprintf("get_memory %d", id)
		if !strings.Contains(out, ptr) {
			t.Errorf("debía incluir el puntero %q: %q", ptr, out)
		}
	}
	if strings.Contains(out, largo) {
		t.Error("el contenido largo debía recortarse a un extracto, no aparecer íntegro")
	}
}

func TestBuildCompactionContext_TopeYNotaDeOmitidas(t *testing.T) {
	sessions := &fakeCompactionSessions{active: &domain.Session{ID: "s1", Project: "proj"}}
	var mems []domain.Memory
	for i := int64(1); i <= 50; i++ {
		mems = append(mems, domain.Memory{ID: i, Type: domain.Learning, Title: fmt.Sprintf("aprendizaje %d", i),
			Content: strings.Repeat("relleno ", 40)})
	}
	repo := &fakeCompactionMemories{mems: map[string][]domain.Memory{"s1": mems}}

	budget := 2000 // tope pequeño para forzar el recorte
	out, err := BuildCompactionContext(sessions, repo, "proj", budget)
	if err != nil {
		t.Fatalf("BuildCompactionContext: %v", err)
	}
	if len(out) > budget {
		t.Errorf("la salida (%d chars) no debía superar el tope (%d)", len(out), budget)
	}
	if !strings.Contains(out, "entradas más de esta sesión") {
		t.Errorf("debía indicar cuántas entradas quedaron fuera: %q", out)
	}
}

// TestBuildCompactionContext_TopePriorizaAccionables cubre S-006 (ACR feature
// 030): cuando el tope obliga a recortar, las decisiones y bugfixes deben
// sobrevivir aunque sean las memorias más antiguas (van al final de
// ListBySession, que devuelve de la más reciente a la más antigua) y lo
// recortado deben ser las no accionables.
func TestBuildCompactionContext_TopePriorizaAccionables(t *testing.T) {
	sessions := &fakeCompactionSessions{active: &domain.Session{ID: "s1", Project: "proj"}}
	var mems []domain.Memory
	for i := int64(50); i >= 3; i-- {
		mems = append(mems, domain.Memory{ID: i, Type: domain.Learning, Title: fmt.Sprintf("aprendizaje %d", i),
			Content: strings.Repeat("relleno ", 40)})
	}
	mems = append(mems,
		domain.Memory{ID: 2, Type: domain.Bugfix, Title: "bug antiguo", Content: "causa raíz"},
		domain.Memory{ID: 1, Type: domain.Decision, Title: "decisión antigua", Content: "motivo"},
	)
	repo := &fakeCompactionMemories{mems: map[string][]domain.Memory{"s1": mems}}

	out, err := BuildCompactionContext(sessions, repo, "proj", 2000)
	if err != nil {
		t.Fatalf("BuildCompactionContext: %v", err)
	}
	if !strings.Contains(out, "entradas más de esta sesión") {
		t.Fatalf("precondición: el tope debía forzar el recorte: %q", out)
	}
	for _, ptr := range []string{"(get_memory 1)", "(get_memory 2)"} {
		if !strings.Contains(out, ptr) {
			t.Errorf("la memoria accionable %q debía sobrevivir al recorte: %q", ptr, out)
		}
	}
	if strings.Contains(out, "(get_memory 3)") {
		t.Errorf("el recorte debía caer sobre los aprendizajes más antiguos, no sobre lo accionable: %q", out)
	}
}

func TestBuildCompactionContext_SinTopeConBudgetCero(t *testing.T) {
	sessions := &fakeCompactionSessions{active: &domain.Session{ID: "s1", Project: "proj"}}
	mems := &fakeCompactionMemories{mems: map[string][]domain.Memory{
		"s1": {{ID: 1, Type: domain.Learning, Title: "t", Content: "c"}},
	}}
	out, err := BuildCompactionContext(sessions, mems, "proj", 0)
	if err != nil {
		t.Fatalf("BuildCompactionContext: %v", err)
	}
	if !strings.Contains(out, "get_memory 1") {
		t.Errorf("budget<=0 debía incluir todo sin recortar: %q", out)
	}
}

func TestBuildCompactionContext_ActiveError(t *testing.T) {
	want := errors.New("session unavailable")
	out, err := BuildCompactionContext(&fakeCompactionSessions{err: want}, nil, "proj", 24000)
	if out != "" || !errors.Is(err, want) {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
