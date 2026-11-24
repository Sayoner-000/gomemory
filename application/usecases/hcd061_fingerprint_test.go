package usecases_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
)

// countingWorkspace cuenta las huellas calculadas: cada una lanza varios
// procesos git y lee los archivos cambiados (HCD-061).
type countingWorkspace struct {
	ports.HCDWorkspace
	fingerprints int
}

func (w *countingWorkspace) Fingerprint(ctx context.Context) (string, error) {
	w.fingerprints++
	return w.HCDWorkspace.Fingerprint(ctx)
}

// TestHCD061FingerprintPorComando: las mutaciones que solo escriben en hcd/ o
// en la base de datos calculan la huella una vez; Verify la recalcula tras
// ejecutar su comando, porque el comando puede cambiar el código.
func TestHCD061FingerprintPorComando(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	ws := &countingWorkspace{HCDWorkspace: hcdfiles.Workspace{Root: env.root}}
	store := persistence.NewHCDStore(env.db, "p")
	tpl, err := os.ReadFile("../../infrastructure/templates/hcd/unit/unit.md")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	uc := usecases.NewHCD(store, ws, &hcdTickClock{t: time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)}, func() string { n++; return fmt.Sprintf("id-%d", n) }, string(tpl))
	uc.ConfigureDefinitionReviews(store)
	uc.ConfigureDecisions(env.decisions, "p")
	env.uc = uc

	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "technical", Lane: "fast", Title: "Medir", Why: "w", Intent: "i"})
	if err != nil {
		t.Fatal(err)
	}
	editDoc(t, env, view, append(commonFill, "## Checklist\n", "## Checklist\n- [ ] 1. Paso → git --version\n")...)

	count := func(name string, want int, f func() error) {
		t.Helper()
		before := ws.fingerprints
		if err := f(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := ws.fingerprints - before; got != want {
			t.Errorf("%s calculó la huella %d veces, quiero %d", name, got, want)
		}
	}
	count("clarify ask", 1, func() error {
		_, _, err := env.uc.Clarify(ctx, usecases.HCDClarifyInput{Ask: "¿a o b?"})
		return err
	})
	count("clarify answer", 1, func() error {
		_, _, err := env.uc.Clarify(ctx, usecases.HCDClarifyInput{Question: "Q1", Answer: "a"})
		return err
	})
	count("rename", 1, func() error {
		v, err := env.uc.Rename(ctx, usecases.HCDRenameInput{Title: "Medir huellas"})
		view = v
		return err
	})
	count("verify", 2, func() error {
		_, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git --version"})
		return err
	})
	count("close", 1, func() error {
		_, err := env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo"})
		return err
	})

	view, err = env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "fast", Title: "Promover", Why: "w", Intent: "i"})
	if err != nil {
		t.Fatal(err)
	}
	editDoc(t, env, view, commonFill...)
	count("promote", 1, func() error {
		v, err := env.uc.Promote(ctx, "")
		view = v
		return err
	})
	editDoc(t, env, view, storyFill...)
	view, err = env.uc.Show(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	count("review socratic", 1, func() error {
		v := completeSocraticExam(t, env, view)
		view = v
		return nil
	})
	count("review splash", 1, func() error {
		view = skipSplash(t, env, view)
		return nil
	})
	count("approve", 1, func() error {
		_, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"})
		return err
	})
	count("cancel", 1, func() error {
		_, err := env.uc.Cancel(ctx, usecases.HCDCloseInput{Quote: "ya no"})
		return err
	})
}
