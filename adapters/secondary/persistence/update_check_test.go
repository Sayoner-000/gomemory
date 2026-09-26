package persistence

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"mem/domain"
)

// R4: la caché de versiones vive en el almacén global, se escribe de forma
// atómica y privada, y una caché ausente o corrupta cuenta como vencida.
func TestUpdateCheckRepository(t *testing.T) {
	home := t.TempDir()
	t.Setenv(dataHomeEnvOverride, home)
	repo := UpdateCheckRepository{}
	ctx := context.Background()

	if _, ok := repo.Read(ctx); ok {
		t.Fatal("sin archivo no hay caché")
	}

	want := domain.UpdateCheck{
		Latest: "v2.27.0", ETag: `"abc"`,
		CheckedAt: time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC),
	}
	if err := repo.Write(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, ok := repo.Read(ctx)
	if !ok || got.Latest != want.Latest || got.ETag != want.ETag || !got.CheckedAt.Equal(want.CheckedAt) {
		t.Fatalf("Read = %+v, %v", got, ok)
	}

	path := filepath.Join(home, updateCheckFile)
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Errorf("permisos = %o; quiero 0600", info.Mode().Perm())
		}
	}
	if left, _ := filepath.Glob(filepath.Join(home, "*.tmp")); len(left) != 0 {
		t.Errorf("quedaron temporales: %v", left)
	}

	if err := os.WriteFile(path, []byte("{roto"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.Read(ctx); ok {
		t.Error("una caché corrupta se trata como ausente")
	}
}
