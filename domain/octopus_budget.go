package domain

// --- Presupuesto jerárquico (feature 027) ---
//
// El reparto NO impone porcentajes: los de fábrica son un punto de partida
// configurable (FR-029). Lo que sí es innegociable es que la reserva de
// validación no se toque para delegación opcional (INV-AAR-006): quedarse sin
// presupuesto justo antes de verificar el trabajo es el fallo más caro que puede
// producir un enrutador.

// Porcentajes de fábrica del reparto de la sesión. Se declaran UNA sola vez,
// aquí, y se sobrescriben desde la configuración del proyecto — nunca se
// repiten en los sitios de uso.
const (
	DefaultMainAgentPct  = 55
	DefaultDelegationPct = 30
	DefaultValidationPct = 15
)

// BudgetSplit es el reparto porcentual de la sesión.
type BudgetSplit struct {
	MainAgentPct  int
	DelegationPct int
	ValidationPct int
}

func DefaultBudgetSplit() BudgetSplit {
	return BudgetSplit{
		MainAgentPct:  DefaultMainAgentPct,
		DelegationPct: DefaultDelegationPct,
		ValidationPct: DefaultValidationPct,
	}
}

// Valid exige que el reparto sume exactamente 100 y no tenga negativos. Un
// reparto que no suma 100 dejaría tokens sin asignar o prometería de más.
func (s BudgetSplit) Valid() bool {
	if s.MainAgentPct < 0 || s.DelegationPct < 0 || s.ValidationPct < 0 {
		return false
	}
	return s.MainAgentPct+s.DelegationPct+s.ValidationPct == 100
}

// Budget es el estado del presupuesto de la sesión.
type Budget struct {
	TotalTokens       int
	MainAgentMax      int
	DelegationPoolMax int
	ValidationReserve int
	DelegationSpent   int
}

// NewBudget reparte un total según el porcentaje indicado. Un reparto inválido
// cae al de fábrica en vez de producir un presupuesto silenciosamente roto:
// fallar rápido y de forma visible, no a mitad de un plan.
func NewBudget(total int, split BudgetSplit) Budget {
	if !split.Valid() {
		split = DefaultBudgetSplit()
	}
	if total <= 0 {
		return Budget{}
	}
	return Budget{
		TotalTokens:       total,
		MainAgentMax:      total * split.MainAgentPct / 100,
		DelegationPoolMax: total * split.DelegationPct / 100,
		ValidationReserve: total * split.ValidationPct / 100,
	}
}

// Declarado distingue "sin presupuesto" de "presupuesto agotado". Sin
// presupuesto declarado la política funciona igual y omite esta validación
// (INV-AAR-016): no tener conteo exacto no puede impedir enrutar.
func (b Budget) Declarado() bool { return b.TotalTokens > 0 }

// DelegationRemaining es lo que queda del fondo de delegación, nunca negativo.
func (b Budget) DelegationRemaining() int {
	r := b.DelegationPoolMax - b.DelegationSpent
	if r < 0 {
		return 0
	}
	return r
}

// Cabe responde si un costo estimado entra en el fondo de delegación. Repara en
// que NO consulta la reserva de validación ni el fondo del agente principal: el
// fondo de delegación es el único del que puede tirar una delegación, y por eso
// agotar el fondo protege la reserva sin ningún caso especial (AC-008).
func (b Budget) Cabe(costo int) bool {
	if !b.Declarado() {
		return true
	}
	return costo <= b.DelegationRemaining()
}

// Gastar devuelve una copia con el consumo aplicado. Budget es un valor: nadie
// muta el presupuesto de otro por referencia.
func (b Budget) Gastar(costo int) Budget {
	if costo <= 0 {
		return b
	}
	// La reserva solo puede consumirse DESPUÉS del fondo ordinario. Mantener
	// DelegationSpent acotado hace que Valid siga expresando el contrato y que
	// la salida no pueda afirmar falsamente que la reserva permanece intacta.
	ordinario := b.DelegationRemaining()
	if costo <= ordinario {
		b.DelegationSpent += costo
		return b
	}
	b.DelegationSpent = b.DelegationPoolMax
	b.ValidationReserve -= costo - ordinario
	if b.ValidationReserve < 0 {
		b.ValidationReserve = 0
	}
	return b
}

// Valid comprueba los invariantes del reparto: las tres bolsas no pueden
// prometer más que el total, y el gasto no puede superar su fondo (INV-AAR-005).
func (b Budget) Valid() bool {
	if !b.Declarado() {
		return true
	}
	if b.MainAgentMax+b.DelegationPoolMax+b.ValidationReserve > b.TotalTokens {
		return false
	}
	return b.DelegationSpent <= b.DelegationPoolMax && b.ValidationReserve >= 0
}

// CostEstimate desglosa el costo estimado de delegar. Se desglosa en vez de
// llevar un solo número porque la especificación exige explicar de qué se compone
// (FR-032) y porque el desglose es lo que permite contrastarlo después contra el
// consumo real que informa el runtime.
type CostEstimate struct {
	ContextTokens      int
	ContractTokens     int
	OutputTokens       int
	CoordinationTokens int
	IntegrationTokens  int
}

func (c CostEstimate) Total() int {
	return c.ContextTokens + c.ContractTokens + c.OutputTokens +
		c.CoordinationTokens + c.IntegrationTokens
}
