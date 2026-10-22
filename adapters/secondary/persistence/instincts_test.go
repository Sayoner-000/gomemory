package persistence

import (
	"context"
	"math"
	"mem/domain"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestInstinctFoldIdempotenceAndGlobal(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	repo := NewInstinctRepository(db, "project")
	batch := []domain.Observation{}
	for i := range 16 {
		batch = append(batch, domain.Observation{ID: fmtInstinctID(i), T: now, Kind: "cmd", Key: "cmd:go-test", Sample: "go test ./..."})
	}
	for range 2 {
		if err := repo.Fold(ctx, batch, now); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(ctx, true)
	if err != nil || len(list) != 1 || list[0].Observations != 16 || math.Abs(list[0].Confidence-.7) > 1e-9 {
		t.Fatalf("%+v %v", list, err)
	}
	if err := repo.SetStatus(ctx, "cmd:go-test", "rejected"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fold(ctx, []domain.Observation{{ID: "new", Kind: "cmd", Key: "cmd:go-test"}}, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStatus(ctx, "cmd:go-test", "confirmed"); err == nil {
		t.Fatal("revived rejected instinct")
	}
	global, err := OpenInstinctGlobal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer global.Close()
	for _, p := range []string{"one", "one", "two"} {
		if err := global.RecordProject(ctx, p, list[0], now); err != nil {
			t.Fatal(err)
		}
	}
	all, err := global.GlobalFor(ctx, .7)
	if err != nil || len(all) != 1 || all[0].Scope != "global" || len(all[0].Projects) != 2 {
		t.Fatalf("%+v %v", all, err)
	}
	info, err := os.Stat(global.path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("permissions: %v %v", info, err)
	}
	// Additive migration preserves existing memory data.
	if _, err := db.Exec(`INSERT INTO memories(project,type,title,content) VALUES(?,?,?,?)`, "project", "learning", "keep", "keep"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM memories WHERE title=?`, "keep").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
func fmtInstinctID(i int) string { return string(rune('a' + i)) }
