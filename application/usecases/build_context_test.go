package usecases_test

import (
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// fakeCodeProvider implementa ports.CodeGraphProvider sin tocar ningún binario
// externo: devuelve un snapshot fijo y registra si se disparó el refresco.
type fakeCodeProvider struct {
	snap      domain.CodeProviderSnapshot
	refreshed bool
	// impactByFile permite configurar el resultado de ImpactFor por ruta,
	// para probar la sección "Memoria conectada a código activo" de Build()
	// sin depender de un proveedor real. Ruta ausente = sin match (false).
	impactByFile map[string]domain.CodeImpactAnnotation
}

func (f *fakeCodeProvider) Name() string                          { return f.snap.Provider }
func (f *fakeCodeProvider) Snapshot() domain.CodeProviderSnapshot { return f.snap }
func (f *fakeCodeProvider) MaybeRefresh()                         { f.refreshed = true }

func (f *fakeCodeProvider) ImpactFor(filepath string) (domain.CodeImpactAnnotation, bool) {
	ann, ok := f.impactByFile[filepath]
	return ann, ok
}

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

// TestBuild_HotCodeSection_MatchAparece verifica que una memoria cuyo
// Filepath resuelve a un hotspot vigente del grafo externo aparece en la
// nueva sección "Memoria conectada a código activo" — la relación se
// recalcula en cada Build() contra ImpactFor, no queda congelada como la
// anotación estática de InsertMemory.
func TestBuild_HotCodeSection_MatchAparece(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Bugfix, Title: "fix parser de rutas",
		Content: "...", Filepath: "adapters/secondary/persistence/memory.go",
	})
	memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision, Title: "no toca hotspot",
		Content: "...", Filepath: "docs/README.md",
	})

	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	fake := &fakeCodeProvider{
		snap: domain.CodeProviderSnapshot{Provider: "codebase-memory-mcp", Available: true, Architecture: &domain.CodeArchitecture{}},
		impactByFile: map[string]domain.CodeImpactAnnotation{
			"adapters/secondary/persistence/memory.go": {Hotspot: true, Symbol: "InsertMemory", FanIn: 41},
		},
	}
	builder.CodeProviders = []ports.CodeGraphProvider{fake}

	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(out, "Memoria conectada a código activo") {
		t.Fatalf("esperaba la sección de hotspots en vivo, got:\n%s", out)
	}
	if !strings.Contains(out, "fix parser de rutas") || !strings.Contains(out, "fan-in 41") {
		t.Fatalf("esperaba la memoria con match y su fan-in, got:\n%s", out)
	}
	// "no toca hotspot" SÍ aparece en el output (en su propia sección de
	// Decisiones Técnicas, legítimamente) — lo que no debe hacer es aparecer
	// DENTRO de la sección nueva de hotspots.
	start := strings.Index(out, "## 🔥 Memoria conectada a código activo")
	if start < 0 {
		t.Fatalf("no se encontró la sección de hotspots en vivo, got:\n%s", out)
	}
	section := out[start:]
	if end := strings.Index(section[len("## 🔥 Memoria conectada a código activo"):], "\n## "); end >= 0 {
		section = section[:len("## 🔥 Memoria conectada a código activo")+end]
	}
	if strings.Contains(section, "no toca hotspot") {
		t.Fatalf("una memoria sin match no debe aparecer en la sección de hotspots, got sección:\n%s", section)
	}
}

// TestBuild_HotCodeSection_AusenteSinMatchNiProveedor cubre dos casos donde
// la sección nueva no debe aparecer: sin ningún match de hotspot, y sin
// CodeProviders configurados (no debe romper nada).
func TestBuild_HotCodeSection_AusenteSinMatchNiProveedor(t *testing.T) {
	t.Run("proveedor presente pero sin match", func(t *testing.T) {
		root := t.TempDir()
		db, _ := persistence.Init(root)
		defer db.Close()
		memRepo := persistence.NewMemoryRepository(db)
		memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Bugfix, Title: "algo", Content: "...", Filepath: "no/existe.go"})

		builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
		fake := &fakeCodeProvider{snap: domain.CodeProviderSnapshot{Provider: "codebase-memory-mcp", Available: true, Architecture: &domain.CodeArchitecture{}}}
		builder.CodeProviders = []ports.CodeGraphProvider{fake}

		out, err := builder.Build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if strings.Contains(out, "Memoria conectada a código activo") {
			t.Fatalf("sin ningún match no debe aparecer la sección, got:\n%s", out)
		}
	})

	t.Run("sin CodeProviders configurados", func(t *testing.T) {
		root := t.TempDir()
		db, _ := persistence.Init(root)
		defer db.Close()
		memRepo := persistence.NewMemoryRepository(db)
		memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Bugfix, Title: "algo", Content: "...", Filepath: "cualquier/archivo.go"})

		builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
		out, err := builder.Build()
		if err != nil {
			t.Fatalf("build sin CodeProviders no debe fallar: %v", err)
		}
		if strings.Contains(out, "Memoria conectada a código activo") {
			t.Fatalf("sin CodeProviders no debe aparecer la sección, got:\n%s", out)
		}
	})
}

// fakeUsageRecorder implementa ports.UsageRecorder para inspeccionar qué
// registró Build() sin pasar por ningún canal real (MCP, CLI o TUI) — cubre
// la corrección V1 del agnosticismo (research.md §1): el registro nace en el
// caso de uso, no en un adaptador de protocolo concreto.
type fakeUsageRecorder struct {
	calls []struct {
		operation string
		baseline  int
		emitted   int
	}
}

func (f *fakeUsageRecorder) Record(operation string, baselineTokens, emittedTokens int) {
	f.calls = append(f.calls, struct {
		operation string
		baseline  int
		emitted   int
	}{operation, baselineTokens, emittedTokens})
}

func TestBuild_TightBudget_RecordsBaselineGreaterThanEmitted(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	// Contenido largo a propósito: debe superar entryExtractChars (200) para
	// forzar la truncación en acota(), y el conjunto debe superar el
	// presupuesto para forzar además el descarte en fits().
	long := strings.Repeat("contenido largo de sobra para forzar el truncado del extracto. ", 20)
	for i := 0; i < 6; i++ {
		memRepo.Insert(&domain.Memory{
			Project: "proj", Type: domain.Decision,
			Title: fmt.Sprintf("decisión %d", i), Content: long,
		})
	}

	rec := &fakeUsageRecorder{}
	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	builder.Budget = 600 // deliberadamente bajo: fuerza truncación Y descarte
	builder.Recorder = rec
	builder.Counter = tokens.ApproximateTokenCounter{}

	if _, err := builder.Build(); err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("se esperaba exactamente 1 registro (sin pasar por MCP/CLI/TUI), got %d", len(rec.calls))
	}
	call := rec.calls[0]
	if call.operation != domain.OpBuildContext {
		t.Fatalf("operation = %q, want %q", call.operation, domain.OpBuildContext)
	}
	if call.baseline <= call.emitted {
		t.Fatalf("con Budget bajo, baseline (%d) debe ser mayor que emitted (%d)", call.baseline, call.emitted)
	}
}

func TestBuild_UnlimitedBudget_BaselineNeverBelowEmitted(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)
	memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "d", Content: "contenido corto"})

	rec := &fakeUsageRecorder{}
	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	// Budget <= 0 = sin límite (comportamiento histórico): nada se descarta,
	// así que baseline no puede quedar por debajo de lo emitido (invariante
	// I1 de data-model.md).
	builder.Recorder = rec
	builder.Counter = tokens.ApproximateTokenCounter{}

	if _, err := builder.Build(); err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("se esperaba 1 registro, got %d", len(rec.calls))
	}
	if rec.calls[0].baseline < rec.calls[0].emitted {
		t.Fatalf("baseline (%d) nunca puede ser menor que emitted (%d)", rec.calls[0].baseline, rec.calls[0].emitted)
	}
}

func TestBuild_NilRecorder_DoesNotPanic(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	// Recorder deliberadamente sin asignar (nil): debe seguir funcionando
	// exactamente igual, sin medición.
	if _, err := builder.Build(); err != nil {
		t.Fatalf("build con Recorder nil no debe fallar: %v", err)
	}
}

// TestBuild_IndexMode_NoBodiesButAllIDsPresent cubre SC-009: en modo índice
// la salida contiene todos los identificadores de las memorias seleccionadas
// y NINGÚN cuerpo de memoria.
func TestBuild_IndexMode_NoBodiesButAllIDsPresent(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	secretBody := "CONTENIDO-SECRETO-QUE-NO-DEBE-APARECER-JAMAS-EN-MODO-INDICE"
	id1, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "decisión uno", Content: secretBody})
	id2, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Bugfix, Title: "bug dos", Content: secretBody})

	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	builder.IndexMode = true

	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if strings.Contains(out, secretBody) {
		t.Fatalf("modo índice no debe emitir NINGÚN cuerpo de memoria, pero apareció el contenido:\n%s", out)
	}
	if !strings.Contains(out, fmt.Sprintf("get_memory %d", id1)) {
		t.Fatalf("falta el identificador de la memoria %d en el índice:\n%s", id1, out)
	}
	if !strings.Contains(out, fmt.Sprintf("get_memory %d", id2)) {
		t.Fatalf("falta el identificador de la memoria %d en el índice:\n%s", id2, out)
	}
	if !strings.Contains(out, "decisión uno") || !strings.Contains(out, "bug dos") {
		t.Fatalf("el índice debe conservar el título de cada memoria:\n%s", out)
	}
}

// TestBuild_IndexMode_StructureUnaffectedOutsideContent cubre FR-032: el
// protocolo/estructura que Build() ya emite fuera del contenido de cada
// memoria (encabezados de sección, conflictos, sinapsis) nunca se recorta en
// modo índice — solo el CUERPO de cada memoria colapsa a un puntero.
func TestBuild_IndexMode_StructureUnaffectedOutsideContent(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	idA, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Redis para cache", Content: "..."})
	idB, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Memcached para cache", Content: "..."})
	if _, _, err := usecases.RecordVerdict(relRepo, "proj", idA, idB, domain.ConflictsWith, 0.9, "se contradicen"); err != nil {
		t.Fatalf("record verdict: %v", err)
	}

	full := usecases.New(memRepo, persistence.NewSessionRepository(db), relRepo, root, "proj")
	full.IndexMode = false
	outFull, err := full.Build()
	if err != nil {
		t.Fatalf("build (completo): %v", err)
	}

	idx := usecases.New(memRepo, persistence.NewSessionRepository(db), relRepo, root, "proj")
	idx.IndexMode = true
	outIdx, err := idx.Build()
	if err != nil {
		t.Fatalf("build (índice): %v", err)
	}

	for _, want := range []string{
		"## Decisiones Técnicas",
		"## ⚠ Conflictos sin resolver",
		"usa Redis para cache",
		"usa Memcached para cache",
	} {
		if !strings.Contains(outFull, want) {
			t.Fatalf("fixture inválida: %q no aparece en el modo completo", want)
		}
		if !strings.Contains(outIdx, want) {
			t.Fatalf("la estructura %q debe sobrevivir intacta en modo índice, got:\n%s", want, outIdx)
		}
	}
}

// TestBuild_IndexMode_ReversibleToIdenticalOutput cubre SC-010: activar y
// desactivar el modo índice devuelve la emisión a un resultado idéntico al
// de partida.
func TestBuild_IndexMode_ReversibleToIdenticalOutput(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)
	memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Learning, Title: "algo", Content: "contenido de prueba"})

	before := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	outBefore, err := before.Build()
	if err != nil {
		t.Fatalf("build antes: %v", err)
	}

	toggled := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	toggled.IndexMode = true
	if _, err := toggled.Build(); err != nil {
		t.Fatalf("build en modo índice: %v", err)
	}
	toggled.IndexMode = false
	outAfter, err := toggled.Build()
	if err != nil {
		t.Fatalf("build tras desactivar el modo índice: %v", err)
	}

	if outBefore != outAfter {
		t.Fatalf("activar y desactivar el modo índice debe devolver una emisión idéntica.\nantes:\n%s\ndespués:\n%s", outBefore, outAfter)
	}
}

func TestBuild_IndexMode_EmptyProject_ExplicitEmptyIndex(t *testing.T) {
	root := t.TempDir()
	db, _ := persistence.Init(root)
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	builder := usecases.New(memRepo, persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "proj")
	builder.IndexMode = true

	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "índice vacío") && !strings.Contains(strings.ToLower(out), "sin memorias") {
		t.Fatalf("sin memorias, el modo índice debe declararlo explícitamente, got:\n%s", out)
	}
}
