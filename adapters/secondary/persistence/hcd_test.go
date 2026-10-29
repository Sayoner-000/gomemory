package persistence

import (
	"context"
	"errors"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

func newHCDTestStore(t *testing.T) *HCDStore {
	t.Helper()
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewHCDStore(db, "test-project")
}

func TestHCDStore_CreateAndReadUnit(t *testing.T) {
	store := newHCDTestStore(t)
	want := domain.Unit{ID: "HCD-001", Kind: domain.KindFeature, Title: "Feature", Slug: "feature", Approach: "hcd", State: domain.StateDraft, StateRevision: 1, Implementer: "dev"}
	if err := store.CreateUnit(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetUnit(context.Background(), want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.Kind != want.Kind || got.State != want.State || got.StateRevision != want.StateRevision {
		t.Fatalf("GetUnit() = %+v, want %+v", got, want)
	}
}

func TestHCDStore_WithTransactionIsAtomic(t *testing.T) {
	store := newHCDTestStore(t)
	wantErr := errors.New("rollback")
	err := store.WithTransaction(context.Background(), func(repo ports.HCDRepository) error {
		if err := repo.CreateUnit(context.Background(), domain.Unit{ID: "HCD-001", Kind: domain.KindFix, Title: "fix", Slug: "fix", Approach: "hcd", State: domain.StateDraft}); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTransaction error = %v, want %v", err, wantErr)
	}
	if _, err := store.GetUnit(context.Background(), "HCD-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("rolled back unit lookup = %v, want ErrNotFound", err)
	}
}

func TestHCDStore_RevisionConflictAndOperationIdempotency(t *testing.T) {
	store := newHCDTestStore(t)
	unit := domain.Unit{ID: "HCD-001", Kind: domain.KindFix, Title: "fix", Slug: "fix", Approach: "hcd", State: domain.StateDraft, StateRevision: 2}
	if err := store.CreateUnit(context.Background(), unit); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateUnit(context.Background(), unit, 1); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("stale revision error = %v", err)
	}
	first, replay, err := store.RecordOperation(context.Background(), "op-1", unit.ID, "define", "hash-a", `{"ok":true}`)
	if err != nil || replay || first != `{"ok":true}` {
		t.Fatalf("first operation = %q, %v, %v", first, replay, err)
	}
	second, replay, err := store.RecordOperation(context.Background(), "op-1", unit.ID, "define", "hash-a", `{"ok":false}`)
	if err != nil || !replay || second != first {
		t.Fatalf("replayed operation = %q, %v, %v", second, replay, err)
	}
	if _, _, err := store.RecordOperation(context.Background(), "op-1", unit.ID, "define", "hash-b", `{"ok":false}`); !errors.Is(err, domain.ErrOperationConflict) {
		t.Fatalf("conflicting operation error = %v", err)
	}
}

func TestHCDStore_DeduplicatesLinksAndRecoversTrace(t *testing.T) {
	store := newHCDTestStore(t)
	ctx := context.Background()
	links := []domain.Link{
		{UnitID: "HCD-001", FromKind: "story", FromID: "US1", ToKind: "criterion", ToID: "C-01", Relation: "covers"},
		{UnitID: "HCD-001", FromKind: "criterion", FromID: "C-01", ToKind: "slice", ToID: "VS-01", Relation: "belongs_to"},
		{UnitID: "HCD-001", FromKind: "slice", FromID: "VS-01", ToKind: "task", ToID: "T-001", Relation: "belongs_to"},
		{UnitID: "HCD-001", FromKind: "task", FromID: "T-001", ToKind: "evidence", ToID: "EV-01", Relation: "verifies"},
	}
	for _, link := range links {
		if err := store.AddLink(ctx, link); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.AddLink(ctx, links[0]); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListLinks(ctx, "HCD-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(links) {
		t.Fatalf("links = %d, want %d", len(got), len(links))
	}
	seen := map[string]bool{"story:US1": true}
	for changed := true; changed; {
		changed = false
		for _, edge := range got {
			from, to := edge.FromKind+":"+edge.FromID, edge.ToKind+":"+edge.ToID
			if seen[from] && !seen[to] {
				seen[to], changed = true, true
			}
		}
	}
	if !seen["evidence:EV-01"] {
		t.Fatal("trace did not reach evidence from story")
	}
}
