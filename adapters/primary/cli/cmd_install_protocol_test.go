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

// TestProtocolVersionMarker_SubioAV8 fija la versión vigente del bloque.
// Subió a v7 al añadir la guía del grafo de código externo a
// buildIntegrationBlock(): cambiar el contenido sin subir el marcador dejaría a
// los proyectos ya instalados en v6 (este mismo repo, entre otros) sin forma de
// detectar que hay una versión nueva al reinstalar. Subió a v8 cuando el
// protocolo pasó de `./mem …` a `mem …` (feature 034, FR-005). Subió a v9 con
// la directiva condicional por capacidad (feature 038): al próximo bump,
// renombrar este test y su mensaje.
func TestProtocolVersionMarker_SubioAV9(t *testing.T) {
	if integrationVersionMarker != "<!-- gomemory-protocol-v12 -->" {
		t.Errorf("integrationVersionMarker = %q, se esperaba la v9 (feature 038)", integrationVersionMarker)
	}
}

// Claude trunca las instrucciones MCP hacia el carácter 3900: las reglas deben
// quedar antes de ese punto para no depender del resto del bloque.
func TestIntegrationBlock_ReglasSobrevivenAlTruncado(t *testing.T) {
	const corteObservado = 3800
	block := buildIntegrationBlock()
	if len(block) <= corteObservado {
		return
	}
	visible := block[:corteObservado]
	for _, regla := range []string{"### GUARDAR PROACTIVAMENTE", "### Juez imparcial", "### Privacidad", "### Al entrar en modo plan", "### Al inicio de cada sesión", "### Al cerrar la sesión", "### Consultar memoria", "Si el MCP no está disponible"} {
		if !strings.Contains(visible, regla) {
			t.Errorf("%q queda después del corte", regla)
		}
	}
}
