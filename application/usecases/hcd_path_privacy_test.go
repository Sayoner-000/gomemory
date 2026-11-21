package usecases_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type privacyWorkspace struct {
	ports.HCDWorkspace
	privateReads int
}

func (w *privacyWorkspace) ReadDoc(p string) ([]byte, error) {
	if domain.NeedsSanitize(p) {
		w.privateReads++
		return nil, fmt.Errorf("unsafe read: %s", p)
	}
	return w.HCDWorkspace.ReadDoc(p)
}

func TestHCDPrivatePathsRejectedBeforeUse(t *testing.T) {
	for _, route := range []string{"discovered", "ambiguous", "stored", "pending-from", "pending-to"} {
		t.Run(route, func(t *testing.T) {
			env, ctx := newHCDEnv(t), context.Background()
			v, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "privacy"})
			if err != nil {
				t.Fatal(err)
			}
			old := v.Unit.DocPath
			bad := "hcd/" + v.Unit.ID + "-ghp_" + strings.Repeat("A", 36) + ".md"
			switch route {
			case "discovered", "ambiguous":
				if err := os.Rename(filepath.Join(env.root, old), filepath.Join(env.root, bad)); err != nil {
					t.Fatal(err)
				}
				if route == "ambiguous" {
					if err := os.WriteFile(filepath.Join(env.root, "hcd/"+v.Unit.ID+"-other.md"), []byte("# Other"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "stored":
				if _, err := env.db.Exec(`UPDATE hcd_units SET doc_path = ? WHERE id = ?`, bad, v.Unit.ID); err != nil {
					t.Fatal(err)
				}
			default:
				from, to := old, bad
				if route == "pending-from" {
					from, to = bad, old
				}
				if _, err := env.db.Exec(`INSERT INTO hcd_pending_retargets(project,unit_id,from_path,to_path) VALUES('p',?,?,?)`, v.Unit.ID, from, to); err != nil {
					t.Fatal(err)
				}
			}
			ws := &privacyWorkspace{HCDWorkspace: hcdfiles.Workspace{Root: env.root}}
			uc := usecases.NewHCD(persistence.NewHCDStore(env.db, "p"), ws, auditFixedClock{}, func() string { return "unused" }, "")
			history := &retargets{hcdDecisions: &hcdDecisions{}}
			uc.ConfigureDecisions(history, "p")
			got, err := uc.Show(ctx, v.Unit.ID)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatal("private route must be rejected")
			}
			if strings.Contains(fmt.Sprint(got, err), bad) || ws.privateReads != 0 {
				t.Fatal("private route reached workspace or output")
			}
		})
	}
}

func TestHCDPrivacyPersistenceBoundaries(t *testing.T) {
	for _, route := range []string{"metadata-to", "metadata-from", "metadata-equal", "retarget-to", "retarget-from", "retarget-equal"} {
		t.Run(route, func(t *testing.T) {
			env, ctx := newHCDEnv(t), context.Background()
			v, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "privacy"})
			if err != nil {
				t.Fatal(err)
			}
			bad := "hcd/" + v.Unit.ID + "-ghp_" + strings.Repeat("A", 36) + ".md"
			from, to := v.Unit.DocPath, bad
			if strings.HasSuffix(route, "from") {
				from, to = bad, from
			}
			if strings.HasSuffix(route, "equal") {
				from, to = bad, bad
			}
			if strings.HasPrefix(route, "metadata") {
				u := v.Unit
				u.DocPath = to
				err = persistence.NewHCDStore(env.db, "p").SaveMetadata(ctx, u, from, true)
			} else {
				err = persistence.NewDecisionHistoryRepository(env.db).RetargetPath("p", from, to)
			}
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatal("persistence boundary must reject private identity")
			}
			if strings.Contains(err.Error(), bad) {
				t.Fatal("private path leaked in error")
			}
			var count int
			if err := env.db.QueryRow(`SELECT COUNT(*) FROM hcd_pending_retargets`).Scan(&count); err != nil || count != 0 {
				t.Fatal("private pending persisted")
			}
			stored, err := persistence.NewHCDStore(env.db, "p").Get(ctx, v.Unit.ID)
			if err != nil || stored.DocPath != v.Unit.DocPath {
				t.Fatal("private unit persisted")
			}
		})
	}
}
