package usecases_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
	"mem/domain"
)

// recordingUsage guarda cada observación de uso; es seguro para uso
// concurrente, como el recorder real.
type recordingUsage struct {
	mu  sync.Mutex
	obs [][2]int
}

func (r *recordingUsage) Record(op string, raw, final int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.obs = append(r.obs, [2]int{raw, final})
}

func (r *recordingUsage) all() [][2]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]int(nil), r.obs...)
}

// switchingLister devuelve el conjunto grande o el pequeño según el paso,
// para comparar una construcción con descartes y otra sin ellos sobre la
// misma instancia.
type switchingLister struct {
	mu    sync.Mutex
	big   []domain.Memory
	small []domain.Memory
	useBy bool
}

func (l *switchingLister) List(project string, limit int) ([]domain.Memory, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.useBy {
		return l.small, nil
	}
	return l.big, nil
}

func (l *switchingLister) setSmall() { l.mu.Lock(); l.useBy = true; l.mu.Unlock() }

func memories037(n, size int) []domain.Memory {
	out := make([]domain.Memory, n)
	for i := range out {
		out[i] = domain.Memory{ID: int64(i + 1), Project: "proj", Type: domain.Decision,
			Title:     fmt.Sprintf("decisión %d", i),
			Content:   strings.Repeat(fmt.Sprintf("contenido %d ", i), size),
			CreatedAt: "2026-10-01 00:00:00", UpdatedAt: "2026-10-01 00:00:00"}
	}
	return out
}

func isolationBuilder(t *testing.T, lister interface {
	List(string, int) ([]domain.Memory, error)
}, rec *recordingUsage) *usecases.Builder {
	t.Helper()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &usecases.Builder{
		Lister:    lister,
		Session:   persistence.NewSessionRepository(db),
		Relations: persistence.NewRelationRepository(db),
		Project:   "proj",
		Budget:    3000,
		Counter:   tokens.ApproximateTokenCounter{},
		Recorder:  rec,
	}
}

type fixedLister []domain.Memory

func (f fixedLister) List(string, int) ([]domain.Memory, error) { return f, nil }

// TestBuild_TresEmisionesIguales cubre US3.1 (FR-009): el contador de
// descartes no se acumula entre llamadas de la misma instancia.
func TestBuild_TresEmisionesIguales(t *testing.T) {
	rec := &recordingUsage{}
	b := isolationBuilder(t, fixedLister(memories037(40, 30)), rec)
	var outs []string
	for i := 0; i < 3; i++ {
		out, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		outs = append(outs, out)
	}
	obs := rec.all()
	if len(obs) != 3 || outs[0] != outs[1] || outs[1] != outs[2] {
		t.Fatalf("contenido distinto entre emisiones")
	}
	if obs[0] != obs[1] || obs[1] != obs[2] {
		t.Fatalf("métricas acumuladas entre emisiones: %v", obs)
	}
	if obs[0][0] <= obs[0][1] {
		t.Fatalf("el caso debe tener descartes (raw > final): %v", obs[0])
	}
}

// TestBuild_SinHerenciaDeDescartes cubre US3.3: una emisión sin descartes
// tras otra con descartes reporta lo mismo que una instancia nueva.
func TestBuild_SinHerenciaDeDescartes(t *testing.T) {
	rec := &recordingUsage{}
	l := &switchingLister{big: memories037(40, 30), small: memories037(2, 1)}
	b := isolationBuilder(t, l, rec)
	if _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	l.setSmall()
	if _, err := b.Build(); err != nil {
		t.Fatal(err)
	}

	fresh := &recordingUsage{}
	if _, err := isolationBuilder(t, fixedLister(memories037(2, 1)), fresh).Build(); err != nil {
		t.Fatal(err)
	}
	got, want := rec.all()[1], fresh.all()[0]
	if got != want {
		t.Fatalf("la segunda emisión heredó descartes: %v, aislada %v", got, want)
	}
}

// TestBuild_OchoConcurrentesIgualesAlAislado cubre US3.2 (FR-010); ejecutar
// con -race para detectar la escritura compartida del contador.
func TestBuild_OchoConcurrentesIgualesAlAislado(t *testing.T) {
	mems := memories037(40, 30)
	isolated := &recordingUsage{}
	wantOut, err := isolationBuilder(t, fixedLister(mems), isolated).Build()
	if err != nil {
		t.Fatal(err)
	}
	want := isolated.all()[0]

	rec := &recordingUsage{}
	shared := isolationBuilder(t, fixedLister(mems), rec)
	var wg sync.WaitGroup
	outs := make([]string, 8)
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outs[i], errs[i] = shared.Build()
		}(i)
	}
	wg.Wait()
	for i := range outs {
		if errs[i] != nil || outs[i] != wantOut {
			t.Fatalf("llamada %d: err=%v contenido distinto del aislado", i, errs[i])
		}
	}
	for _, o := range rec.all() {
		if o != want {
			t.Fatalf("métricas mezcladas: %v, aislada %v (todas: %v)", o, want, rec.all())
		}
	}
}
