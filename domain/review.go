package domain

import (
	"fmt"
	"strings"
	"time"
)

type TargetType string

const (
	TargetCommit         TargetType = "commit"
	TargetDiff           TargetType = "diff"
	TargetFileSet        TargetType = "file_set"
	TargetSpec           TargetType = "spec"
	TargetPlan           TargetType = "plan"
	TargetConfig         TargetType = "config"
	TargetArchitecture   TargetType = "architecture"
	TargetMigration      TargetType = "migration"
	TargetAPIContract    TargetType = "api_contract"
	TargetImplementation TargetType = "implementation"
)

type Target struct {
	Type      TargetType
	Revision  string
	Scope     []string
	CreatedAt time.Time
	digest    string
}

func NewTarget(targetType TargetType, revision, digest string, scope []string) (Target, error) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return Target{}, fmt.Errorf("target digest is required")
	}
	return Target{
		Type:      targetType,
		Revision:  revision,
		Scope:     append([]string(nil), scope...),
		CreatedAt: time.Now(),
		digest:    digest,
	}, nil
}

func (t Target) Digest() string {
	return t.digest
}

func (t Target) ValidateDigest(candidate string) error {
	if strings.TrimSpace(candidate) != t.digest {
		return fmt.Errorf("target changed")
	}
	return nil
}

type ReviewStatus string

const (
	ReviewFrozen            ReviewStatus = "frozen"
	ReviewAwaitingReviewers ReviewStatus = "awaiting_reviewers"
	ReviewConsensusReady    ReviewStatus = "consensus_ready"
	ReviewFixing            ReviewStatus = "fixing"
	ReviewRejudging         ReviewStatus = "rejudging"
	ReviewApproved          ReviewStatus = "approved"
	ReviewEscalated         ReviewStatus = "escalated"
	ReviewIncomplete        ReviewStatus = "incomplete"
)

func (s ReviewStatus) Valid() bool {
	switch s {
	case ReviewFrozen, ReviewAwaitingReviewers, ReviewConsensusReady, ReviewFixing,
		ReviewRejudging, ReviewApproved, ReviewEscalated, ReviewIncomplete:
		return true
	default:
		return false
	}
}

func (s ReviewStatus) Terminal() bool {
	return s == ReviewApproved || s == ReviewEscalated || s == ReviewIncomplete
}

func (s ReviewStatus) CanTransitionTo(next ReviewStatus) bool {
	if s.Terminal() || !next.Valid() {
		return false
	}
	switch s {
	case ReviewFrozen:
		return next == ReviewAwaitingReviewers
	case ReviewAwaitingReviewers:
		return next == ReviewConsensusReady || next == ReviewIncomplete
	case ReviewConsensusReady:
		// consensus_ready -> rejudging es la transición REAL de una corrección.
		// La máquina original obligaba a pasar por `fixing`, pero ningún código
		// escribía nunca ese estado y no podía escribirlo: gomemory no corrige
		// nada, se entera de la corrección cuando el actor externo ya la aplicó y
		// presenta el delta. `fixing` describía un momento que este sistema no
		// llega a observar. Se conserva como estado válido —una base anterior
		// puede tener filas con él— pero deja de ser obligatorio de atravesar.
		return next == ReviewFixing || next == ReviewRejudging ||
			next == ReviewApproved || next == ReviewEscalated || next == ReviewIncomplete
	case ReviewFixing:
		return next == ReviewRejudging || next == ReviewEscalated || next == ReviewIncomplete
	case ReviewRejudging:
		// rejudging -> rejudging es la ronda N+1: cada corrección devuelve la
		// revisión a la espera de revalidación. No es un bucle abierto — el
		// presupuesto de NextFixRound es lo que lo acota.
		return next == ReviewFixing || next == ReviewRejudging ||
			next == ReviewApproved || next == ReviewEscalated || next == ReviewIncomplete
	default:
		return false
	}
}

type IndependenceLevel string

const (
	IndependenceFull     IndependenceLevel = "full"
	IndependenceDegraded IndependenceLevel = "degraded"
)

// ReviewerIdentity es la identidad ESPERADA de un revisor, congelada al iniciar la
// revisión. Sin ella, un resultado puede declarar cualquier proveedor y la
// independencia que la revisión afirma tener no es comprobable (FR-006).
type ReviewerIdentity struct {
	Provider string
	Model    string
}

func (i ReviewerIdentity) Declared() bool {
	return strings.TrimSpace(i.Provider) != "" || strings.TrimSpace(i.Model) != ""
}

// Matches compara con lo que declara un resultado recibido. La comparación ignora
// mayúsculas y espacios porque el proveedor lo escribe una persona o un agente, no
// un identificador canónico.
func (i ReviewerIdentity) Matches(provider, model string) bool {
	return strings.EqualFold(strings.TrimSpace(i.Provider), strings.TrimSpace(provider)) &&
		strings.EqualFold(strings.TrimSpace(i.Model), strings.TrimSpace(model))
}

func (i ReviewerIdentity) String() string {
	return i.Provider + "/" + i.Model
}

type Review struct {
	ID      string
	Project string
	Target  Target
	// CurrentTargetDigest es el target VIGENTE: el original mientras no haya
	// correcciones, el corregido por la última ronda válida después. Los resultados
	// posteriores a una corrección se validan contra este, no contra el original
	// (FR-008, FR-011).
	CurrentTargetDigest string
	MaxFixRounds        int
	AutoFixSeverities   []Severity
	// FixAuthorized distingue una revisión que puede corregir de una de solo
	// lectura. Es política persistida y no ausencia de configuración: sin este
	// campo, una revisión de solo validación con un defecto grave confirmado se
	// quedaba en consensus_ready para siempre, porque el presupuesto de rondas
	// permitía una corrección que su alcance prohibía hacer (FR-018, FR-019).
	FixAuthorized      bool
	ReviewerA          ReviewerIdentity
	ReviewerB          ReviewerIdentity
	IndependenceLevel  IndependenceLevel
	IndependenceReason string
	Round              int
	Status             ReviewStatus
	Verdict            Verdict
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ActiveTargetDigest devuelve el digest contra el que hay que validar ahora mismo.
// Una revisión anterior a esta funcionalidad no tiene CurrentTargetDigest: se
// interpreta como "igual al original", que es su comportamiento histórico.
func (r Review) ActiveTargetDigest() string {
	if strings.TrimSpace(r.CurrentTargetDigest) == "" {
		return r.Target.Digest()
	}
	return r.CurrentTargetDigest
}

// ExpectedReviewer devuelve la identidad congelada del revisor indicado.
func (r Review) ExpectedReviewer(reviewer Reviewer) ReviewerIdentity {
	if reviewer == ReviewerB {
		return r.ReviewerB
	}
	return r.ReviewerA
}

// TransitionTo es el ÚNICO punto por el que debe cambiar el estado de una revisión.
//
// La máquina de estados ya existía en CanTransitionTo y era correcta, pero ningún
// caso de uso la llamaba: todos asignaban Status directamente, así que una revisión
// APPROVED aceptaba resultados nuevos, consenso nuevo y correcciones nuevas sin
// protestar. La invariante no faltaba, faltaba obligar a pasar por ella (FR-015,
// FR-016).
func (r *Review) TransitionTo(next ReviewStatus) error {
	if r.Status.Terminal() {
		return fmt.Errorf("la revisión está en estado terminal %s y no admite cambios", r.Status)
	}
	if !next.Valid() {
		return fmt.Errorf("estado de revisión desconocido: %q", next)
	}
	if !r.Status.CanTransitionTo(next) {
		return fmt.Errorf("transición de revisión no permitida: %s -> %s", r.Status, next)
	}
	r.Status = next
	return nil
}

// EnsureMutable rechaza cualquier operación sobre una revisión ya cerrada. Se llama
// al principio de todo caso de uso que escriba en el ledger (FR-016).
func (r Review) EnsureMutable() error {
	if r.Status.Terminal() {
		return fmt.Errorf("la revisión está en estado terminal %s y no admite cambios", r.Status)
	}
	return nil
}
