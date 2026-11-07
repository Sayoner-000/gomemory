package usecases_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// HCD-046: mem hcd index crea las fichas del historial, consolida las
// memorias sueltas de pasos y es idempotente.

type fakeAudits map[string]string

func (f fakeAudits) ListAudits() ([]string, error) {
	var out []string
	for name := range f {
		out = append(out, name)
	}
	return out, nil
}

func (f fakeAudits) ReadAudit(rel string) ([]byte, error) { return []byte(f[rel]), nil }

func countWhere(t *testing.T, db *sql.DB, where string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// C-11: un documento sin fila en el motor (creado en otro worktree) se indexa
// desde su documento y sus memorias sueltas se consolidan.
func TestIndexHistory_UnidadSinFilaSeIndexaDesdeSuDocumento(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	writeIn(t, env.root, "hcd/HCD-007-huerfana-cerrada.md", "# Huérfana cerrada\n\nID: HCD-007\nTipo: fix\nCarril: fast\nMotivo: creada en otro worktree\n\n## Intención\nCorregir el guardado en `save.go`.\n\n## Evidencia\n- 2026-10-30 10:00 -05:00 · tarea 1 · verde · vigente\n\n## Siguiente\nCerrada con aprobación sobre la revisión abc123abc123.\n")
	memRepo := persistence.NewMemoryRepository(env.db)
	step := domain.Memory{Project: "p", Type: domain.Decision, Title: "HCD-007: cerrada", Content: "Cerrada"}
	if _, err := memRepo.Insert(&step); err != nil {
		t.Fatal(err)
	}
	uc := &usecases.IndexHistory{Units: env.uc, Memories: memRepo.(ports.HistoryMemories), History: persistence.NewDecisionHistoryRepository(env.db), Project: "p"}
	rep, err := uc.Run(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Units != 1 || rep.Created != 1 || rep.Deleted != 1 {
		t.Fatalf("índice = %+v", rep)
	}
	digest, err := memRepo.(ports.HistoryMemories).ByTopicKey("p", "hcd/HCD-007")
	if err != nil || digest == nil || digest.Title != "HCD-007 · Huérfana cerrada (cerrada)" || !strings.Contains(digest.Content, "Inicio: 2026-10-30") || !strings.Contains(digest.Content, "Tipo: fix · Carril: fast") {
		t.Fatalf("ficha = %+v %v", digest, err)
	}
	if n := countWhere(t, env.db, `title = 'HCD-007: cerrada'`); n != 0 {
		t.Fatalf("la memoria suelta sigue: %d", n)
	}
}

func TestIndexHistory_VistaPreviaAplicarYRepetir(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	for _, title := range []string{"Primera", "Segunda"} {
		if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "fast", Title: title, Why: "simple", Intent: "Hacer " + title}); err != nil {
			t.Fatal(err)
		}
	}
	memRepo := persistence.NewMemoryRepository(env.db)
	for _, m := range []domain.Memory{
		{Type: domain.Decision, Title: "HCD-001: carril fast", Content: "Carril fast"},
		{Type: domain.Decision, Title: "HCD-001: aclaración Q1", Content: "Respuesta a Q1"},
		{Type: domain.Bugfix, Title: "HCD-001: el guardado pierde datos", Content: "Causa raíz"},
		{Type: domain.Decision, Title: "HCD-099: cerrada", Content: "Unidad que no existe"},
	} {
		m.Project = "p"
		if _, err := memRepo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	uc := &usecases.IndexHistory{
		Units:    env.uc,
		Audits:   fakeAudits{"docs/audits/2026-11-06-x.md": "# Informe X\n\n## 2. Hallazgos\n\n| ID | Severidad | Problema |\n|---|---|---|\n| H-01 | Alta | Fuga en `adapters/x.go` |\n"},
		Memories: memRepo.(ports.HistoryMemories),
		History:  persistence.NewDecisionHistoryRepository(env.db),
		Project:  "p",
	}

	preview, err := uc.Run(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Created != 3 || preview.Updated != 0 || preview.Deleted != 2 || preview.Applied {
		t.Fatalf("vista previa = %+v", preview)
	}
	if n := countWhere(t, env.db, `topic_key LIKE 'hcd/%' OR topic_key LIKE 'audit/%'`); n != 0 {
		t.Fatalf("la vista previa escribió %d fichas", n)
	}
	if n := countWhere(t, env.db, `title LIKE 'HCD-001: %'`); n != 3 {
		t.Fatalf("la vista previa borró memorias: quedan %d", n)
	}

	applied, err := uc.Run(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Created != 3 || applied.Deleted != 2 || !applied.Applied {
		t.Fatalf("aplicar = %+v", applied)
	}
	for _, topic := range []string{"hcd/HCD-001", "hcd/HCD-002", "audit/2026-11-06-x.md"} {
		if n := countWhere(t, env.db, `topic_key = ?`, topic); n != 1 {
			t.Fatalf("fichas con %s = %d", topic, n)
		}
	}
	for title, want := range map[string]int{"HCD-001: carril fast": 0, "HCD-001: aclaración Q1": 0, "HCD-001: el guardado pierde datos": 1, "HCD-099: cerrada": 1} {
		if n := countWhere(t, env.db, `title = ?`, title); n != want {
			t.Fatalf("%q: %d memorias, quiero %d", title, n, want)
		}
	}
	mems, err := persistence.ListMemoriesByFilepath(env.db, "p", []string{"adapters/x.go"}, 10)
	if err != nil || len(mems) != 1 || mems[0].Type != domain.Discovery {
		t.Fatalf("la ficha de auditoría no está enlazada: %+v %v", mems, err)
	}

	again, err := uc.Run(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if again.Created != 0 || again.Updated != 0 || again.Deleted != 0 || again.Unchanged != 3 {
		t.Fatalf("repetir no es idempotente: %+v", again)
	}
	if got := fmt.Sprint(countWhere(t, env.db, `type = 'decision' AND topic_key LIKE 'hcd/%'`)); got != "2" {
		t.Fatalf("fichas HCD = %s", got)
	}
}
