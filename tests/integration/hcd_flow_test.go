package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	hcdcli "mem/adapters/primary/cli/hcd"
	"mem/adapters/secondary/hcdfiles"
	"mem/domain"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestHCDBinaryMinimalFlow(t *testing.T) {
	runHCDBinaryMinimalFlow(t, false)
}

func runHCDBinaryMinimalFlow(t *testing.T, checkFiles bool) {
	t.Helper()
	bin := buildMemBinary(t)
	root, home, inputs := t.TempDir(), t.TempDir(), t.TempDir()
	code := []byte("datos guardados\n")
	if err := os.WriteFile(filepath.Join(root, "result.txt"), code, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "README.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("Contenido previo que HCD debe preservar\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := hcdProjectSnapshot(t, root)
	var current hcdcli.Response
	call := func(command string, args ...string) hcdcli.Response {
		t.Helper()
		argv := []string{"hcd", command}
		if command != "start" {
			argv = append(argv, current.UnitID)
		}
		argv = append(argv, args...)
		argv = append(argv, "--json", "--client", "codex", "--operation-id", "flow-"+command)
		if command != "start" {
			argv = append(argv, "--expected-revision", strconv.FormatInt(current.Revision.State, 10))
		}
		cmd := exec.Command(bin, argv...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "GOMEMORY_NO_CODE_REFRESH=1")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s: %v stdout=%s", command, err, out)
		}
		var r hcdcli.Response
		if err = json.Unmarshal(out, &r); err != nil || !r.OK {
			t.Fatalf("%s: response=%s err=%v", command, out, err)
		}
		current = r
		return r
	}
	write := func(name string, content []byte) string {
		t.Helper()
		path := filepath.Join(inputs, name)
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	start := call("start", "--kind", "fix", "--title", "Guardar datos", "--implementer", "dev")
	d := domain.Definition{ID: start.UnitID, Kind: domain.KindFix, Title: "Guardar datos", Implementer: "dev", Playbook: start.PlaybookRevision, Problem: "Falla al guardar", Actor: "usuario", ExpectedResult: "Datos guardados", Included: "Persistencia", Excluded: "Borrado", Restrictions: "Compatibilidad", ObservedFailure: "No guarda", Reproduction: "Guardar datos", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "Guardar", Actor: "usuario", Text: "Guardar datos"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "Guardado", Stories: []string{"US1"}, Condition: "Datos persistidos", Verification: domain.VerificationAutomated}}}
	defined := call("define", "--file", write("definition.md", hcdfiles.RenderDefinition(d)))
	call("approve", "--revision", defined.Revision.Definition, "--quote", "apruebo")
	delivery := domain.Delivery{Slices: []domain.Slice{{ID: "VS-01", Observable: "Guardar datos", Limits: "Sin borrar", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}}, Tasks: []domain.Task{{ID: "T-001", Title: "Guardar datos", SliceID: "VS-01", Type: "implementation", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test", Rules: []string{"HCD-TRACE-01"}}}}
	file := write("delivery.md", hcdfiles.RenderDelivery(start.UnitID, delivery))
	call("slice", "--file", file)
	call("tasks", "--slice", "VS-01", "--file", file)
	call("check", "--task", "T-001")
	run := call("run", "--task", "T-001")
	sum := sha256.New()
	fmt.Fprintf(sum, "result.txt\x00%d\x00", len(code))
	sum.Write(code)
	subject := "tree:" + hex.EncodeToString(sum.Sum(nil))[:12] + " (archivos: result.txt)"
	evidence := fmt.Sprintf("Cubre: T-001, C-01\nResultado: pass\nProcedimiento: comprobar result.txt\nEntorno: integración temporal\nSujeto: %s\nEjecutor: codex\n", subject)
	call("report", "--run", run.ExecutionID, "--file", write("evidence.md", []byte(evidence)))
	verified := call("verify")
	if verified.State != "verification" {
		t.Fatalf("verify did not advance: %+v", verified)
	}
	closed := call("close", "--revision", defined.Revision.Definition, "--quote", "apruebo")
	if closed.State != "completed" {
		t.Fatalf("close=%+v", closed)
	}
	if _, err := os.Stat(filepath.Join(root, "specs")); !os.IsNotExist(err) {
		t.Fatalf("HCD created specs/: %v", err)
	}
	if checkFiles {
		after := hcdProjectSnapshot(t, root)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("HCD changed project files outside artifacts/database:\nbefore=%v\nafter=%v", before, after)
		}
	}
}
