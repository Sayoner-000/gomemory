package main

import (
	"encoding/json"
	hcdcli "mem/adapters/primary/cli/hcd"
	"mem/adapters/secondary/hcdfiles"
	"mem/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// Escenarios 2, 3, 4 y 6 del quickstart con el binario compilado desde la rama.
func TestHCDQuickstartInvariantsStaleClarificationAndCoexistence(t *testing.T) {
	bin := buildMemBinary(t)
	root, home, inputs := t.TempDir(), t.TempDir(), t.TempDir()
	legacy := filepath.Join(root, "specs", "044-legado")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "spec.md"), []byte("# Legado\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	specsBefore := hcdProjectSnapshot(t, filepath.Join(root, "specs"))
	var cur hcdcli.Response
	exec1 := func(args ...string) (hcdcli.Response, int) {
		t.Helper()
		cmd := exec.Command(bin, append(append([]string{"hcd"}, args...), "--json", "--client", "codex")...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "GOMEMORY_NO_CODE_REFRESH=1")
		out, err := cmd.Output()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		var r hcdcli.Response
		if jerr := json.Unmarshal(out, &r); jerr != nil {
			t.Fatalf("%v: salida no JSON %q", args, out)
		}
		if r.OK && r.UnitID != "" {
			cur = r
		}
		return r, code
	}
	rev := func() string { return strconv.FormatInt(cur.Revision.State, 10) }
	write := func(name string, content []byte) string {
		path := filepath.Join(inputs, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	start, _ := exec1("start", "--kind", "fix", "--title", "Resume falla", "--implementer", "dev")
	def := domain.Definition{ID: start.UnitID, Kind: domain.KindFix, Title: "Resume falla", Implementer: "dev", Playbook: start.PlaybookRevision, Problem: "p", Actor: "usuario", ExpectedResult: "r", Included: "i", Excluded: "e", Restrictions: "x", ObservedFailure: "f", Reproduction: "rep", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "H", Actor: "usuario", Text: "Como usuario quiero"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "C", Stories: []string{"US1"}, Condition: "obs", Verification: domain.VerificationAutomated}}}
	file := write("d.md", hcdfiles.RenderDefinition(def))
	defined, _ := exec1("define", start.UnitID, "--file", file, "--operation-id", "op1", "--expected-revision", rev())

	// Escenario 2 — C-18: mismo id y contenido → mismo resultado sin avanzar; otro contenido → operation_conflict (exit 4).
	again, code := exec1("define", start.UnitID, "--file", file, "--operation-id", "op1", "--expected-revision", strconv.FormatInt(start.Revision.State, 10))
	if code != 0 || again.Revision.State != defined.Revision.State {
		t.Fatalf("repetición idempotente: code=%d %+v", code, again.Revision)
	}
	other := write("d2.md", append(hcdfiles.RenderDefinition(def), []byte("\nNota\n")...))
	if r, code := exec1("define", start.UnitID, "--file", other, "--operation-id", "op1", "--expected-revision", strconv.FormatInt(start.Revision.State, 10)); code != 4 || r.Error == nil || r.Error.Code != "operation_conflict" {
		t.Fatalf("operation_conflict: code=%d %+v", code, r.Error)
	}
	// C-17: revisión esperada antigua → revision_conflict (exit 4) y el archivo registrado no cambia.
	before, _ := os.ReadFile(filepath.Join(root, "hcd", "features", "001-resume-falla", "definition.md"))
	if r, code := exec1("define", start.UnitID, "--file", other, "--operation-id", "op2", "--expected-revision", "1"); code != 4 || r.Error == nil || r.Error.Code != "revision_conflict" {
		t.Fatalf("revision_conflict: code=%d %+v", code, r.Error)
	}
	if after, _ := os.ReadFile(filepath.Join(root, "hcd", "features", "001-resume-falla", "definition.md")); string(after) != string(before) {
		t.Fatal("una mutación rechazada cambió el archivo")
	}
	// C-11: las consultas no cambian la revisión.
	for _, q := range []string{"status", "next", "show", "resume"} {
		r, code := exec1(q, start.UnitID)
		if code != 0 || r.Revision.State != defined.Revision.State {
			t.Fatalf("%s cambió la revisión: code=%d %+v", q, code, r.Revision)
		}
	}

	// Escenario 4 — aclaración: una pregunta bloqueante impide aprobar hasta responderla (FR-016a).
	if r, code := exec1("clarify", start.UnitID, "--target", "C-01", "--question", "¿Qué medida?", "--options", "p95|media", "--recommendation", "p95", "--blocking", "--operation-id", "cl1", "--expected-revision", rev()); code != 0 || !r.OK {
		t.Fatalf("clarify: code=%d %+v", code, r.Error)
	}
	if r, code := exec1("approve", start.UnitID, "--revision", defined.Revision.Definition, "--quote", "apruebo", "--operation-id", "ap0", "--expected-revision", rev()); code == 0 || r.OK {
		t.Fatalf("approve con aclaración abierta debe bloquear: %+v", r)
	}
	if r, code := exec1("clarify", start.UnitID, "--answer", "CL-01", "--choice", "p95", "--operation-id", "cl2", "--expected-revision", rev()); code != 0 || !r.OK {
		t.Fatalf("responder aclaración: code=%d %+v", code, r.Error)
	}
	approved, code := exec1("approve", start.UnitID, "--revision", defined.Revision.Definition, "--quote", "apruebo", "--operation-id", "ap1", "--expected-revision", rev())
	if code != 0 || approved.State != "ready" {
		t.Fatalf("approve tras responder: code=%d %+v", code, approved)
	}

	// Escenario 3 — C-09: editar definition.md a mano; la consulta no escribe (FR-046) y la primera
	// mutación reconcilia: revision_conflict, unidad en draft sin aprobación, y close no se permite.
	path := filepath.Join(root, "hcd", "features", "001-resume-falla", "definition.md")
	content, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(content, []byte("\nEdición manual\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, _ := exec1("status", start.UnitID); r.State != "ready" {
		t.Fatalf("una consulta no debe reconciliar: %+v", r)
	}
	if r, code := exec1("clarify", start.UnitID, "--target", "C-01", "--question", "¿Otra?", "--options", "a|b", "--recommendation", "a", "--operation-id", "cl3", "--expected-revision", rev()); code != 4 || r.Error == nil || r.Error.Code != "revision_conflict" {
		t.Fatalf("la primera mutación debe detectar la edición externa: code=%d %+v", code, r.Error)
	}
	if r, _ := exec1("status", start.UnitID); r.State != "draft" || r.Revision.Approved != "" {
		t.Fatalf("la edición externa debe devolver a draft sin aprobación: %+v", r)
	}
	if r, code := exec1("close", start.UnitID, "--revision", defined.Revision.Definition, "--quote", "apruebo", "--operation-id", "cl4", "--expected-revision", rev()); code == 0 || r.OK {
		t.Fatalf("close tras edición externa debe rechazarse: %+v", r)
	}

	// Escenario 6 — C-02/C-24: la unidad Spec Kit sigue speckit, define responde approach_mismatch y specs/ no cambia.
	if r, code := exec1("define", "044-legado", "--file", file, "--operation-id", "mm", "--expected-revision", "1"); code == 0 || r.Error == nil || r.Error.Code != "approach_mismatch" {
		t.Fatalf("approach_mismatch: code=%d %+v", code, r.Error)
	}
	if after := hcdProjectSnapshot(t, filepath.Join(root, "specs")); len(after) != len(specsBefore) || after["044-legado/spec.md"] != specsBefore["044-legado/spec.md"] {
		t.Fatalf("specs/ cambió: %v → %v", specsBefore, after)
	}
}
