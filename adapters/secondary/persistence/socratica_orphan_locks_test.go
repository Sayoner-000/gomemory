package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mem/application/usecases"
)

type fixedSocraticaClock struct{ now time.Time }

func (c fixedSocraticaClock) Now() time.Time { return c.now }

// TestSocraticaPurge_RecuperaLocksHuerfanos cierra H-05 (auditoría
// 2026-10-17): un lock que sobrevivió a su flujo (el proceso murió entre el
// commit de la purga y dropLocks) se elimina en la siguiente purga, sin tocar
// el lock de un flujo vivo, uno retenido ni ficheros ajenos.
func TestSocraticaPurge_RecuperaLocksHuerfanos(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := socraticaTestRepo(t)
	lock := NewSocraticaDispatchLock(root)
	if err := repo.CreateFlow(ctx, socraticaTestFlow("vivo", "claude")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"vivo", "huerfano"} {
		release, ok, err := lock.TryAcquire(id)
		if err != nil || !ok {
			t.Fatal(id, ok, err)
		}
		release()
	}
	releaseHeld, ok, err := lock.TryAcquire("retenido")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	defer releaseHeld()
	path := func(id string) string { p, _ := lock.path(id); return p }
	ajeno := filepath.Join(filepath.Dir(path("vivo")), "notas.txt")
	if err = os.WriteFile(ajeno, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	u := &usecases.SocraticaFlow{Repo: repo, UOW: repo, Lock: lock, Clock: fixedSocraticaClock{time.Unix(200, 0).UTC()}}
	if err = u.PurgeStale(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path("huerfano")); !os.IsNotExist(err) {
		t.Fatal("el lock huérfano sigue en disco", err)
	}
	for _, keep := range []string{path("vivo"), path("retenido"), ajeno} {
		if _, err = os.Stat(keep); err != nil {
			t.Fatalf("borró %s: %v", keep, err)
		}
	}
}

func TestSocraticaStore_FlowExists(t *testing.T) {
	ctx := context.Background()
	repo := socraticaTestRepo(t)
	if err := repo.CreateFlow(ctx, socraticaTestFlow("f", "claude")); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.FlowExists(ctx, "f"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := repo.FlowExists(ctx, "nope"); err != nil || ok {
		t.Fatal("un flujo inexistente no puede existir", ok, err)
	}
}
