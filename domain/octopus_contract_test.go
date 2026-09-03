package domain

import "testing"

func contratoDePrueba() ExecutionContract {
	return ExecutionContract{
		TaskID:        "T004",
		Objective:     "Determinar si la limpieza por expiración compite con el refresco",
		Scope:         Scope{Files: []string{"expiration.go", "store.go"}, ReadOnly: true},
		Permissions:   Permissions{Filesystem: FSReadOnly},
		ContextBudget: 3000,
		Output:        OutputSpec{MaxTokens: 1000, Required: []string{"conclusion", "evidence"}},
	}
}

// T055: un contrato incompleto deja al subagente sin poder terminar por su
// cuenta, que es justo lo que INV-AAR-003 exige evitar.
func TestExecutionContract_Validate(t *testing.T) {
	casos := []struct {
		nombre  string
		mutar   func(*ExecutionContract)
		wantErr bool
	}{
		{"completo", func(*ExecutionContract) {}, false},
		{"sin identificador", func(c *ExecutionContract) { c.TaskID = "" }, true},
		{"sin objetivo", func(c *ExecutionContract) { c.Objective = "" }, true},
		{"sin presupuesto de contexto", func(c *ExecutionContract) { c.ContextBudget = 0 }, true},
		{"sin presupuesto de salida", func(c *ExecutionContract) { c.Output.MaxTokens = 0 }, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			contrato := contratoDePrueba()
			c.mutar(&contrato)
			err := contrato.Validate()
			if c.wantErr != (err != nil) {
				t.Errorf("Validate() = %v, esperaba error=%v", err, c.wantErr)
			}
		})
	}
}

// T056 — AC-020: delegar NUNCA eleva privilegios. Un contrato con más permisos
// que el flujo principal se rechaza al construirlo, no al ejecutarlo: Octopus no
// ejecuta, así que este es el único momento en que puede impedirlo.
func TestPermissions_NoElevacion(t *testing.T) {
	casos := []struct {
		nombre        string
		padre, hijo   Permissions
		wantPermitido bool
	}{
		{"solo lectura desde solo lectura", Permissions{Filesystem: FSReadOnly}, Permissions{Filesystem: FSReadOnly}, true},
		{"solo lectura desde escritura", Permissions{Filesystem: FSReadWrite}, Permissions{Filesystem: FSReadOnly}, true},
		{"escritura desde solo lectura", Permissions{Filesystem: FSReadOnly}, Permissions{Filesystem: FSReadWrite}, false},
		{"red desde sin red", Permissions{Filesystem: FSReadWrite}, Permissions{Filesystem: FSReadOnly, Network: true}, false},
		{"red desde con red", Permissions{Filesystem: FSReadWrite, Network: true}, Permissions{Filesystem: FSReadOnly, Network: true}, true},
		{"ninguno desde solo lectura", Permissions{Filesystem: FSReadOnly}, Permissions{Filesystem: FSNone}, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.hijo.DentroDe(c.padre); got != c.wantPermitido {
				t.Errorf("DentroDe = %v, esperaba %v", got, c.wantPermitido)
			}
		})
	}
}

// Una investigación de solo lectura produce un contrato de solo lectura, sin que
// nadie tenga que acordarse de pedirlo.
func TestNewExecutionContract_AlcanceDeSoloLecturaNoEscribe(t *testing.T) {
	u := unidadDelegable()
	d := RouteDecision{WorkUnitID: u.ID, Route: RouteDelegate, ContextBudget: 2200, OutputBudget: 900}

	c, err := NewExecutionContract(u, d, Permissions{Filesystem: FSReadWrite}, DefaultMaxDelegationDepth)
	if err != nil {
		t.Fatalf("NewExecutionContract: %v", err)
	}
	if c.Permissions.Filesystem != FSReadOnly {
		t.Errorf("permisos = %q, un alcance de solo lectura no debe pedir escritura", c.Permissions.Filesystem)
	}
	if c.Permissions.Network {
		t.Error("nadie pidió red: el contrato no debe concederla")
	}
}

// Un contrato nunca puede exceder los permisos del flujo principal.
func TestNewExecutionContract_RechazaElevacion(t *testing.T) {
	u := unidadDelegable()
	u.Scope.ReadOnly = false // pediría escritura
	d := RouteDecision{WorkUnitID: u.ID, Route: RouteDelegate, ContextBudget: 2200, OutputBudget: 900}

	_, err := NewExecutionContract(u, d, Permissions{Filesystem: FSReadOnly}, DefaultMaxDelegationDepth)
	if err == nil {
		t.Fatal("un contrato que escribe bajo un padre de solo lectura debe rechazarse")
	}
}

// AC-010: con profundidad máxima 1, el contrato del hijo no lo autoriza a delegar.
func TestNewExecutionContract_ProfundidadRestante(t *testing.T) {
	u := unidadDelegable()
	d := RouteDecision{WorkUnitID: u.ID, Route: RouteDelegate, ContextBudget: 2200, OutputBudget: 900}

	c, err := NewExecutionContract(u, d, Permissions{Filesystem: FSReadWrite}, DefaultMaxDelegationDepth)
	if err != nil {
		t.Fatalf("NewExecutionContract: %v", err)
	}
	if c.MaxDepth != 0 {
		t.Errorf("MaxDepth = %d, con profundidad máxima 1 el hijo no puede delegar", c.MaxDepth)
	}
}
