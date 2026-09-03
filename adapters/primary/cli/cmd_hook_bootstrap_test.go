package cli

import (
	"strings"
	"testing"

	"mem/domain"
)

// Este archivo cubre una corrección pedida explícitamente: el protocolo se
// declara "OBLIGATORIO y SIEMPRE ACTIVO — no esperes a que el usuario lo pida
// explícitamente" (buildIntegrationBlock), sin excepción por tipo de tarea
// (chat, plan, resumen). El bootstrap de ToolSearch debía cumplir eso también
// para el proveedor externo de grafo de código, y no lo hacía: solo forzaba
// las tools de gomemory, dejando el grafo externo a que el agente recordara
// por su cuenta un mensaje CRITICAL separado.

// TestBuildMemoryToolBootstrap_SinProveedorSoloGomemory es el caso base: sin
// el proveedor externo habilitado, el bootstrap no debe nombrar sus tools —
// nombrarlas igualmente sería ruido sin sentido si nunca va a haber servidor
// que las resuelva.
func TestBuildMemoryToolBootstrap_SinProveedorSoloGomemory(t *testing.T) {
	got := buildMemoryToolBootstrap(false, false)

	if strings.Contains(got, domain.CodebaseMemoryMCPPrefix) {
		t.Error("sin el proveedor habilitado, el bootstrap no debe nombrar sus tools")
	}
	for _, tool := range domain.MCPAllTools() {
		if !strings.Contains(got, "mcp__gomemory__"+tool) {
			t.Errorf("falta mcp__gomemory__%s en el bootstrap base", tool)
		}
	}
}

// TestBuildMemoryToolBootstrap_ConProveedorIncluyeDescubrimiento cubre la
// corrección: con el proveedor habilitado, sus 6 tools de descubrimiento
// (search_graph, trace_path, get_code_snippet, query_graph, get_architecture,
// search_code) deben quedar en el MISMO select:, y las de gomemory se
// conservan — una sola llamada de ToolSearch cubre ambos servidores.
func TestBuildMemoryToolBootstrap_ConProveedorIncluyeDescubrimiento(t *testing.T) {
	got := buildMemoryToolBootstrap(true, false)

	for _, tool := range domain.CodebaseMemoryMCPDiscoveryTools {
		prefijada := domain.CodebaseMemoryMCPPrefix + tool
		if !strings.Contains(got, prefijada) {
			t.Errorf("falta %q en el bootstrap con el proveedor habilitado", prefijada)
		}
	}
	for _, tool := range domain.MCPAllTools() {
		if !strings.Contains(got, "mcp__gomemory__"+tool) {
			t.Errorf("falta mcp__gomemory__%s: habilitar el proveedor no debe desplazar las tools de gomemory", tool)
		}
	}
	if strings.Count(got, "select:") != 1 {
		t.Error("debe ser un único select:, no dos ToolSearch separados")
	}
}

// TestBuildMemoryToolBootstrap_NoIncluyeOperacionesDeAdministracion protege el
// alcance deliberadamente acotado: forzar materialización no debe extenderse a
// las operaciones de escritura/administración de otro servidor.
func TestBuildMemoryToolBootstrap_NoIncluyeOperacionesDeAdministracion(t *testing.T) {
	got := buildMemoryToolBootstrap(true, false)

	for _, admin := range []string{"index_repository", "delete_project", "manage_adr", "ingest_traces"} {
		if strings.Contains(got, domain.CodebaseMemoryMCPPrefix+admin) {
			t.Errorf("el bootstrap no debe forzar la operación de administración %q", admin)
		}
	}
}

// TestMemoryToolBootstrap_CompatibleConTestDeContrato verifica que el export
// usado por mcp_tool_sync_test.go siga devolviendo el bootstrap SIN el
// proveedor externo — ese test compara contra domain.MCPAllTools() (las tools
// que el propio servidor gomemory registra), y el proveedor externo nunca
// aparecerá ahí porque es un servidor distinto.
func TestMemoryToolBootstrap_CompatibleConTestDeContrato(t *testing.T) {
	if MemoryToolBootstrap() != buildMemoryToolBootstrap(false, false) {
		t.Error("MemoryToolBootstrap() debe seguir siendo la variante sin proveedor externo")
	}
}

func TestOctopusDelegationPolicy_SoloApareceConElModuloEncendido(t *testing.T) {
	apagada := buildMemoryToolBootstrap(false, false)
	if strings.Contains(strings.ToLower(apagada), "octopus") {
		t.Errorf("con Octopus apagado el bootstrap no debe mencionarlo: %q", apagada)
	}

	encendida := buildMemoryToolBootstrap(false, true)
	for _, want := range []string{
		"OCTOPUS AAR — REGLA OBLIGATORIA DE DELEGACIÓN",
		"mcp__gomemory__octopus_route_task",
		"DELEGATE es la única autorización",
		"INLINE se ejecuta aquí",
		"WAIT espera las dependencias",
		"REJECT no se ejecuta",
	} {
		if !strings.Contains(encendida, want) {
			t.Errorf("la política habilitada debe incluir %q", want)
		}
	}
}

func TestOctopusDelegationPolicy_AdaptaElNombreDeToolPorRuntime(t *testing.T) {
	got := octopusDelegationPolicy(true, "gomemory_octopus_route_task")
	if !strings.Contains(got, "gomemory_octopus_route_task") {
		t.Errorf("la política debe usar el nombre de tool del runtime: %q", got)
	}
}
