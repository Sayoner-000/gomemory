package usecases_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type hcdTickClock struct{ t time.Time }

func (c *hcdTickClock) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

type hcdDecisions struct{ titles []string }

func (d *hcdDecisions) Insert(m *domain.Memory) (int64, error) {
	d.titles = append(d.titles, m.Title)
	return int64(len(d.titles)), nil
}

type hcdEnv struct {
	uc        *usecases.HCD
	db        *sql.DB
	root      string
	decisions *hcdDecisions
}

func newHCDEnv(t *testing.T) hcdEnv {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		gitIn(t, root, args...)
	}
	writeIn(t, root, "save.go", "package save\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "init")
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	tpl, err := os.ReadFile("../../infrastructure/templates/hcd/unit/unit.md")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	uc := usecases.NewHCD(persistence.NewHCDStore(db, "p"), hcdfiles.Workspace{Root: root}, &hcdTickClock{t: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}, func() string { n++; return fmt.Sprintf("id-%d", n) }, string(tpl))
	d := &hcdDecisions{}
	uc.ConfigureDecisions(d, "p")
	return hcdEnv{uc: uc, db: db, root: root, decisions: d}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeIn(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// editDoc aplica pares de reemplazo al documento de la unidad, como haría la
// persona o el agente al redactarlo.
func editDoc(t *testing.T, env hcdEnv, view usecases.HCDView, pairs ...string) {
	t.Helper()
	full := filepath.Join(env.root, view.Unit.DocPath)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.NewReplacer(pairs...).Replace(string(data))
	if out == string(data) {
		t.Fatalf("editDoc did not change the document:\n%s", data)
	}
	if err := os.WriteFile(full, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

var commonFill = []string{
	"## Reproducción\npendiente", "## Reproducción\ngit grep -q fixed -- save.go",
	"Incluido: pendiente", "Incluido: guardado",
	"Excluido: pendiente", "Excluido: exportación",
	"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nse pierden datos",
	"## Alternativa más barata\npendiente", "## Alternativa más barata\nninguna",
}

var storyFill = []string{"### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Guardar\nComo usuario, quiero guardar, para no perder datos\n- C-01: Dado un cambio, cuando guardo, entonces persiste"}

func stage(t *testing.T, env hcdEnv, want domain.Stage, skill string) usecases.HCDView {
	t.Helper()
	view, err := env.uc.Show(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if view.Stage != want || view.Next.Skill != skill {
		t.Fatalf("stage = %s (%+v), want %s/%s; pending %+v", view.Stage, view.Next, want, skill, view.State.Pending)
	}
	return view
}

func TestHCDFullCeremonyStepByStep(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "full", Title: "Exportar informes", Why: "decisiones de formato interdependientes", Intent: "Exportar informes a CSV", Client: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	stage(t, env, domain.StageDefine, "hcd-define")
	editDoc(t, env, view, append(commonFill, storyFill...)...)

	view, qid, err := env.uc.Clarify(ctx, usecases.HCDClarifyInput{Ask: "¿Separador coma o punto y coma?"})
	if err != nil || qid != "Q1" {
		t.Fatalf("clarify ask = %s %v", qid, err)
	}
	stage(t, env, domain.StageClarify, "hcd-clarify")
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("approve with open clarification = %v", err)
	}
	if _, _, err = env.uc.Clarify(ctx, usecases.HCDClarifyInput{Question: "Q1", Answer: "coma"}); err != nil {
		t.Fatal(err)
	}
	view = stage(t, env, domain.StageApprove, "hcd-approve")
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "vale"}); !errors.Is(err, domain.ErrInvalidQuote) {
		t.Fatalf("approve with invalid quote = %v", err)
	}
	if _, err = env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo", Client: "claude"}); err != nil {
		t.Fatal(err)
	}
	stage(t, env, domain.StageSlice, "hcd-slice")
	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git --version"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("verify before tasks = %v", err)
	}
	editDoc(t, env, view, "## Slices\n", "## Slices\n### VS-01. Exportar CSV\nTareas: 1\nCriterios: C-01\n")
	stage(t, env, domain.StageTasks, "hcd-tasks")
	editDoc(t, env, view, "## Checklist\n", "## Checklist\n- [ ] 1. Escribir el exportador → git grep -q csv -- save.go\n")
	stage(t, env, domain.StageImplement, "hcd-implement")

	writeIn(t, env.root, "save.go", "package save // csv\n")
	view, ev, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q csv -- save.go"})
	if err != nil || !ev.Passed() || view.Stage != domain.StageClose {
		t.Fatalf("verify = %+v %s %v", ev, view.Stage, err)
	}
	_, findings, err := env.uc.Analyze(ctx, "")
	if err != nil || domain.HasCritical(findings) {
		t.Fatalf("analyze = %+v %v", findings, err)
	}
	view, err = env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo", Client: "claude"})
	if err != nil || view.Unit.State != domain.UnitClosed || view.Stage != domain.StageClosed {
		t.Fatalf("close = %+v %v", view.Unit, err)
	}
	want := "HCD-001: carril full | HCD-001: aclaración Q1 | HCD-001: definición aprobada | HCD-001: cerrada"
	if got := strings.Join(env.decisions.titles, " | "); got != want {
		t.Fatalf("decisions = %s", got)
	}
}

func TestHCDEditingApprovedDefinitionReturnsToApprove(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "evolution", Lane: "full", Title: "Cambiar guardado", Why: "riesgo alto", Intent: "i"})
	editDoc(t, env, view, append(commonFill, storyFill...)...)
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); err != nil {
		t.Fatal(err)
	}
	editDoc(t, env, view, "## Slices\n", "## Slices\n### VS-01. a\nTareas: 1\nCriterios: C-01\n", "## Checklist\n", "## Checklist\n- [ ] 1. a → git --version\n")
	stage(t, env, domain.StageImplement, "hcd-implement")
	editDoc(t, env, view, "Incluido: guardado", "Incluido: guardado y carga")
	stage(t, env, domain.StageApprove, "hcd-approve")
}

func TestHCDFastFixWithRedGreenAndClose(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Guardado pierde datos", Why: "fix claro", Intent: "El guardado pierde datos"})
	if err != nil || view.Unit.DocPath != "hcd/HCD-001-guardado-pierde-datos.md" {
		t.Fatalf("start = %+v %v", view.Unit, err)
	}
	stage(t, env, domain.StageDefine, "hcd-define")
	editDoc(t, env, view, commonFill...)
	view = stage(t, env, domain.StageImplement, "hcd-implement")
	if !strings.Contains(view.Next.Why, "--expect-fail") {
		t.Fatalf("fast fix must ask for the red first: %+v", view.Next)
	}
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "Vía rápida") {
		t.Fatalf("approve in fast lane must explain the lane = %v", err)
	}
	view, ev, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go", ExpectFail: true})
	if err != nil || !ev.RedObserved() || len(view.RedMissing) != 0 {
		t.Fatalf("red = %+v %v %v", ev, view.RedMissing, err)
	}
	if _, err := env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("close before green = %v", err)
	}
	writeIn(t, env.root, "save.go", "package save // fixed\n")
	view, _, err = env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go"})
	if err != nil || view.Stage != domain.StageClose {
		t.Fatalf("green = %s %v", view.Stage, err)
	}
	doc, _ := os.ReadFile(filepath.Join(env.root, view.Unit.DocPath))
	for _, want := range []string{"- [x] 1.", "rojo observado", "verde · vigente"} {
		if !strings.Contains(string(doc), want) {
			t.Fatalf("doc lacks %q:\n%s", want, doc)
		}
	}
	if view, err = env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo", Client: "claude"}); err != nil || view.Unit.State != domain.UnitClosed {
		t.Fatalf("close = %+v %v", view.Unit, err)
	}
	out, _ := exec.Command("git", "-C", env.root, "status", "--porcelain").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if p := strings.TrimSpace(line[2:]); p != "save.go" && !strings.HasPrefix(p, "hcd/") {
			t.Errorf("HCD changed a file outside hcd/: %s", p)
		}
	}
}

func TestHCDFixWithoutRedCannotClose(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Sin rojo", Why: "w", Intent: "i"})
	editDoc(t, env, view, commonFill...)
	writeIn(t, env.root, "save.go", "package save // fixed\n")
	view, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go"})
	if err != nil || view.Stage != domain.StageVerify || len(view.RedMissing) != 1 {
		t.Fatalf("view = %s %v %v", view.Stage, view.RedMissing, err)
	}
	if _, err := env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("close without red = %v", err)
	}
}

func TestHCDEvidenceGoesStaleWhenCodeChanges(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "technical", Lane: "fast", Title: "Refactor", Why: "w", Intent: "i"})
	editDoc(t, env, view, append(commonFill, "## Checklist\n", "## Checklist\n- [ ] 1. Extraer función → git --version\n")...)
	view, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git --version"})
	if err != nil || view.Stage != domain.StageClose {
		t.Fatalf("verify = %s %v", view.Stage, err)
	}
	writeIn(t, env.root, "save.go", "package save // otro cambio\n")
	if view, _ = env.uc.Show(ctx, ""); view.Stage != domain.StageVerify || view.Tasks[0].Status != domain.TaskUnverified {
		t.Fatalf("stale evidence still counts: %s %+v", view.Stage, view.Tasks)
	}
	view, _, _ = env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q no-existe -- save.go"})
	if view.Tasks[0].Status != domain.TaskFailed {
		t.Fatalf("failed check = %+v", view.Tasks)
	}
}

func TestHCDQueriesDoNotWrite(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "full", Title: "Consulta", Why: "w", Intent: "i"})
	path := filepath.Join(env.root, view.Unit.DocPath)
	before, _ := os.ReadFile(path)
	a, _ := env.uc.Show(ctx, "")
	_, findings, _ := env.uc.Analyze(ctx, view.Unit.ID)
	b, _ := env.uc.Show(ctx, view.Unit.ID)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || a.WorkRevision != b.WorkRevision || len(a.State.Evidence) != len(b.State.Evidence) || a.Stage != b.Stage {
		t.Fatal("a query changed the unit")
	}
	if !domain.HasCritical(findings) {
		t.Fatalf("an empty full unit must have critical findings: %+v", findings)
	}
}

func TestHCDStartValidates(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	for _, in := range []usecases.HCDStartInput{
		{Kind: "fix", Lane: "fast", Title: "t"},
		{Kind: "bug", Lane: "fast", Title: "t", Why: "w"},
		{Kind: "fix", Lane: "slow", Title: "t", Why: "w"},
	} {
		if _, err := env.uc.Start(ctx, in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("Start(%+v) = %v", in, err)
		}
	}
}

func TestHCDPromoteKeepsEvidenceAndAddsCeremony(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "fast", Title: "Sincronizar", Why: "parecía simple", Intent: "i"})
	editDoc(t, env, view, append(commonFill, "## Checklist\n", "## Checklist\n- [ ] 1. Primer paso → git --version\n")...)
	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git --version"}); err != nil {
		t.Fatal(err)
	}
	view, err := env.uc.Promote(ctx, "")
	if err != nil || view.Unit.Lane != domain.LaneFull || len(view.State.Evidence) != 1 || view.Stage != domain.StageDefine {
		t.Fatalf("promote = %+v %s %v", view.Unit, view.Stage, err)
	}
	if _, err := env.uc.Promote(ctx, ""); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("second promote = %v", err)
	}
	editDoc(t, env, view, storyFill...)
	stage(t, env, domain.StageApprove, "hcd-approve")
}

func TestHCDCancelNeedsQuote(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "x", Why: "w"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.uc.Cancel(ctx, usecases.HCDCloseInput{Quote: " "}); !errors.Is(err, domain.ErrInvalidQuote) {
		t.Fatalf("cancel without quote = %v", err)
	}
	view, err := env.uc.Cancel(ctx, usecases.HCDCloseInput{Quote: "ya no hace falta"})
	if err != nil || view.Unit.State != domain.UnitCancelled {
		t.Fatalf("cancel = %+v %v", view.Unit, err)
	}
	if _, _, err := env.uc.Clarify(ctx, usecases.HCDClarifyInput{ID: view.Unit.ID, Ask: "x"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("mutation on a cancelled unit = %v", err)
	}
}

type hcdTopics map[string]string

func (t hcdTopics) ByTopicKey(_, key string) (*domain.Memory, error) {
	if body, ok := t[key]; ok {
		return &domain.Memory{Content: body}, nil
	}
	return nil, nil
}

// FR-029: cambiar la Constitución tras aprobar se reporta y bloquea el cierre
// hasta reaprobar la definición con las reglas nuevas.
func TestHCDConstitutionChangeRequiresReapproval(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	topics := hcdTopics{domain.TopicConstitution: "Constitución v1"}
	env.uc.ConfigureNorms(topics, "p", "plantilla constitución", "plantilla playbook")
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "full", Title: "Normas", Why: "riesgo alto", Intent: "i"})
	editDoc(t, env, view, append(append(commonFill, storyFill...), "## Slices\n", "## Slices\n### VS-01. a\nTareas: 1\nCriterios: C-01\n", "## Checklist\n", "## Checklist\n- [ ] 1. a → git --version\n")...)
	if view.State.ConstitutionRev == "" || view.State.PlaybookRev == "" {
		t.Fatalf("norm revisions not resolved: %+v", view.State)
	}
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git --version"}); err != nil {
		t.Fatal(err)
	}
	topics[domain.TopicConstitution] = "Constitución v2"
	view = stage(t, env, domain.StageClose, "hcd-approve")
	if _, err := env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "Constitución") {
		t.Fatalf("close with changed constitution = %v", err)
	}
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); err != nil {
		t.Fatalf("re-approval with new norms = %v", err)
	}
	stage(t, env, domain.StageClose, "hcd-close")
	if view, err := env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo"}); err != nil || view.Unit.State != domain.UnitClosed {
		t.Fatalf("close after re-approval = %+v %v", view.Unit, err)
	}
	// Sin cambio de normas, approve fuera de su paso sigue rechazado.
	if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{ID: view.Unit.ID, Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("approve on a closed unit = %v", err)
	}
}

// ACR C-001: con las normas cambiadas no se reaprueba una definición con
// faltantes ni con aclaraciones abiertas.
func TestHCDNormsReapprovalRequiresCompleteDefinition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, env hcdEnv, view usecases.HCDView)
	}{
		{"sección obligatoria vaciada", func(t *testing.T, env hcdEnv, view usecases.HCDView) {
			editDoc(t, env, view, "## ¿Qué pasa si no lo hacemos?\nse pierden datos", "## ¿Qué pasa si no lo hacemos?\npendiente")
		}},
		{"marcador de aclaración en una sección obligatoria", func(t *testing.T, env hcdEnv, view usecases.HCDView) {
			editDoc(t, env, view, "## Alternativa más barata\nninguna", "## Alternativa más barata\n[NEEDS CLARIFICATION: ¿sirve un script?]")
		}},
		{"aclaración abierta", func(t *testing.T, env hcdEnv, view usecases.HCDView) {
			if _, _, err := env.uc.Clarify(context.Background(), usecases.HCDClarifyInput{Ask: "¿Y los borradores?"}); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, ctx := newHCDEnv(t), context.Background()
			topics := hcdTopics{domain.TopicConstitution: "Constitución v1"}
			env.uc.ConfigureNorms(topics, "p", "c", "pb")
			view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "full", Title: "Reaprobar", Why: "riesgo alto", Intent: "i"})
			editDoc(t, env, view, append(append(commonFill, storyFill...), "## Slices\n", "## Slices\n### VS-01. a\nTareas: 1\nCriterios: C-01\n", "## Checklist\n", "## Checklist\n- [ ] 1. a → git --version\n")...)
			if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); err != nil {
				t.Fatal(err)
			}
			topics[domain.TopicConstitution] = "Constitución v2"
			tc.mutate(t, env, view)
			if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) {
				t.Fatalf("reapproval of an incomplete definition = %v", err)
			}
		})
	}
}

// ACR acr_604f2f04 #3: con las normas cambiadas y la aprobación aún vigente
// (el defecto está fuera de la revisión de la definición), el guard de
// Approve rechaza los pasos define y clarify.
func TestHCDNormsReapprovalGuardWhileApprovalStillValid(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		stage          domain.Stage
	}{
		{"tarea sin comprobación", "- [ ] 1. a → git --version\n", "- [ ] 1. a → git --version\n- [ ] 2. b sin comprobación\n", domain.StageDefine},
		{"marcador en un slice", "### VS-01. a\n", "### VS-01. a [NEEDS CLARIFICATION: ¿alcance?]\n", domain.StageClarify},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, ctx := newHCDEnv(t), context.Background()
			topics := hcdTopics{domain.TopicConstitution: "Constitución v1"}
			env.uc.ConfigureNorms(topics, "p", "c", "pb")
			view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "feature", Lane: "full", Title: "Guard", Why: "riesgo alto", Intent: "i"})
			editDoc(t, env, view, append(append(commonFill, storyFill...), "## Slices\n", "## Slices\n### VS-01. a\nTareas: 1\nCriterios: C-01\n", "## Checklist\n", "## Checklist\n- [ ] 1. a → git --version\n")...)
			if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); err != nil {
				t.Fatal(err)
			}
			topics[domain.TopicConstitution] = "Constitución v2"
			editDoc(t, env, view, tc.from, tc.to)
			view, err := env.uc.Show(ctx, "")
			if err != nil || !view.Approved || !view.State.NormsChanged() || view.Stage != tc.stage {
				t.Fatalf("precondition: approved=%v norms=%v stage=%s err=%v", view.Approved, view.State.NormsChanged(), view.Stage, err)
			}
			if _, err := env.uc.Approve(ctx, usecases.HCDApproveInput{Quote: "apruebo"}); !errors.Is(err, domain.ErrNotAllowed) {
				t.Fatalf("approve in %s with changed norms = %v", tc.stage, err)
			}
		})
	}
}
