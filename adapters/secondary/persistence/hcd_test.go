package persistence

import (
	"context"
	"errors"
	"mem/domain"
	"testing"
	"time"
)

func TestHCDStoreUnitEvidenceApproval(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	s, other := NewHCDStore(db, "p"), NewHCDStore(db, "q")
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	if n, err := s.NextNumber(ctx); err != nil || n != 1 {
		t.Fatalf("NextNumber = %d, %v", n, err)
	}
	for i, id := range []string{"HCD-001", "HCD-002"} {
		u := domain.Unit{ID: id, Number: i + 1, Title: "t", Slug: "t", Kind: domain.KindFix, Lane: domain.LaneFast, Why: "w", State: domain.UnitActive, DocPath: "hcd/" + id + "-t.md", CreatedAt: now, UpdatedAt: now}
		if err := s.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Activate(ctx, "HCD-001"); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate(ctx, "HCD-002"); err != nil {
		t.Fatal(err)
	}
	active, ok, err := s.Active(ctx)
	if err != nil || !ok || active.ID != "HCD-002" {
		t.Fatalf("Active = %+v %v %v", active, ok, err)
	}
	if _, ok, _ := other.Active(ctx); ok {
		t.Fatal("active unit leaked across projects")
	}
	if _, err := other.Get(ctx, "HCD-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get other project = %v", err)
	}
	active.Lane, active.State = domain.LaneFull, domain.UnitClosed
	if err := s.Save(ctx, active); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, "HCD-002")
	if got.Lane != domain.LaneFull || got.State != domain.UnitClosed || !got.Active {
		t.Fatalf("saved unit = %+v", got)
	}
	ev := domain.Evidence{ID: "e1", UnitID: "HCD-002", Tasks: []string{"1", "2"}, Kind: domain.EvidenceCommand, ExpectFail: true, Command: "go test", ExitCode: 1, Excerpt: "FAIL", Revision: "r", Fingerprint: "f", CreatedAt: now}
	if err := s.AddEvidence(ctx, ev); err != nil {
		t.Fatal(err)
	}
	list, err := s.Evidence(ctx, "HCD-002")
	if err != nil || len(list) != 1 || !list[0].ExpectFail || list[0].Tasks[1] != "2" || !list[0].CreatedAt.Equal(now) {
		t.Fatalf("Evidence = %+v %v", list, err)
	}
	for i, action := range []string{domain.ApprovalApprove, domain.ApprovalClose} {
		if err := s.AddApproval(ctx, domain.Approval{ID: action, UnitID: "HCD-002", Action: action, Revision: "r", Fingerprint: "f", Origin: "conversation", Client: "claude", Quote: "apruebo", ConstitutionRev: "c1", PlaybookRev: "p1", CreatedAt: now.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	approvals, err := s.Approvals(ctx, "HCD-002")
	if err != nil || len(approvals) != 2 || approvals[0].Action != domain.ApprovalApprove || approvals[1].Quote != "apruebo" || approvals[0].ConstitutionRev != "c1" || approvals[0].PlaybookRev != "p1" {
		t.Fatalf("Approvals = %+v %v", approvals, err)
	}
	if other, _ := other.Approvals(ctx, "HCD-002"); len(other) != 0 {
		t.Fatal("approvals leaked across projects")
	}
}
