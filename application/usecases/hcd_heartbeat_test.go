package usecases_test

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestHCDHeartbeatNoFocusAndRetention(t *testing.T) {
	uc, repo, db, _, _ := newHCDFlow(t)
	ctx := context.Background()
	if e := uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{}); e != nil {
		t.Fatal(e)
	}
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	before := u.StateRevision
	var decisionCount int
	if e = db.QueryRow("SELECT count(*) FROM hcd_decisions").Scan(&decisionCount); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e = uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: "codex", SessionID: "session"}); e != nil {
			t.Fatal(e)
		}
	}
	events, e := repo.ListHeartbeats(ctx, u.ID)
	if e != nil || len(events) != 1 {
		t.Fatalf("retention=%+v %v", events, e)
	}
	if e = uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: "codex", SessionID: "session", Files: []string{"domain/file.go"}}); e != nil {
		t.Fatal(e)
	}
	if e = uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: "codex", SessionID: "session"}); e != nil {
		t.Fatal(e)
	}
	events, e = repo.ListHeartbeats(ctx, u.ID)
	if e != nil || len(events) != 2 || len(events[1].Drift) != 1 || events[1].Drift[0] != "files_without_execution" {
		t.Fatalf("drift retention=%+v %v", events, e)
	}
	u, e = repo.GetUnit(ctx, u.ID)
	if e != nil || u.StateRevision != before {
		t.Fatalf("heartbeat mutated state: %+v %v", u, e)
	}
	var after int
	if e = db.QueryRow("SELECT count(*) FROM hcd_decisions").Scan(&after); e != nil || after != decisionCount {
		t.Fatalf("heartbeat created decisions: %d %d %v", decisionCount, after, e)
	}
}

func TestHCDHeartbeatGrantAndInactiveUnit(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	if _, e := uc.Run(ctx, u.ID, u.StateRevision, "grant-heartbeat", "T-001", "codex"); e != nil {
		t.Fatal(e)
	}
	if e := uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: "codex", SessionID: "session", Files: []string{"file.go"}}); e != nil {
		t.Fatal(e)
	}
	drift, e := uc.Drift(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, d := range drift {
		found = found || d == "execution_unreported"
	}
	if !found {
		t.Fatalf("drift=%v", drift)
	}
	uc, repo, _, _, _ := newHCDFlow(t)
	u, e = uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	if e = repo.RecordHeartbeat(ctx, domain.Heartbeat{ID: "old", UnitID: u.ID, SessionID: "old", SeenAt: "2026-10-18T00:00:00Z"}); e != nil {
		t.Fatal(e)
	}
	view, e := uc.Resume(ctx, u.ID, "codex", "session")
	if e != nil {
		t.Fatal(e)
	}
	found = false
	for _, d := range view.Drift {
		found = found || d == "inactive_unit"
	}
	if !found {
		t.Fatalf("inactive drift=%v", view.Drift)
	}
}

func TestHCDDriftUnreportedClearsAfterReport(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	r, e := uc.Run(ctx, u.ID, u.StateRevision, "grant", "T-001", "codex")
	if e != nil {
		t.Fatal(e)
	}
	if e = uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: "codex", SessionID: "s"}); e != nil {
		t.Fatal(e)
	}
	view, _ := uc.Show(ctx, u.ID)
	ev := domain.Evidence{Procedure: "go test", Environment: "tmp", Result: "pass", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001"}, Executor: "codex"}
	if _, e = uc.Report(ctx, u.ID, view.Unit.StateRevision, "report", r.Execution.ID, ev); e != nil {
		t.Fatal(e)
	}
	drift, e := uc.Drift(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range drift {
		if d == "execution_unreported" {
			t.Fatalf("drift after report=%v", drift)
		}
	}
}
