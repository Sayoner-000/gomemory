package cli

import (
	"database/sql"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

func insertRawTopicDup(t *testing.T, db *sql.DB, project, topicKey, content string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO memories (project, type, title, content, topic_key) VALUES (?, 'decision', 'rev', ?, ?)`,
		project, content, topicKey,
	); err != nil {
		t.Fatalf("insertRawTopicDup: %v", err)
	}
}

func TestCmdConsolidate_Preview_DoesNotModify(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	insertRawTopicDup(t, db, "proj", "same-topic", "v1")
	insertRawTopicDup(t, db, "proj", "same-topic", "v2")

	deps := &Deps{Root: root, Project: "proj", MemoryRepo: persistence.NewMemoryRepository(db)}

	out := captureStdout(t, func() { CmdConsolidate(deps, nil) })

	if !strings.Contains(out, "Previsualización") {
		t.Fatalf("sin --apply debe declarar que es previsualización, got:\n%s", out)
	}

	mems, _ := deps.MemoryRepo.ListAll("proj")
	if len(mems) != 2 {
		t.Fatalf("la previsualización no debe modificar nada: quedaron %d filas, want 2", len(mems))
	}
}

func TestCmdConsolidate_Apply_MergesGroup(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	insertRawTopicDup(t, db, "proj", "same-topic", "v1")
	insertRawTopicDup(t, db, "proj", "same-topic", "v2")

	deps := &Deps{Root: root, Project: "proj", MemoryRepo: persistence.NewMemoryRepository(db)}

	out := captureStdout(t, func() { CmdConsolidate(deps, []string{"--apply"}) })

	if !strings.Contains(out, "se consolidaron") {
		t.Fatalf("con --apply debe confirmar que se aplicó, got:\n%s", out)
	}

	mems, _ := deps.MemoryRepo.ListAll("proj")
	if len(mems) != 1 {
		t.Fatalf("tras --apply debe quedar 1 fila, got %d", len(mems))
	}
}

func TestCmdConsolidate_NoGroups_SaysSoWithoutError(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	deps := &Deps{Root: root, Project: "proj", MemoryRepo: persistence.NewMemoryRepository(db)}

	out := captureStdout(t, func() { CmdConsolidate(deps, nil) })
	if !strings.Contains(out, "No hay memorias consolidables") {
		t.Fatalf("sin grupos debe decirlo explícitamente, got:\n%s", out)
	}
}
