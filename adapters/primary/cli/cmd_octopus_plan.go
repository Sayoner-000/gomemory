package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

// --- `mem octopus plan` (feature 027) ---
//
// Simulación POR DEFINICIÓN: describe qué quedaría inline, qué se delegaría y
// qué podría ejecutarse a la vez, y no inicia ningún subagente. No hace falta
// una bandera para garantizarlo: Octopus no ejecuta, nunca (INV-AAR-018).

// planJSON es la forma del archivo que acepta --file. Deliberadamente plana y
// tolerante: describe el grafo, no la implementación.
type planJSON struct {
	PlanID string `json:"plan_id"`
	Budget struct {
		TotalTokens     int `json:"total_tokens"`
		DelegationSpent int `json:"delegation_spent"`
	} `json:"budget"`
	Capabilities struct {
		Subagents       bool `json:"subagents"`
		Parallel        bool `json:"parallel"`
		IsolatedContext bool `json:"isolated_context"`
		MaxParallel     int  `json:"max_parallel"`
	} `json:"capabilities"`
	Resolved []string `json:"resolved"`
	Tasks    []struct {
		ID               string   `json:"id"`
		Objective        string   `json:"objective"`
		TaskClass        string   `json:"task_class"`
		Dependencies     []string `json:"dependencies"`
		Files            []string `json:"files"`
		ReadOnly         bool     `json:"read_only"`
		Complexity       string   `json:"complexity"`
		Risk             string   `json:"risk"`
		ContextTokens    int      `json:"context_tokens"`
		OutputTokens     int      `json:"output_tokens"`
		NearlyFullParent bool     `json:"nearly_full_parent"`
		CriticalPath     bool     `json:"critical_path"`
		Optional         bool     `json:"optional"`
	} `json:"tasks"`
}

func (p planJSON) toRequest() usecases.RoutePlanRequest {
	req := usecases.RoutePlanRequest{
		PlanID: p.PlanID,
		Capabilities: domain.RuntimeCapabilities{
			Subagents:       p.Capabilities.Subagents,
			Parallel:        p.Capabilities.Parallel,
			IsolatedContext: p.Capabilities.IsolatedContext,
			MaxParallel:     p.Capabilities.MaxParallel,
		},
		Budget:   domain.NewBudget(p.Budget.TotalTokens, domain.DefaultBudgetSplit()),
		Resolved: conjuntoDesdeLista(p.Resolved),
	}
	req.Budget.DelegationSpent = p.Budget.DelegationSpent

	for _, t := range p.Tasks {
		req.Units = append(req.Units, domain.WorkUnit{
			ID:           t.ID,
			Objective:    t.Objective,
			Class:        domain.TaskClass(t.TaskClass),
			Dependencies: t.Dependencies,
			Scope:        domain.Scope{Files: t.Files, ReadOnly: t.ReadOnly},
			Complexity:   domain.ParseLevel(t.Complexity),
			Risk:         domain.ParseLevel(t.Risk),
			ContextNeed: domain.ContextNeed{
				EstimatedTokens:  t.ContextTokens,
				NearlyFullParent: t.NearlyFullParent,
			},
			ExpectedOutput: domain.OutputSpec{MaxTokens: t.OutputTokens},
			CriticalPath:   t.CriticalPath,
			Optional:       t.Optional,
		})
	}
	return req
}

// ParseOctopusPlanFlags parsea `mem octopus plan`. Separada para poder probarla
// sin que un error dispare os.Exit.
func ParseOctopusPlanFlags(args []string) (ruta string, overrides domain.PolicyOverrides, presupuesto int, asJSON bool, err error) {
	fs := flag.NewFlagSet("octopus plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	file := fs.String("file", "", "Archivo JSON con el grafo de tareas")
	budget := fs.Int("budget", 0, "Presupuesto total de tokens (sobrescribe el del archivo)")
	maxParallel := fs.Int("max-parallel", 0, "Tope de concurrencia")
	maxAgents := fs.Int("max-agents", 0, "Tope de agentes delegados del plan")
	preferInline := fs.Bool("prefer-inline", false, "Inclinar el desempate hacia inline")
	allowValidationReserve := fs.Bool("allow-validation-reserve", false, "Autoriza explícitamente consumir la reserva de validación (FR-031)")
	j := fs.Bool("json", false, "Emitir el plan como JSON")

	if err = fs.Parse(args); err != nil {
		return "", domain.PolicyOverrides{}, 0, false, err
	}
	return *file, domain.PolicyOverrides{
		MaxParallel:            *maxParallel,
		MaxSubagents:           *maxAgents,
		PreferInline:           *preferInline,
		AllowValidationReserve: *allowValidationReserve,
	}, *budget, *j, nil
}

func cmdOctopusPlan(deps *Deps, args []string) {
	ruta, overrides, presupuesto, asJSON, err := ParseOctopusPlanFlags(args)
	if err != nil {
		fail("%v", err)
	}

	var req usecases.RoutePlanRequest
	if ruta != "" {
		req, err = leerPlanDesdeArchivo(ruta)
		if err != nil {
			fail("%v", err)
		}
	} else {
		// Sin archivo, el grafo sale de la funcionalidad activa de Spec Kit. Si
		// no hay ninguna, el caso de uso lo dice: no se inventa un plan.
		req = usecases.RoutePlanRequest{
			Root: deps.Root,
			Capabilities: domain.RuntimeCapabilities{
				Subagents: true, Parallel: true, IsolatedContext: true, MaxParallel: 3,
			},
		}
	}
	ajustes, reparto := politicaDesdeAjustes(deps)
	req.Policy = combinarPolitica(overrides, ajustes)
	if presupuesto > 0 {
		req.Budget = domain.NewBudget(presupuesto, reparto)
	} else if req.Budget.Declarado() {
		// El reparto configurado también manda sobre el total que trae el
		// archivo: el archivo describe el grafo, el proyecto decide el reparto.
		req.Budget = domain.NewBudget(req.Budget.TotalTokens, reparto)
	}

	req.Project = deps.Project
	if req.ContextMaterial == nil {
		req.ContextMaterial = make(map[string]string)
	}
	for _, unit := range req.Units {
		req.ContextMaterial[unit.ID] = unit.Objective + leerAlcance(deps.Root, unit.Scope.Files)
	}
	uc := usecases.NewRoutePlanUseCase(deps.TokenCounter, deps.SpecKitReader).WithEvidence(deps.OctopusRepo).WithMemoryRepository(deps.MemoryRepo)
	plan, err := uc.Route(req)
	if err != nil {
		fail("%v", err)
	}

	usecases.NewReportUseCase(deps.OctopusRepo).
		RecordPlan(deps.Project, plan, clasesDeLasUnidades(req.Units))

	if asJSON {
		emitirJSON(plan)
		return
	}
	fmt.Print(RenderRoutingPlan(plan))
}

func leerPlanDesdeArchivo(ruta string) (usecases.RoutePlanRequest, error) {
	data, err := os.ReadFile(ruta)
	if err != nil {
		return usecases.RoutePlanRequest{}, fmt.Errorf("leer %s: %w", ruta, err)
	}
	var p planJSON
	if err := json.Unmarshal(data, &p); err != nil {
		return usecases.RoutePlanRequest{}, fmt.Errorf("interpretar %s: %w", ruta, err)
	}
	return p.toRequest(), nil
}

// RenderRoutingPlan formatea el plan para consumo humano: qué queda inline, qué
// se delega, qué corre a la vez, con qué presupuestos y por qué (FR-044).
func RenderRoutingPlan(p domain.RoutingPlan) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Plan de enrutamiento: %s\n", p.PlanID)
	fmt.Fprintf(&b, "Simulación: no se inicia ningún subagente.\n\n")

	porRuta := map[domain.Route][]domain.RouteDecision{}
	for _, d := range p.Decisions {
		porRuta[d.Route] = append(porRuta[d.Route], d)
	}

	// Orden fijo de secciones: la salida de una simulación debe ser comparable
	// entre corridas, no depender del recorrido de un mapa.
	for _, ruta := range domain.AllRoutes() {
		decisiones := porRuta[ruta]
		if len(decisiones) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s (%d)\n", ruta, len(decisiones))
		for _, d := range decisiones {
			fmt.Fprintf(&b, "  %s — %s\n", d.WorkUnitID, d.Reason.Text())
			if d.Route.Delegada() {
				fmt.Fprintf(&b, "      contexto %d · salida %d · costo estimado %d",
					d.ContextBudget, d.OutputBudget, d.EstimatedCost.Total())
				if d.ParallelGroup != "" {
					fmt.Fprintf(&b, " · grupo %s", d.ParallelGroup)
				}
				b.WriteString("\n")
			}
			if len(d.BlockedBy) > 0 {
				fmt.Fprintf(&b, "      bloqueada por: %s\n", strings.Join(d.BlockedBy, ", "))
			}
		}
		b.WriteString("\n")
	}

	if len(p.ParallelGroups) > 0 {
		b.WriteString("Grupos paralelos\n")
		grupos := append([]domain.ParallelGroup(nil), p.ParallelGroups...)
		sort.Slice(grupos, func(i, j int) bool { return grupos[i].ID < grupos[j].ID })
		for _, g := range grupos {
			fmt.Fprintf(&b, "  %s: %s\n", g.ID, strings.Join(g.Tasks, ", "))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Agentes delegados: %d\n", p.DelegatedCount)
	if p.Budget.Declarado() {
		fmt.Fprintf(&b, "Fondo de delegación: %d de %d tokens consumidos (estimado)\n",
			p.Budget.DelegationSpent, p.Budget.DelegationPoolMax)
		fmt.Fprintf(&b, "Reserva de validación restante: %d tokens\n", p.Budget.ValidationReserve)
	}
	return b.String()
}
