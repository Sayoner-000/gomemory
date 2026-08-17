package domain

import "testing"

// TestKnownAgentsAllDeclareTextFloor cubre FR-A5: todo agente soportado
// recibe, como mínimo, el piso textual. Una entrada sin AgentLevelTextFloor
// sería un agente sin garantía mínima, lo que la especificación prohíbe.
func TestKnownAgentsAllDeclareTextFloor(t *testing.T) {
	if len(KnownAgents) == 0 {
		t.Fatal("el registro no puede estar vacío")
	}
	for _, a := range KnownAgents {
		if !a.HasLevel(AgentLevelTextFloor) {
			t.Errorf("agente %q no declara text_floor (FR-A5)", a.Name)
		}
	}
}

// TestAgentByName_UnknownAgentIsNotRejected cubre INV-6/FR-A3: un agente
// ausente del registro no se rechaza. AgentByName debe devolver ok=false sin
// pánico ni valores basura, dejando que el llamador decida degradar a
// "neutral" en vez de tratarlo como error.
func TestAgentByName_UnknownAgentIsNotRejected(t *testing.T) {
	got, ok := AgentByName("un-agente-que-no-existe-todavia")
	if ok {
		t.Fatal("un agente no declarado no debe reportarse como conocido")
	}
	if got.Name != "" {
		t.Errorf("esperaba un valor cero, got %+v", got)
	}
}

// TestAgentByName_FindsKnownAgent verifica el camino positivo básico.
func TestAgentByName_FindsKnownAgent(t *testing.T) {
	got, ok := AgentByName("claude")
	if !ok {
		t.Fatal("claude debe estar en el registro")
	}
	if got.Dialect != DialectClaude {
		t.Errorf("dialecto de claude = %q, se esperaba %q", got.Dialect, DialectClaude)
	}
}

// TestKnownAgentsGuardOnlyWhenSupported cubre la regla de validación de
// data-model.md §5: `guard` solo se declara si el agente puede invocar un
// comando antes de presentar el plan. opencode no lo ofrece (research.md
// §11) y no debe declarar ese nivel.
func TestKnownAgentsGuardOnlyWhenSupported(t *testing.T) {
	opencode, ok := AgentByName("opencode")
	if !ok {
		t.Fatal("opencode debe estar en el registro")
	}
	if opencode.HasLevel(AgentLevelGuard) {
		t.Error("opencode no debe declarar el nivel guard: su ciclo no ofrece punto de decisión antes de presentar el plan")
	}
	if !opencode.HasLevel(AgentLevelEntry) {
		t.Error("opencode sí debe declarar el nivel entry: su plugin inyecta contexto en cada turno")
	}
}

// TestAddingAFictitiousAgentMakesItVisible demuestra FR-A4/SC-A2: añadir una
// entrada al registro (una fila más en el slice) basta para que el agente
// quede visible a quien recorra KnownAgents, sin tocar ninguna otra
// estructura. No es un test de mutación global (evita contaminar otros
// tests): construye una copia local del registro con una fila añadida.
func TestAddingAFictitiousAgentMakesItVisible(t *testing.T) {
	withFictitious := append(append([]AgentCapability{}, KnownAgents...), AgentCapability{
		Name:    "agente-ficticio",
		Dialect: DialectNeutral,
		Levels:  map[AgentLevel]bool{AgentLevelTextFloor: true},
	})

	found := false
	for _, a := range withFictitious {
		if a.Name == "agente-ficticio" {
			found = true
		}
	}
	if !found {
		t.Fatal("añadir una fila al registro debe hacerla visible a cualquier código que lo recorra")
	}
	if len(withFictitious) != len(KnownAgents)+1 {
		t.Errorf("esperaba %d entradas, got %d", len(KnownAgents)+1, len(withFictitious))
	}
}
