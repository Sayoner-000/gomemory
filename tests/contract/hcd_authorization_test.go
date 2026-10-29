package main

import (
	"context"
	"mem/adapters/primary/cli/clikit"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestHCDAuthorizationRejectsStateAndUnquotedDecisions(t *testing.T) {
	root := t.TempDir()
	db, e := persistence.Init(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	repo := persistence.NewHCDStore(db, "contract")
	files, e := hcdfiles.NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	uc := usecases.NewHCD(repo, repo, files, repo, hcdContractClock{}, nil)
	uc.ConfigureDocuments("contract", nil, "# {Título}\nID: HCD-001\nTipo: fix\nImplementador: {persona}\nPlaybook: {revisión}\n", "playbook", "constitution")
	uc.ConfigureDefinitionParser(func(data []byte) (domain.Definition, error) { d, _, e := hcdfiles.ParseDefinition(data); return d, e })
	deps := &clikit.Deps{HCD: uc}
	u, e := uc.Start(context.Background(), usecases.HCDStartInput{Kind: domain.KindFix, Title: "Guardar", Implementer: "dev"})
	if e != nil {
		t.Fatal(e)
	}
	for _, command := range []string{"define", "tasks", "approve"} {
		code, r := invokeHCD(t, deps, command, u.ID, "--state", "verified")
		if code != 2 || r.Error == nil || r.Error.Code != "usage" {
			t.Fatalf("%s accepted state: %+v code=%d", command, r, code)
		}
	}
	path := filepath.Join(t.TempDir(), "definition.md")
	if e = os.WriteFile(path, []byte("# Guardar\nEstado: verified\n"), 0600); e != nil {
		t.Fatal(e)
	}
	code, r := invokeHCD(t, deps, "define", u.ID, "--file", path, "--expected-revision", "1")
	if code != 2 {
		t.Fatalf("state artifact accepted %+v code=%d", r, code)
	}
	decisionFile := filepath.Join(t.TempDir(), "decision.json")
	if e = os.WriteFile(decisionFile, []byte(`{"Kind":"scope-change","Summary":"Cambiar alcance","Status":"accepted","Refs":["T-001"]}`), 0600); e != nil {
		t.Fatal(e)
	}
	code, r = invokeHCD(t, deps, "decision", u.ID, "--file", decisionFile, "--expected-revision", "1", "--operation-id", "propose")
	if code != 0 {
		t.Fatalf("propose=%+v code=%d", r, code)
	}
	ds, e := repo.ListDecisions(context.Background(), u.ID)
	if e != nil || ds[len(ds)-1].Status != "proposed" {
		t.Fatalf("agent accepted decision: %+v %v", ds, e)
	}
	id := ds[len(ds)-1].ID
	rev := strconv.FormatInt(r.Revision.State, 10)
	for _, action := range []string{"accept", "supersede"} {
		code, r = invokeHCD(t, deps, "decision", u.ID, "--"+action, id, "--expected-revision", rev, "--operation-id", "unquoted-"+action)
		if code != 5 || r.Error == nil || r.Error.Code != "invalid_quote" {
			t.Fatalf("unquoted %s=%+v code=%d", action, r, code)
		}
	}
	for _, command := range []string{"approve", "close", "cancel"} {
		code, r = invokeHCD(t, deps, command, u.ID, "--client", "tui", "--quote", "apruebo", "--revision", u.DefinitionRev, "--expected-revision", rev)
		if code != 5 || r.Error == nil || r.Error.Code != "not_allowed" {
			t.Fatalf("TUI %s=%+v code=%d", command, r, code)
		}
	}
	orphan := []byte("# Huérfana\nID: HCD-002\nTipo: fix\nImplementador: dev\nPlaybook: playbook\n")
	if e = files.Write(context.Background(), "002-orphan", "definition.md", orphan); e != nil {
		t.Fatal(e)
	}
	code, r = invokeHCD(t, deps, "classify", "hcd/features/002-orphan", "--approach", "hcd", "--quote", "apruebo", "--client", "codex")
	if code != 0 || r.Approach != "hcd" || r.UnitID != "HCD-002" {
		t.Fatalf("classify=%+v code=%d", r, code)
	}

}
