package main

import (
	"testing"

	"mem/internal/server"
	"mem/store"
)

func TestPluginIntegration(t *testing.T) {
	root := t.TempDir()
	_ = store.EnsureDir(root)

	db, err := store.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	project := "test-project"

	// Server can be created and generates correct context
	srv := server.New(db, project, 19735)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	// Start a session to have context
	session, err := store.StartSession(db, project)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	// End the session to create history
	if err := store.EndSession(db, session.ID, "integration test completed"); err != nil {
		t.Fatalf("end session: %v", err)
	}

	// Verify recent sessions
	recent, err := store.RecentSessions(db, project, 5)
	if err != nil {
		t.Fatalf("recent sessions: %v", err)
	}
	if len(recent) < 1 {
		t.Fatal("expected at least 1 recent session")
	}
}
