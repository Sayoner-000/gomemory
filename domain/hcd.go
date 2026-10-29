package domain

type Kind string

const (
	KindFeature   Kind = "feature"
	KindEvolution Kind = "evolution"
	KindFix       Kind = "fix"
	KindTechnical Kind = "technical"
)

type State string

const (
	StateDraft        State = "draft"
	StateReady        State = "ready"
	StateInProgress   State = "in_progress"
	StateVerification State = "verification"
	StateCompleted    State = "completed"
	StateBlocked      State = "blocked"
	StateCancelled    State = "cancelled"
	StateSuperseded   State = "superseded"
)

type Stage string

const (
	HCDStageContext       Stage = "context"
	HCDStageIntent        Stage = "intent"
	HCDStageDefinition    Stage = "definition"
	HCDStageClarification Stage = "clarification"
	HCDStageApproval      Stage = "approval"
	HCDStageSlicing       Stage = "slicing"
	HCDStageDecomposition Stage = "decomposition"
	HCDStageExecution     Stage = "execution"
	HCDStageVerification  Stage = "verification"
	HCDStageContinuity    Stage = "continuity"
	HCDStageClosure       Stage = "closure"
)

type Unit struct {
	ID, Title, Slug string
	SourceDir       string
	Kind            Kind
	Approach        string
	State           State
	PrevState       State
	StateRevision   int64
	DefinitionRev   string
	DeliveryRev     string
	ApprovedRev     string
	PlaybookRev     string
	ConstitutionRev string
	Implementer     string

	// These derived facts are populated by the use case before asking the domain
	// to select a stage; they are not persisted as independent state.
	IntentComplete, OpenClarification, ValidationPassed bool
	ReadySlice, UnadmittedTask, AdmittedTask            bool
	GrantedExecution, AllSlicesVerified, WorkRemaining  bool
}

type PendingCheck struct{ Code, Message, Field string }

type NextStep struct{ Skill, Command, Why string }

func StageFor(unit Unit) Stage {
	if unit.State == "" {
		return HCDStageContext
	}
	switch unit.State {
	case StateCompleted, StateCancelled, StateSuperseded:
		return HCDStageClosure
	case StateVerification:
		return HCDStageVerification
	case StateDraft:
		if unit.OpenClarification {
			return HCDStageClarification
		}
		if unit.ValidationPassed {
			return HCDStageApproval
		}
		if !unit.IntentComplete {
			return HCDStageIntent
		}
		return HCDStageDefinition
	case StateReady, StateInProgress:
		if unit.GrantedExecution || unit.AdmittedTask {
			return HCDStageExecution
		}
		if unit.ReadySlice && unit.UnadmittedTask {
			return HCDStageDecomposition
		}
		if unit.AllSlicesVerified && unit.WorkRemaining {
			return HCDStageContinuity
		}
		// Todas las tareas del slice ya informaron: lo que sigue es verificar.
		if unit.State == StateInProgress && unit.ReadySlice && !unit.UnadmittedTask {
			return HCDStageVerification
		}
		return HCDStageSlicing
	default:
		return HCDStageContext
	}
}

func AllowedActions(unit Unit) []string {
	if unit.Approach == "speckit" || unit.Approach == "unclassified" {
		return []string{"show", "resume"}
	}
	switch unit.State {
	case StateDraft:
		return []string{"show", "resume", "define", "validate", "clarify", "approve", "focus", "exception", "playbook"}
	case StateReady:
		return []string{"show", "resume", "slice", "tasks", "check", "run", "block", "cancel", "focus", "exception", "playbook"}
	case StateInProgress:
		return []string{"show", "resume", "slice", "tasks", "check", "run", "report", "block", "verify", "cancel", "focus", "exception", "playbook"}
	case StateVerification:
		return []string{"show", "resume", "verify", "close", "cancel", "focus", "exception", "playbook"}
	case StateBlocked:
		return []string{"show", "resume", "unblock", "cancel", "exception", "playbook"}
	case StateCompleted, StateCancelled, StateSuperseded:
		return []string{"show", "resume"}
	default:
		return []string{"show"}
	}
}

func NextAction(unit Unit, pending []PendingCheck) NextStep {
	if len(pending) > 0 {
		if pending[0].Code == "CLARIFICATION-OPEN" || pending[0].Code == "DECISION-CONFLICT" {
			return NextStep{Skill: "hcd-clarify", Command: "clarify", Why: pending[0].Message}
		}
		return NextStep{Skill: "hcd-define", Command: "validate", Why: pending[0].Message}
	}
	switch StageFor(unit) {
	case HCDStageIntent, HCDStageDefinition:
		return NextStep{Skill: "hcd-define", Command: "define", Why: "la definición requiere completar o revisar su contenido"}
	case HCDStageClarification:
		return NextStep{Skill: "hcd-clarify", Command: "clarify", Why: "hay una aclaración abierta"}
	case HCDStageApproval:
		return NextStep{Skill: "hcd-approve", Command: "approve", Why: "la definición está validada y espera aprobación"}
	case HCDStageSlicing:
		return NextStep{Skill: "hcd-slice", Command: "slice", Why: "se necesita preparar un slice"}
	case HCDStageDecomposition:
		return NextStep{Skill: "hcd-tasks", Command: "tasks", Why: "el slice requiere tareas atómicas"}
	case HCDStageExecution:
		return NextStep{Skill: "hcd-implement", Command: "run", Why: "hay trabajo admitido para ejecutar"}
	case HCDStageVerification:
		// verify solo deja la unidad en verification con un PASS; lo que falta
		// entonces es la aceptación citada del cierre.
		if unit.State == StateVerification {
			return NextStep{Skill: "hcd-close", Command: "close", Why: "la verificación dio PASS y falta la aceptación citada del cierre"}
		}
		return NextStep{Skill: "hcd-verify", Command: "verify", Why: "la unidad espera verificación"}
	case HCDStageContinuity:
		return NextStep{Skill: "hcd", Command: "resume", Why: "el trabajo verificado requiere decidir continuidad"}
	case HCDStageClosure:
		// StageFor solo da closure a estados terminales: ya no queda paso en esta unidad.
		return NextStep{Skill: "hcd", Command: "list", Why: "la unidad está terminada; retoma otra o inicia una nueva"}
	default:
		return NextStep{Skill: "hcd", Command: "start", Why: "no hay unidad activa"}
	}
}

func ValidHCDTransition(entity, from, to string) bool {
	transitions := map[string]map[string][]string{
		"story":     {"draft": {"ready", "blocked", "rejected"}, "ready": {"in_progress", "blocked", "rejected"}, "in_progress": {"satisfied", "blocked", "rejected"}, "blocked": {"draft", "ready", "in_progress", "rejected"}},
		"criterion": {"defined": {"covered", "blocked", "failed", "not_applicable"}, "covered": {"verified", "failed", "blocked", "not_applicable"}, "failed": {"covered", "blocked"}, "blocked": {"defined", "covered"}},
		"slice":     {"proposed": {"ready", "blocked", "rejected"}, "ready": {"in_progress", "blocked", "rejected"}, "in_progress": {"implemented", "blocked"}, "implemented": {"verified", "blocked"}, "blocked": {"proposed", "ready", "in_progress"}},
		"task":      {"pending": {"in_progress", "blocked", "cancelled"}, "in_progress": {"implemented", "blocked", "cancelled"}, "implemented": {"verified", "blocked"}, "blocked": {"pending", "in_progress", "cancelled"}},
		"exception": {"proposed": {"approved", "rejected"}, "approved": {"expired", "revoked"}},
		"migration": {"proposed": {"active", "cancelled"}},
	}
	for _, candidate := range transitions[entity][from] {
		if candidate == to {
			return true
		}
	}
	return false
}
