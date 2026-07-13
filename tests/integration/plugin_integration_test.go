package main

import (
	"testing"

	"mem/adapters/secondary/persistence"
)

// TestPluginIntegration verifica el flujo de sesión de extremo a extremo
// (crear proyecto, abrir y cerrar sesión, recuperarla) sobre la base real.
// Antes también instanciaba el servidor HTTP legacy; ese servidor se retiró
// (el MCP vive en `mem mcp` por stdio), así que la prueba queda enfocada en
// la persistencia de sesiones, que es lo sustantivo que cubría.
func TestPluginIntegration(t *testing.T) {
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
