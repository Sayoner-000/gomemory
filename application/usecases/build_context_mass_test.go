package usecases_test

import (
	"fmt"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

const synapsesHeader031 = "## 🔗 Sinapsis (memorias enlazadas)"

func relate031(t *testing.T, repo ports.RelationRepository, a, b int64, kind domain.RelationType, conf float64) {
	t.Helper()
	if _, err := repo.Insert(&domain.Relation{Project: "proj", MemoryIDA: a, MemoryIDB: b, Relation: kind, Confidence: conf}); err != nil {
		t.Fatalf("relación %d-%d: %v", a, b, err)
	}
}

func TestBuild_ConflictoAntiguoSobreviveAMasDe20Relaciones(t *testing.T) {
	_, memRepo, _, relRepo, b := newCtx031Fixture(t)
	a := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "usa Redis para caché", Content: "a"})
	c := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "usa Memcached para caché", Content: "c"})
	// El conflicto es el más antiguo: sin la lectura completa quedaría fuera del
	// recorte a 20 de ListRelations.
	if _, err := relRepo.ImportRelation(&domain.Relation{Project: "proj", MemoryIDA: a, MemoryIDB: c, Relation: domain.ConflictsWith, Confidence: 0.9, CreatedAt: "2020-01-01 00:00:00"}); err != nil {
		t.Fatalf("import conflicto: %v", err)
	}
	ids := make([]int64, 26)
	for i := range ids {
		ids[i] = mustInsert031(t, memRepo, domain.Memory{Type: domain.Learning, Title: fmt.Sprintf("tema %d", i), Content: fmt.Sprintf("contenido %d", i)})
	}
	for i := 0; i < 25; i++ {
		relate031(t, relRepo, ids[i], ids[i+1], domain.Related, 0.5)
	}

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if sec := sectionOf031(out, "## ⚠ Conflictos sin resolver"); !strings.Contains(sec, fmt.Sprintf("[%d] ", a)) {
		t.Fatalf("el conflicto antiguo desapareció del contexto:\n%s", out)
	}
}

func TestBuild_SinapsisExcluyeCheckpointsYHuerfanas(t *testing.T) {
	_, memRepo, _, relRepo, b := newCtx031Fixture(t)
	d1 := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "decisión uno", Content: "d1"})
	d2 := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "decisión dos", Content: "d2"})
	cp := mustInsert031(t, memRepo, domain.Memory{Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: x.go"})
	gone := mustInsert031(t, memRepo, domain.Memory{Type: domain.Learning, Title: "se borrará", Content: "g"})
	relate031(t, relRepo, d1, d2, domain.Related, 0.5)
	relate031(t, relRepo, d1, cp, domain.Related, 0.5)
	relate031(t, relRepo, d2, gone, domain.Related, 0.5)
	if _, err := memRepo.Delete("proj", gone); err != nil {
		t.Fatalf("delete: %v", err)
	}

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sec := sectionOf031(out, synapsesHeader031)
	if !strings.Contains(sec, fmt.Sprintf("[%d] ", d1)) {
		t.Fatalf("falta la sinapsis válida:\n%s", out)
	}
	for _, id := range []int64{cp, gone} {
		if strings.Contains(sec, fmt.Sprintf("[%d] ", id)) {
			t.Fatalf("la sección no debe incluir la memoria %d:\n%s", id, sec)
		}
	}
}

func TestBuild_SinapsisPriorizaLaSesionActiva(t *testing.T) {
	_, memRepo, sessRepo, relRepo, b := newCtx031Fixture(t)
	xs := make([]int64, 4)
	for i := range xs {
		xs[i] = mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: fmt.Sprintf("ajena %d", i), Content: fmt.Sprintf("x%d", i)})
	}
	for i := range xs {
		relate031(t, relRepo, xs[i], xs[(i+1)%len(xs)], domain.Related, 1)
	}
	sess, err := sessRepo.Start("proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	s1 := mustInsert031(t, memRepo, domain.Memory{SessionID: sess.ID, Type: domain.Learning, Title: "sesión uno", Content: "s1"})
	s2 := mustInsert031(t, memRepo, domain.Memory{SessionID: sess.ID, Type: domain.Learning, Title: "sesión dos", Content: "s2"})
	if rel, _ := relRepo.GetByPair("proj", s1, s2); rel == nil {
		if rel, _ := relRepo.GetByPair("proj", s2, s1); rel == nil {
			relate031(t, relRepo, s1, s2, domain.Related, 0.5)
		}
	}

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sec := sectionOf031(out, synapsesHeader031)
	sessionAt := strings.Index(sec, fmt.Sprintf("[%d] ", s1))
	if at := strings.Index(sec, fmt.Sprintf("[%d] ", s2)); sessionAt < 0 || (at >= 0 && at < sessionAt) {
		sessionAt = at
	}
	if sessionAt < 0 {
		t.Fatalf("falta la sinapsis de la sesión:\n%s", sec)
	}
	for _, x := range xs {
		if at := strings.Index(sec, fmt.Sprintf("[%d] ", x)); at >= 0 && at < sessionAt {
			t.Fatalf("una sinapsis ajena (%d) aparece antes que la de la sesión activa:\n%s", x, sec)
		}
	}
	if !strings.Contains(sec, "sembrado en sesión activa (2)") {
		t.Fatalf("la leyenda debe nombrar las semillas de sesión:\n%s", sec)
	}
}

// Con la misma masa, la relación más reciente va primero: es el orden previo a
// la masa y evita que el empate muestre antes las aristas más viejas.
func TestBuild_SinapsisEmpateDeMasaPrefiereLaMasReciente(t *testing.T) {
	_, memRepo, _, relRepo, b := newCtx031Fixture(t)
	a1 := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "par antiguo uno", Content: "a1"})
	a2 := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "par antiguo dos", Content: "a2"})
	b1 := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "par nuevo uno", Content: "b1"})
	b2 := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "par nuevo dos", Content: "b2"})
	relate031(t, relRepo, a1, a2, domain.Related, 0.5)
	relate031(t, relRepo, b1, b2, domain.Related, 0.5)

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sec := sectionOf031(out, synapsesHeader031)
	older, newer := strings.Index(sec, fmt.Sprintf("[%d] ", a1)), strings.Index(sec, fmt.Sprintf("[%d] ", b1))
	if older < 0 || newer < 0 || newer > older {
		t.Fatalf("en empate de masa la relación más reciente debe ir primero:\n%s", sec)
	}
}

func TestBuild_SinapsisTopeLeyendaYDeterminismo(t *testing.T) {
	_, memRepo, _, relRepo, b := newCtx031Fixture(t)
	ids := make([]int64, 15)
	for i := range ids {
		ids[i] = mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: fmt.Sprintf("cadena %d", i), Content: fmt.Sprintf("c%d", i)})
	}
	for i := 0; i < len(ids)-1; i++ {
		relate031(t, relRepo, ids[i], ids[i+1], domain.Related, 0.5)
	}

	first, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	second, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if first != second {
		t.Fatalf("dos Build() seguidos difieren")
	}
	sec := sectionOf031(first, synapsesHeader031)
	if n := entryLines031(sec); n != 12 {
		t.Fatalf("líneas de sinapsis = %d, want 12:\n%s", n, sec)
	}
	legend := "_Orden: masa = centralidad en el grafo de memorias sembrado en todas las memorias (uniforme); no mide importancia ni corrección._"
	if !strings.Contains(sec, legend) {
		t.Fatalf("falta la leyenda de masa:\n%s", sec)
	}
}
