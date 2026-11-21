package persistence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mem/domain"
)

func TestHCDMetadataPreservesLifecycleAndRollsBackPendingFailure(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, now := context.Background(), time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	store := NewHCDStore(db, "p")
	unit, err := store.CreateActive(ctx, func(n int) domain.Unit {
		return domain.Unit{ID: "HCD-001", Number: n, Title: "Old", Slug: "old", Kind: domain.KindFix, Lane: domain.LaneFast, Why: "why", State: domain.UnitActive, DocPath: "hcd/HCD-001-old.md", CreatedAt: now, UpdatedAt: now}
	}, func(domain.Unit) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	closed := unit
	closed.State, closed.Active, closed.Lane, closed.Why = domain.UnitClosed, false, domain.LaneFull, "new why"
	if err = store.Save(ctx, closed); err != nil {
		t.Fatal(err)
	}
	old := unit.DocPath
	unit.Title, unit.Slug, unit.DocPath = "New", "new", "hcd/HCD-001-new.md"
	if _, err = db.Exec(`CREATE TRIGGER fail_pending BEFORE INSERT ON hcd_pending_retargets BEGIN SELECT RAISE(ABORT, 'pending unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveMetadata(ctx, unit, old, true); err == nil {
		t.Fatal("expected pending write failure")
	}
	got, err := store.Get(ctx, unit.ID)
	if err != nil || got.DocPath != old {
		t.Fatalf("metadata escaped rollback: %+v %v", got, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_pending`); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveMetadata(ctx, unit, old, true); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(ctx, unit.ID)
	if err != nil || got.State != closed.State || got.Active != closed.Active || got.Lane != closed.Lane || got.Why != closed.Why || got.DocPath != unit.DocPath || !got.CreatedAt.Equal(closed.CreatedAt) {
		t.Fatalf("lifecycle overwritten: %+v %v", got, err)
	}
	if err = store.SaveMetadata(ctx, unit, unit.DocPath, true); err == nil {
		t.Fatal("pending must not be overwritten")
	}
	from, to, err := store.PendingRetarget(ctx, unit.ID)
	if err != nil || from != old || to != unit.DocPath {
		t.Fatalf("pending = %q -> %q, %v", from, to, err)
	}
	if from, _, err = NewHCDStore(db, "other").PendingRetarget(ctx, unit.ID); err != nil || from != "" {
		t.Fatalf("pending leaked across projects: %q, %v", from, err)
	}
}

func TestMigrateV13AddsRetargetRecovery(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`DROP TABLE hcd_pending_retargets`); err != nil {
		t.Fatal(err)
	}
	seedMemoryAtVersion(t, db, 13)
	assertMigratedToCurrent(t, db)
}
