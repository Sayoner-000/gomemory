package main

import (
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
)

func TestMemoryProtocolContract(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)

	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	project := "test-project"

	session, err := persistence.StartSession(db, project)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if session.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	active, err := persistence.ActiveSession(db, project)
	if err != nil {
		t.Fatalf("active session: %v", err)
	}
	if active == nil {
		t.Fatal("expected active session")
	}

	summary := "Test session completed at " + time.Now().Format(time.RFC3339)
	if err := persistence.EndSession(db, session.ID, summary); err != nil {
		t.Fatalf("end session: %v", err)
	}

	active, err = persistence.ActiveSession(db, project)
	if err != nil {
		t.Fatalf("active session after end: %v", err)
	}
	if active != nil {
		t.Fatal("expected no active session after end")
	}

	recent, err := persistence.RecentSessions(db, project, 5)
	if err != nil {
		t.Fatalf("recent sessions: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent session, got %d", len(recent))
	}
	if recent[0].Summary != summary {
		t.Fatalf("expected summary %q, got %q", summary, recent[0].Summary)
	}
}
