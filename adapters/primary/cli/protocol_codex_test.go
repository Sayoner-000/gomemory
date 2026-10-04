package cli

import (
	"strings"
	"testing"
)

// T076 (feature 035, FR-026): donde el host no permite que gomemory comprima
// las salidas de herramientas externas (Codex), el protocolo orienta a explorar
// código con las tools de gomemory, que comprimen en origen. El bloque es uno
// solo para todos los agentes: la orientación es neutral y nombra a Codex como
// ejemplo, no como regla exclusiva.
func TestIntegrationBlock_OrientaALaCompresionEnOrigen(t *testing.T) {
	block := buildIntegrationBlock()
	for _, want := range []string{"comprimen en origen", "`search_code`", "`get_symbol`", "Codex"} {
		if !strings.Contains(block, want) {
			t.Errorf("el protocolo debe contener %q", want)
		}
	}
}
