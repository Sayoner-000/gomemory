package domain

import (
	"errors"
	"strings"
)

var (
	ErrNoExecutor        = errors.New("no executor available")
	ErrOperationConflict = errors.New("operation conflict")
	ErrApproachMismatch  = errors.New("approach mismatch")
	ErrInvalidQuote      = errors.New("invalid approval quote")
	ErrAtomicity         = errors.New("task is not atomic")
	ErrBlocked           = errors.New("work is blocked")
	ErrUncertain         = errors.New("execution outcome is uncertain")
	// ErrConstitutionConflict: la Constitución vigente exige artefactos Spec
	// Kit por unidad (§18 anterior a la enmienda) y no reconoce HCD.
	ErrConstitutionConflict = errors.New("constitution requires Spec Kit artifacts and does not admit HCD")
)

// ConstitutionAdmitsHCD informa si una Constitución permite crear unidades
// HCD: la que exige la carpeta de Spec Kit por unidad debe declarar también la
// ubicación HCD (§18 enmendada); una que no exige Spec Kit no entra en
// conflicto.
func ConstitutionAdmitsHCD(content string) bool {
	return !strings.Contains(content, "specs/{NNN}-{name}/") || strings.Contains(content, "hcd/features/{NNN}-{name}/")
}

type HCDValidationError struct{ Checks []PendingCheck }

func (e *HCDValidationError) Error() string { return "HCD fields are missing or invalid" }
func (e *HCDValidationError) Unwrap() error { return ErrValidation }
