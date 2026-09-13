package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

func TestCmdSave_AvisaEnStderrYConservaStdout(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	deps := &Deps{MemoryRepo: persistence.NewMemoryRepository(db), ProjectRepo: &fakeProjectRepo{root: "/tmp/demo"}, SessionRepo: persistence.NewSessionRepository(db)}
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout = outW
	os.Stderr = errW
	CmdSave(deps, []string{"-t", "Caché Redis compartida para latencia", "la caché redis compartida reduce latencia"})
	CmdSave(deps, []string{"-t", "Caché Redis compartida para reducir latencia", "la caché redis compartida reduce latencia"})
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	out, _ := io.ReadAll(outR)
	notice, _ := io.ReadAll(errR)
	if !strings.Contains(string(out), "✓ Memoria guardada") || !strings.Contains(string(notice), "Posible duplicado de #") {
		t.Fatalf("stdout=%q stderr=%q", out, notice)
	}
}
