package domain

import (
	"errors"
	"fmt"
	"time"
)

// Kind es el tipo de una unidad HCD. Solo fix añade una sección obligatoria
// propia (Reproducción).
type Kind string

const (
	KindFeature   Kind = "feature"
	KindEvolution Kind = "evolution"
	KindFix       Kind = "fix"
	KindTechnical Kind = "technical"
)

func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindFeature, KindEvolution, KindFix, KindTechnical:
		return k, nil
	}
	return "", fmt.Errorf("%w: tipo HCD desconocido %q", ErrValidation, s)
}

// Lane es el carril de una unidad: full recorre la ceremonia completa; fast
// es el atajo sin approve ni slice, con la única aprobación al cierre.
type Lane string

const (
	LaneFast Lane = "fast"
	LaneFull Lane = "full"
)

func ParseLane(s string) (Lane, error) {
	switch l := Lane(s); l {
	case LaneFast, LaneFull:
		return l, nil
	}
	return "", fmt.Errorf("%w: carril HCD desconocido %q", ErrValidation, s)
}

type UnitState string

const (
	UnitActive    UnitState = "active"
	UnitClosed    UnitState = "closed"
	UnitCancelled UnitState = "cancelled"
)

// Stage es el paso de la ceremonia. Se deriva siempre del documento, las
// aprobaciones y la evidencia; ningún adaptador lo fija.
type Stage string

const (
	StageDefine    Stage = "define"
	StageClarify   Stage = "clarify"
	StageApprove   Stage = "approve"
	StageSlice     Stage = "slice"
	StageTasks     Stage = "tasks"
	StageImplement Stage = "implement"
	StageVerify    Stage = "verify"
	StageClose     Stage = "close"
	StageClosed    Stage = "closed"
)

type Unit struct {
	ID        string
	Number    int
	Title     string
	Slug      string
	Kind      Kind
	Lane      Lane
	Why       string
	State     UnitState
	Active    bool
	DocPath   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Criterion struct{ ID, Text string }

type Story struct {
	ID, Title, Text string
	Criteria        []Criterion
}

type Clarification struct {
	ID, Question, Answer string
	Open                 bool
}

type Slice struct {
	ID, Title string
	Tasks     []string
	Criteria  []string
}

type Task struct {
	ID    string
	Text  string
	Check string
	Done  bool
}

type PendingCheck struct{ Code, Field, Message string }

const (
	EvidenceCommand = "command"
	EvidenceManual  = "manual"
)

type Evidence struct {
	ID          string
	UnitID      string
	Tasks       []string
	Kind        string
	ExpectFail  bool
	Command     string
	ExitCode    int
	Excerpt     string
	Revision    string
	Fingerprint string
	CreatedAt   time.Time
}

// Passed: una evidencia manual cuenta como observación declarada; una de
// comando solo pasa con código de salida 0. Una evidencia roja nunca verifica.
func (e Evidence) Passed() bool {
	return !e.ExpectFail && (e.Kind == EvidenceManual || e.ExitCode == 0)
}

// RedObserved: la evidencia roja solo vale si el comando falló de verdad.
func (e Evidence) RedObserved() bool {
	return e.ExpectFail && e.Kind == EvidenceCommand && e.ExitCode != 0
}

func (e Evidence) Fresh(revision, fingerprint string) bool {
	return e.Revision == revision && e.Fingerprint == fingerprint
}

// Acciones que registra hcd_approvals.
const (
	ApprovalApprove = "approve"
	ApprovalClose   = "close"
	ApprovalCancel  = "cancel"
)

type Approval struct {
	ID          string
	UnitID      string
	Action      string
	Revision    string
	Fingerprint string
	Origin      string
	Client      string
	Quote       string
	// ConstitutionRev y PlaybookRev son las huellas de las normas vigentes al
	// aprobar: si cambian después, analyze lo reporta (FR-029).
	ConstitutionRev string
	PlaybookRev     string
	CreatedAt       time.Time
}

type NextStep struct{ Skill, Command, Why string }

// HCDFinding es un hallazgo de analyze; reutiliza la Severity de las revisiones.
type HCDFinding struct {
	Severity           Severity
	Code, Ref, Message string
}

// HCDActiveContext es lo que el contexto de arranque muestra de la unidad activa.
type HCDActiveContext struct {
	Unit       Unit
	Stage      Stage
	NextAction NextStep
	Unverified []string
}

var ErrInvalidQuote = errors.New("invalid approval quote")

type HCDValidationError struct{ Checks []PendingCheck }

func (e *HCDValidationError) Error() string { return "HCD fields are missing or invalid" }
func (e *HCDValidationError) Unwrap() error { return ErrValidation }
