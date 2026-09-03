package domain

import "fmt"

// --- Contrato de ejecución (feature 027) ---
//
// Es lo que recibe un subagente: todo lo que necesita para terminar por su
// cuenta, y nada más. Se construye AQUÍ, no en el runtime, porque es el último
// momento en que Octopus puede impedir una elevación de privilegios: Octopus no
// ejecuta, así que después ya no tiene forma de intervenir (INV-AAR-014).

// FSAccess es el nivel de acceso al sistema de archivos.
type FSAccess string

const (
	FSNone      FSAccess = "none"
	FSReadOnly  FSAccess = "read-only"
	FSReadWrite FSAccess = "read-write"
)

func (a FSAccess) nivel() int {
	switch a {
	case FSReadWrite:
		return 2
	case FSReadOnly:
		return 1
	default:
		return 0
	}
}

// Permissions son las capacidades que un contrato declara necesitar.
type Permissions struct {
	Filesystem FSAccess
	Network    bool
}

// DentroDe responde si estos permisos caben dentro de los del flujo principal.
// Delegar nunca concede lo que el padre no tenía (FR-028, AC-020).
func (p Permissions) DentroDe(padre Permissions) bool {
	if p.Filesystem.nivel() > padre.Filesystem.nivel() {
		return false
	}
	return !p.Network || padre.Network
}

// ExecutionContract es la descripción acotada que recibe un subagente.
type ExecutionContract struct {
	TaskID        string
	Objective     string
	Scope         Scope
	Permissions   Permissions
	ContextBudget int
	Output        OutputSpec
	// MaxDepth es la profundidad de delegación que le queda AL HIJO. Con el
	// valor de fábrica (1) sale en 0: el hijo no engendra otro hijo.
	MaxDepth int
}

// Validate comprueba que el contrato baste para terminar de forma independiente.
func (c ExecutionContract) Validate() error {
	if c.TaskID == "" {
		return fmt.Errorf("%w: el contrato necesita identificador de tarea", ErrValidation)
	}
	if c.Objective == "" {
		return fmt.Errorf("%w: el contrato de %s necesita un objetivo acotado", ErrValidation, c.TaskID)
	}
	if c.ContextBudget <= 0 {
		return fmt.Errorf("%w: el contrato de %s necesita presupuesto de contexto", ErrValidation, c.TaskID)
	}
	if c.Output.MaxTokens <= 0 {
		return fmt.Errorf("%w: el contrato de %s necesita presupuesto de salida", ErrValidation, c.TaskID)
	}
	return nil
}

// camposDeResultadoPorDefecto es la forma mínima de resultado que sirve para
// integrar sin leer una transcripción.
var camposDeResultadoPorDefecto = []string{"conclusion", "evidence", "affected_symbols"}

// NewExecutionContract arma el contrato de una unidad delegada.
//
// Los permisos se DERIVAN del alcance, no se piden: un alcance de solo lectura
// produce un contrato de solo lectura sin que nadie tenga que acordarse. Y se
// comprueban contra los del flujo principal antes de devolverlo.
func NewExecutionContract(u WorkUnit, d RouteDecision, padre Permissions, maxDepth int) (ExecutionContract, error) {
	permisos := Permissions{Filesystem: FSReadOnly}
	if len(u.Scope.Files) > 0 && !u.Scope.ReadOnly {
		permisos.Filesystem = FSReadWrite
	}
	if !permisos.DentroDe(padre) {
		return ExecutionContract{}, fmt.Errorf(
			"%w: el contrato de %s pediría %q bajo un flujo principal con %q — delegar no eleva privilegios",
			ErrValidation, u.ID, permisos.Filesystem, padre.Filesystem)
	}

	salida := d.OutputBudget
	if salida <= 0 {
		salida = DefaultOutputBudgetTokens
	}
	requeridos := u.ExpectedOutput.Required
	if len(requeridos) == 0 {
		requeridos = camposDeResultadoPorDefecto
	}

	// La profundidad que le queda al hijo es la del padre menos uno, nunca
	// negativa: 0 significa "no puedes delegar" (INV-AAR-009).
	restante := maxDepth - 1
	if restante < 0 {
		restante = 0
	}

	c := ExecutionContract{
		TaskID:        u.ID,
		Objective:     u.Objective,
		Scope:         u.Scope,
		Permissions:   permisos,
		ContextBudget: d.ContextBudget,
		Output:        OutputSpec{MaxTokens: salida, Required: requeridos, Format: "structured-summary"},
		MaxDepth:      restante,
	}
	if err := c.Validate(); err != nil {
		return ExecutionContract{}, err
	}
	return c, nil
}
