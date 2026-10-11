package domain

import (
	"fmt"
	"sort"
	"time"
)

// Saneamiento histórico de privacidad (spec 037, US2, FR-006–008).
//
// Los registros guardados antes de la protección uniforme pueden contener
// regiones privadas o credenciales reconocidas. `mem sanitize` los enumera
// (vista previa) y los reescribe solo con --apply. Este archivo define los
// modelos puros: candidatos internos, informe depurado, estados y códigos.

// SanitizeCategory identifica qué clase de registro necesita saneamiento.
type SanitizeCategory string

const (
	SanitizeSession      SanitizeCategory = "session"
	SanitizeRelation     SanitizeCategory = "relation"
	SanitizeChannelError SanitizeCategory = "channel_error"
	// SanitizeOriginal es un original comprimido con contenido protegido:
	// no se reescribe (prometía recuperación exacta), se elimina.
	SanitizeOriginal SanitizeCategory = "compression_original"
	// SanitizeMemoryReview es solo informativa: una memoria con un
	// `</private>` huérfano es el remanente de un bloque anidado que la regex
	// anterior cortó a medias. El texto privado no se puede delimitar, así que
	// nunca se modifica automáticamente: se reporta para revisión manual.
	SanitizeMemoryReview SanitizeCategory = "memory_review"
)

// sanitizeFields es la lista permitida categoría → campo. Nada fuera de ella
// se lee ni se escribe durante el saneamiento.
var sanitizeFields = map[SanitizeCategory]string{
	SanitizeSession:      "summary",
	SanitizeRelation:     "reasoning",
	SanitizeChannelError: "last_error",
	SanitizeOriginal:     "content",
	SanitizeMemoryReview: "content",
}

// SanitizeCandidate es un registro que requiere saneamiento. Text vive solo
// en memoria del proceso: nunca se serializa ni aparece en informes o errores.
type SanitizeCandidate struct {
	Category SanitizeCategory `json:"category"`
	// RecordID es la identidad en persistencia (simple o compuesta).
	RecordID string `json:"record_id"`
	Project  string `json:"project"`
	Field    string `json:"field"`
	Text     string `json:"-"`
}

// Validate comprueba categoría y campo permitidos, proyecto en alcance e
// identidad presente.
func (c SanitizeCandidate) Validate(project string) error {
	field, ok := sanitizeFields[c.Category]
	switch {
	case !ok:
		return fmt.Errorf("%w: categoría de saneamiento no permitida", ErrValidation)
	case c.Field != field:
		return fmt.Errorf("%w: campo no permitido para %s", ErrValidation, c.Category)
	case c.Project != project:
		return fmt.Errorf("%w: registro fuera del proyecto", ErrValidation)
	case c.RecordID == "":
		return fmt.Errorf("%w: registro sin identidad", ErrValidation)
	}
	return nil
}

// Writable dice si el saneamiento aplica cambios a esta categoría.
func (c SanitizeCandidate) Writable() bool { return c.Category != SanitizeMemoryReview }

// NeedsSanitize dice si un texto cambia al aplicar la protección vigente.
func NeedsSanitize(s string) bool { return ProtectText(s) != s }

// ContainsProtected dice si un texto conserva contenido protegido: una
// apertura privada (región completa o abierta) o una credencial reconocida.
// A diferencia de NeedsSanitize, un `</private>` huérfano no cuenta: es la
// señal del remanente que se reporta como memory_review, no contenido que
// la protección pueda eliminar. Es la regla para verificar residuos.
func ContainsProtected(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			if kind, _ := matchPrivateTag(s[i:]); kind == privateOpen {
				return true
			}
		}
	}
	return RedactSecrets(s) != s
}

// HasOrphanPrivateClose detecta un `</private>` sin apertura previa.
func HasOrphanPrivateClose(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		switch kind, _ := matchPrivateTag(s[i:]); kind {
		case privateOpen:
			depth++
		case privateClose:
			if depth == 0 {
				return true
			}
			depth--
		}
	}
	return false
}

// SanitizeMode distingue vista previa de aplicación.
type SanitizeMode string

const (
	SanitizeModePreview SanitizeMode = "preview"
	SanitizeModeApply   SanitizeMode = "apply"
)

// SanitizeStatus es el estado del informe.
type SanitizeStatus string

const (
	SanitizeStatusPreview        SanitizeStatus = "preview"
	SanitizeStatusPendingCleanup SanitizeStatus = "pending_cleanup"
	SanitizeStatusCompleted      SanitizeStatus = "completed"
	SanitizeStatusFailed         SanitizeStatus = "failed"
)

// Códigos de diagnóstico estables (contracts/privacy-maintenance.md).
const (
	SanitizeCodeFailed         = "SANITIZE_FAILED"
	SanitizeCodeBusy           = "SANITIZE_BUSY"
	SanitizeCodePendingCleanup = "SANITIZE_PENDING_CLEANUP"
	SanitizeCodeTimeout        = "SANITIZE_TIMEOUT"
)

// SanitizeExternalCopiesNotice acompaña cada informe: gomemory no controla
// exportaciones arbitrarias, snapshots del sistema ni medios externos.
const SanitizeExternalCopiesNotice = "Fuera de control de gomemory: revisa exportaciones manuales, " +
	"copias del sistema y medios externos; el saneamiento no promete borrado físico en ellos."

// SanitizeCategoryCount resume una categoría sin texto.
type SanitizeCategoryCount struct {
	Category      SanitizeCategory `json:"category"`
	AffectedCount int              `json:"affected_count"`
	RecordIDs     []string         `json:"record_ids"`
}

// SanitizeArtifact es el resultado sobre un archivo o derivado controlado.
type SanitizeArtifact struct {
	// Identity es relativa a la raíz administrada; nunca una ruta absoluta.
	Identity  string `json:"identity"`
	Category  string `json:"category"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
}

// Estados de artefacto.
const (
	ArtifactClean     = "clean"
	ArtifactSanitized = "sanitized"
	ArtifactRemoved   = "removed"
	ArtifactPending   = "pending"
)

// SanitizePending es una acción operativa pendiente, sin contenido.
type SanitizePending struct {
	Code   string `json:"code"`
	Action string `json:"action"`
}

// SanitizeReport es el informe depurado de una ejecución.
type SanitizeReport struct {
	SchemaVersion  int                     `json:"schema_version"`
	RunID          string                  `json:"run_id,omitempty"`
	Mode           SanitizeMode            `json:"mode"`
	Project        string                  `json:"project"`
	Status         SanitizeStatus          `json:"status"`
	Categories     []SanitizeCategoryCount `json:"categories"`
	RowsChanged    int                     `json:"rows_changed"`
	Artifacts      []SanitizeArtifact      `json:"artifacts"`
	Pending        []SanitizePending       `json:"pending"`
	ExternalCopies string                  `json:"external_copies"`
	StartedAt      *time.Time              `json:"started_at,omitempty"`
	FinishedAt     *time.Time              `json:"finished_at,omitempty"`
}

// NewSanitizeReport arma un informe vacío con listas no nulas (JSON estable).
func NewSanitizeReport(project string, mode SanitizeMode) SanitizeReport {
	status := SanitizeStatusPreview
	if mode == SanitizeModeApply {
		status = SanitizeStatusPendingCleanup
	}
	return SanitizeReport{
		SchemaVersion:  1,
		Mode:           mode,
		Project:        project,
		Status:         status,
		Categories:     []SanitizeCategoryCount{},
		Artifacts:      []SanitizeArtifact{},
		Pending:        []SanitizePending{},
		ExternalCopies: SanitizeExternalCopiesNotice,
	}
}

// SummarizeCandidates agrupa candidatos por categoría, con categorías e ids
// en orden determinista.
func SummarizeCandidates(cands []SanitizeCandidate) []SanitizeCategoryCount {
	byCat := map[SanitizeCategory][]string{}
	for _, c := range cands {
		byCat[c.Category] = append(byCat[c.Category], c.RecordID)
	}
	out := make([]SanitizeCategoryCount, 0, len(byCat))
	for cat, ids := range byCat {
		sort.Strings(ids)
		out = append(out, SanitizeCategoryCount{Category: cat, AffectedCount: len(ids), RecordIDs: ids})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Category < out[j].Category })
	return out
}

// SanitizeError lleva un código estable. Error() nunca incluye la causa
// cruda (puede venir del driver o de un archivo); Unwrap la conserva para
// clasificar con errors.Is.
type SanitizeError struct {
	Code  string
	cause error
}

func NewSanitizeError(code string, cause error) error {
	return &SanitizeError{Code: code, cause: cause}
}

func (e *SanitizeError) Error() string { return e.Code }
func (e *SanitizeError) Unwrap() error { return e.cause }
