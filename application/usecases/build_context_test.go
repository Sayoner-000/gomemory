package usecases_test

import (
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// fakeCodeProvider implementa ports.CodeGraphProvider sin tocar ningún binario
// externo: devuelve un snapshot fijo y registra si se disparó el refresco.
type fakeCodeProvider struct {
	snap      domain.CodeProviderSnapshot
	refreshed bool
}

func (f *fakeCodeProvider) Name() string                          { return f.snap.Provider }
func (f *fakeCodeProvider) Snapshot() domain.CodeProviderSnapshot { return f.snap }
func (f *fakeCodeProvider) MaybeRefresh()                         { f.refreshed = true }

var _ ports.CodeGraphProvider = (*fakeCodeProvider)(nil)

func TestBuild_SurfacesUnresolvedConflicts(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	idA, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Redis para cache", Content: "..."})
	if err != nil {
		t.Fatalf("insert memory a: %v", err)
	}
	idB, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Memcached para cache", Content: "..."})
	if err != nil {
		t.Fatalf("insert memory b: %v", err)
	}

	if _, _, err := usecases.RecordVerdict(relRepo, "proj", idA, idB, domain.ConflictsWith, 0.9, "se contradicen"); err != nil {
		t.Fatalf("record verdict: %v", err)
	}

	builder := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(out, "Conflictos sin resolver") {
		t.Fatalf("expected conflicts section in context, got:\n%s", out)
	}
	if !strings.Contains(out, "usa Redis para cache") || !strings.Contains(out, "usa Memcached para cache") {
		t.Fatalf("expected both conflicting titles in context, got:\n%s", out)
	}
}

func TestBuild_NoConflictsSectionWhenResolved(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	idA, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "A", Content: "..."})
	idB, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "B", Content: "..."})

	if _, _, err := usecases.RecordVerdict(relRepo, "proj", idA, idB, domain.NotConflict, 1.0, "verifiqué, no hay conflicto real"); err != nil {
		t.Fatalf("record verdict: %v", err)
	}

	builder := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if strings.Contains(out, "Conflictos sin resolver") {
		t.Fatalf("did not expect conflicts section once resolved, got:\n%s", out)
	}
}

func TestBuild_ExternalGraphSection(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	builder := usecases.New(persistence.NewMemoryRepository(db), persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	fake := &fakeCodeProvider{snap: domain.CodeProviderSnapshot{
		Provider:  "codebase-memory-mcp",
		Available: true,
		Architecture: &domain.CodeArchitecture{
			TotalNodes: 2121,
			TotalEdges: 4462,
			Languages:  []domain.CodeLangStat{{Language: "Go", FileCount: 95}},
			Clusters:   []domain.CodeCluster{{Label: "adapters", Members: 54, Cohesion: 0.94, TopNodes: []string{"IndexProject", "NodesByName"}}},
			Hotspots:   []domain.CodeHotspot{{Name: "FindRoot", FanIn: 10}},
		},
	}}
	builder.CodeProviders = []ports.CodeGraphProvider{fake}

	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, want := range []string{
		"## Grafo de código externo (codebase-memory-mcp)",
		"Grafo estructural indexado: 2121 nodos, 4462 relaciones",
		"Go (95)",
		"**adapters**",
		"IndexProject",
		"FindRoot (fan-in 10)",
		"trace_path", // nota de protocolo / división de trabajo
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("esperaba %q en el contexto, got:\n%s", want, out)
		}
	}
	if !fake.refreshed {
		t.Fatal("Build debería disparar MaybeRefresh (refresco eventual)")
	}
}

// insertLongMemories inserta n memorias de tipo Decision con contenido largo
// (para forzar el presupuesto) y devuelve el repositorio listo para construir.
func longContent() string {
	return strings.Repeat("lorem ipsum dolor sit amet consectetur ", 30) // ~1170 chars
}

func TestBuild_RespetaPresupuestoYConservaConflictos(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	// Par en conflicto (debe sobrevivir al presupuesto, íntegro).
	idA, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Redis para cache", Content: longContent()})
	idB, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Memcached para cache", Content: longContent()})
	if _, _, err := usecases.RecordVerdict(relRepo, "proj", idA, idB, domain.ConflictsWith, 0.9, "se contradicen"); err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	// Muchas memorias largas (títulos ÚNICOS para que el dedup por identidad no
	// las colapse) para exceder el techo sin acotar.
	for i := 0; i < 100; i++ {
		memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: fmt.Sprintf("decisión de relleno %d", i), Content: longContent()})
	}

	builder := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
	builder.Budget = 24000
	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(out) > 24000 {
		t.Fatalf("la salida excede el presupuesto: %d bytes", len(out))
	}
	if !strings.Contains(out, "get_memory") {
		t.Fatalf("esperaba punteros get_memory al truncar, got:\n%s", out[:min(len(out), 1500)])
	}
	// Conflictos intactos pese al presupuesto.
	if !strings.Contains(out, "Conflictos sin resolver") ||
		!strings.Contains(out, "usa Redis para cache") || !strings.Contains(out, "usa Memcached para cache") {
		t.Fatalf("los conflictos deben sobrevivir al presupuesto, got:\n%s", out[:min(len(out), 1500)])
	}
}

func TestBuild_SinLimiteYProyectoPequeno(t *testing.T) {
	// (a) Budget <= 0 (opt-out): sin truncar, contenido largo íntegro.
	t.Run("opt-out sin límite", func(t *testing.T) {
		root := t.TempDir()
		db, _ := persistence.Init(root)
		defer db.Close()
		memRepo := persistence.NewMemoryRepository(db)
		full := longContent()
		for i := 0; i < 30; i++ {
			memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: fmt.Sprintf("d%d", i), Content: full})
		}
		builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
		builder.Budget = -1
		out, _ := builder.Build()
		if !strings.Contains(out, full) {
			t.Fatalf("con Budget<=0 el contenido largo debe ir íntegro (sin truncar)")
		}
	})

	// (b) Proyecto pequeño: contenido total < Budget ⇒ nada truncado, sin punteros.
	t.Run("proyecto pequeño sin truncado", func(t *testing.T) {
		root := t.TempDir()
		db, _ := persistence.Init(root)
		defer db.Close()
		memRepo := persistence.NewMemoryRepository(db)
		memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "chica", Content: "contenido corto y completo"})
		builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
		builder.Budget = 24000
		out, _ := builder.Build()
		if strings.Contains(out, "get_memory") || strings.Contains(out, "…") {
			t.Fatalf("proyecto pequeño no debe truncar ni inyectar punteros, got:\n%s", out)
		}
		if !strings.Contains(out, "contenido corto y completo") {
			t.Fatalf("el contenido corto debe ir íntegro")
		}
	})
}

func TestBuild_ExternalGraphAbsentWhenUnavailable(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	builder := usecases.New(persistence.NewMemoryRepository(db), persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	fake := &fakeCodeProvider{snap: domain.CodeProviderSnapshot{Provider: "codebase-memory-mcp", Available: false}}
	builder.CodeProviders = []ports.CodeGraphProvider{fake}

	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(out, "Grafo de código externo") {
		t.Fatalf("no esperaba bloque de grafo externo cuando no está disponible, got:\n%s", out)
	}
	if !fake.refreshed {
		t.Fatal("aún sin proveedor disponible, MaybeRefresh debe intentarse")
	}
}
