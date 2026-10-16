package cli

import (
	"context"
	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type socraticaGCMaintenance struct{ ports.MaintenanceRepository }

func (socraticaGCMaintenance) Purge(ports.PurgeFilter) (int64, error) { return 0, nil }
func TestGC_PurgaSocraticaPendientes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	t.Chdir(root)
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repo := persistence.NewSocraticaStore(db, time.Second)
	project := persistence.ProjectKey(root)
	for _, tc := range []struct {
		id, client string
		phase      domain.SocraticaPhase
		age        time.Duration
	}{{"stale", "claude", domain.PhaseQuestions, 8 * 24 * time.Hour}, {"recent", "codex", domain.PhaseQuestions, time.Hour}, {"approved", "opencode", domain.PhaseApproved, 8 * 24 * time.Hour}, {"terminal", "claude", domain.PhaseDispatched, 25 * time.Hour}} {
		at := time.Now().Add(-tc.age)
		f := domain.RefinementFlow{ID: tc.id, Project: project, Client: tc.client, Phase: tc.phase, LastStablePhase: domain.PhaseCaptured, CreatedAt: at, UpdatedAt: at, SchemaVersion: 1}
		if err = repo.CreateFlow(context.Background(), f); err != nil {
			t.Fatal(err)
		}
	}
	lock := persistence.NewSocraticaDispatchLock(root)
	release, ok, err := lock.TryAcquire("stale")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	release()
	deps := &Deps{Root: root, Project: project, ProjectRepo: persistence.NewProjectRepository(), MaintenanceRepo: socraticaGCMaintenance{}, Socratica: &usecases.SocraticaFlow{Repo: repo, UOW: repo, Lock: lock, Clock: clock.SystemClock{}}}
	CmdGC(deps, []string{"--yes"})
	if _, err = os.Stat(filepath.Join(root, ".memory", "socratica", "locks", "stale.lock")); !os.IsNotExist(err) {
		t.Fatal("lock file of purged flow kept", err)
	}
	for _, id := range []string{"stale", "terminal"} {
		if _, err = repo.GetFlow(context.Background(), id); err == nil {
			t.Fatalf("%s not purged", id)
		}
	}
	for _, id := range []string{"recent", "approved"} {
		if _, err = repo.GetFlow(context.Background(), id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
}
