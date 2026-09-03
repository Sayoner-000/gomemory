package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

// --- `mem octopus` (feature 027) ---
//
// La línea de comandos es una comodidad, no un requisito del protocolo (FR-053):
// la política funciona igual sin ella. Su valor es poder inspeccionar una
// decisión sin montar un cliente MCP.

// octopusApagadoMsg es lo único que responde el comando con el módulo apagado.
// Es deliberadamente accionable: decir "desactivado" sin decir cómo activarlo
// deja al usuario buscando.
const octopusApagadoMsg = `Octopus AAR está desactivado en este proyecto.

Actívalo desde la TUI: mem → Configuración → "Octopus AAR"

Es un módulo opt-in: mientras esté apagado, gomemory se comporta exactamente
igual que antes de esta funcionalidad.`

// CmdOctopus despacha `mem octopus <subcomando>`, mismo patrón que CmdPack.
func CmdOctopus(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("subcomando requerido: plan, route, status, usage, history\nEjemplo: mem octopus route \"Investigar la condición de carrera\"")
	}

	// La puerta del módulo va ANTES de mirar el subcomando: apagado significa
	// apagado para toda la superficie, sin excepciones por comando (FR-003).
	if !octopusHabilitado(deps) {
		fmt.Fprintln(os.Stderr, octopusApagadoMsg)
		os.Exit(1)
	}

	sub, subArgs := args[0], args[1:]
	switch sub {
	case "route":
		cmdOctopusRoute(deps, subArgs)
	case "plan":
		cmdOctopusPlan(deps, subArgs)
	case "status":
		cmdOctopusStatus(deps, subArgs)
	case "usage":
		cmdOctopusUsage(deps, subArgs)
	case "history":
		cmdOctopusHistory(deps, subArgs)
	default:
		fail("subcomando desconocido: %s (opciones: plan, route, status, usage, history)", sub)
	}
}

// octopusHabilitado es la única lectura del interruptor en el CLI. Un
// SettingsRepo nil (comandos que no lo cablean) cuenta como apagado: el default
// conservador vale también aquí.
func octopusHabilitado(deps *Deps) bool {
	if deps == nil || deps.SettingsRepo == nil {
		return false
	}
	return deps.SettingsRepo.Read(deps.Root).OctopusEnabled
}

// ParseOctopusRouteFlags parsea `mem octopus route`. Separada del comando para
// poder probarla sin que un error dispare os.Exit (mismo patrón que
// ParsePackBuildFlags).
func ParseOctopusRouteFlags(args []string) (usecases.RouteTaskRequest, bool, error) {
	// El objetivo es POSICIONAL y va primero. Se extrae ANTES de parsear porque
	// el paquete flag detiene el parseo en el primer argumento que no empieza
	// por "-": dejarlo dentro de args haría que todas las banderas posteriores
	// se colaran en el objetivo y conservaran su valor por defecto en silencio
	// — un fallo que ninguna prueba unitaria de la política puede ver, porque
	// la política recibiría una entrada perfectamente válida pero equivocada.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return usecases.RouteTaskRequest{}, false, fmt.Errorf("falta el objetivo de la unidad de trabajo (va primero: mem octopus route \"<objetivo>\" [banderas])")
	}
	objetivo := strings.TrimSpace(args[0])
	args = args[1:]

	fs := flag.NewFlagSet("octopus route", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	class := fs.String("class", "", "Clase de tarea (investigation, implementation, documentation...)")
	taskID := fs.String("id", "T001", "Identificador de la tarea para enlazar su reporte")
	deps := fs.String("deps", "", "Dependencias separadas por coma")
	resolved := fs.String("resolved", "", "Dependencias ya completadas, separadas por coma")
	files := fs.String("files", "", "Artefactos del alcance, separados por coma")
	readOnly := fs.Bool("read-only", false, "El trabajo solo lee, no escribe")
	complexity := fs.String("complexity", "medium", "trivial|low|medium|high")
	risk := fs.String("risk", "medium", "trivial|low|medium|high")
	subagents := fs.Bool("subagents", true, "El runtime admite subagentes")
	parallel := fs.Bool("parallel", true, "El runtime admite ejecución paralela")
	isolated := fs.Bool("isolated-context", true, "El runtime aísla el contexto de los subagentes")
	maxParallel := fs.Int("max-parallel", 0, "Tope de concurrencia del runtime (0 = por defecto)")
	budget := fs.Int("budget", 0, "Presupuesto total de tokens de la sesión (0 = sin declarar)")
	preferInline := fs.Bool("prefer-inline", false, "Inclinar el desempate hacia ejecución inline")
	allowValidationReserve := fs.Bool("allow-validation-reserve", false, "Autoriza explícitamente consumir la reserva de validación (FR-031)")
	contextTokens := fs.Int("context-tokens", 0, "Contexto estimado de la unidad, en tokens (0 = medirlo del objetivo y del alcance)")
	asJSON := fs.Bool("json", false, "Emitir la decisión como JSON")

	if err := fs.Parse(args); err != nil {
		return usecases.RouteTaskRequest{}, false, err
	}

	req := usecases.RouteTaskRequest{
		Unit: domain.WorkUnit{
			ID:           strings.TrimSpace(*taskID),
			Objective:    objetivo,
			Class:        domain.TaskClass(strings.TrimSpace(*class)),
			Dependencies: listaSeparadaPorComas(*deps),
			Scope: domain.Scope{
				Files:    listaSeparadaPorComas(*files),
				ReadOnly: *readOnly,
			},
			Complexity:  domain.ParseLevel(*complexity),
			Risk:        domain.ParseLevel(*risk),
			ContextNeed: domain.ContextNeed{EstimatedTokens: *contextTokens},
		},
		Resolved: conjuntoDesdeLista(listaSeparadaPorComas(*resolved)),
		Capabilities: domain.RuntimeCapabilities{
			Subagents:       *subagents,
			Parallel:        *parallel,
			IsolatedContext: *isolated,
			MaxParallel:     *maxParallel,
		},
		Budget: domain.NewBudget(*budget, domain.DefaultBudgetSplit()),
		Policy: domain.PolicyOverrides{PreferInline: *preferInline, AllowValidationReserve: *allowValidationReserve},
	}
	return req, *asJSON, nil
}

func cmdOctopusRoute(deps *Deps, args []string) {
	req, asJSON, err := ParseOctopusRouteFlags(args)
	if err != nil {
		fail("%v", err)
	}

	// Medir el alcance DECLARADO, no solo el objetivo. Estimar el contexto de
	// una investigación a partir de su frase de objetivo da ~40 tokens y hace
	// que nada se delegue nunca: la cifra sería honesta pero inútil. Leer los
	// archivos del alcance es una medición real, y esta capa es un adaptador —
	// tiene acceso al disco de forma legítima. Un archivo ausente se omite: el
	// alcance puede nombrar artefactos que aún no existen.
	req.ContextMaterial = req.Unit.Objective + leerAlcance(deps.Root, req.Unit.Scope.Files)

	ajustes, reparto := politicaDesdeAjustes(deps)
	req.Policy = combinarPolitica(req.Policy, ajustes)
	if req.Budget.Declarado() {
		presupuesto := domain.NewBudget(req.Budget.TotalTokens, reparto)
		req.Budget = presupuesto
	}

	req.Project = deps.Project
	uc := usecases.NewRouteTaskUseCase(deps.TokenCounter).WithEvidence(deps.OctopusRepo).WithMemoryRepository(deps.MemoryRepo)
	decision, err := uc.Route(req)
	if err != nil {
		fail("%v", err)
	}

	// Fire-and-forget: registrar no puede impedir responder.
	usecases.NewReportUseCase(deps.OctopusRepo).
		RecordDecision(deps.Project, req.Unit.Class, decision)

	if asJSON {
		emitirJSON(decision)
		return
	}
	fmt.Print(RenderRouteDecision(decision, contratoDe(deps, req, decision)))
}

// RenderRouteDecision formatea una decisión para consumo humano. Exportada
// porque las pruebas comparan su salida sin pasar por os.Stdout.
// contrato admite nil: una decisión inline no lo tiene, y una delegada puede
// consultarse sin haberlo armado todavía.
func RenderRouteDecision(d domain.RouteDecision, contrato *domain.ExecutionContract) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s → %s\n", d.WorkUnitID, d.Route)
	fmt.Fprintf(&b, "  Razón: %s\n", d.Reason.Text())

	if len(d.BlockedBy) > 0 {
		fmt.Fprintf(&b, "  Bloqueada por: %s\n", strings.Join(d.BlockedBy, ", "))
	}
	if d.Route.Delegada() {
		fmt.Fprintf(&b, "  Presupuesto de contexto: %d tokens\n", d.ContextBudget)
		fmt.Fprintf(&b, "  Presupuesto de salida: %d tokens\n", d.OutputBudget)
		if d.ParallelGroup != "" {
			fmt.Fprintf(&b, "  Grupo paralelo: %s\n", d.ParallelGroup)
		}
	}

	if contrato != nil {
		fmt.Fprintf(&b, "  Contrato de ejecución\n")
		fmt.Fprintf(&b, "    permisos: sistema de archivos %s · red %v\n",
			contrato.Permissions.Filesystem, contrato.Permissions.Network)
		fmt.Fprintf(&b, "    resultado esperado: %s\n", strings.Join(contrato.Output.Required, ", "))
		fmt.Fprintf(&b, "    puede delegar a su vez: %v\n", contrato.MaxDepth > 0)
	}

	c := d.EstimatedCost
	fmt.Fprintf(&b, "  Costo estimado de delegar: %d tokens", c.Total())
	if d.Estimated {
		// Nunca se presenta una estimación como medición (FR-033).
		b.WriteString(" (estimado)")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "    contexto %d · contrato %d · salida %d · coordinación %d · integración %d\n",
		c.ContextTokens, c.ContractTokens, c.OutputTokens, c.CoordinationTokens, c.IntegrationTokens)

	return b.String()
}

// renderPartialResult formatea el resultado parcial de una delegación que
// terminó en FALLBACK_INLINE, para que el agente principal reciba algo
// aprovechable en vez de tener que rehacer desde cero (FR-043).
func renderPartialResult(r domain.DelegatedResult) string {
	var b strings.Builder
	b.WriteString("Resultado parcial:\n")
	if r.Summary != "" {
		fmt.Fprintf(&b, "  resumen: %s\n", r.Summary)
	}
	// Slice ordenado, no mapa: el orden de iteración de un mapa en Go es
	// aleatorio a propósito (mismo criterio que domain/octopus_plan.go), y una
	// salida que cambia de orden en cada corrida rompería SC-006 para algo que
	// además consumen pruebas de texto exacto.
	listas := []struct {
		etiqueta string
		items    []string
	}{
		{"evidencia", r.Evidence},
		{"artefactos", r.Artifacts},
		{"símbolos afectados", r.AffectedSymbols},
		{"pendientes", r.Unresolved},
		{"faltante", r.Missing},
	}
	for _, l := range listas {
		if len(l.items) > 0 {
			fmt.Fprintf(&b, "  %s: %s\n", l.etiqueta, strings.Join(l.items, "; "))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func emitirJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail("serializar salida: %v", err)
	}
	fmt.Println(string(data))
}

func listaSeparadaPorComas(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	partes := strings.Split(s, ",")
	out := make([]string, 0, len(partes))
	for _, p := range partes {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func conjuntoDesdeLista(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}

// leerAlcance resuelve cada ruta relativa contra root antes de leerla — sin
// esto, el cwd del proceso decide qué se mide (ej. `mem mcp --root <dir>` no
// cambia el cwd, o una invocación de CLI desde un subdirectorio del proyecto),
// y el alcance real queda sin leer en silencio: el objetivo cae a ~40 tokens,
// la unidad nunca alcanza el mínimo delegable y la ruta se decide con una
// medición que nunca existió (ACR 029, hallazgo A-002). Una ruta absoluta se
// respeta tal cual. Concatena el contenido de los archivos del alcance que existen y
// son legibles. Best-effort a propósito: medir mal es mejor que no enrutar, y un
// archivo que falta no es un error del usuario (FR-032).
func leerAlcance(root string, files []string) string {
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range files {
		if !filepath.IsAbs(f) {
			f = filepath.Join(root, f)
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		b.Write(data)
	}
	return b.String()
}

// contratoDe arma el contrato cuando la decisión es delegada, para que la salida
// muestre permisos y forma del resultado y no solo la ruta. Devuelve nil si la
// unidad no se delega o si el contrato no puede construirse — un contrato que no
// sale no invalida la decisión, que ya está tomada.
func contratoDe(deps *Deps, req usecases.RouteTaskRequest, d domain.RouteDecision) *domain.ExecutionContract {
	if !d.Route.Delegada() {
		return nil
	}
	uc := usecases.NewPackContractUseCase(deps.MemoryRepo, deps.Compressor, deps.TokenCounter, deps.SpecKitReader)
	pkg, err := uc.Build(usecases.PackContractRequest{
		Unit:              req.Unit,
		Decision:          d,
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadWrite, Network: true},
	})
	if err != nil {
		return nil
	}
	return &pkg.Contract
}

// politicaDesdeAjustes traduce la configuración del proyecto a la política del
// enrutador. Los ceros significan "sin opinión" y el dominio aplica su valor de
// fábrica: las cifras viven en domain/octopus_policy.go, no aquí.
func politicaDesdeAjustes(deps *Deps) (domain.PolicyOverrides, domain.BudgetSplit) {
	if deps == nil || deps.SettingsRepo == nil {
		return domain.PolicyOverrides{}, domain.DefaultBudgetSplit()
	}
	s := deps.SettingsRepo.Read(deps.Root)

	reparto := domain.BudgetSplit{
		MainAgentPct:  s.OctopusMainAgentPct,
		DelegationPct: s.OctopusDelegationPct,
		ValidationPct: s.OctopusValidationPct,
	}
	if !reparto.Valid() {
		// Un reparto a medio configurar no es una preferencia: es un descuido.
		// Caer al de fábrica es preferible a repartir mal en silencio.
		reparto = domain.DefaultBudgetSplit()
	}

	return domain.PolicyOverrides{
		MaxSubagents: s.OctopusMaxSubagents,
		MaxParallel:  s.OctopusMaxParallel,
		MaxDepth:     s.OctopusMaxDepth,
		MaxRetries:   s.OctopusMaxRetries,
	}, reparto
}

// combinarPolitica hace ganar al MÁS RESTRICTIVO entre lo que dice la línea de
// comandos y lo que dice la configuración. Un tope configurado no puede
// relajarse pasando una bandera, ni al revés.
func combinarPolitica(banderas, ajustes domain.PolicyOverrides) domain.PolicyOverrides {
	out := banderas
	out.MaxSubagents = menorTope(banderas.MaxSubagents, ajustes.MaxSubagents)
	out.MaxParallel = menorTope(banderas.MaxParallel, ajustes.MaxParallel)
	out.MaxDepth = menorTope(banderas.MaxDepth, ajustes.MaxDepth)
	out.MaxRetries = menorTope(banderas.MaxRetries, ajustes.MaxRetries)
	return out
}

func menorTope(a, b int) int {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}
