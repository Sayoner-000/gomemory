package main

import (
	"fmt"
	"mem/adapters/secondary/persistence"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// FR-051: la ceremonia de Especificación de Socrática (socratica_flow mode=specify
// y mem socratica) conserva su comportamiento y no crea filas hcd_* ni archivos bajo hcd/.
func TestSocraticaSpecifyDoesNotTouchHCD(t *testing.T) {
	home, target, dataHome := t.TempDir(), t.TempDir(), t.TempDir()
	f := &specifyFixture{socraticaSession: newSocraticaSessionAt(t, home, target, dataHome), home: home, counter: filepath.Join(t.TempDir(), "speckit-invocations"), workdir: t.TempDir()}
	id, review := f.toReview("Quiero exportar mi historial")
	if review["phase"] != "review" {
		t.Fatalf("specify no llegó a revisión: %v", review["phase"])
	}
	for _, args := range [][]string{{"socratica", "status"}, {"socratica", "pending", "--resume", id}} {
		cmd := exec.Command(buildMemBinary(t), args...)
		cmd.Dir = target
		cmd.Env = append(os.Environ(), "HOME="+home, "GOMEMORY_DATA_HOME="+dataHome, "GOMEMORY_NO_CODE_REFRESH=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mem %v: %v\n%s", args, err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "hcd")); !os.IsNotExist(err) {
		t.Fatalf("Socrática creó hcd/: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'hcd\_%' ESCAPE '\'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err = rows.Close(); err != nil || len(tables) == 0 {
		t.Fatalf("las tablas hcd_* deben existir para que la prueba tenga sentido: %v %v", tables, err)
	}
	for _, table := range tables {
		var n int
		if err = db.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&n); err != nil || n != 0 {
			t.Errorf("Socrática escribió en %s: %d filas (%v)", table, n, err)
		}
	}
}
