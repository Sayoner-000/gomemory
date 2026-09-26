package usecases_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func TestExportProjects_IndicePrivadoYFallosPorClave(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mem := persistence.NewMemoryRepository(db)
	rel := persistence.NewRelationRepository(db)
	if _, err := mem.ImportMemory(&domain.Memory{Project: "ok", Type: domain.Decision, Title: "A", Content: "cuerpo"}); err != nil {
		t.Fatal(err)
	}
	opened := map[string]bool{}
	open := func(key string) (ports.MemoryRepository, ports.RelationRepository, func(), bool, error) {
		opened[key] = true
		if key == "fallo" {
			return nil, nil, nil, false, errors.New("store ilegible")
		}
		return mem, rel, func() {}, key == "ok", nil
	}
	dir := filepath.Join(t.TempDir(), "export")
	index, failed := usecases.ExportProjects(dir, []usecases.ExportTarget{
		{Key: "ok", Root: "/proyecto/ok"},
		{Key: "ausente", Root: "/proyecto/ausente"},
		{Key: "fallo", Root: "/proyecto/fallo"},
	}, open)
	if len(index) != 1 || index[0].Key != "ok" || index[0].Memories != 1 || len(failed) != 1 || failed["fallo"] == nil {
		t.Fatalf("índice=%+v, fallos=%+v", index, failed)
	}
	if !opened["ok"] || !opened["ausente"] || !opened["fallo"] {
		t.Fatalf("no se procesaron todos los proyectos: %+v", opened)
	}
	for _, path := range []string{dir, filepath.Join(dir, "ok.json"), filepath.Join(dir, "index.json")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0o600)
		if info.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s: permisos %04o; quiero %04o", path, info.Mode().Perm(), want)
		}
	}

	blocked := filepath.Join(t.TempDir(), "archivo")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, bad := usecases.ExportProjects(filepath.Join(blocked, "no-es-directorio"), []usecases.ExportTarget{{Key: "ok"}}, open); bad["ok"] == nil {
		t.Fatal("un destino inválido debe bloquear la exportación de la clave")
	}
}
