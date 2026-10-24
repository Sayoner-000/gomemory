package persistence

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenDBFile_PragmasOnEveryConnection(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, conn *sql.Conn, busy int) {
		t.Helper()
		expected := map[string]int64{"busy_timeout": int64(busy), "synchronous": 1, "cache_size": -16000, "temp_store": 2, "mmap_size": 268435456, "journal_size_limit": 67108864, "foreign_keys": 0}
		for pragma, want := range expected {
			var got int64
			if err := conn.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil {
				t.Fatalf("read %s: %v", pragma, err)
			}
			if got != want {
				t.Errorf("%s = %d, want %d", pragma, got, want)
			}
		}
		var mode string
		if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if mode != "wal" {
			t.Errorf("journal_mode = %q, want wal", mode)
		}
	}
	t.Run("project pool", func(t *testing.T) {
		db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var conns []*sql.Conn
		defer func() {
			for _, conn := range conns {
				conn.Close()
			}
		}()
		for i := 0; i < 3; i++ {
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			conns = append(conns, conn)
		}
		for _, conn := range conns {
			check(t, conn, 5000)
		}
	})
	t.Run("global instincts", func(t *testing.T) {
		store, err := OpenInstinctGlobal(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		conn, err := store.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		check(t, conn, 100)
	})
}
