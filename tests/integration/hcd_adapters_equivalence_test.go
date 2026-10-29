package main

import (
	"context"
	"encoding/json"
	"io"
	"mem/adapters/primary/cli/clikit"
	hcdcli "mem/adapters/primary/cli/hcd"
	"mem/adapters/primary/tui"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"reflect"
	"testing"
	"time"
)

type equivalenceClock struct{}

func (equivalenceClock) Now() time.Time { return time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC) }
func equivalenceHCD(t *testing.T) (*usecases.HCD, *persistence.HCDStore) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := persistence.NewHCDStore(db, "equivalence")
	files, err := hcdfiles.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	uc := usecases.NewHCD(repo, repo, files, repo, equivalenceClock{}, nil)
	uc.ConfigureDocuments("equivalence", nil, "# {Título}\n", "playbook", "constitution")
	return uc, repo
}
func TestHCDAdaptersStartEquivalent(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindFeature, domain.KindFix, "invalid"} {
		t.Run(string(kind), func(t *testing.T) {
			cli, cliRepo := equivalenceHCD(t)
			ui, uiRepo := equivalenceHCD(t)
			input := usecases.HCDStartInput{Kind: kind, Title: "Guardar datos", Implementer: "dev"}
			unit, uiErr := tui.StartSpecification(context.Background(), ui, input)
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			old := os.Stdout
			os.Stdout = w
			code := hcdcli.Run(&clikit.Deps{HCD: cli}, []string{"start", "--kind", string(kind), "--title", input.Title, "--implementer", input.Implementer, "--client", "terminal", "--json"})
			os.Stdout = old
			_ = w.Close()
			data, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
			var response hcdcli.Response
			if err = json.Unmarshal(data, &response); err != nil {
				t.Fatal(err)
			}
			if (uiErr == nil) != (code == 0) {
				t.Fatalf("ui=%v cli=%d %s", uiErr, code, data)
			}
			if uiErr != nil {
				return
			}
			stored, err := cliRepo.GetUnit(context.Background(), response.UnitID)
			if err != nil || !reflect.DeepEqual(unit, stored) {
				t.Fatalf("ui=%+v cli=%+v err=%v", unit, stored, err)
			}
			for _, repo := range []*persistence.HCDStore{cliRepo, uiRepo} {
				a, err := repo.ListApprovals(context.Background(), unit.ID)
				if err != nil || len(a) != 0 {
					t.Fatalf("start exposed approval: %+v %v", a, err)
				}
			}
		})
	}
	starter := reflect.TypeOf((*tui.HCDStarter)(nil)).Elem()
	if starter.NumMethod() != 1 || starter.Method(0).Name != "Start" {
		t.Fatal("TUI creation boundary exposes other mutations")
	}
}
