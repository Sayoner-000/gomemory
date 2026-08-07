package cli

import (
	"strings"
	"testing"
)

// TestIntegrationBlock_DeclaraElDisparadorDeModoPlan cubre FR-001 y FR-002: el
// disparador viaja en el bloque de protocolo, que es el único canal que TODOS
// los agentes leen. Es lo que hace universal la cobertura sin escribir una
// integración por agente.
func TestIntegrationBlock_DeclaraElDisparadorDeModoPlan(t *testing.T) {
	b := buildIntegrationBlock()

	if !strings.Contains(b, "get_plan_context") {
		t.Error("el bloque debe nombrar la tool get_plan_context")
	}
	if !strings.Contains(b, "mem plan-context") {
		t.Error("el bloque debe ofrecer la alternativa de línea de comandos (FR-003)")
	}
	if !strings.Contains(strings.ToLower(b), "modo plan") {
		t.Error("el bloque debe nombrar el disparador de modo plan")
	}
}

// TestIntegrationBlock_SeccionDeModoPlanEsBreve protege la decisión D5: este
// bloque vive en el prompt de sistema de TODOS los turnos, no solo los de
// planificación. La feature 008 se hizo para reducir esa huella, y el método
// completo llega por la llamada, no por aquí.
func TestIntegrationBlock_SeccionDeModoPlanEsBreve(t *testing.T) {
	b := buildIntegrationBlock()

	idx := strings.Index(b, "### Al entrar en modo plan")
	if idx < 0 {
		t.Fatal("falta la sección de modo plan en el bloque de protocolo")
	}
	seccion := b[idx:]
	if fin := strings.Index(seccion[3:], "\n### "); fin >= 0 {
		seccion = seccion[:fin+3]
	}

	lineas := 0
	for _, l := range strings.Split(strings.TrimSpace(seccion), "\n") {
		if strings.TrimSpace(l) != "" {
			lineas++
		}
	}
	if lineas > 12 {
		t.Errorf("la sección de modo plan tiene %d líneas con contenido; se esperaban <= 12 para no inflar el prompt de sistema:\n%s", lineas, seccion)
	}
}

// TestProtocolVersionMarker_SubioAV6 verifica el mecanismo de actualización: al
// subir el número de versión, composeAgentFile reemplaza el bloque anterior
// completo sin dejar restos (FR-030) y sin necesidad de escribir migración.
func TestProtocolVersionMarker_SubioAV6(t *testing.T) {
	if integrationVersionMarker != "<!-- gomemory-protocol-v6 -->" {
		t.Errorf("integrationVersionMarker = %q, se esperaba la v6", integrationVersionMarker)
	}
}

// TestComposeAgentFile_ReemplazaV5SinDejarRestos es la prueba del camino de
// actualización real: un proyecto instalado con la versión anterior debe quedar
// con la nueva y sin rastro de la vieja.
func TestComposeAgentFile_ReemplazaV5SinDejarRestos(t *testing.T) {
	previo := "# Instrucciones\n\nTexto propio del proyecto.\n\n" +
		"<!-- gomemory-protocol-v5 -->\n" +
		"## Memoria Persistente (`mem`) — Protocolo Activo\n\n" +
		"contenido viejo del protocolo\n"

	out, changed := composeAgentFile(previo, "", buildIntegrationBlock())

	if !changed {
		t.Fatal("composeAgentFile debía reportar cambios al subir de versión")
	}
	if strings.Contains(out, "gomemory-protocol-v5") {
		t.Error("quedaron restos del marcador de la versión anterior (FR-030)")
	}
	if strings.Contains(out, "contenido viejo del protocolo") {
		t.Error("quedó contenido de la versión anterior")
	}
	if strings.Count(out, "gomemory-protocol-v6") != 1 {
		t.Errorf("se esperaba exactamente un marcador v6, hay %d", strings.Count(out, "gomemory-protocol-v6"))
	}
	if !strings.Contains(out, "Texto propio del proyecto.") {
		t.Error("se perdió el contenido propio del proyecto")
	}
}

// TestComposeAgentFile_EsIdempotente cubre FR-029: reinstalar sobre un archivo
// que ya tiene la versión vigente no debe modificarlo.
func TestComposeAgentFile_EsIdempotente(t *testing.T) {
	base := "# Instrucciones\n"
	once, _ := composeAgentFile(base, "", buildIntegrationBlock())

	twice, changed := composeAgentFile(once, "", buildIntegrationBlock())
	if changed {
		t.Error("una segunda composición no debía reportar cambios")
	}
	if once != twice {
		t.Error("la segunda composición alteró el contenido")
	}
}
