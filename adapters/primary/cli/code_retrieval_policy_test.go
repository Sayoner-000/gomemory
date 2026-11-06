package cli

import (
	"strings"
	"testing"

	"mem/domain"
)

// T009 (feature 038, FR-001/FR-003): verificación de integración posterior al
// cambio. newMCPServer entrega buildIntegrationBlock() verbatim como
// Instructions de initialize (cmd_mcp.go), así que el contenido verificado
// aquí es el que recibe el cliente MCP: el fragmento canónico, una sola
// precedencia y sin exigencia externa incondicional.
func TestMCPInitialize_EntregaPoliticaCanonica(t *testing.T) {
	block := buildIntegrationBlock()
	if !strings.Contains(block, domain.CodeRetrievalPolicy) {
		t.Errorf("initialize debe entregar el fragmento canónico CodeRetrievalPolicy")
	}
	if strings.Count(block, domain.CodeRetrievalPolicy) != 1 {
		t.Errorf("el fragmento canónico debe aparecer exactamente una vez")
	}
	if strings.Contains(block, "úsalo SIEMPRE para exploración de código en vez de leer archivos a mano") {
		t.Errorf("initialize no debe exigir el proveedor externo incondicionalmente")
	}
}

// T013 (feature 038, FR-003/FR-004): los recordatorios de sesión comparten el
// fragmento canónico con el protocolo permanente: misma prioridad, excepción y
// alternativa; sin promesa automática ante capacidad desconocida; el
// descubrimiento externo sigue permitido como alternativa. En rojo hasta T014:
// memoryProtocolReminder aún exige el grafo externo incondicionalmente.
//
// HCD-040 (Q2): el recordatorio se repite en el primer prompt y al final de
// get_context y get_plan_context, así que lleva la versión corta canónica, que
// vive junto a la larga en domain. Las instrucciones MCP conservan la larga.
func TestRecordatorio_SigueLaMismaPoliticaCanonica(t *testing.T) {
	if !strings.Contains(memoryProtocolReminder, domain.CodeRetrievalPolicyShort) {
		t.Errorf("el recordatorio debe contener el fragmento canónico corto CodeRetrievalPolicyShort")
	}
	if strings.Contains(memoryProtocolReminder, domain.CodeRetrievalPolicy) {
		t.Errorf("el recordatorio no debe repetir la política larga: ya va en las instrucciones MCP")
	}
	if n, max := len(domain.CodeRetrievalPolicyShort), len(domain.CodeRetrievalPolicy)/3; n > max {
		t.Errorf("la versión corta mide %d bytes; quiero como mucho %d", n, max)
	}
	// Misma prioridad que la larga: grafo antes que lecturas, recuperación
	// comprimida en origen sin compresión externa, y sin prometer ahorro.
	for _, want := range []string{"grafo", "lectura", "search_code", "get_context", "compresión"} {
		if !strings.Contains(domain.CodeRetrievalPolicyShort, want) {
			t.Errorf("la versión corta debe conservar %q", want)
		}
	}
	if strings.Contains(memoryProtocolReminder, "úsalo SIEMPRE para exploración de código en vez de leer archivos a mano") {
		t.Errorf("el recordatorio no debe exigir el proveedor externo incondicionalmente")
	}
}
