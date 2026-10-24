package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func containerCloseFixture(t *testing.T, channel string) (*Container, string) {
	t.Helper()
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".memory"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".memory", "settings.json"), []byte(`{"code_graph_disabled":true,"synapse_disabled":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := NewContainer(root, channel)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	for i := 0; i < 3; i++ {
		_, err := persistence.InsertMemory(c.db, &domain.Memory{Project: c.Project, Type: domain.Learning, Title: fmt.Sprintf("memory %d", i), Content: fmt.Sprintf("distinct content %d", i)})
		if err != nil {
			t.Fatal(err)
		}
	}
	return c, root
}

func TestContainerClose_HookChannelSkipsOptimize(t *testing.T) {
	c, root := containerCloseFixture(t, "hook")
	path := persistence.DbPath(root)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='sqlite_stat1'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("hook close created sqlite_stat1; optimize must be skipped")
	}
}

func TestContainerClose_NoErrorAndValidStatement(t *testing.T) {
	c, _ := containerCloseFixture(t, "cli")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA analysis_limit=400; PRAGMA optimize;"); err != nil {
		t.Fatalf("invalid optimize statement: %v", err)
	}
}

func TestUnderTempDir(t *testing.T) {
	tmp := t.TempDir()
	inside := filepath.Join(tmp, "proj")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		root, tmp string
		want      bool
	}{
		{inside, tmp, true},
		{tmp, tmp, true},
		{filepath.Dir(tmp), tmp, false},
		{tmp + "x", tmp, false},
		{inside, "", false},
	} {
		if got := underTempDir(tc.root, tc.tmp); got != tc.want {
			t.Errorf("underTempDir(%q, %q) = %v, want %v", tc.root, tc.tmp, got, tc.want)
		}
	}
}
