package domain

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)

func stateOf(t *testing.T, doc string, kind Kind, lane Lane) HCDState {
	t.Helper()
	d, pending := ParseUnitDoc([]byte(doc), kind, lane)
	return HCDState{Unit: Unit{ID: "HCD-001", Kind: kind, Lane: lane, State: UnitActive}, Doc: d, Pending: pending, Fingerprint: "f1"}
}

func green(s HCDState, fp string, at time.Time, tasks ...string) Evidence {
	return Evidence{Tasks: tasks, Kind: EvidenceCommand, Revision: s.WorkRevision(), Fingerprint: fp, CreatedAt: at}
}

func red(s HCDState, fp string, at time.Time, tasks ...string) Evidence {
	return Evidence{Tasks: tasks, Kind: EvidenceCommand, ExpectFail: true, ExitCode: 1, Revision: s.WorkRevision(), Fingerprint: fp, CreatedAt: at}
}

func approve(s HCDState) Approval {
	return Approval{Action: ApprovalApprove, Revision: s.DefinitionRevision()}
}

func TestFullLaneWalksTheWholeCeremony(t *testing.T) {
	doc := render(t, KindFeature, LaneFull)
	s := stateOf(t, doc, KindFeature, LaneFull)
	expect := func(want Stage, skill string) {
		t.Helper()
		if got := s.Stage(); got != want {
			t.Fatalf("stage = %s, want %s (pending %+v)", got, want, s.Pending)
		}
		if next := s.NextAction(); next.Skill != skill {
			t.Fatalf("next = %+v, want %s", next, skill)
		}
	}
	expect(StageDefine, "hcd-define")

	doc = fill(doc, LaneFull)
	data, _ := AddClarification([]byte(doc), s.Doc, "¿Incluye borradores?")
	doc = string(data)
	s = stateOf(t, doc, KindFeature, LaneFull)
	expect(StageClarify, "hcd-clarify")

	data, _ = AnswerClarification([]byte(doc), "Q1", "no")
	doc = string(data)
	s = stateOf(t, doc, KindFeature, LaneFull)
	expect(StageApprove, "hcd-approve")

	approval := approve(s)
	s.Approvals = []Approval{approval}
	expect(StageSlice, "hcd-slice")

	doc = withSlice(doc)
	s = stateOf(t, doc, KindFeature, LaneFull)
	s.Approvals = []Approval{approval}
	expect(StageTasks, "hcd-tasks")

	doc = withTask(doc)
	s = stateOf(t, doc, KindFeature, LaneFull)
	s.Approvals = []Approval{approval}
	expect(StageImplement, "hcd-implement")

	doc = string(MarkTasksDone([]byte(doc), []string{"1"}))
	s = stateOf(t, doc, KindFeature, LaneFull)
	s.Approvals = []Approval{approval}
	expect(StageVerify, "hcd-verify")

	s.Evidence = []Evidence{green(s, "f1", t0, "1")}
	expect(StageClose, "hcd-close")
	if f := s.Analyze(); HasCritical(f) {
		t.Fatalf("clean unit has critical findings: %+v", f)
	}

	s.Unit.State = UnitClosed
	expect(StageClosed, "hcd")
}

func TestEditingDefinitionAfterApprovalReturnsToApprove(t *testing.T) {
	doc := withTask(withSlice(fill(render(t, KindFeature, LaneFull), LaneFull)))
	s := stateOf(t, doc, KindFeature, LaneFull)
	approval := approve(s)
	s.Approvals = []Approval{approval}
	if s.Stage() != StageImplement {
		t.Fatalf("stage = %s", s.Stage())
	}
	// Más tareas tras aprobar: la aprobación sigue vigente.
	s = stateOf(t, strings.Replace(doc, "## Checklist\n", "## Checklist\n- [ ] 2. Otra → go vet\n", 1), KindFeature, LaneFull)
	s.Approvals = []Approval{approval}
	if !s.Approved() {
		t.Fatal("adding a task invalidated the definition approval")
	}
	// Cambiar el alcance tras aprobar: vuelve a approve.
	s = stateOf(t, strings.Replace(doc, "Incluido: guardado", "Incluido: guardado y carga", 1), KindFeature, LaneFull)
	s.Approvals = []Approval{approval}
	if s.Stage() != StageApprove || s.Approved() {
		t.Fatalf("edited definition still approved: %s", s.Stage())
	}
}

func TestFastLaneSkipsApproveAndSlice(t *testing.T) {
	doc := fill(render(t, KindFix, LaneFast), LaneFast)
	s := stateOf(t, doc, KindFix, LaneFast)
	if s.Stage() != StageImplement || !strings.Contains(s.NextAction().Why, "--expect-fail") {
		t.Fatalf("fast fix: %s %+v", s.Stage(), s.NextAction())
	}
	feature := strings.Replace(fill(render(t, KindFeature, LaneFast), LaneFast), "## Checklist\n", "## Checklist\n", 1)
	if s := stateOf(t, feature, KindFeature, LaneFast); s.Stage() != StageTasks {
		t.Fatalf("fast feature without tasks: %s", s.Stage())
	}
	marked := strings.Replace(doc, "Excluido: exportación", "Excluido: [NEEDS CLARIFICATION: ¿y la exportación?]", 1)
	if s := stateOf(t, marked, KindFix, LaneFast); s.Stage() != StageClarify {
		t.Fatalf("fast with marker: %s", s.Stage())
	}
}

func TestEvidenceGoesStaleAndFailedChecksFail(t *testing.T) {
	s := stateOf(t, withTask(fill(render(t, KindFeature, LaneFast), LaneFast)), KindFeature, LaneFast)
	s.Evidence = []Evidence{green(s, "f1", t0, "1")}
	if s.Stage() != StageClose {
		t.Fatalf("stage = %s", s.Stage())
	}
	s.Fingerprint = "f2"
	if s.TaskStatuses()["1"] != TaskPending || s.Stage() != StageImplement {
		t.Fatalf("stale evidence still counts: %s", s.Stage())
	}
	s.Fingerprint = "f1"
	failed := green(s, "f1", t0.Add(time.Second), "1")
	failed.ExitCode = 2
	s.Evidence = append(s.Evidence, failed)
	if s.TaskStatuses()["1"] != TaskFailed || s.Stage() != StageVerify {
		t.Fatalf("later failure ignored: %s", s.Stage())
	}
}

func TestRedGate(t *testing.T) {
	s := stateOf(t, string(MarkTasksDone([]byte(fill(render(t, KindFix, LaneFast), LaneFast)), []string{"1"})), KindFix, LaneFast)
	g := green(s, "after", t0.Add(time.Minute), "1")
	s.Fingerprint = "after"
	s.Evidence = []Evidence{g}
	if s.Stage() != StageVerify || len(s.RedMissing()) != 1 || !strings.Contains(s.NextAction().Why, "rojo") {
		t.Fatalf("fix without red: %s %v %+v", s.Stage(), s.RedMissing(), s.NextAction())
	}
	notRed := red(s, "before", t0, "1")
	notRed.ExitCode = 0
	sameCode := red(s, "after", t0, "1")
	s.Evidence = []Evidence{notRed, sameCode, g}
	if len(s.RedMissing()) != 1 {
		t.Fatal("a passing «red» or a red on the green's code was accepted")
	}
	s.Evidence = []Evidence{red(s, "before", t0, "1"), g}
	if len(s.RedMissing()) != 0 || s.Stage() != StageClose {
		t.Fatalf("valid red rejected: %v %s", s.RedMissing(), s.Stage())
	}
	// Antes del verde, el rojo sobre el código actual ya cuenta.
	s.Fingerprint = "before"
	s.Evidence = []Evidence{red(s, "before", t0, "1")}
	if len(s.RedMissing()) != 0 {
		t.Fatal("red before any green rejected")
	}
	// Sin git, cuenta el orden.
	s.Fingerprint = FingerprintNoGit
	s.Evidence = []Evidence{red(s, FingerprintNoGit, t0, "1"), green(s, FingerprintNoGit, t0.Add(time.Second), "1")}
	if len(s.RedMissing()) != 0 {
		t.Fatal("red before green rejected without git")
	}
	s.Evidence = []Evidence{green(s, FingerprintNoGit, t0, "1"), red(s, FingerprintNoGit, t0.Add(time.Second), "1")}
	if len(s.RedMissing()) != 1 {
		t.Fatal("red after green accepted without git")
	}
	// En una feature el rojo solo se exige si se intentó.
	f := stateOf(t, withTask(fill(render(t, KindFeature, LaneFast), LaneFast)), KindFeature, LaneFast)
	f.Evidence = []Evidence{green(f, "f1", t0, "1")}
	if len(f.RedMissing()) != 0 {
		t.Fatal("feature should not require red")
	}
	attempt := red(f, "f0", t0.Add(-time.Second), "1")
	attempt.ExitCode = 0
	f.Evidence = append(f.Evidence, attempt)
	if len(f.RedMissing()) != 1 {
		t.Fatal("feature with a failed red attempt must require a real red")
	}
}

func codes(findings []HCDFinding) map[string]Severity {
	out := map[string]Severity{}
	for _, f := range findings {
		out[f.Code+":"+f.Ref] = f.Severity
	}
	return out
}

func TestAnalyzeTraceability(t *testing.T) {
	doc := fill(render(t, KindFeature, LaneFull), LaneFull)
	doc = strings.Replace(doc, "- C-01: Dado un cambio, cuando guardo, entonces persiste", "- C-01: Dado un cambio, cuando guardo, entonces persiste\n- C-02: Dado un error, cuando guardo, entonces aviso", 1)
	doc = string(ReplaceSection([]byte(doc), SectionSlices, "### VS-01. Guardado\nTareas: 1, 9\nCriterios: C-01, C-07", ""))
	doc = strings.Replace(doc, "## Checklist\n", "## Checklist\n- [ ] 1. Persistir → go test\n- [ ] 2. Huérfana → go vet\n", 1)
	s := stateOf(t, doc, KindFeature, LaneFull)
	got := codes(s.Analyze())
	for key, sev := range map[string]Severity{
		"APPROVAL-MISSING:" + s.DefinitionRevision(): SeverityCritical,
		"SLICE-REF:VS-01":          SeverityHigh,
		"CRITERION-UNCOVERED:C-02": SeverityHigh,
		"TASK-ORPHAN:2":            SeverityHigh,
		"TASK-UNVERIFIED:1":        SeverityMedium,
	} {
		if got[key] != sev {
			t.Errorf("finding %s = %q, want %q (all: %v)", key, got[key], sev, got)
		}
	}
	// Con aprobación y tarea verificada fuera de plazo, la evidencia pasa a crítica.
	s.Approvals = []Approval{approve(s)}
	doc = string(MarkTasksDone([]byte(doc), []string{"1", "2"}))
	s2 := stateOf(t, doc, KindFeature, LaneFull)
	s2.Approvals = []Approval{approve(s2)}
	if got := codes(s2.Analyze()); got["TASK-UNVERIFIED:1"] != SeverityCritical {
		t.Fatalf("unverified task in verify stage must be critical: %v", got)
	}
}

func TestAnalyzeFastLaneAndManualEvidence(t *testing.T) {
	s := stateOf(t, withTask(fill(render(t, KindFeature, LaneFast), LaneFast)), KindFeature, LaneFast)
	s.Evidence = []Evidence{{Tasks: []string{"1"}, Kind: EvidenceManual, Excerpt: "lo vi", Revision: s.WorkRevision(), Fingerprint: "f1", CreatedAt: t0}}
	findings := s.Analyze()
	if HasCritical(findings) {
		t.Fatalf("fast unit with manual evidence has critical findings: %+v", findings)
	}
	if got := codes(findings); got["EVIDENCE-MANUAL:1"] != SeverityMedium {
		t.Fatalf("manual evidence not reported: %v", got)
	}
	for _, f := range findings {
		if f.Code == "APPROVAL-MISSING" || strings.HasPrefix(f.Code, "SLICE") {
			t.Fatalf("fast lane reported full-lane finding: %+v", f)
		}
	}
}
