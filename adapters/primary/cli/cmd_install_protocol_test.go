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

// TestProtocolVersionMarker_SubioAV7 verifica el mecanismo de actualización: al
// subir el número de versión, composeAgentFile reemplaza el bloque anterior
// completo sin dejar restos (FR-030) y sin necesidad de escribir migración.
// Subió a v7 al añadir la guía del grafo de código externo a
// buildIntegrationBlock(): cambiar el contenido sin subir el marcador dejaría a
// los proyectos ya instalados en v6 (este mismo repo, entre otros) sin forma de
// detectar que hay una versión nueva al reinstalar.
func TestProtocolVersionMarker_SubioAV7(t *testing.T) {
	if integrationVersionMarker != "<!-- gomemory-protocol-v7 -->" {
		t.Errorf("integrationVersionMarker = %q, se esperaba la v7", integrationVersionMarker)
	}
}

// TestComposeAgentFile_ReemplazaV5SinDejarRestos es la prueba del camino de
// actualización real: un proyecto instalado con una versión antigua debe
// quedar con la nueva y sin rastro de la vieja.
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
	if strings.Count(out, "gomemory-protocol-v7") != 1 {
		t.Errorf("se esperaba exactamente un marcador v7, hay %d", strings.Count(out, "gomemory-protocol-v7"))
	}
	if !strings.Contains(out, "Texto propio del proyecto.") {
		t.Error("se perdió el contenido propio del proyecto")
	}
}

// TestComposeAgentFile_ReemplazaV6SinDejarRestos es el caso real de este mismo
// repositorio: CLAUDE.md/AGENTS.md quedaron en v6 (feature 013, modo plan) y
// deben poder subir a v7 (grafo de código externo) sin dejar restos, igual que
// v5→v6.
func TestComposeAgentFile_ReemplazaV6SinDejarRestos(t *testing.T) {
	previo := "# Instrucciones\n\nTexto propio del proyecto.\n\n" +
		"<!-- gomemory-protocol-v6 -->\n" +
		"## Memoria Persistente (`mem`) — Protocolo Activo\n\n" +
		"contenido de la v6, sin la guía del grafo externo\n"

	out, changed := composeAgentFile(previo, "", buildIntegrationBlock())

	if !changed {
		t.Fatal("composeAgentFile debía reportar cambios al subir de v6 a v7")
	}
	if strings.Contains(out, "gomemory-protocol-v6") {
		t.Error("quedaron restos del marcador v6 (FR-030)")
	}
	if strings.Contains(out, "contenido de la v6, sin la guía del grafo externo") {
		t.Error("quedó contenido de la versión anterior")
	}
	if !strings.Contains(out, "codebase-memory-mcp") {
		t.Error("la v7 debe incluir la guía del grafo de código externo")
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

func TestComposeAgentFile_InstalaYActualizaBaselineUniversal(t *testing.T) {
	const universalV1 = "<!-- gomemory-universal-agent-instructions-v1 -->\n# Universal\n<!-- gomemory-universal-agent-instructions-end -->"
	const universalV2 = "<!-- gomemory-universal-agent-instructions-v2 -->\n# Universal actualizado\n<!-- gomemory-universal-agent-instructions-end -->"

	base := "# Instrucciones\n\nTexto propio.\n"
	once, changed := composeAgentFile(base, universalV1, buildIntegrationBlock())
	if !changed || !strings.Contains(once, universalV1) {
		t.Fatal("debía instalar el baseline universal")
	}
	if strings.Index(once, universalV1) > strings.Index(once, integrationVersionMarker) {
		t.Error("el baseline universal debe preceder al protocolo de memoria")
	}

	updated, changed := composeAgentFile(once, universalV2, buildIntegrationBlock())
	if !changed || strings.Contains(updated, universalV1) || !strings.Contains(updated, universalV2) {
		t.Error("debía reemplazar solamente la versión gestionada del baseline")
	}
	if !strings.Contains(updated, "Texto propio.") {
		t.Error("la actualización no debe perder instrucciones ajenas")
	}
}
