package usecases_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// All documents and databases live in t.TempDir; tokens are synthetic.
func TestAuditC001DiscoveredPrivatePathReachesMemory(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	history := persistence.NewDecisionHistoryRepository(env.db)
	env.uc.ConfigureDecisions(history, "p")
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "audit"})
	if err != nil {
		t.Fatal(err)
	}
	old := view.Unit.DocPath
	linked := &domain.Memory{Project: "p", Type: domain.Decision, Title: "Synthetic audit reference", Content: "Reference only", Filepath: old, TopicKey: "audit/synthetic-reference"}
	id, err := history.UpsertDigest(linked, []string{old}, true)
	if err != nil {
		t.Fatal(err)
	}
	moved := "hcd/" + view.Unit.ID + "-ghp_" + strings.Repeat("A", 36) + ".md"
	if domain.ValidateMemoryMetadata(moved, "") == nil {
		t.Fatal("synthetic fixture must be rejected by metadata validation")
	}
	if err := os.Rename(filepath.Join(env.root, old), filepath.Join(env.root, moved)); err != nil {
		t.Fatal(err)
	}
	got, err := env.uc.Show(ctx, view.Unit.ID)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatal("Show must reject the discovered private path")
	}
	if strings.Contains(got.Unit.DocPath, moved) || strings.Contains(err.Error(), moved) {
		t.Fatal("private path exposed in result")
	}
	var fp, path string
	if err := env.db.QueryRow(`SELECT filepath FROM memories WHERE id = ?`, id).Scan(&fp); err != nil {
		t.Fatal(err)
	}
	if err := env.db.QueryRow(`SELECT path FROM memory_files WHERE memory_id = ?`, id).Scan(&path); err != nil {
		t.Fatal(err)
	}
	if fp != old || path != old {
		t.Fatal("rejected path changed memory metadata")
	}
	var unitPath string
	if err := env.db.QueryRow(`SELECT doc_path FROM hcd_units WHERE id = ?`, view.Unit.ID).Scan(&unitPath); err != nil || unitPath != old {
		t.Fatal("rejected path changed unit")
	}
	var pending int
	if err := env.db.QueryRow(`SELECT COUNT(*) FROM hcd_pending_retargets`).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("rejected path created pending retarget")
	}
}

type auditPausedSave struct {
	ports.HCDStore
	entered chan struct{}
	resume  chan struct{}
}

func (s *auditPausedSave) SaveMetadata(ctx context.Context, u domain.Unit, from string, queue bool) error {
	close(s.entered)
	select {
	case <-s.resume:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.HCDStore.SaveMetadata(ctx, u, from, queue)
}

type auditFixedClock struct{}

func (auditFixedClock) Now() time.Time { return time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC) }

func TestAuditC003ReconcilePreservesCancelledUnit(t *testing.T) {
	env := newHCDEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "audit"})
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(env.root, view.Unit.DocPath)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, domain.ReplaceUnitTitle(data, "Edited manually"), 0600); err != nil {
		t.Fatal(err)
	}
	store := persistence.NewHCDStore(env.db, "p")
	paused := &auditPausedSave{HCDStore: store, entered: make(chan struct{}), resume: make(chan struct{})}
	defer close(paused.resume)
	reader := usecases.NewHCD(paused, hcdfiles.Workspace{Root: env.root}, auditFixedClock{}, func() string { return "unused" }, "")
	done := make(chan error, 1)
	go func() { _, err := reader.Show(ctx, view.Unit.ID); done <- err }()
	select {
	case <-paused.entered:
	case <-ctx.Done():
		t.Fatal("Show never reached reconciliation Save")
	}
	if _, err := env.uc.Cancel(ctx, usecases.HCDCloseInput{ID: view.Unit.ID, Quote: "cancel synthetic audit"}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Get(ctx, view.Unit.ID)
	if err != nil || before.State != domain.UnitCancelled || before.Active {
		t.Fatalf("Cancel was not persisted: state=%s active=%v err=%v", before.State, before.Active, err)
	}
	// Release precisely the stale full-unit save, after Cancel has committed.
	paused.resume <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	after, err := store.Get(ctx, view.Unit.ID)
	if err != nil || after.State != domain.UnitCancelled || after.Active {
		t.Fatalf("reconciliation overwrote cancellation: state=%s active=%v err=%v", after.State, after.Active, err)
	}
	approvals, err := store.Approvals(ctx, view.Unit.ID)
	if err != nil || len(approvals) != 1 {
		t.Fatalf("cancel evidence must remain: count=%d err=%v", len(approvals), err)
	}
	t.Log("REGRESSION_CLOSED: metadata-only reconciliation preserves committed cancellation")
}
