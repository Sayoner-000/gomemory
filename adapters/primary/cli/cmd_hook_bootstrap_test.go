package cli

import (
	"reflect"
	"regexp"
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
	got := octopusDelegationPolicy(true, "opencode")
	if !strings.Contains(got, "gomemory_octopus_route_task") {
		t.Errorf("la política debe usar el nombre de tool del runtime: %q", got)
	}
}

// TestOctopusDelegationPolicy_NombraSoloClavesDelSchema: la política en prosa
// ("con el objetivo, … presupuesto/contexto") hacía que el agente tradujera
// las palabras a claves inventadas (`goal`, `budget_context`) y la tool las
// rechazaba por faltar `objective`. La política debe listar las claves
// literales del schema real, y cada una debe existir en él.
func TestOctopusDelegationPolicy_NombraSoloClavesDelSchema(t *testing.T) {
	got := octopusDelegationPolicy(true, "")

	var linea string
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "claves exactas") {
			linea = l
			break
		}
	}
	if linea == "" {
		t.Fatalf("la política debe listar las claves exactas del schema en una línea: %q", got)
	}

	schema := map[string]bool{}
	tipo := reflect.TypeOf(octopusRouteTaskInput{})
	for i := 0; i < tipo.NumField(); i++ {
		clave, _, _ := strings.Cut(tipo.Field(i).Tag.Get("json"), ",")
		schema[clave] = true
	}

	nombradas := map[string]bool{}
	for _, m := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(linea, -1) {
		nombradas[m[1]] = true
		if !schema[m[1]] {
			t.Errorf("la política nombra %q, que no existe en el schema de octopus_route_task", m[1])
		}
	}
	for _, obligatoria := range []string{"objective", "subagents"} {
		if !nombradas[obligatoria] {
			t.Errorf("la política debe nombrar la clave %q", obligatoria)
		}
	}
	if strings.Contains(got, "presupuesto/contexto") {
		t.Error("la política no debe describir parámetros en prosa ambigua (presupuesto/contexto)")
	}
}

// TestOctopusRouteToolFor_PorRuntime: OpenCode 2.x expone las tools MCP en
// modo código, dentro de `execute` como tools.<servidor>.<tool>; el nombre
// plano de 1.x da "Unknown tool".
func TestOctopusRouteToolFor_PorRuntime(t *testing.T) {
	casos := []struct {
		runtime  string
		quiere   []string
		noQuiere string
	}{
		{"", []string{"mcp__gomemory__octopus_route_task"}, "tools.gomemory"},
		{"opencode", []string{"gomemory_octopus_route_task"}, "tools.gomemory"},
		{"opencode-v2", []string{"tools.gomemory.octopus_route_task", "execute"}, "gomemory_octopus_route_task"},
	}
	for _, c := range casos {
		got := octopusRouteToolFor(c.runtime)
		for _, q := range c.quiere {
			if !strings.Contains(got, q) {
				t.Errorf("runtime %q: %q debe contener %q", c.runtime, got, q)
			}
		}
		if strings.Contains(got, c.noQuiere) {
			t.Errorf("runtime %q: %q no debe contener %q", c.runtime, got, c.noQuiere)
		}
	}
}

// TestOctopusDelegationPolicy_V2ExplicaLaToolSubagent: en OpenCode 2.x la tool
// `subagent` exige `agent`; sin decirlo, el agente la llamaba sin esa clave.
// La pista solo aplica a 2.x: en otros runtimes sería ruido.
func TestOctopusDelegationPolicy_V2ExplicaLaToolSubagent(t *testing.T) {
	v2 := octopusDelegationPolicy(true, "opencode-v2")
	if !strings.Contains(v2, "`subagent`") || !strings.Contains(v2, "`agent`") {
		t.Errorf("en OpenCode 2.x la política debe indicar que `subagent` exige `agent`: %q", v2)
	}
	for _, rt := range []string{"", "opencode"} {
		if strings.Contains(octopusDelegationPolicy(true, rt), "`subagent`") {
			t.Errorf("runtime %q: la pista de `subagent` solo aplica a OpenCode 2.x", rt)
		}
	}
}
