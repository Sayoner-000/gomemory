package domain

import (
	"regexp"
	"strings"
)

type SliceState string
type TaskState string

type Slice struct {
	ID, UnitID, Title, Observable                            string
	Stories, Criteria, Dependencies                          []string
	Limits, ArchitecturalImpact, Verification, Uncertainties string
	State                                                    SliceState
}

type Task struct {
	ID, UnitID, SliceID, Type, Title, Justification         string
	Unlocks                                                 []string
	Stories, Criteria, Dependencies, Rules                  []string
	Result, Scope, Excluded, Verification, ExpectedEvidence string
	State                                                   TaskState
}

type Execution struct {
	ID, UnitID, TaskID, DefinitionRev, DeliveryRev, State string
	GrantedAt, ExpiresAt, ReportedAt, ResultHash          string
}

type Evidence struct {
	ID, UnitID, ExecutionID, Procedure, Environment, Result            string
	Covers, SubjectRef, InspectRef                                     []string
	DefinitionRev, DeliveryRev, PlaybookRev, ConstitutionRev, Executor string
	RecordedAt                                                         string
}

// HCDWork is the resolved artifact aggregate used for structural checks. Its
// contents are loaded from the current definition and delivery revisions.
type HCDWork struct {
	Unit       Unit
	Definition Definition
	Slices     []Slice
	Tasks      []Task
	Decisions  []HCDDecision
}

func AtomicityViolations(task Task, work HCDWork, clarifications []Clarification) []PendingCheck {
	var out []PendingCheck
	add := func(code, msg string) { out = append(out, PendingCheck{Code: code, Field: task.ID, Message: msg}) }
	words := strings.Fields(task.Title)
	if len(words) < 2 || !(strings.HasSuffix(strings.ToLower(words[0]), "ar") || strings.HasSuffix(strings.ToLower(words[0]), "er") || strings.HasSuffix(strings.ToLower(words[0]), "ir")) {
		add("ATOM-VERB", "el título requiere verbo en infinitivo y objeto")
	}
	if isHCDPending(task.Result) || len(regexp.MustCompile(`(?m)^\s*(?:[-*]|[0-9]+[.)])\s+`).FindAllString(task.Result, -1)) > 1 || strings.Contains(task.Result, ";") {
		add("ATOM-RESULT", "se requiere un único resultado")
	}
	if isHCDPending(task.Verification) {
		add("ATOM-VERIFY", "falta verificación")
	}
	if isHCDPending(task.Scope) || isHCDPending(task.Excluded) {
		add("ATOM-SCOPE", "falta alcance o exclusiones")
	}
	if len(task.Rules) == 0 {
		add("ATOM-RULES", "faltan reglas")
	}
	knownTasks := map[string]bool{}
	for _, t := range work.Tasks {
		knownTasks[t.ID] = true
	}
	for _, d := range task.Dependencies {
		if !knownTasks[d] {
			add("ATOM-INPUTS", "dependencia inexistente: "+d)
		}
	}
	slices := map[string]bool{}
	for _, s := range work.Slices {
		slices[s.ID] = true
	}
	stories := map[string]bool{}
	for _, s := range work.Definition.Stories {
		stories[s.ID] = true
	}
	criteria := map[string]bool{}
	for _, c := range work.Definition.Criteria {
		criteria[c.ID] = true
	}
	if !slices[task.SliceID] || len(task.Stories) == 0 || len(task.Criteria) == 0 {
		add("ATOM-LINK", "faltan referencias de slice, historia o criterio")
	}
	for _, id := range task.Stories {
		if !stories[id] {
			add("ATOM-LINK", "historia inexistente: "+id)
		}
	}
	for _, id := range task.Criteria {
		if !criteria[id] {
			add("ATOM-LINK", "criterio inexistente: "+id)
		}
	}
	for _, c := range clarifications {
		if c.Status == "open" && c.Blocking && (c.TargetRef == task.ID || c.TargetRef == task.SliceID) {
			add("ATOM-PENDING", "aclaración bloqueante: "+c.ID)
		}
	}
	if task.Type == "enabling" {
		valid := len(task.Unlocks) > 0 && !isHCDPending(task.Justification)
		for _, id := range task.Unlocks {
			valid = valid && slices[id]
		}
		if !valid {
			add("ATOM-ENABLING", "falta slice desbloqueado o justificación")
		}
	}
	if task.Type == "scope-change" {
		accepted := false
		for _, d := range work.Decisions {
			if d.Status == "accepted" {
				for _, ref := range d.Refs {
					if ref == task.ID {
						accepted = true
					}
				}
			}
		}
		if !accepted {
			add("ATOM-DECISION", "falta decisión aceptada vinculada")
		}
	}
	return out
}
func DependencyCycle(tasks []Task) bool {
	edges := map[string][]string{}
	for _, t := range tasks {
		edges[t.ID] = t.Dependencies
	}
	color := map[string]int{}
	var visit func(string) bool
	visit = func(id string) bool {
		if color[id] == 1 {
			return true
		}
		if color[id] == 2 {
			return false
		}
		color[id] = 1
		for _, dep := range edges[id] {
			if visit(dep) {
				return true
			}
		}
		color[id] = 2
		return false
	}
	for _, t := range tasks {
		if visit(t.ID) {
			return true
		}
	}
	return false
}
func SliceViolations(slice Slice) []PendingCheck {
	var out []PendingCheck
	for _, f := range []struct{ name, value string }{{"Comportamiento observable", slice.Observable}, {"Límites", slice.Limits}, {"Verificación esperada", slice.Verification}} {
		if isHCDPending(f.value) {
			out = append(out, PendingCheck{Code: "SLICE-FIELDS", Field: slice.ID, Message: "falta " + f.name})
		}
	}
	return out
}

// DecisionTaskCancelled retira una tarea: una decisión aceptada de este tipo
// con la tarea en Refs la saca de la admisión, la verificación y el cierre.
const DecisionTaskCancelled = "task_cancelled"

// ActiveTasks devuelve las tareas no retiradas por una decisión aceptada.
func ActiveTasks(tasks []Task, decisions []HCDDecision) []Task {
	cancelled := map[string]bool{}
	for _, d := range decisions {
		if d.Status == "accepted" && d.Kind == DecisionTaskCancelled {
			for _, ref := range d.Refs {
				cancelled[ref] = true
			}
		}
	}
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if !cancelled[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

type Delivery struct {
	Slices []Slice
	Tasks  []Task
}
