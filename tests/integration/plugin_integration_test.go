package main

import (
	"testing"

	"mem/adapters/primary/mcp"
	"mem/adapters/secondary/persistence"
)

func TestPluginIntegration(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)

	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	project := "test-project"

	srv := mcp.New(db, project, 19735)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	session, err := persistence.StartSession(db, project)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := persistence.EndSession(db, session.ID, "integration test completed"); err != nil {
		t.Fatalf("end session: %v", err)
	}

	recent, err := persistence.RecentSessions(db, project, 5)
	if err != nil {
		t.Fatalf("recent sessions: %v", err)
	}
	if len(recent) < 1 {
		t.Fatal("expected at least 1 recent session")
	}
}
