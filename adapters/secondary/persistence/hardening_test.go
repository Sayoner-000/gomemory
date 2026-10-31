package persistence

import (
	"context"
	"mem/domain"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSetSessionLastPrompt_CortaEnRuna(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`INSERT INTO sessions (id, project, created_at) VALUES ('s', 'p', '2026-10-31 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	if err = SetSessionLastPrompt(db, "p", strings.Repeat("a", maxOriginPromptLen-1)+"ñz"); err != nil {
		t.Fatal(err)
	}
	var got string
	if err = db.QueryRow(`SELECT last_prompt FROM sessions WHERE id = 's'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(got) || len(got) > maxOriginPromptLen {
		t.Fatalf("prompt inválido: %d bytes, válido=%v", len(got), utf8.ValidString(got))
	}
}

func TestHCDStore_TimestampCorruptoFalla(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	s := NewHCDStore(db, "p")
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	u, err := s.CreateActive(ctx, func(n int) domain.Unit {
		return domain.Unit{ID: "HCD-001", Number: n, Title: "t", Slug: "t", Kind: domain.KindFix, Lane: domain.LaneFast, Why: "w", State: domain.UnitActive, DocPath: "hcd/HCD-001-t.md", CreatedAt: now, UpdatedAt: now}
	}, func(domain.Unit) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AddEvidence(ctx, domain.Evidence{ID: "e", UnitID: u.ID, Tasks: []string{"1"}, Kind: "run", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = s.AddApproval(ctx, domain.Approval{ID: "a", UnitID: u.ID, Action: "close", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`UPDATE hcd_evidence SET created_at = 'roto'`, `UPDATE hcd_approvals SET created_at = 'roto'`, `UPDATE hcd_units SET updated_at = 'roto'`} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Evidence(ctx, u.ID); err == nil {
		t.Error("Evidence aceptó un timestamp corrupto")
	}
	if _, err = s.Approvals(ctx, u.ID); err == nil {
		t.Error("Approvals aceptó un timestamp corrupto")
	}
	if _, err = s.Get(ctx, u.ID); err == nil {
		t.Error("Get aceptó un timestamp corrupto")
	}
}

func TestDeleteMemory_NoDejaIndice(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewMemoryRepository(db)
	id, err := repo.ImportMemory(&domain.Memory{Project: "p", Type: domain.Decision, Title: "t", Content: "olvidable"})
	if err != nil {
		t.Fatal(err)
	}
	// Un índice que no se puede borrar no debe dejar el contenido olvidado atrás.
	if _, err = db.Exec(`DROP TABLE memory_search`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE memory_search (rowid_viejo INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err = DeleteMemory(db, "p", id); err == nil {
		t.Fatal("DeleteMemory debía fallar si no puede limpiar el índice")
	}
	var n int
	if err = db.QueryRow(`SELECT COUNT(*) FROM memories WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("la memoria se borró sin limpiar su índice")
	}
	// Sin FTS5 la tabla no existe: borrar debe seguir funcionando.
	if _, err = db.Exec(`DROP TABLE memory_search`); err != nil {
		t.Fatal(err)
	}
	if ok, err := DeleteMemory(db, "p", id); err != nil || !ok {
		t.Fatalf("DeleteMemory sin FTS5 = %v, %v", ok, err)
	}
}

func TestRelation_ReasoningSinEscapes(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewMemoryRepository(db)
	a, _ := repo.ImportMemory(&domain.Memory{Project: "p", Type: domain.Decision, Title: "a", Content: "a"})
	b, _ := repo.ImportMemory(&domain.Memory{Project: "p", Type: domain.Decision, Title: "b", Content: "b"})
	id, err := InsertRelation(db, &domain.Relation{Project: "p", MemoryIDA: a, MemoryIDB: b, Relation: domain.Related, Confidence: 1, Reasoning: "razon \x1b]0;pwned\x07 \x1b[2J fin"})
	if err != nil {
		t.Fatal(err)
	}
	one, err := GetRelation(db, "p", id)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := GetRelationByPair(db, "p", a, b)
	if err != nil {
		t.Fatal(err)
	}
	list, err := ListRelations(db, "p", 10)
	if err != nil {
		t.Fatal(err)
	}
	all, err := ListAllRelations(db, "p")
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{"get": one.Reasoning, "pair": pair.Reasoning, "list": list[0].Reasoning, "all": all[0].Reasoning} {
		if strings.ContainsRune(got, '\x1b') || strings.ContainsRune(got, '\x07') {
			t.Errorf("%s conserva controles: %q", name, got)
		}
	}
}
