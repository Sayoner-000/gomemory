package domain

import "testing"

// El reparto no puede prometer más de lo que hay: si las tres bolsas suman más
// que el total, alguna se quedaría sin fondo en silencio — exactamente el
// desborde silencioso que prohíbe INV-AAR-005.
func TestBudget_Valid(t *testing.T) {
	casos := []struct {
		nombre string
		b      Budget
		want   bool
	}{
		{"reparto exacto", Budget{TotalTokens: 100, MainAgentMax: 55, DelegationPoolMax: 30, ValidationReserve: 15}, true},
		{"reparto con holgura", Budget{TotalTokens: 100, MainAgentMax: 50, DelegationPoolMax: 20, ValidationReserve: 10}, true},
		{"reparto que se pasa", Budget{TotalTokens: 100, MainAgentMax: 60, DelegationPoolMax: 30, ValidationReserve: 20}, false},
		{"sin presupuesto declarado", Budget{}, true},
		{"gasto mayor que el fondo", Budget{TotalTokens: 100, DelegationPoolMax: 30, DelegationSpent: 40}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.b.Valid(); got != c.want {
				t.Errorf("Valid() = %v, esperaba %v", got, c.want)
			}
		})
	}
}

// Sin presupuesto declarado la política debe seguir funcionando: no es un error,
// es la ausencia de una restricción (INV-AAR-016).
func TestBudget_SinDeclararEsIlimitado(t *testing.T) {
	var b Budget
	if b.Declarado() {
		t.Error("un presupuesto en cero no está declarado")
	}
	if !b.Cabe(999999) {
		t.Error("sin presupuesto declarado cualquier costo debe caber")
	}
}

func TestBudget_DelegationRemaining(t *testing.T) {
	b := Budget{TotalTokens: 1000, MainAgentMax: 550, DelegationPoolMax: 300, ValidationReserve: 150, DelegationSpent: 120}
	if got := b.DelegationRemaining(); got != 180 {
		t.Errorf("DelegationRemaining = %d, esperaba 180", got)
	}
	b.DelegationSpent = 300
	if got := b.DelegationRemaining(); got != 0 {
		t.Errorf("fondo agotado debería dar 0, dio %d", got)
	}
}

// La reserva de validación no es un fondo del que se pueda tirar: Cabe() la
// excluye siempre (INV-AAR-006, AC-008).
func TestBudget_CabeNuncaTocaLaReserva(t *testing.T) {
	b := Budget{TotalTokens: 1000, MainAgentMax: 550, DelegationPoolMax: 300, ValidationReserve: 150, DelegationSpent: 300}

	if b.DelegationRemaining() != 0 {
		t.Fatal("preparación: el fondo de delegación debería estar agotado")
	}
	if b.Cabe(10) {
		t.Error("con el fondo agotado no debe caber nada, aunque la reserva tenga tokens")
	}
}

// La autorización de reserva no puede dejar DelegationSpent por encima de su
// bolsa: el consumo adicional se refleja explícitamente en la reserva.
func TestBudget_GastarConsumeReservaSinRomperInvariantes(t *testing.T) {
	b := Budget{TotalTokens: 1000, MainAgentMax: 550, DelegationPoolMax: 300, ValidationReserve: 150, DelegationSpent: 280}
	b = b.Gastar(100)
	if b.DelegationSpent != 300 || b.ValidationReserve != 70 {
		t.Fatalf("gasto = delegación %d, reserva %d; esperaba 300 y 70", b.DelegationSpent, b.ValidationReserve)
	}
	if !b.Valid() {
		t.Fatal("un gasto cubierto por la reserva debe conservar los invariantes")
	}
}

// El reparto por porcentajes es configurable y no impone cifras fijas (FR-029).
func TestNewBudget_RepartoConfigurable(t *testing.T) {
	b := NewBudget(1000, BudgetSplit{MainAgentPct: 50, DelegationPct: 40, ValidationPct: 10})

	if b.MainAgentMax != 500 || b.DelegationPoolMax != 400 || b.ValidationReserve != 100 {
		t.Errorf("reparto = %d/%d/%d, esperaba 500/400/100", b.MainAgentMax, b.DelegationPoolMax, b.ValidationReserve)
	}
	if !b.Valid() {
		t.Error("un reparto que suma 100 % debe ser válido")
	}
}

func TestNewBudget_RepartoPorDefecto(t *testing.T) {
	s := DefaultBudgetSplit()
	if s.MainAgentPct+s.DelegationPct+s.ValidationPct != 100 {
		t.Errorf("el reparto por defecto debe sumar 100, suma %d", s.MainAgentPct+s.DelegationPct+s.ValidationPct)
	}
	if !NewBudget(60000, s).Valid() {
		t.Error("el reparto por defecto debe producir un presupuesto válido")
	}
}

// Un reparto inválido (no suma 100, o trae negativos) cae al de fábrica en vez
// de producir un presupuesto silenciosamente roto.
func TestNewBudget_RepartoInvalidoCaeAlDeFabrica(t *testing.T) {
	got := NewBudget(1000, BudgetSplit{MainAgentPct: 200, DelegationPct: 50, ValidationPct: 3})
	want := NewBudget(1000, DefaultBudgetSplit())
	if got != want {
		t.Errorf("un reparto inválido debería caer al de fábrica: %+v vs %+v", got, want)
	}
}

func TestCostEstimate_Total(t *testing.T) {
	c := CostEstimate{ContextTokens: 100, ContractTokens: 20, OutputTokens: 50, CoordinationTokens: 10, IntegrationTokens: 5}
	if got := c.Total(); got != 185 {
		t.Errorf("Total = %d, esperaba 185", got)
	}
}

// --- Historia 5: presión de presupuesto en la política ---

// T064 — AC-007: presupuesto insuficiente NUNCA se desborda en silencio.
func TestRouteTask_AC007_PresupuestoInsuficiente(t *testing.T) {
	in := RouteInput{
		Unit:         unidadDelegable(),
		Capabilities: capacidadesPlenas(),
		// Fondo minúsculo, sin reserva: no hay a qué recurrir.
		Budget: Budget{TotalTokens: 5000, MainAgentMax: 4900, DelegationPoolMax: 100},
	}

	got := RouteTask(in)

	if got.Route.Delegada() {
		t.Fatalf("ruta = %q: no puede delegarse sin fondo", got.Route)
	}
	if got.Reason != ReasonBudgetExhausted {
		t.Errorf("razón = %q, esperaba %q", got.Reason, ReasonBudgetExhausted)
	}
	// El presupuesto no se movió: rechazar no consume.
	if in.Budget.DelegationSpent != 0 {
		t.Error("una delegación rechazada no debe consumir presupuesto")
	}
}

// T065 — AC-008: los tokens que quedan están en la reserva de validación y la
// unidad es prescindible. La reserva se protege y la razón lo DICE: no es lo
// mismo "no hay presupuesto" que "lo que queda no se toca".
func TestRouteTask_AC008_ReservaProtegida(t *testing.T) {
	u := unidadDelegable()
	u.Optional = true

	got := RouteTask(RouteInput{
		Unit:         u,
		Capabilities: capacidadesPlenas(),
		Budget: Budget{
			TotalTokens: 5000, MainAgentMax: 2000,
			DelegationPoolMax: 1000, DelegationSpent: 1000,
			ValidationReserve: 2000,
		},
	})

	if got.Route.Delegada() {
		t.Fatalf("ruta = %q: la reserva no se toca", got.Route)
	}
	if got.Reason != ReasonValidationReserveProtected {
		t.Errorf("razón = %q, esperaba %q", got.Reason, ReasonValidationReserveProtected)
	}
}

// Con autorización explícita, la reserva SÍ puede usarse (FR-031).
func TestRouteTask_ReservaConAutorizacionExplicita(t *testing.T) {
	u := unidadDelegable()
	u.Optional = true

	got := RouteTask(RouteInput{
		Unit:         u,
		Capabilities: capacidadesPlenas(),
		Budget: Budget{
			TotalTokens: 50000, MainAgentMax: 20000,
			DelegationPoolMax: 1000, DelegationSpent: 1000,
			ValidationReserve: 20000,
		},
		Policy: PolicyOverrides{AllowValidationReserve: true},
	})

	if !got.Route.Delegada() {
		t.Errorf("ruta = %q: con autorización explícita la reserva es utilizable (razón: %q)", got.Route, got.Reason)
	}
}

// Una unidad NO opcional no dispara la protección de la reserva: si es
// imprescindible, la razón honesta es que no hay presupuesto, no que se esté
// protegiendo algo.
func TestRouteTask_ReservaNoAplicaAUnidadImprescindible(t *testing.T) {
	got := RouteTask(RouteInput{
		Unit:         unidadDelegable(), // Optional = false
		Capabilities: capacidadesPlenas(),
		Budget: Budget{
			TotalTokens: 5000, MainAgentMax: 2000,
			DelegationPoolMax: 1000, DelegationSpent: 1000,
			ValidationReserve: 2000,
		},
	})

	if got.Reason != ReasonBudgetExhausted {
		t.Errorf("razón = %q, esperaba %q para una unidad imprescindible", got.Reason, ReasonBudgetExhausted)
	}
}

// T067 — FR-033: sin medición real, toda cifra sale marcada como estimada.
func TestRouteTask_CifrasMarcadasComoEstimadas(t *testing.T) {
	for _, ruta := range []RouteInput{
		{Unit: unidadDelegable(), Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio()},
		{Unit: unidadDelegable(), Capabilities: RuntimeCapabilities{}, Budget: presupuestoAmplio()},
	} {
		if got := RouteTask(ruta); !got.Estimated {
			t.Error("toda decisión sin reporte real debe marcarse como estimada")
		}
	}
}

// El fondo se consume a medida que el plan delega, y nunca lo desborda.
func TestRoutePlan_ConsumeElFondoSinDesbordarlo(t *testing.T) {
	var unidades []WorkUnit
	for i := 1; i <= 6; i++ {
		unidades = append(unidades, unidadAislable(idTarea(i)))
	}
	in := planDePrueba(unidades...)
	in.Policy.MaxSubagents = 6

	plan, err := RoutePlan(in)
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}

	if plan.Budget.DelegationSpent > plan.Budget.DelegationPoolMax {
		t.Errorf("el fondo se desbordó: %d de %d", plan.Budget.DelegationSpent, plan.Budget.DelegationPoolMax)
	}
	if plan.Budget.ValidationReserve != in.Budget.ValidationReserve {
		t.Errorf("la reserva cambió: %d → %d", in.Budget.ValidationReserve, plan.Budget.ValidationReserve)
	}
	if !plan.Budget.Valid() {
		t.Error("el presupuesto resultante debe seguir siendo válido")
	}
}
