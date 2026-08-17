package domain

import "sort"

// ChannelArm identifica a qué brazo pertenece un canal de activación
// (data-model.md §3): el que gomemory administra, o el brazo extensor de
// grafo de código, que solo se observa (INV-1).
type ChannelArm string

const (
	ArmGomemory  ChannelArm = "gomemory"
	ArmCodegraph ChannelArm = "codegraph"
)

// ChannelKind es el tipo de canal inspeccionado (doctor-report.md).
type ChannelKind string

const (
	KindPlanEntry       ChannelKind = "plan_entry"
	KindPlanGuard       ChannelKind = "plan_guard"
	KindTurnReminder    ChannelKind = "turn_reminder"
	KindInstructions    ChannelKind = "instructions"
	KindNativeWrapper   ChannelKind = "native_wrapper"
	KindMCPInstructions ChannelKind = "mcp_instructions"
)

// ChannelState es el estado de un canal concreto. StateNotApplicable está
// reservado a "agente no instalado" o "el agente no soporta este tipo de
// canal" — nunca se usa para ocultar un canal roto (FR-019).
// StateDuplicated es un estado propio, distinto de StateOK: es la regresión
// que produce una reinstalación con el filtro de idempotencia incompleto.
type ChannelState string

const (
	StateOK            ChannelState = "ok"
	StateOutdated      ChannelState = "outdated"
	StateDuplicated    ChannelState = "duplicated"
	StateMissing       ChannelState = "missing"
	StateNotApplicable ChannelState = "not_applicable"
)

// ActivationChannel describe una vía concreta por la que una garantía del
// modo plan llega a un agente (data-model.md §3). Los canales del brazo
// ArmCodegraph son de solo lectura: se inspeccionan y se reportan, nunca se
// escriben ni se corrigen (INV-1).
type ActivationChannel struct {
	Arm    ChannelArm
	Agent  string
	Scope  AgentScope
	Kind   ChannelKind
	State  ChannelState
	Detail string
}

// CoverageReport es el agregado que compone el caso de uso de diagnóstico
// (data-model.md §4). No se persiste: se calcula al invocarlo.
type CoverageReport struct {
	Channels     []ActivationChannel
	Degradations []string
}

// Problems cuenta los canales en un estado roto (outdated, duplicated o
// missing). Una degradación declarada NO cuenta como problema: es
// información, no fallo. Problems == 0 es la única condición de éxito en
// modo estricto.
func (r CoverageReport) Problems() int {
	n := 0
	for _, c := range r.Channels {
		switch c.State {
		case StateOutdated, StateDuplicated, StateMissing:
			n++
		}
	}
	return n
}

// SortChannels ordena channels por Arm, Agent, Scope, Kind — el orden
// determinista que exige doctor-report.md para que un script pueda comparar
// dos ejecuciones byte a byte.
func SortChannels(channels []ActivationChannel) {
	sort.Slice(channels, func(i, j int) bool {
		a, b := channels[i], channels[j]
		if a.Arm != b.Arm {
			return a.Arm < b.Arm
		}
		if a.Agent != b.Agent {
			return a.Agent < b.Agent
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		return a.Kind < b.Kind
	})
}
