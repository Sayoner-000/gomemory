package usecases

import (
	"strings"
	"testing"

	"mem/domain"
)

// contadorFalso mide 1 token por carácter: hace las cifras predecibles a mano
// sin depender de la heurística concreta del adaptador real.
type contadorFalso struct{ llamadas int }

func (c *contadorFalso) Count(text string) int {
	c.llamadas++
	return len([]rune(text))
}

// El reparto de responsabilidades es el punto de esta prueba: el caso de uso
// MIDE (con el puerto TokenCounter) y el dominio DECIDE. Invertirlo crearía un
// ciclo de imports domain → ports → domain y destruiría la pureza que hace
// verificable a la política.
func TestRouteTaskUseCase_MideYDelegaLaDecision(t *testing.T) {
	contador := &contadorFalso{}
	uc := NewRouteTaskUseCase(contador)

	req := RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID:         "T004",
			Objective:  "Investigar la condición de carrera en la expiración",
			Class:      domain.ClassInvestigation,
			Scope:      domain.Scope{Files: []string{"a.go", "b.go"}, ReadOnly: true},
			Complexity: domain.LevelMedium,
		},
		ContextMaterial: "cuatro mil caracteres de contexto...",
		Capabilities:    domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
		Budget:          domain.NewBudget(60000, domain.DefaultBudgetSplit()),
	}

	got, err := uc.Route(req)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if contador.llamadas == 0 {
		t.Error("el caso de uso debe medir con el TokenCounter, no adivinar")
	}
	if got.EstimatedCost.ContextTokens == 0 {
		t.Error("el contexto medido debe llegar al costo estimado")
	}
	if got.Reason.Text() == "" {
		t.Error("la decisión debe salir explicada")
	}
}

// Una unidad inválida se rechaza en el borde, antes de decidir nada: fallar
// rápido, no producir una decisión sobre basura.
func TestRouteTaskUseCase_ValidaEnElBorde(t *testing.T) {
	uc := NewRouteTaskUseCase(&contadorFalso{})

	_, err := uc.Route(RouteTaskRequest{Unit: domain.WorkUnit{ID: "T001"}})

	if err == nil {
		t.Fatal("una unidad sin objetivo debería rechazarse antes de enrutar")
	}
}

// Sin contador inyectado el caso de uso sigue funcionando: la medición es una
// mejora de la estimación, no un requisito (INV-AAR-016).
func TestRouteTaskUseCase_SinContadorSigueDecidiendo(t *testing.T) {
	uc := NewRouteTaskUseCase(nil)

	got, err := uc.Route(RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID: "T001", Objective: "algo", Complexity: domain.LevelMedium,
			ContextNeed: domain.ContextNeed{EstimatedTokens: 3000},
		},
		Capabilities: domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.Route == "" {
		t.Error("sin contador la decisión no puede quedar vacía")
	}
}

// Las cifras que el llamador ya trae medidas no se pisan: si ContextNeed viene
// poblado y no hay material que medir, se respeta.
func TestRouteTaskUseCase_RespetaCifrasYaMedidas(t *testing.T) {
	uc := NewRouteTaskUseCase(&contadorFalso{})

	got, err := uc.Route(RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID: "T001", Objective: "algo", Complexity: domain.LevelMedium,
			Class:       domain.ClassInvestigation,
			ContextNeed: domain.ContextNeed{EstimatedTokens: 4321},
		},
		Capabilities: domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.EstimatedCost.ContextTokens != 4321 {
		t.Errorf("ContextTokens = %d, esperaba respetar el 4321 ya medido", got.EstimatedCost.ContextTokens)
	}
}

// C-001 (ACR 027, reintento): ContractTokens quedaba permanentemente en 0
// porque ningún adaptador de CLI/MCP arma ContractMaterial. Prueba el
// fallback, no un ContractMaterial inyectado a mano: sin él, buildInput debe
// medir un proxy derivado de la propia unidad (objetivo + alcance).
func TestRouteTaskUseCase_ContractTokensSinMaterialExplicito(t *testing.T) {
	uc := NewRouteTaskUseCase(&contadorFalso{})

	got, err := uc.Route(RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "Investigar la condición de carrera en la expiración",
			Class: domain.ClassInvestigation, Complexity: domain.LevelMedium,
			Scope: domain.Scope{Files: []string{"expirer.go", "store.go"}, ReadOnly: true},
		},
		ContextMaterial: "algo de contexto",
		Capabilities:    domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.EstimatedCost.ContractTokens == 0 {
		t.Error("sin ContractMaterial explícito, ContractTokens debe salir del proxy (objetivo+alcance), no quedar en 0")
	}
}

// Hallazgo real de un re-juicio posterior: buildInput recalculaba
// InlineCostTokens/ContractTokens incondicionalmente desde el material de
// texto, descartando en silencio cualquier cifra que el llamador ya hubiera
// puesto directamente en WorkUnit (el mismo criterio que ya se respetaba para
// ContextNeed.EstimatedTokens). Un llamador que mida el costo inline por su
// cuenta, o que reutilice el mismo WorkUnit entre route_task y route_plan,
// no puede perder esa cifra.
func TestRouteTaskUseCase_RespetaContractTokensYaPuesto(t *testing.T) {
	uc := NewRouteTaskUseCase(&contadorFalso{})

	got, err := uc.Route(RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "algo", Complexity: domain.LevelMedium,
			ContractTokens: 777,
		},
		ContextMaterial: "contexto",
		Capabilities:    domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.EstimatedCost.ContractTokens != 777 {
		t.Errorf("ContractTokens = %d, esperaba respetar el 777 ya puesto en WorkUnit, no recalcularlo", got.EstimatedCost.ContractTokens)
	}
}

// InlineCostTokens no se expone en RouteDecision, así que se verifica por su
// EFECTO en la regla 8: con InlineCostTokens ya puesto y pequeño frente al
// costo de delegar, la unidad debe quedar INLINE por esa regla — antes de la
// corrección, buildInput lo recalculaba desde InlineMaterial vacío (0), la
// regla 8 quedaba omitida, y una unidad claramente aislable con contexto
// suficiente terminaba DELEGATE en su lugar.
func TestRouteTaskUseCase_RespetaInlineCostTokensYaPuesto(t *testing.T) {
	uc := NewRouteTaskUseCase(&contadorFalso{})

	got, err := uc.Route(RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "investigar", Class: domain.ClassInvestigation,
			Complexity:       domain.LevelMedium,
			Scope:            domain.Scope{Files: []string{"a.go"}, ReadOnly: true},
			InlineCostTokens: 100,
		},
		// >=500 caracteres para superar MinDelegableContextTokens y que, SIN la
		// regla 8, la regla 12 delegaría por ser aislable.
		ContextMaterial: strings.Repeat("contexto real de la investigación ", 20),
		Capabilities:    domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.Route != domain.RouteInline || got.Reason != domain.ReasonOverheadExceedsBenefit {
		t.Errorf("Route=%q Reason=%q, esperaba INLINE por overhead_exceeds_benefit (regla 8) con InlineCostTokens=100 ya puesto", got.Route, got.Reason)
	}
}

// Un ContractMaterial explícito del llamador siempre gana al proxy.
func TestRouteTaskUseCase_ContractMaterialExplicitoGanaAlProxy(t *testing.T) {
	uc := NewRouteTaskUseCase(&contadorFalso{})

	got, err := uc.Route(RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "algo", Complexity: domain.LevelMedium,
		},
		ContextMaterial:  "contexto",
		ContractMaterial: "1234567890", // 10 caracteres exactos con contadorFalso
		Capabilities:     domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.EstimatedCost.ContractTokens != 10 {
		t.Errorf("ContractTokens = %d, esperaba respetar el ContractMaterial explícito (10)", got.EstimatedCost.ContractTokens)
	}
}
