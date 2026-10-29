package domain

import "testing"

func atomicFixture() (Task, HCDWork) {
	task := Task{ID: "T-001", Title: "Guardar datos", SliceID: "VS-01", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Rules: []string{"HCD-TRACE-01"}, Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test de persistencia"}
	work := HCDWork{Definition: Definition{Stories: []Story{{ID: "US1"}}, Criteria: []Criterion{{ID: "C-01"}}}, Slices: []Slice{{ID: "VS-01"}}, Tasks: []Task{task}}
	return task, work
}
func TestAtomicityViolationsEachCode(t *testing.T) {
	for _, tc := range []struct {
		code           string
		change         func(*Task, *HCDWork)
		clarifications []Clarification
	}{
		{"ATOM-VERB", func(t *Task, _ *HCDWork) { t.Title = "Persistencia" }, nil},
		{"ATOM-RESULT", func(t *Task, _ *HCDWork) { t.Result = "1. Guardar\n2. Borrar" }, nil},
		{"ATOM-VERIFY", func(t *Task, _ *HCDWork) { t.Verification = "" }, nil},
		{"ATOM-INPUTS", func(t *Task, _ *HCDWork) { t.Dependencies = []string{"T-999"} }, nil},
		{"ATOM-SCOPE", func(t *Task, _ *HCDWork) { t.Excluded = "" }, nil},
		{"ATOM-PENDING", func(t *Task, _ *HCDWork) {}, []Clarification{{TargetRef: "T-001", Status: "open", Blocking: true}}},
		{"ATOM-LINK", func(t *Task, _ *HCDWork) { t.Criteria = []string{"C-99"} }, nil},
		{"ATOM-RULES", func(t *Task, _ *HCDWork) { t.Rules = nil }, nil},
		{"ATOM-ENABLING", func(t *Task, _ *HCDWork) { t.Type = "enabling" }, nil},
		{"ATOM-DECISION", func(t *Task, _ *HCDWork) { t.Type = "scope-change" }, nil},
	} {
		t.Run(tc.code, func(t *testing.T) {
			task, w := atomicFixture()
			tc.change(&task, &w)
			found := false
			for _, v := range AtomicityViolations(task, w, tc.clarifications) {
				if v.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s", tc.code)
			}
		})
	}
	task, w := atomicFixture()
	if got := AtomicityViolations(task, w, nil); len(got) != 0 {
		t.Fatalf("valid task: %+v", got)
	}
	task.Type = "scope-change"
	w.Decisions = []HCDDecision{{Status: "accepted", Refs: []string{task.ID}}}
	if got := AtomicityViolations(task, w, nil); len(got) != 0 {
		t.Fatalf("accepted scope change: %+v", got)
	}
}
func TestDependencyCycle(t *testing.T) {
	if !DependencyCycle([]Task{{ID: "a", Dependencies: []string{"b"}}, {ID: "b", Dependencies: []string{"a"}}}) {
		t.Fatal("cycle not detected")
	}
	if DependencyCycle([]Task{{ID: "a"}, {ID: "b", Dependencies: []string{"a"}}}) {
		t.Fatal("acyclic graph rejected")
	}
}
func TestSliceRequiredFields(t *testing.T) {
	if checks := SliceViolations(Slice{ID: "VS-01"}); len(checks) != 3 {
		t.Fatalf("checks=%+v", checks)
	}
}
