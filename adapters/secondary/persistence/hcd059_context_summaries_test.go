package persistence_test

import (
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"strings"
	"testing"
)

func TestHCD059ContextSummaries(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	titles := []string{"normal", "", "\x1b[31m\x1b[0m", "日本語", "\n"}
	for i, title := range titles {
		_, err := db.Exec(`INSERT INTO memories(project,session_id,type,title,content,filepath,created_at,updated_at) VALUES('perf','session','learning',?,?,?,'2026','2026')`, title, strings.Repeat("\x1b[31m日本語 árbol\x1b[0m", 30), "file.go")
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}
	_, err = db.Exec(`INSERT INTO memories(project,type,title,content,created_at,updated_at) VALUES('other','learning','other','body','2026','2026')`)
	if err != nil {
		t.Fatal(err)
	}
	repo := persistence.NewMemoryRepository(db)
	p, ok := repo.(ports.ContextMemorySummaryLister)
	if !ok {
		t.Fatal("repository lacks summary port")
	}
	rows, err := p.ListContextSummaries("perf")
	if err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListAll("perf")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(all) {
		t.Fatal("project isolation/count")
	}
	for i, m := range all {
		title := m.Title
		if title == "" {
			r := []rune(m.Content)
			if len(r) > 60 {
				title = string(r[:57]) + "..."
			} else {
				title = string(r)
			}
		}
		s := rows[i]
		if s.ID != m.ID || s.Type != m.Type || s.SessionID != m.SessionID || s.DisplayTitle != title || s.Filepath != m.Filepath {
			t.Fatalf("row%d: %+v want title%q", i, s, title)
		}
	}
	empty, err := p.ListContextSummaries("absent")
	if err != nil || len(empty) != 0 {
		t.Fatal("empty project")
	}
}
