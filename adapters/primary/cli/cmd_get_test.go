package cli

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func TestCmdGet_ExistingID_PrintsDetail(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	id, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "una decisión", Content: "contenido de prueba"})

	deps := &Deps{Root: root, Project: "proj", MemoryRepo: memRepo}

	out := captureStdout(t, func() { CmdGet(deps, []string{"1"}) })
	_ = id
	if !strings.Contains(out, "una decisión") || !strings.Contains(out, "contenido de prueba") {
		t.Fatalf("se esperaba el detalle completo, got:\n%s", out)
	}
}
