package main

import (
	"context"
	"encoding/json"
	"io"
	"mem/adapters/primary/cli/clikit"
	hcdcli "mem/adapters/primary/cli/hcd"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type hcdContractClock struct{}

func (hcdContractClock) Now() time.Time { return time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC) }
func invokeHCD(t *testing.T, deps *clikit.Deps, args ...string) (int, hcdcli.Response) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := hcdcli.Run(deps, append(args, "--json"))
	os.Stdout = old
	_ = w.Close()
	data, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	var response hcdcli.Response
	if err = json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return code, response
}
func TestHCDConnectedQueriesAndConflictingDefinition(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := persistence.NewHCDStore(db, "contract")
	files, err := hcdfiles.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	uc := usecases.NewHCD(repo, repo, files, repo, hcdContractClock{}, nil)
	uc.ConfigureDeliveryCodec(hcdfiles.ParseDelivery, hcdfiles.RenderDelivery)
	uc.ConfigureEvidenceParser(hcdfiles.ParseEvidence)
	uc.ConfigureDocuments("contract", nil, "# {Título}\nID: HCD-001\nTipo: fix\nImplementador: {persona}\nPlaybook: {revisión}\n", "playbook", "constitution")
	uc.ConfigureDefinitionParser(func(data []byte) (domain.Definition, error) { d, _, e := hcdfiles.ParseDefinition(data); return d, e })
	deps := &clikit.Deps{HCD: uc}
	code, start := invokeHCD(t, deps, "start", "--kind", "fix", "--title", "Guardar", "--implementer", "dev", "--client", "codex")
	if code != 0 {
		t.Fatalf("start=%+v code=%d", start, code)
	}
	d := domain.Definition{ID: start.UnitID, Kind: domain.KindFix, Title: "Guardar", Implementer: "dev", Playbook: start.PlaybookRevision, Problem: "fallo", Actor: "usuario", ExpectedResult: "guardar", Included: "persistir", Excluded: "borrar", Restrictions: "compatibilidad", ObservedFailure: "no guarda", Reproduction: "guardar", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "Guardar", Actor: "usuario", Text: "guardar datos"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "Guardado", Stories: []string{"US1"}, Condition: "queda guardado", Verification: domain.VerificationAutomated}}}
	content := hcdfiles.RenderDefinition(d)
	input := filepath.Join(t.TempDir(), "definition.md")
	if err = os.WriteFile(input, content, 0600); err != nil {
		t.Fatal(err)
	}
	code, defined := invokeHCD(t, deps, "define", start.UnitID, "--file", input, "--expected-revision", "1", "--operation-id", "define-1")
	if code != 0 || defined.Revision.State != 2 {
		t.Fatalf("define=%+v code=%d", defined, code)
	}
	if err = os.WriteFile(input, []byte("# Otra definición\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, conflict := invokeHCD(t, deps, "define", start.UnitID, "--file", input, "--expected-revision", "1", "--operation-id", "define-2")
	if code != 4 || conflict.Error == nil || conflict.Error.Code != "revision_conflict" {
		t.Fatalf("conflict=%+v code=%d", conflict, code)
	}
	saved, err := files.Read(context.Background(), "001-guardar", "definition.md")
	if err != nil || string(saved) != string(content) {
		t.Fatalf("stale write changed artifact err=%v", err)
	}
	for _, command := range []string{"show", "resume", "next", "validate"} {
		code, r := invokeHCD(t, deps, command, start.UnitID)
		if code != 0 || r.Revision.State != 2 {
			t.Fatalf("%s=%+v code=%d", command, r, code)
		}
	}
	code, opened := invokeHCD(t, deps, "clarify", start.UnitID, "--open", "--target", "C-01", "--question", "Comment vérifier ?", "--blocking", "--expected-revision", "2", "--operation-id", "clarify-open")
	if code != 0 || opened.Revision.State != 3 {
		t.Fatalf("clarify=%+v code=%d", opened, code)
	}
	code, answered := invokeHCD(t, deps, "clarify", start.UnitID, "--answer", "CL-01", "--choice", "Test automatique", "--expected-revision", "3", "--operation-id", "clarify-answer")
	if code != 0 || answered.Revision.State != 4 {
		t.Fatalf("answer=%+v code=%d", answered, code)
	}
	code, focused := invokeHCD(t, deps, "focus", start.UnitID)
	if code != 0 || !focused.OK {
		t.Fatalf("focus=%+v code=%d", focused, code)
	}
	code, approved := invokeHCD(t, deps, "approve", start.UnitID, "--revision", defined.Revision.Definition, "--quote", "apruebo", "--expected-revision", "4", "--client", "codex", "--operation-id", "approve-1")
	if code != 0 || approved.State != "ready" {
		t.Fatalf("approve=%+v code=%d", approved, code)
	}
	delivery := domain.Delivery{Slices: []domain.Slice{{ID: "VS-01", Observable: "Guardar datos", Limits: "Sin borrar", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}}, Tasks: []domain.Task{{ID: "T-001", Title: "Guardar datos", SliceID: "VS-01", Type: "implementation", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test", Rules: []string{"HCD-TRACE-01"}}}}
	deliveryFile := filepath.Join(t.TempDir(), "delivery.md")
	if err = os.WriteFile(deliveryFile, hcdfiles.RenderDelivery(start.UnitID, delivery), 0600); err != nil {
		t.Fatal(err)
	}
	code, sliced := invokeHCD(t, deps, "slice", start.UnitID, "--file", deliveryFile, "--expected-revision", strconv.FormatInt(approved.Revision.State, 10), "--operation-id", "slice")
	if code != 0 {
		t.Fatalf("slice=%+v", sliced)
	}
	code, tasks := invokeHCD(t, deps, "tasks", start.UnitID, "--slice", "VS-01", "--file", deliveryFile, "--expected-revision", strconv.FormatInt(sliced.Revision.State, 10), "--operation-id", "tasks")
	if code != 0 {
		t.Fatalf("tasks=%+v", tasks)
	}
	code, checked := invokeHCD(t, deps, "check", start.UnitID, "--task", "T-001")
	if code != 0 || checked.Revision.State != tasks.Revision.State {
		t.Fatalf("check=%+v code=%d", checked, code)
	}
	code, listed := invokeHCD(t, deps, "list")
	if code != 0 || !listed.OK {
		t.Fatalf("list=%+v", listed)
	}
	v, e := uc.Show(context.Background(), start.UnitID)
	if e != nil || v.Unit.StateRevision != tasks.Revision.State {
		t.Fatalf("list changed unit revision: %+v %v", v, e)
	}
	code, run := invokeHCD(t, deps, "run", start.UnitID, "--task", "T-001", "--client", "codex", "--expected-revision", strconv.FormatInt(tasks.Revision.State, 10), "--operation-id", "run")
	if code != 0 || run.ExecutionID == "" {
		t.Fatalf("run=%+v code=%d", run, code)
	}
	evFile := filepath.Join(t.TempDir(), "evidence.md")
	if err = os.WriteFile(evFile, []byte("Cubre: T-001, C-01\nResultado: pass\nProcedimiento: go test ./domain\nEntorno: temporary\nSujeto: tree:abc\nEjecutor: codex\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, reported := invokeHCD(t, deps, "report", start.UnitID, "--run", run.ExecutionID, "--file", evFile, "--expected-revision", strconv.FormatInt(run.Revision.State, 10), "--operation-id", "report")
	if code != 0 {
		t.Fatalf("report=%+v", reported)
	}
	code, blocked := invokeHCD(t, deps, "block", start.UnitID, "--target", "T-001", "--reason", "datos pendientes", "--expected-revision", strconv.FormatInt(reported.Revision.State, 10), "--operation-id", "block")
	if code != 0 || blocked.State != "in_progress" {
		t.Fatalf("block=%+v", blocked)
	}

	code, status := invokeHCD(t, deps, "status", start.UnitID)
	if code != 0 || status.Revision.State != blocked.Revision.State {
		t.Fatalf("status mutated state: %+v", status)
	}
	definitionPath := filepath.Join(root, "hcd", "features", "001-guardar", "definition.md")
	edited := append(append([]byte(nil), content...), []byte("\nReferencia externa: cambio\n")...)
	if err = os.WriteFile(definitionPath, edited, 0600); err != nil {
		t.Fatal(err)
	}
	code, closed := invokeHCD(t, deps, "close", start.UnitID, "--revision", defined.Revision.Definition, "--quote", "apruebo", "--expected-revision", strconv.FormatInt(blocked.Revision.State, 10), "--operation-id", "close-external")
	if code != 5 || closed.Error == nil || closed.Error.Code != "not_allowed" {
		t.Fatalf("external edit close=%+v code=%d", closed, code)
	}
	v, e = uc.Show(context.Background(), start.UnitID)
	if e != nil || v.Unit.State != domain.StateDraft || v.Unit.ApprovedRev != "" || v.Unit.DefinitionRev != domain.ContentRevision(edited) {
		t.Fatalf("external edit not reconciled: %+v %v", v.Unit, e)
	}

}
