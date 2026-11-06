package cli

import (
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
)

// HCD-036 C-02: el estado por turno de .memory/ es de cada conversación. La
// marca del bootstrap que deja un agente no la borra el arranque de otro.
func TestEstadoPorTurnoPorConversacion(t *testing.T) {
	root := t.TempDir()
	conv := &conversacionDeHook{}
	defer func() { stateConversation = nil }()
	stateConversation = conv
	deps := &Deps{ProjectRepo: persistence.NewProjectRepository(), Conversation: conv}

	conv.Adopt("agente-A")
	marcaA := sessionMarkerPath(deps, root)
	if err := writeHookMarker(marcaA); err != nil {
		t.Fatal(err)
	}
	conv.Adopt("agente-B")
	marcaB := sessionMarkerPath(deps, root)
	if marcaA == marcaB {
		t.Fatalf("cada conversación tiene su propia marca: %s", marcaA)
	}
	// El arranque de B borra SU estado por turno, no el de A.
	for _, p := range perTurnStatePaths(deps, root) {
		_ = os.Remove(p)
	}
	if _, err := os.Stat(marcaA); err != nil {
		t.Fatalf("C-02: la marca del bootstrap de A sobrevive al arranque de B: %v", err)
	}
	// Todas las rutas por turno, y el registro de la conversación, viven bajo
	// el directorio de la conversación.
	dirB := filepath.Dir(marcaB)
	for _, p := range append(perTurnStatePaths(deps, root), conversationPath(root)) {
		if filepath.Dir(p) != dirB {
			t.Errorf("%s no está en el directorio de la conversación %s", p, dirB)
		}
	}
	// Sin conversación, las rutas de siempre (C-05).
	stateConversation = nil
	if got := sessionMarkerPath(deps, root); got != filepath.Join(root, persistence.MemDir, ".session-tools-injected") {
		t.Errorf("sin conversación la ruta no cambia: %s", got)
	}
}
