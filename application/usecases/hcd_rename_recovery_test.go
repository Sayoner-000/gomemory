package usecases_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// La recuperación usa SQLite y el historial real, incluso después de reabrir
// la base: ningún estado en memoria puede ocultar el reapunte pendiente.
func TestHCDRetargetRecoveryAfterRestart(t *testing.T) {
	for _, failure := range []struct{ name, trigger string }{
		{"memory", `BEFORE UPDATE OF filepath ON memories`},
		{"links", `BEFORE UPDATE OF path ON memory_files`},
		{"acknowledgement", `BEFORE DELETE ON hcd_pending_retargets`},
	} {
		t.Run(failure.name, func(t *testing.T) {
			for _, manual := range []bool{false, true} {
				t.Run(map[bool]string{false: "rename", true: "manual"}[manual], func(t *testing.T) {
					env, ctx := newHCDEnv(t), context.Background()
					history := persistence.NewDecisionHistoryRepository(env.db)
					env.uc.ConfigureDecisions(history, "p")
					view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "w"})
					if err != nil {
						t.Fatal(err)
					}
					old, next := view.Unit.DocPath, "hcd/HCD-001-nuevo.md"
					m := &domain.Memory{Project: "p", Type: "decision", Title: "Related", Content: "Related decision", Filepath: old, TopicKey: "audit/recovery"}
					id, err := history.UpsertDigest(m, []string{old}, true)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = env.db.Exec(`CREATE TRIGGER fail_retarget ` + failure.trigger + ` BEGIN SELECT RAISE(ABORT, 'retarget unavailable'); END`); err != nil {
						t.Fatal(err)
					}
					if manual {
						if err = os.Rename(filepath.Join(env.root, old), filepath.Join(env.root, next)); err != nil {
							t.Fatal(err)
						}
						data, readErr := os.ReadFile(filepath.Join(env.root, next))
						if readErr != nil {
							t.Fatal(readErr)
						}
						writeIn(t, env.root, next, string(domain.ReplaceUnitTitle(data, "Nuevo")))
						_, err = env.uc.Show(ctx, view.Unit.ID)
					} else {
						_, err = env.uc.Rename(ctx, usecases.HCDRenameInput{ID: view.Unit.ID, Title: "Nuevo"})
					}
					if err == nil {
						t.Fatal("expected injected retarget failure")
					}
					wantPath := old
					if failure.name == "acknowledgement" {
						wantPath = next
					}
					var path string
					if err = env.db.QueryRow(`SELECT filepath FROM memories WHERE id = ?`, id).Scan(&path); err != nil || path != wantPath {
						t.Fatalf("path after failure = %q, %v", path, err)
					}
					if err = env.db.QueryRow(`SELECT path FROM memory_files WHERE memory_id = ?`, id).Scan(&path); err != nil || path != wantPath {
						t.Fatalf("link after failure = %q, %v", path, err)
					}
					if err = env.db.Close(); err != nil {
						t.Fatal(err)
					}
					db, err := persistence.Init(env.root)
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					store := persistence.NewHCDStore(db, "p")
					from, to, err := store.PendingRetarget(ctx, view.Unit.ID)
					if err != nil || from != old || to != next {
						t.Fatalf("durable pending = %q -> %q, %v", from, to, err)
					}
					uc := usecases.NewHCD(store, hcdfiles.Workspace{Root: env.root}, &hcdTickClock{t: time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)}, func() string { return "retry" }, "")
					if _, err = uc.Show(ctx, view.Unit.ID); err == nil {
						t.Fatal("unconfigured history silently discarded pending retarget")
					}
					uc.ConfigureDecisions(persistence.NewDecisionHistoryRepository(db), "p")
					if manual {
						_, err = uc.Show(ctx, view.Unit.ID)
					} else {
						_, err = uc.Rename(ctx, usecases.HCDRenameInput{ID: view.Unit.ID, Title: "Nuevo"})
					}
					if err == nil {
						t.Fatal("retry falsely succeeded while retarget still fails")
					}
					if _, err = db.Exec(`DROP TRIGGER fail_retarget`); err != nil {
						t.Fatal(err)
					}
					if manual {
						_, err = uc.Show(ctx, view.Unit.ID)
					} else {
						_, err = uc.Rename(ctx, usecases.HCDRenameInput{ID: view.Unit.ID, Title: "Nuevo"})
					}
					if err != nil {
						t.Fatal(err)
					}
					if err = db.QueryRow(`SELECT filepath FROM memories WHERE id = ?`, id).Scan(&path); err != nil || path != next {
						t.Fatalf("memory path = %q, %v", path, err)
					}
					if err = db.QueryRow(`SELECT path FROM memory_files WHERE memory_id = ?`, id).Scan(&path); err != nil || path != next {
						t.Fatalf("linked path = %q, %v", path, err)
					}
					from, _, err = store.PendingRetarget(ctx, view.Unit.ID)
					if err != nil || from != "" {
						t.Fatalf("pending not cleared: %q, %v", from, err)
					}
					digests, err := persistence.NewDecisionHistoryRepository(db).Digests("p")
					if err != nil {
						t.Fatal(err)
					}
					var found bool
					for _, digest := range digests {
						if digest.TopicKey == domain.HCDDigestTopic(view.Unit.ID) {
							found = digest.Filepath == next && strings.Contains(digest.Title, "Nuevo")
						}
					}
					if !found {
						t.Fatalf("recovery did not refresh digest: %+v", digests)
					}
				})
			}
		})
	}
}
