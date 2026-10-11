package usecases_test

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// rawSessions devuelve resúmenes tal cual, como haría un puerto que no
// depura: el builder no debe depender de que el adaptador lo haga.
type rawSessions struct{ recent []domain.Session }

func (r rawSessions) Active(string) (*domain.Session, error)       { return nil, nil }
func (r rawSessions) Recent(string, int) ([]domain.Session, error) { return r.recent, nil }

// TestBuild_ProtegeResumenAntesDeRecortar cubre US1.7 (FR-002): un resumen
// histórico con un bloque privado que empieza antes del punto de recorte y
// termina después se emite sin el contenido privado. Recortar primero
// cortaría la etiqueta de cierre y dejaría el texto privado como apertura
// abierta a medias, o lo emitiría.
func TestBuild_ProtegeResumenAntesDeRecortar(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ended := "2026-10-10 10:00:00"
	summary := "resumen público " + strings.Repeat("a", 150) + " <private>SECRETO_037 " + strings.Repeat("b", 200) + "</private> cola pública AKIAIOSFODNN7EXAMPLE"
	b := &usecases.Builder{
		Lister:    persistence.NewMemoryRepository(db),
		Session:   rawSessions{recent: []domain.Session{{ID: "s1", Project: "proj", Summary: summary, CreatedAt: "2026-10-10 09:00:00", EndedAt: &ended}}},
		Relations: persistence.NewRelationRepository(db),
		Project:   "proj",
		Budget:    20000,
	}
	out, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, gone := range []string{"SECRETO_037", "bbbb", "AKIAIOSFODNN7EXAMPLE", "<private", "</private"} {
		if strings.Contains(out, gone) {
			t.Fatalf("contexto revela %q:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "resumen público") {
		t.Fatalf("contexto perdió el texto público:\n%s", out)
	}
}

// TestRecordVerdict_DevuelveRazonamientoProtegido cubre US1.2 (FR-002): la
// relación devuelta, que la consola y judge_memories muestran, coincide con
// lo persistido y no repite el razonamiento crudo (hallado por el runner de
// la 037 sobre `mem compare -m`).
func TestRecordVerdict_DevuelveRazonamientoProtegido(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()
	rels := persistence.NewRelationRepository(db)

	const reasoning = "motivo público <private>SECRETO_037</private> AKIAIOSFODNN7EXAMPLE"
	for i, want := range []bool{false, true} {
		rel, updated, err := usecases.RecordVerdict(rels, "proj", 1, 2, domain.Related, 0.5, reasoning)
		if err != nil || updated != want {
			t.Fatalf("vuelta %d: updated=%v err=%v", i, updated, err)
		}
		if strings.Contains(rel.Reasoning, "SECRETO_037") || strings.Contains(rel.Reasoning, "AKIAIOSFODNN7EXAMPLE") || !strings.Contains(rel.Reasoning, "motivo público") {
			t.Fatalf("vuelta %d: reasoning devuelto sin proteger: %q", i, rel.Reasoning)
		}
	}
}
