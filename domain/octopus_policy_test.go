package domain

import (
	"encoding/json"
	"testing"
)

// unidadDelegable es la unidad "sana" de referencia: independiente, con contexto
// acotado pero no trivial, y de una clase aislable. Cada caso de la tabla la
// modifica en un solo eje para aislar qué regla dispara.
func unidadDelegable() WorkUnit {
	return WorkUnit{
		ID:             "T004",
		Objective:      "Investigar si la limpieza por expiración compite con el refresco",
		Class:          ClassInvestigation,
		Scope:          Scope{Files: []string{"a.go", "b.go"}, ReadOnly: true},
		Complexity:     LevelMedium,
		ContextNeed:    ContextNeed{EstimatedTokens: 2200},
		ExpectedOutput: OutputSpec{MaxTokens: 900},
	}
}

func capacidadesPlenas() RuntimeCapabilities {
	return RuntimeCapabilities{Subagents: true, Parallel: true, IsolatedContext: true, MaxParallel: 3}
}

// presupuestoAmplio deja sitio de sobra para varias delegaciones.
func presupuestoAmplio() Budget { return NewBudget(60000, DefaultBudgetSplit()) }

// T015: el orden de evaluación es parte del contrato. Cada fila fuerza una regla
// concreta manteniendo el resto de la entrada sana, y comprueba que gana la
// regla esperada y no otra. Si alguien reordena las reglas, esta tabla lo caza.
func TestRouteTask_OrdenDeEvaluacion(t *testing.T) {
	casos := []struct {
		nombre     string
		mutar      func(*RouteInput)
		wantRuta   Route
		wantRazon  Reason
		wantBloque []string
	}{
		{
			nombre:    "1. la política del llamador desactiva la delegación",
			mutar:     func(in *RouteInput) { in.Policy.DelegationDisabled = true },
			wantRuta:  RouteInline,
			wantRazon: ReasonPolicyForcedInline,
		},
		{
			nombre: "2. dependencia sin resolver gana incluso sin subagentes",
			mutar: func(in *RouteInput) {
				in.Unit.Dependencies = []string{"T001", "T002"}
				in.Resolved = map[string]bool{"T001": true}
				in.Capabilities = RuntimeCapabilities{}
			},
			wantRuta:   RouteWait,
			wantRazon:  ReasonUnresolvedDependency,
			wantBloque: []string{"T002"},
		},
		{
			nombre:    "3. sin subagentes declarados",
			mutar:     func(in *RouteInput) { in.Capabilities = RuntimeCapabilities{} },
			wantRuta:  RouteInline,
			wantRazon: ReasonNoSubagents,
		},
		{
			nombre:    "4. profundidad máxima alcanzada",
			mutar:     func(in *RouteInput) { in.Depth = 1 },
			wantRuta:  RouteInline,
			wantRazon: ReasonDepthLimit,
		},
		{
			nombre:    "5. trabajo equivalente ya cubierto",
			mutar:     func(in *RouteInput) { in.DuplicateWork = true },
			wantRuta:  RouteInline,
			wantRazon: ReasonDuplicateWork,
		},
		{
			nombre:    "6. complejidad trivial",
			mutar:     func(in *RouteInput) { in.Unit.Complexity = LevelTrivial },
			wantRuta:  RouteInline,
			wantRazon: ReasonTrivial,
		},
		{
			nombre:    "7. requiere casi todo el contexto del padre",
			mutar:     func(in *RouteInput) { in.Unit.ContextNeed.NearlyFullParent = true },
			wantRuta:  RouteInline,
			wantRazon: ReasonContextNearlyFull,
		},
		{
			nombre:    "8. delegar cuesta igual o más que hacerlo inline",
			mutar:     func(in *RouteInput) { in.InlineCostTokens = 100 },
			wantRuta:  RouteInline,
			wantRazon: ReasonOverheadExceedsBenefit,
		},
		{
			nombre: "9. el fondo de delegación no cubre el costo",
			mutar: func(in *RouteInput) {
				in.Budget = Budget{TotalTokens: 5000, MainAgentMax: 4900, DelegationPoolMax: 100, ValidationReserve: 0}
			},
			wantRuta:  RouteInline,
			wantRazon: ReasonBudgetExhausted,
		},
		{
			nombre: "10. lo único que queda es la reserva y la unidad es opcional",
			mutar: func(in *RouteInput) {
				in.Unit.Optional = true
				in.Budget = Budget{TotalTokens: 5000, MainAgentMax: 2000, DelegationPoolMax: 1000, ValidationReserve: 2000, DelegationSpent: 1000}
			},
			wantRuta:  RouteInline,
			wantRazon: ReasonValidationReserveProtected,
		},
		{
			nombre: "11. tope de agentes del plan alcanzado",
			mutar: func(in *RouteInput) {
				in.DelegatedSoFar = DefaultMaxSubagentsPerPlan
			},
			wantRuta:  RouteInline,
			wantRazon: ReasonFanOutLimit,
		},
		{
			nombre:    "12. independiente, aislable y con beneficio",
			mutar:     func(in *RouteInput) {},
			wantRuta:  RouteDelegate,
			wantRazon: ReasonIsolatableInvestigation,
		},
		{
			nombre: "13. nada la hace buena candidata",
			mutar: func(in *RouteInput) {
				in.Unit.Class = ClassImplementation
				in.Unit.Scope = Scope{}
				in.Unit.Complexity = LevelMedium
			},
			wantRuta:  RouteInline,
			wantRazon: ReasonOverheadExceedsBenefit,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in := RouteInput{
				Unit:         unidadDelegable(),
				Capabilities: capacidadesPlenas(),
				Budget:       presupuestoAmplio(),
			}
			c.mutar(&in)

			got := RouteTask(in)

			if got.Route != c.wantRuta {
				t.Errorf("ruta = %q, esperaba %q (razón dada: %q)", got.Route, c.wantRuta, got.Reason)
			}
			if got.Reason != c.wantRazon {
				t.Errorf("razón = %q, esperaba %q", got.Reason, c.wantRazon)
			}
			if got.Reason.Text() == "" {
				t.Error("ninguna decisión puede salir con razón sin texto")
			}
			if got.WorkUnitID != in.Unit.ID {
				t.Errorf("WorkUnitID = %q, esperaba %q", got.WorkUnitID, in.Unit.ID)
			}
			if len(c.wantBloque) > 0 {
				if len(got.BlockedBy) != len(c.wantBloque) || got.BlockedBy[0] != c.wantBloque[0] {
					t.Errorf("BlockedBy = %v, esperaba %v", got.BlockedBy, c.wantBloque)
				}
			}
		})
	}
}

// T016 — AC-001: un cambio trivial se queda inline y lo explica.
func TestRouteTask_AC001_TareaTrivial(t *testing.T) {
	in := RouteInput{
		Unit: WorkUnit{
			ID: "T001", Objective: "Corregir una errata en un comentario",
			Class: ClassTrivial, Complexity: LevelTrivial,
			ContextNeed: ContextNeed{EstimatedTokens: 120},
		},
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
	}

	got := RouteTask(in)

	if got.Route != RouteInline {
		t.Fatalf("ruta = %q, esperaba INLINE", got.Route)
	}
	if got.Reason != ReasonTrivial {
		t.Errorf("razón = %q, esperaba %q", got.Reason, ReasonTrivial)
	}
}

// T016 — AC-003: sin capacidad de subagentes NUNCA se devuelve una ruta que
// exija ejecución delegada, ni siquiera con delegación forzada por política.
func TestRouteTask_AC003_SinSubagentes(t *testing.T) {
	for _, forzada := range []bool{false, true} {
		in := RouteInput{
			Unit:         unidadDelegable(),
			Capabilities: RuntimeCapabilities{},
			Budget:       presupuestoAmplio(),
			Policy:       PolicyOverrides{DelegationForced: forzada},
		}

		got := RouteTask(in)

		if got.Route.Delegada() {
			t.Errorf("delegación forzada=%v: sin subagentes la ruta no puede ser delegada, fue %q", forzada, got.Route)
		}
		if got.Reason != ReasonNoSubagents {
			t.Errorf("delegación forzada=%v: razón = %q, esperaba %q", forzada, got.Reason, ReasonNoSubagents)
		}
	}
}

// T017 — AC-002: la investigación independiente y aislable se delega, con
// presupuestos de contexto y de salida declarados y mayores que cero (FR-025).
func TestRouteTask_AC002_InvestigacionAislable(t *testing.T) {
	in := RouteInput{
		Unit:         unidadDelegable(),
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
	}

	got := RouteTask(in)

	if got.Route != RouteDelegate {
		t.Fatalf("ruta = %q, esperaba DELEGATE (razón: %q)", got.Route, got.Reason)
	}
	if got.ContextBudget <= 0 {
		t.Error("una ruta delegada debe traer presupuesto de contexto mayor que cero")
	}
	if got.OutputBudget <= 0 {
		t.Error("una ruta delegada debe traer presupuesto de salida mayor que cero")
	}
	if !got.Estimated {
		t.Error("sin reporte real del runtime las cifras deben marcarse como estimadas")
	}
	if got.EstimatedCost.Total() <= 0 {
		t.Error("el costo estimado debe desglosarse y sumar más que cero")
	}
}

// El presupuesto de contexto debe ser el MENOR con el que la tarea pueda
// completarse, no el que quepa en el fondo (FR-025): un fondo enorme no puede
// inflar el contexto de una tarea pequeña.
func TestRouteTask_PresupuestoDeContextoNoCreceConElFondo(t *testing.T) {
	base := RouteInput{Unit: unidadDelegable(), Capabilities: capacidadesPlenas(), Budget: NewBudget(20000, DefaultBudgetSplit())}
	amplio := base
	amplio.Budget = NewBudget(2000000, DefaultBudgetSplit())

	if RouteTask(base).ContextBudget != RouteTask(amplio).ContextBudget {
		t.Error("el presupuesto de contexto no debe depender del fondo global disponible")
	}
}

// T018: una dependencia sin resolver produce WAIT enumerando lo que falta, en
// orden estable y sin incluir lo ya resuelto.
func TestRouteTask_DependenciasSinResolver(t *testing.T) {
	u := unidadDelegable()
	u.Dependencies = []string{"T002", "T001", "T003"}

	got := RouteTask(RouteInput{
		Unit:         u,
		Resolved:     map[string]bool{"T001": true},
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
	})

	if got.Route != RouteWait {
		t.Fatalf("ruta = %q, esperaba WAIT", got.Route)
	}
	want := []string{"T002", "T003"}
	if len(got.BlockedBy) != len(want) {
		t.Fatalf("BlockedBy = %v, esperaba %v", got.BlockedBy, want)
	}
	for i := range want {
		if got.BlockedBy[i] != want[i] {
			t.Fatalf("BlockedBy = %v, esperaba %v (orden estable de Dependencies)", got.BlockedBy, want)
		}
	}
}

// T019 — SC-006: mismas entradas, misma salida. Cien veces.
func TestRouteTask_Reproducible(t *testing.T) {
	in := RouteInput{
		Unit:         unidadDelegable(),
		Resolved:     map[string]bool{"T001": true, "T002": true, "T003": true},
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
	}

	primera, err := json.Marshal(RouteTask(in))
	if err != nil {
		t.Fatalf("serializar: %v", err)
	}
	for i := 0; i < 100; i++ {
		otra, err := json.Marshal(RouteTask(in))
		if err != nil {
			t.Fatalf("serializar en la corrida %d: %v", i, err)
		}
		if string(otra) != string(primera) {
			t.Fatalf("corrida %d difiere:\n%s\n%s", i, primera, otra)
		}
	}
}

// T020 — AC-015: sin evidencia histórica la política decide igual. El arranque
// en frío no es un caso degradado, es el caso normal el primer día.
func TestRouteTask_ArranqueEnFrio(t *testing.T) {
	in := RouteInput{
		Unit:         unidadDelegable(),
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
		Evidence:     nil,
	}

	got := RouteTask(in)

	if got.Route == "" {
		t.Fatal("sin historial la decisión no puede quedar vacía")
	}
	if got.Reason.Text() == "" {
		t.Error("sin historial la decisión debe seguir explicándose")
	}
}

// El recargo por falta de aislamiento encarece la delegación pero no la
// prohíbe (FR-036).
func TestRouteTask_SinAislamientoEncareceLaDelegacion(t *testing.T) {
	conAislamiento := RouteInput{Unit: unidadDelegable(), Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio()}

	sinAislamiento := conAislamiento
	caps := capacidadesPlenas()
	caps.IsolatedContext = false
	sinAislamiento.Capabilities = caps

	a := RouteTask(conAislamiento).EstimatedCost.Total()
	b := RouteTask(sinAislamiento).EstimatedCost.Total()

	if b <= a {
		t.Errorf("sin aislamiento el costo estimado debería ser mayor: %d vs %d", b, a)
	}
}

// Delegación forzada bajo presupuesto insuficiente da REJECT, no INLINE: el
// llamador dijo que esa unidad EXIGE delegación, así que la respuesta honesta es
// "esta delegación no debe ocurrir", no "la hago yo".
func TestRouteTask_ForzadaSinPresupuestoDaReject(t *testing.T) {
	in := RouteInput{
		Unit:         unidadDelegable(),
		Capabilities: capacidadesPlenas(),
		Budget:       Budget{TotalTokens: 5000, MainAgentMax: 4900, DelegationPoolMax: 100},
		Policy:       PolicyOverrides{DelegationForced: true},
	}

	got := RouteTask(in)

	if got.Route != RouteReject {
		t.Errorf("ruta = %q, esperaba REJECT", got.Route)
	}
	if got.Reason != ReasonBudgetExhausted {
		t.Errorf("razón = %q, esperaba %q", got.Reason, ReasonBudgetExhausted)
	}
}

// Sin presupuesto declarado la política funciona igual (INV-AAR-016).
func TestRouteTask_SinPresupuestoDeclarado(t *testing.T) {
	got := RouteTask(RouteInput{
		Unit:         unidadDelegable(),
		Capabilities: capacidadesPlenas(),
		Budget:       Budget{},
	})

	if got.Route != RouteDelegate {
		t.Errorf("ruta = %q, esperaba DELEGATE: la ausencia de presupuesto no es una restricción", got.Route)
	}
}

// --- Historia 8: la evidencia histórica mueve el desempate, nada más ---

// evidenciaFavorable describe un patrón que sale más barato delegado.
func evidenciaFavorable(clase TaskClass) *ClassEvidence {
	return &ClassEvidence{
		Class: clase, Executions: 34,
		InlineAvgTokens: 6200, DelegatedAvgTokens: 3100,
		DelegatedAvgContextTokens: 1800, SuccessRate: 0.94,
	}
}

// T093 — AC-014: con evidencia favorable, un caso que por sí solo no llegaba a
// delegarse puede pasar a hacerlo.
func TestRouteTask_AC014_LaEvidenciaMueveElDesempate(t *testing.T) {
	// Unidad deliberadamente sosa: ni clase aislable ni alcance acotado, así que
	// sin evidencia cae en la regla 13.
	u := WorkUnit{
		ID: "T010", Objective: "tarea de un patrón repetido",
		Class: ClassMigration, Complexity: LevelMedium,
		ContextNeed: ContextNeed{EstimatedTokens: 3000},
	}
	base := RouteInput{Unit: u, Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio()}

	if got := RouteTask(base); got.Route.Delegada() {
		t.Fatalf("preparación: sin evidencia esta unidad no debería delegarse (%q)", got.Reason)
	}

	conEvidencia := base
	conEvidencia.Evidence = evidenciaFavorable(ClassMigration)

	got := RouteTask(conEvidencia)
	if !got.Route.Delegada() {
		t.Errorf("con evidencia favorable debería delegarse: %q / %q", got.Route, got.Reason)
	}
	if got.Reason != ReasonHistoricalEvidence {
		t.Errorf("razón = %q, esperaba %q", got.Reason, ReasonHistoricalEvidence)
	}
}

// T094: la evidencia es ASESORA. Nunca salta una restricción dura, y se prueba
// regla a regla para que ninguna quede sin cubrir por descuido.
func TestRouteTask_LaEvidenciaNoSaltaRestriccionesDuras(t *testing.T) {
	casos := []struct {
		nombre    string
		mutar     func(*RouteInput)
		wantRazon Reason
	}{
		{"política del llamador", func(in *RouteInput) { in.Policy.DelegationDisabled = true }, ReasonPolicyForcedInline},
		{"dependencias sin resolver", func(in *RouteInput) {
			in.Unit.Dependencies = []string{"T001"}
		}, ReasonUnresolvedDependency},
		{"sin subagentes", func(in *RouteInput) { in.Capabilities = RuntimeCapabilities{} }, ReasonNoSubagents},
		{"profundidad agotada", func(in *RouteInput) { in.Depth = 1 }, ReasonDepthLimit},
		{"trabajo duplicado", func(in *RouteInput) { in.DuplicateWork = true }, ReasonDuplicateWork},
		{"complejidad trivial", func(in *RouteInput) { in.Unit.Complexity = LevelTrivial }, ReasonTrivial},
		{"contexto casi completo", func(in *RouteInput) { in.Unit.ContextNeed.NearlyFullParent = true }, ReasonContextNearlyFull},
		{"presupuesto agotado", func(in *RouteInput) {
			in.Budget = Budget{TotalTokens: 5000, MainAgentMax: 4900, DelegationPoolMax: 100}
		}, ReasonBudgetExhausted},
		{"reserva protegida", func(in *RouteInput) {
			in.Unit.Optional = true
			in.Budget = Budget{TotalTokens: 5000, MainAgentMax: 2000, DelegationPoolMax: 1000, DelegationSpent: 1000, ValidationReserve: 2000}
		}, ReasonValidationReserveProtected},
		{"tope de agentes", func(in *RouteInput) { in.DelegatedSoFar = DefaultMaxSubagentsPerPlan }, ReasonFanOutLimit},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in := RouteInput{
				Unit:         WorkUnit{ID: "T010", Objective: "algo", Class: ClassMigration, Complexity: LevelMedium, ContextNeed: ContextNeed{EstimatedTokens: 3000}},
				Capabilities: capacidadesPlenas(),
				Budget:       presupuestoAmplio(),
				Evidence:     evidenciaFavorable(ClassMigration),
			}
			c.mutar(&in)

			got := RouteTask(in)

			if got.Route.Delegada() {
				t.Errorf("la evidencia no puede saltarse esta restricción: ruta = %q", got.Route)
			}
			if got.Reason != c.wantRazon {
				t.Errorf("razón = %q, esperaba %q", got.Reason, c.wantRazon)
			}
		})
	}
}

// Evidencia insuficiente o mala no mueve nada: pocos datos son ruido, no señal.
func TestClassEvidence_Favorece(t *testing.T) {
	casos := []struct {
		nombre string
		e      *ClassEvidence
		want   bool
	}{
		{"sin historial (nil)", nil, false},
		{"pocas ejecuciones", &ClassEvidence{Executions: 2, InlineAvgTokens: 6000, DelegatedAvgTokens: 3000, SuccessRate: 1}, false},
		{"tasa de éxito baja", &ClassEvidence{Executions: 30, InlineAvgTokens: 6000, DelegatedAvgTokens: 3000, SuccessRate: 0.5}, false},
		{"delegado sale más caro", &ClassEvidence{Executions: 30, InlineAvgTokens: 3000, DelegatedAvgTokens: 6000, SuccessRate: 0.95}, false},
		{"sin cifras de inline", &ClassEvidence{Executions: 30, DelegatedAvgTokens: 3000, SuccessRate: 0.95}, false},
		{"favorable", &ClassEvidence{Executions: 30, InlineAvgTokens: 6000, DelegatedAvgTokens: 3000, SuccessRate: 0.95}, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.e.Favorece(); got != c.want {
				t.Errorf("Favorece = %v, esperaba %v", got, c.want)
			}
		})
	}
}

// --- Ramas del desempate que faltaban por fijar ---

// PreferInline no PROHÍBE delegar: sube el listón a lo inequívocamente
// aislable, que es lo que pide quien marca esa preferencia (FR-050).
func TestRouteTask_PreferInlineSubeElListon(t *testing.T) {
	casos := []struct {
		nombre    string
		mutar     func(*RouteInput)
		wantDeleg bool
	}{
		{
			"investigación aislable y acotada sigue delegándose",
			func(in *RouteInput) {},
			true,
		},
		{
			"investigación sin alcance acotado se queda inline",
			func(in *RouteInput) { in.Unit.Scope = Scope{} },
			false,
		},
		{
			"runtime sin aislamiento de contexto se queda inline",
			func(in *RouteInput) { in.Capabilities.IsolatedContext = false },
			false,
		},
		{
			"clase no aislable se queda inline",
			func(in *RouteInput) { in.Unit.Class = ClassImplementation },
			false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in := RouteInput{
				Unit:         unidadDelegable(),
				Capabilities: capacidadesPlenas(),
				Budget:       presupuestoAmplio(),
				Policy:       PolicyOverrides{PreferInline: true},
			}
			c.mutar(&in)

			if got := RouteTask(in); got.Route.Delegada() != c.wantDeleg {
				t.Errorf("ruta = %q (razón %q), esperaba delegada=%v", got.Route, got.Reason, c.wantDeleg)
			}
		})
	}
}

// La delegación forzada vence a PreferInline y al umbral de contexto mínimo,
// pero sigue sujeta a capacidades y seguridad (FR-051, ya cubierto en AC-003).
func TestRouteTask_ForzadaVenceAlDesempate(t *testing.T) {
	u := unidadDelegable()
	u.Class = ClassImplementation
	u.Scope = Scope{}
	u.ContextNeed.EstimatedTokens = 10 // por debajo del mínimo delegable

	got := RouteTask(RouteInput{
		Unit:         u,
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
		Policy:       PolicyOverrides{DelegationForced: true, PreferInline: true},
	})

	if !got.Route.Delegada() {
		t.Errorf("la delegación forzada debería vencer al desempate: %q / %q", got.Route, got.Reason)
	}
}

// Una unidad con contexto minúsculo no amortiza el arranque del agente, por muy
// aislable que parezca su clase.
func TestRouteTask_ContextoMinusculoNoAmortizaElArranque(t *testing.T) {
	u := unidadDelegable()
	u.ContextNeed.EstimatedTokens = MinDelegableContextTokens - 1

	got := RouteTask(RouteInput{
		Unit: u, Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio(),
	})

	if got.Route.Delegada() {
		t.Errorf("ruta = %q: con %d tokens de contexto delegar no compensa",
			got.Route, u.ContextNeed.EstimatedTokens)
	}
}

// Alcance acotado + complejidad suficiente delega como interfaz acotada, aunque
// la clase no sea de las "aislables".
func TestRouteTask_InterfazAcotada(t *testing.T) {
	u := unidadDelegable()
	u.Class = ClassIntegration
	u.Scope = Scope{Files: []string{"cmd_mcp.go"}}

	got := RouteTask(RouteInput{
		Unit: u, Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio(),
	})

	if got.Reason != ReasonBoundedInterface {
		t.Errorf("razón = %q, esperaba %q", got.Reason, ReasonBoundedInterface)
	}
}

// Un alcance con demasiados archivos deja de ser acotado: el contexto necesario
// ya no es aislable y delegar duplicaría el del padre.
func TestRouteTask_AlcanceDemasiadoAmplioNoEsAcotado(t *testing.T) {
	u := unidadDelegable()
	u.Class = ClassIntegration
	var muchos []string
	for i := 0; i <= MaxBoundedScopeFiles; i++ {
		muchos = append(muchos, idTarea(i)+".go")
	}
	u.Scope = Scope{Files: muchos}

	if got := RouteTask(RouteInput{Unit: u, Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio()}); got.Route.Delegada() {
		t.Errorf("ruta = %q: %d archivos no son un alcance acotado", got.Route, len(muchos))
	}
}

func TestRouteTask_ContextoNoMedidoNoEsUnVeto(t *testing.T) {
	u := unidadDelegable()
	u.ContextNeed.EstimatedTokens = 0
	got := RouteTask(RouteInput{Unit: u, Capabilities: capacidadesPlenas(), Budget: presupuestoAmplio()})
	if !got.Route.Delegada() {
		t.Fatalf("ruta = %q (%q); contexto no medido no debe equivaler a contexto minúsculo", got.Route, got.Reason)
	}
	if got.ContextBudget != MinDelegableContextTokens {
		t.Errorf("ContextBudget = %d, esperaba el mínimo seguro", got.ContextBudget)
	}
}

func TestEstimateDelegationCost_IncluyeRiesgoYRutaCritica(t *testing.T) {
	base := RouteInput{Unit: unidadDelegable()}.EstimateDelegationCost(capacidadesPlenas())
	u := unidadDelegable()
	u.Risk = LevelHigh
	u.CriticalPath = true
	got := (RouteInput{Unit: u}).EstimateDelegationCost(capacidadesPlenas())
	if got.CoordinationTokens != base.CoordinationTokens+HighRiskCoordinationTokens {
		t.Errorf("coordinación = %d", got.CoordinationTokens)
	}
	if got.IntegrationTokens != base.IntegrationTokens+CriticalPathIntegrationTokens {
		t.Errorf("integración = %d", got.IntegrationTokens)
	}
}
