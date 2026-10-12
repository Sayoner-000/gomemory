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
func TestRecordatorio_SigueLaMismaPoliticaCanonica(t *testing.T) {
	if !strings.Contains(memoryProtocolReminder, domain.CodeRetrievalPolicy) {
		t.Errorf("el recordatorio debe contener el fragmento canónico CodeRetrievalPolicy")
	}
	if strings.Contains(memoryProtocolReminder, "úsalo SIEMPRE para exploración de código en vez de leer archivos a mano") {
		t.Errorf("el recordatorio no debe exigir el proveedor externo incondicionalmente")
	}
}
