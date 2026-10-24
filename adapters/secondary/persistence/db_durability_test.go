package persistence

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"mem/domain"
)

func TestDurability_CommitSurvivesAbruptExit(t *testing.T) {
	if path := os.Getenv("GOMEMORY_DURABILITY_CHILD"); path != "" {
		db, err := openDBFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := InsertMemory(db, &domain.Memory{Project: "durability", Type: domain.Learning, Title: "committed", Content: "survives abrupt process exit"}); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	path := filepath.Join(t.TempDir(), "mem.db")
	child := exec.Command(os.Args[0], "-test.run=^TestDurability_CommitSurvivesAbruptExit$")
	child.Env = append(os.Environ(), "GOMEMORY_DURABILITY_CHILD="+path)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("durability child failed: %v\n%s", err, out)
	}
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var content string
	if err := db.QueryRow("SELECT content FROM memories WHERE project='durability' AND title='committed'").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "survives abrupt process exit" {
		t.Fatalf("committed content changed: %q", content)
	}
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatalf("database integrity check: %s", integrity)
	}
}
