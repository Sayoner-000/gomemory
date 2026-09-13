package usecases

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

// C-001 (ACR acr_0106584b): solo "no existe" cuenta como ausencia. Cualquier
// otro error de stat (permisos, E/S) deja el ancla como no verificable.
func TestAnchorStatOf(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.go")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, missingErr := os.Stat(filepath.Join(dir, "no-existe.go"))

	for _, tc := range []struct {
		name string
		info fs.FileInfo
		err  error
		want domain.AnchorStat
	}{
		{"archivo", fileInfo, nil, domain.StatFile},
		{"directorio", dirInfo, nil, domain.StatOther},
		{"no existe", nil, missingErr, domain.StatMissing},
		{"permiso denegado", nil, &fs.PathError{Op: "stat", Path: "x", Err: fs.ErrPermission}, domain.StatOther},
		{"error de E/S", nil, errors.New("input/output error"), domain.StatOther},
	} {
		if got := anchorStatOf(tc.info, tc.err); got != tc.want {
			t.Fatalf("%s: anchorStatOf = %q, want %q", tc.name, got, tc.want)
		}
	}
}
