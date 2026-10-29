package domain

import (
	"slices"
	"strings"
)

// FingerprintNoGit es la huella de un proyecto fuera de git: no identifica el
// código evaluado.
const FingerprintNoGit = "nogit"

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskUnverified TaskStatus = "unverified"
	TaskFailed     TaskStatus = "failed"
	TaskVerified   TaskStatus = "verified"
)

// HCDState reúne lo que el caso de uso lee de una unidad; de él se derivan
// el paso, la siguiente acción y el análisis.
type HCDState struct {
	Unit        Unit
	Doc         UnitDoc
	Pending     []PendingCheck
	Evidence    []Evidence
	Approvals   []Approval
	Fingerprint string
}

func (s HCDState) DefinitionRevision() string { return DefinitionRevision(s.Doc) }
func (s HCDState) WorkRevision() string       { return WorkRevision(s.Doc) }

// Approved: la aprobación más reciente de la definición cubre su revisión
// actual. Editar la definición después de aprobar la deja sin aprobar.
func (s HCDState) Approved() bool {
	rev := ""
	for _, a := range s.Approvals {
		if a.Action == ApprovalApprove {
			rev = a.Revision
		}
	}
	return rev != "" && rev == s.DefinitionRevision()
}

// TaskStatuses aplica el gate de evidencia: una tarea solo está verificada si
// su evidencia más reciente vigente (misma revisión de trabajo y huella)
// pasó. La evidencia caducada se ignora, no se borra.
func (s HCDState) TaskStatuses() map[string]TaskStatus {
	rev := s.WorkRevision()
	latest := map[string]Evidence{}
	for _, e := range s.Evidence {
		if e.ExpectFail || !e.Fresh(rev, s.Fingerprint) {
			continue
		}
		for _, id := range e.Tasks {
			if prev, ok := latest[id]; !ok || !e.CreatedAt.Before(prev.CreatedAt) {
				latest[id] = e
			}
		}
	}
	out := make(map[string]TaskStatus, len(s.Doc.Tasks))
	for _, t := range s.Doc.Tasks {
		e, ok := latest[t.ID]
		switch {
		case ok && e.Passed():
			out[t.ID] = TaskVerified
		case ok:
			out[t.ID] = TaskFailed
		case t.Done:
			out[t.ID] = TaskUnverified
		default:
			out[t.ID] = TaskPending
		}
	}
	return out
}

// Unverified lista las tareas que aún no pasan el gate de evidencia.
func (s HCDState) Unverified() []string {
	status := s.TaskStatuses()
	var ids []string
	for _, t := range s.Doc.Tasks {
		if status[t.ID] != TaskVerified {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// RedMissing aplica el gate TDD: en un fix, la tarea 1 exige un rojo
// observado (el comando falló) sobre un código distinto del de su verde más
// reciente; en los demás tipos, solo las tareas que ya registraron un intento
// rojo. Antes de existir el verde, cualquier rojo observado cuenta.
func (s HCDState) RedMissing() []string {
	green := map[string]Evidence{}
	for _, e := range s.Evidence {
		if !e.Passed() {
			continue
		}
		for _, id := range e.Tasks {
			if prev, ok := green[id]; !ok || !e.CreatedAt.Before(prev.CreatedAt) {
				green[id] = e
			}
		}
	}
	attempted, observed := map[string]bool{}, map[string]bool{}
	for _, e := range s.Evidence {
		if !e.ExpectFail {
			continue
		}
		for _, id := range e.Tasks {
			attempted[id] = true
			g, hasGreen := green[id]
			// Sin git la huella es constante: el orden rojo → verde es la única prueba.
			distinct := e.Fingerprint != g.Fingerprint || (g.Fingerprint == FingerprintNoGit && e.CreatedAt.Before(g.CreatedAt))
			if e.RedObserved() && (!hasGreen || distinct) {
				observed[id] = true
			}
		}
	}
	var missing []string
	for _, t := range s.Doc.Tasks {
		required := attempted[t.ID] || (s.Unit.Kind == KindFix && t.ID == "1")
		if required && !observed[t.ID] {
			missing = append(missing, t.ID)
		}
	}
	return missing
}

// Stage deriva el paso de la ceremonia. full: define → clarify → approve →
// slice → tasks → implement → verify → close. fast omite approve y slice.
func (s HCDState) Stage() Stage {
	full := s.Unit.Lane == LaneFull
	switch {
	case s.Unit.State != UnitActive:
		return StageClosed
	case len(s.Pending) > 0:
		return StageDefine
	case len(s.Doc.OpenClarifications()) > 0:
		return StageClarify
	case full && !s.Approved():
		return StageApprove
	case full && len(s.Doc.Slices) == 0:
		return StageSlice
	case len(s.Doc.Tasks) == 0:
		return StageTasks
	}
	stage := StageClose
	for _, status := range s.TaskStatuses() {
		switch status {
		case TaskPending:
			return StageImplement
		case TaskUnverified, TaskFailed:
			stage = StageVerify
		}
	}
	if stage == StageClose && len(s.RedMissing()) > 0 {
		return StageVerify
	}
	return stage
}

// NextAction nombra la skill del paso y el motivo.
func (s HCDState) NextAction() NextStep {
	switch stage := s.Stage(); stage {
	case StageDefine:
		return NextStep{Skill: "hcd-define", Command: "show", Why: s.Pending[0].Message}
	case StageClarify:
		return NextStep{Skill: "hcd-clarify", Command: "clarify", Why: "aclaración abierta: " + strings.Join(s.Doc.OpenClarifications(), ", ")}
	case StageApprove:
		return NextStep{Skill: "hcd-approve", Command: "approve", Why: "la definición " + s.DefinitionRevision() + " espera tu aprobación"}
	case StageSlice:
		return NextStep{Skill: "hcd-slice", Command: "show", Why: "la definición aprobada necesita slices"}
	case StageTasks:
		return NextStep{Skill: "hcd-tasks", Command: "show", Why: "faltan tareas atómicas en el checklist"}
	case StageImplement:
		why := "implementa la tarea " + strings.Join(s.pendingTasks(), ", ")
		if red := s.RedMissing(); len(red) > 0 {
			why = "antes del cambio, observa el rojo de la tarea " + strings.Join(red, ", ") + " con --expect-fail; después " + why
		}
		return NextStep{Skill: "hcd-implement", Command: "verify", Why: why}
	case StageVerify:
		if unverified := s.Unverified(); len(unverified) > 0 {
			return NextStep{Skill: "hcd-verify", Command: "verify", Why: "falta evidencia vigente de la tarea " + strings.Join(unverified, ", ")}
		}
		return NextStep{Skill: "hcd-verify", Command: "verify", Why: "falta el rojo observado de la tarea " + strings.Join(s.RedMissing(), ", ") + ": ejecútala con --expect-fail sobre el código sin el cambio"}
	case StageClose:
		return NextStep{Skill: "hcd-close", Command: "close", Why: "todo verificado; ejecuta analyze y pide la aprobación del cierre"}
	default:
		return NextStep{Skill: "hcd", Command: "start", Why: "la unidad está terminada; inicia o retoma otra"}
	}
}

func (s HCDState) pendingTasks() []string {
	status := s.TaskStatuses()
	var ids []string
	for _, t := range s.Doc.Tasks {
		if status[t.ID] == TaskPending {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// Analyze comprueba la coherencia de la unidad sin escribir nada. close exige
// cero hallazgos critical.
func (s HCDState) Analyze() []HCDFinding {
	var out []HCDFinding
	add := func(sev Severity, code, ref, msg string) {
		out = append(out, HCDFinding{Severity: sev, Code: code, Ref: ref, Message: msg})
	}
	for _, p := range s.Pending {
		add(SeverityCritical, p.Code, p.Field, p.Message)
	}
	for _, id := range s.Doc.OpenClarifications() {
		add(SeverityCritical, "CLARIFICATION-OPEN", id, "aclaración abierta "+id)
	}
	if s.Unit.Lane == LaneFull {
		if !s.Approved() {
			add(SeverityCritical, "APPROVAL-MISSING", s.DefinitionRevision(), "la definición actual no tiene aprobación vigente")
		}
		s.analyzeTraceability(add)
	}
	// La evidencia solo es crítica cuando ya toca cerrar; antes es avance normal.
	sev := SeverityMedium
	if stage := s.Stage(); stage == StageClose || stage == StageVerify {
		sev = SeverityCritical
	}
	status := s.TaskStatuses()
	for _, t := range s.Doc.Tasks {
		switch status[t.ID] {
		case TaskFailed:
			add(sev, "TASK-FAILED", t.ID, "la última comprobación vigente de la tarea "+t.ID+" falló")
		case TaskPending, TaskUnverified:
			add(sev, "TASK-UNVERIFIED", t.ID, "la tarea "+t.ID+" no tiene evidencia vigente")
		}
	}
	for _, id := range s.RedMissing() {
		add(sev, "RED-MISSING", id, "la tarea "+id+" no tiene un rojo observado antes del verde")
	}
	if s.Fingerprint == FingerprintNoGit {
		add(SeverityMedium, "NO-GIT", "", "sin git la huella no identifica el código; el rojo se acepta por orden")
	}
	for _, e := range s.Evidence {
		if e.Kind == EvidenceManual && e.Fresh(s.WorkRevision(), s.Fingerprint) {
			add(SeverityMedium, "EVIDENCE-MANUAL", strings.Join(e.Tasks, ","), "evidencia manual: es una observación declarada, no ejecutada")
		}
	}
	return out
}

func (s HCDState) analyzeTraceability(add func(sev Severity, code, ref, msg string)) {
	tasks, criteria := map[string]bool{}, map[string]bool{}
	for _, t := range s.Doc.Tasks {
		tasks[t.ID] = true
	}
	for _, st := range s.Doc.Stories {
		for _, c := range st.Criteria {
			criteria[c.ID] = true
		}
	}
	coveredTask, coveredCriterion := map[string]bool{}, map[string]bool{}
	for _, sl := range s.Doc.Slices {
		for _, id := range sl.Tasks {
			if !tasks[id] {
				add(SeverityHigh, "SLICE-REF", sl.ID, "el slice "+sl.ID+" cita la tarea inexistente "+id)
			}
			coveredTask[id] = true
		}
		for _, id := range sl.Criteria {
			if !criteria[id] {
				add(SeverityHigh, "SLICE-REF", sl.ID, "el slice "+sl.ID+" cita el criterio inexistente "+id)
			}
			coveredCriterion[id] = true
		}
		if len(sl.Tasks) == 0 {
			add(SeverityHigh, "SLICE-EMPTY", sl.ID, "el slice "+sl.ID+" no tiene tareas")
		}
	}
	for _, st := range s.Doc.Stories {
		for _, c := range st.Criteria {
			if !coveredCriterion[c.ID] {
				add(SeverityHigh, "CRITERION-UNCOVERED", c.ID, "ningún slice cubre el criterio "+c.ID+" de "+st.ID)
			}
		}
	}
	for _, t := range s.Doc.Tasks {
		if len(s.Doc.Slices) > 0 && !coveredTask[t.ID] {
			add(SeverityHigh, "TASK-ORPHAN", t.ID, "la tarea "+t.ID+" no pertenece a ningún slice")
		}
	}
}

// HasCritical informa si algún hallazgo bloquea el cierre.
func HasCritical(findings []HCDFinding) bool {
	return slices.ContainsFunc(findings, func(f HCDFinding) bool { return f.Severity == SeverityCritical })
}

// hcdCodeExtensions son las extensiones que cuentan como código para el aviso
// pre-edición (FR-025); documentación y configuración no lo disparan.
var hcdCodeExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".rs": true, ".java": true, ".kt": true, ".rb": true, ".php": true, ".cs": true,
	".c": true, ".cc": true, ".cpp": true, ".h": true, ".hpp": true, ".swift": true, ".sh": true,
	".sql": true, ".vue": true, ".svelte": true, ".dart": true, ".scala": true, ".ex": true, ".exs": true,
}

// IsHCDCodePath informa si editar path debería tener una unidad HCD activa.
func IsHCDCodePath(path string) bool {
	p := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if p == "" || strings.HasPrefix(p, "hcd/") || strings.Contains(p, "/hcd/") {
		return false
	}
	dot := strings.LastIndex(p, ".")
	if dot < 0 || dot < strings.LastIndex(p, "/") {
		return false
	}
	return hcdCodeExtensions[strings.ToLower(p[dot:])]
}

// HCDNoUnitWarning es el aviso no bloqueante del hook pre-edición.
const HCDNoUnitWarning = "gomemory HCD · vas a editar código sin una unidad HCD activa. Si es un cambio, inícialo con /hcd (Vía rápida para lo ordinario, ceremonia completa para decisiones interdependientes). Este aviso no bloquea y no se repite en la sesión."
