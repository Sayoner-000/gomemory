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
	"strconv"
	"strings"
	"testing"
)

// Escenario 9 del quickstart (SC-002): una unidad de cada tipo restante, de principio a fin,
// primero con el campo particular ausente y después completa hasta close.
func TestHCDKindsEndToEnd(t *testing.T) {
	cases := []struct {
		kind   domain.Kind
		title  string
		field  string // campo particular que validate debe exigir
		fill   func(*domain.Definition)
		absent func(*domain.Definition)
	}{
		{domain.KindFeature, "Filtro por estado", "Actor", func(d *domain.Definition) {}, func(d *domain.Definition) { d.Actor = "" }},
		{domain.KindEvolution, "Verificaciones desactualizadas", "Compatibilidad", func(d *domain.Definition) {
			d.CurrentBehavior, d.Change, d.Compatibility = "status no las muestra", "mostrarlas", "sin cambiar el JSON existente"
		}, func(d *domain.Definition) {
			d.CurrentBehavior, d.Change, d.Compatibility = "status no las muestra", "mostrarlas", ""
		}},
		{domain.KindTechnical, "Resume con 50 tareas", "Condición de mejora", func(d *domain.Definition) {
			d.TechnicalProblem, d.Beneficiary, d.ImprovementCondition = "resume lento", "equipo", "menos de 200 ms"
		}, func(d *domain.Definition) {
			d.TechnicalProblem, d.Beneficiary, d.ImprovementCondition = "resume lento", "equipo", ""
		}},
	}
	bin := buildMemBinary(t)
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			root, home, inputs := t.TempDir(), t.TempDir(), t.TempDir()
			code := []byte("resultado\n")
			if err := os.WriteFile(filepath.Join(root, "result.txt"), code, 0o600); err != nil {
				t.Fatal(err)
			}
			var cur hcdcli.Response
			call := func(command string, args ...string) hcdcli.Response {
				t.Helper()
				argv := []string{"hcd", command}
				if command != "start" {
					argv = append(argv, cur.UnitID)
				}
				argv = append(argv, args...)
				argv = append(argv, "--json", "--client", "codex", "--operation-id", "k-"+command+"-"+strconv.FormatInt(cur.Revision.State, 10))
				if command != "start" && command != "validate" {
					argv = append(argv, "--expected-revision", strconv.FormatInt(cur.Revision.State, 10))
				}
				cmd := exec.Command(bin, argv...)
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "GOMEMORY_NO_CODE_REFRESH=1")
				out, err := cmd.Output()
				var r hcdcli.Response
				if jerr := json.Unmarshal(out, &r); jerr != nil || !r.OK {
					t.Fatalf("%s: %v response=%s", command, err, out)
				}
				codes := []string{}
				for _, p := range r.PendingChecks {
					codes = append(codes, p.Code+"@"+p.Ref)
				}
				t.Logf("%-8s → estado=%s rev=%d pendientes=%v", command, r.State, r.Revision.State, codes)
				cur = r
				return r
			}
			write := func(name string, content []byte) string {
				path := filepath.Join(inputs, name)
				if err := os.WriteFile(path, content, 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			}
			start := call("start", "--kind", string(c.kind), "--title", c.title, "--implementer", "dev")
			d := domain.Definition{ID: start.UnitID, Kind: c.kind, Title: c.title, Implementer: "dev", Playbook: start.PlaybookRevision, Problem: "Necesidad", Actor: "usuario", ExpectedResult: "Resultado", Included: "Núcleo", Excluded: "Resto", Restrictions: "Ninguna", Stories: []domain.Story{{ID: "US1", Title: "Historia", Actor: "usuario", Text: "Como usuario quiero esto"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "Criterio", Stories: []string{"US1"}, Condition: "Observable", Verification: domain.VerificationAutomated}}}
			c.fill(&d)
			missing := d
			c.absent(&missing)
			call("define", "--file", write("missing.md", hcdfiles.RenderDefinition(missing)))
			r := call("validate")
			found := false
			for _, p := range r.PendingChecks {
				found = found || p.Ref == c.field
			}
			if !found || r.State != "draft" {
				t.Fatalf("validate no exige %q: %+v", c.field, r)
			}
			defined := call("define", "--file", write("definition.md", hcdfiles.RenderDefinition(d)))
			if r = call("validate"); len(r.PendingChecks) != 0 {
				t.Fatalf("definición completa con pendientes: %+v", r.PendingChecks)
			}
			call("approve", "--revision", defined.Revision.Definition, "--quote", "apruebo")
			delivery := domain.Delivery{Slices: []domain.Slice{{ID: "VS-01", Observable: "Entrega", Limits: "Sin extras", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}}, Tasks: []domain.Task{{ID: "T-001", Title: "Entregar resultado", SliceID: "VS-01", Type: "implementation", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Result: "Entrega hecha", Scope: "Núcleo", Excluded: "Resto", Verification: "Test", Rules: []string{"HCD-TRACE-01"}}}}
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
			if v := call("verify"); v.State != "verification" {
				t.Fatalf("verify=%+v", v)
			}
			closed := call("close", "--revision", defined.Revision.Definition, "--quote", "apruebo")
			if closed.State != "completed" || !strings.HasPrefix(closed.UnitID, "HCD-") {
				t.Fatalf("close=%+v", closed)
			}
		})
	}
}
