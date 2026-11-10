package persistence

import (
	"context"
	"fmt"
	"mem/domain"
	"sync"
	"testing"
	"time"
)

func TestHCD057ReviewMigration(t *testing.T) {
	for _, previous := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "v12"}[previous], func(t *testing.T) {
			db, err := Init(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if previous {
				for _, q := range []string{
					`INSERT INTO hcd_units(project,id,number,title,slug,kind,lane,why,doc_path,created_at,updated_at) VALUES('p','HCD-056',56,'keep','keep','evolution','full','keep','hcd/keep.md','x','x')`,
					`INSERT INTO hcd_approvals(id,project,unit_id,action,revision,fingerprint,origin,client,quote,created_at) VALUES('a','p','HCD-056','approve','revision','f','conversation','codex','keep','x')`,
					`INSERT INTO hcd_evidence(id,project,unit_id,tasks,kind,expect_fail,command,exit_code,excerpt,revision,fingerprint,created_at) VALUES('e','p','HCD-056','1','command',0,'go test',0,'keep','r','f','x')`,
					`DROP TABLE IF EXISTS hcd_definition_reviews`, `PRAGMA user_version = 12`,
				} {
					if _, err := db.Exec(q); err != nil {
						t.Fatal(err)
					}
				}
				if err := migrate(db); err != nil {
					t.Fatal(err)
				}
				for _, table := range []string{"hcd_units", "hcd_approvals", "hcd_evidence"} {
					var count int
					if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE project='p'").Scan(&count); err != nil || count != 1 {
						t.Fatalf("%s: %d %v", table, count, err)
					}
				}
			}
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM hcd_definition_reviews").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if userVersion(t, db) <= 12 {
				t.Fatal("schema version not incremented")
			}
			before := schemaCatalog(t, db)
			if err := migrate(db); err != nil {
				t.Fatal(err)
			}
			if before != schemaCatalog(t, db) {
				t.Fatal("migration is not idempotent")
			}
		})
	}
}

func TestHCD057ReviewRepository(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	s, other := NewHCDStore(db, "p"), NewHCDStore(db, "q")
	now := time.Unix(1, 0).UTC()
	for _, store := range []*HCDStore{s, other} {
		_, err := store.CreateActive(ctx, func(n int) domain.Unit {
			return domain.Unit{ID: "HCD-001", Number: n, Title: "t", Slug: "t", Kind: domain.KindEvolution, Lane: domain.LaneFull, Why: "w", State: domain.UnitActive, DocPath: "hcd/t.md", CreatedAt: now, UpdatedAt: now}
		}, func(domain.Unit) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	e := domain.HCDDefinitionReviewEvent{OperationID: "op-1", UnitID: "HCD-001", Revision: "012345abcdef", Kind: "socratic", Action: "complete", Client: "codex", CreatedAt: now, Payload: domain.HCDDefinitionReviewPayload{Summary: "Evidence establishes scope."}}
	first, err := s.AppendReview(ctx, e, "")
	if err != nil {
		t.Fatal(err)
	}
	e.CreatedAt = now.Add(time.Hour)
	replay, err := s.AppendReview(ctx, e, "")
	if err != nil || !replay.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("retry: %+v %v", replay, err)
	}
	e.Payload.Summary = "different"
	if _, err := s.AppendReview(ctx, e, ""); err == nil {
		t.Fatal("conflicting replay accepted")
	}
	e.Payload.Summary = "Evidence establishes scope."
	if events, err := other.DefinitionReviews(ctx, e.UnitID); err != nil || len(events) != 0 {
		t.Fatalf("project leak: %+v %v", events, err)
	}
	if events, err := s.DefinitionReviews(ctx, "HCD-002"); err != nil || len(events) != 0 {
		t.Fatalf("unit leak: %+v %v", events, err)
	}
	if _, err := other.AppendReview(ctx, e, ""); err != nil {
		t.Fatalf("operation id not scoped to project: %v", err)
	}
	var wg sync.WaitGroup
	successes := make(chan string, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := e
			next.OperationID = domain.HCDReviewOperationID(fmt.Sprintf("racer-%d", i))
			if _, err := s.AppendReview(ctx, next, "op-1"); err == nil {
				successes <- string(next.OperationID)
			}
		}(i)
	}
	wg.Wait()
	close(successes)
	var winner string
	count := 0
	for id := range successes {
		winner = id
		count++
	}
	if count != 1 {
		t.Fatalf("CAS winners=%d", count)
	}
	events, err := s.DefinitionReviews(ctx, e.UnitID)
	if err != nil || len(events) != 2 || events[1].OperationID != domain.HCDReviewOperationID(winner) {
		t.Fatalf("event order: %+v %v", events, err)
	}
	a := domain.Approval{ID: "approve", UnitID: e.UnitID, Action: domain.ApprovalApprove, Revision: string(e.Revision), Fingerprint: "f", Origin: "conversation", Client: "codex", Quote: "apruebo", CreatedAt: now}
	if err := s.AddReviewedApproval(ctx, a, "op-1"); err == nil {
		t.Fatal("stale approval accepted")
	}
	if list, _ := s.Approvals(ctx, e.UnitID); len(list) != 0 {
		t.Fatal("partial approval write")
	}
	if err := s.AddReviewedApproval(ctx, a, domain.HCDReviewOperationID(winner)); err != nil {
		t.Fatal(err)
	}
	e.OperationID = "late"
	if _, err := s.AppendReview(ctx, e, domain.HCDReviewOperationID(winner)); err == nil {
		t.Fatal("review changed after atomic approval")
	}
	e.OperationID = "op-1"
	if _, err := s.AppendReview(ctx, e, ""); err != nil {
		t.Fatalf("retry after approval: %v", err)
	}
	e.OperationID = "new-revision"
	e.Revision = "abcdef012345"
	if _, err := s.AppendReview(ctx, e, ""); err != nil {
		t.Fatalf("new revision rejected: %v", err)
	}
	e.UnitID = "HCD-999"
	e.OperationID = "unknown-unit"
	if _, err := s.AppendReview(ctx, e, ""); err == nil {
		t.Fatal("nonexistent unit accepted")
	}
}
