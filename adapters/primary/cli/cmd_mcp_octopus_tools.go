package cli

import (
	"context"
	"fmt"

	"mem/application/usecases"
	"mem/domain"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerOctopusTools expone el enrutador adaptativo como tools MCP (feature
// 027).
//
// A diferencia de registerTools y registerCodeTools, esta función se invoca de
// forma CONDICIONAL: solo con el módulo encendido. Ver domain.MCPToolsFor y el
// comentario de domain.MCPOctopusTools para el porqué — en resumen, el esquema
// de cada tool viaja al agente en cada arranque de sesión, y apagado tiene que
// significar huella cero.
//
// Ninguna de estas tools ejecuta nada: Octopus produce política, el runtime
// ejecuta (INV-AAR-018).
func registerOctopusTools(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: domain.ToolOctopusRouteTask,
		Description: "Decide si una unidad de trabajo debe ejecutarse en el agente principal (INLINE) o " +
			"delegarse a un subagente (DELEGATE), esperar dependencias (WAIT) o rechazarse (REJECT). " +
			"Devuelve la ruta con una razón explicable y, si delega, el presupuesto de contexto y de " +
			"salida. NO ejecuta subagentes: eso es responsabilidad del runtime.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Objective     string   `json:"objective" jsonschema:"Objetivo de la unidad de trabajo"`
		TaskID        string   `json:"task_id,omitempty" jsonschema:"Identificador de la tarea (default T001)"`
		TaskClass     string   `json:"task_class,omitempty" jsonschema:"Clase: investigation|implementation|documentation|testing|research|review|architecture|migration|integration|validation|local-change|trivial|repository-exploration"`
		Dependencies  []string `json:"dependencies,omitempty" jsonschema:"Identificadores de tareas de las que depende"`
		Resolved      []string `json:"resolved,omitempty" jsonschema:"Dependencias ya completadas"`
		Files         []string `json:"files,omitempty" jsonschema:"Artefactos del alcance"`
		ReadOnly      bool     `json:"read_only,omitempty" jsonschema:"El trabajo solo lee, no escribe"`
		Complexity    string   `json:"complexity,omitempty" jsonschema:"trivial|low|medium|high (default medium)"`
		Risk          string   `json:"risk,omitempty" jsonschema:"trivial|low|medium|high (default medium)"`
		ContextTokens int      `json:"context_tokens,omitempty" jsonschema:"Contexto estimado de la unidad, en tokens"`
		Optional      bool     `json:"optional,omitempty" jsonschema:"El plan puede completarse sin esta unidad"`

		Subagents       bool `json:"subagents,omitempty" jsonschema:"El runtime admite subagentes (ausente = NO, default conservador)"`
		Parallel        bool `json:"parallel,omitempty" jsonschema:"El runtime admite ejecución paralela"`
		IsolatedContext bool `json:"isolated_context,omitempty" jsonschema:"El runtime aísla el contexto de los subagentes"`
		MaxParallel     int  `json:"max_parallel,omitempty" jsonschema:"Tope de concurrencia del runtime"`

		TotalTokens        int  `json:"total_tokens,omitempty" jsonschema:"Presupuesto total de tokens de la sesión"`
		DelegationSpent    int  `json:"delegation_spent,omitempty" jsonschema:"Tokens ya consumidos del fondo de delegación"`
		DelegationDisabled bool `json:"delegation_disabled,omitempty" jsonschema:"Forzar ejecución inline"`
		DelegationForced   bool `json:"delegation_forced,omitempty" jsonschema:"Preferir delegación (sujeto a capacidades y seguridad)"`
		PreferInline       bool `json:"prefer_inline,omitempty" jsonschema:"Inclinar el desempate hacia inline"`
		MaxSubagents       int  `json:"max_subagents,omitempty" jsonschema:"Tope de agentes delegados"`
		MaxDepth           int  `json:"max_depth,omitempty" jsonschema:"Profundidad máxima de delegación"`
		Depth              int  `json:"depth,omitempty" jsonschema:"Profundidad actual (0 = agente principal)"`

		AllowValidationReserve bool `json:"allow_validation_reserve,omitempty" jsonschema:"Autoriza explícitamente consumir la reserva de validación (FR-031)"`
	}) (*mcp.CallToolResult, any, error) {
		taskID := in.TaskID
		if taskID == "" {
			taskID = "T001"
		}

		ajustes, reparto := politicaDesdeAjustes(deps)
		presupuesto := domain.NewBudget(in.TotalTokens, reparto)
		presupuesto.DelegationSpent = in.DelegationSpent

		uc := usecases.NewRouteTaskUseCase(deps.TokenCounter).WithEvidence(deps.OctopusRepo).WithMemoryRepository(deps.MemoryRepo)
		decision, err := uc.Route(usecases.RouteTaskRequest{
			Unit: domain.WorkUnit{
				ID:           taskID,
				Objective:    in.Objective,
				Class:        domain.TaskClass(in.TaskClass),
				Dependencies: in.Dependencies,
				Scope:        domain.Scope{Files: in.Files, ReadOnly: in.ReadOnly},
				Complexity:   domain.ParseLevel(in.Complexity),
				Risk:         domain.ParseLevel(in.Risk),
				ContextNeed:  domain.ContextNeed{EstimatedTokens: in.ContextTokens},
				Optional:     in.Optional,
			},
			ContextMaterial: in.Objective + leerAlcance(in.Files),
			Project:         deps.Project,
			Resolved:        conjuntoDesdeLista(in.Resolved),
			Capabilities: domain.RuntimeCapabilities{
				Subagents:       in.Subagents,
				Parallel:        in.Parallel,
				IsolatedContext: in.IsolatedContext,
				MaxParallel:     in.MaxParallel,
			},
			Budget: presupuesto,
			Policy: combinarPolitica(domain.PolicyOverrides{
				DelegationDisabled:     in.DelegationDisabled,
				DelegationForced:       in.DelegationForced,
				PreferInline:           in.PreferInline,
				MaxSubagents:           in.MaxSubagents,
				MaxDepth:               in.MaxDepth,
				AllowValidationReserve: in.AllowValidationReserve,
			}, ajustes),
			Depth: in.Depth,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("enrutar unidad: %w", err)
		}

		usecases.NewReportUseCase(deps.OctopusRepo).
			RecordDecision(deps.Project, domain.TaskClass(in.TaskClass), decision)

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: RenderRouteDecision(decision, nil)}},
		}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: domain.ToolOctopusRoutePlan,
		Description: "Enruta un grafo de tareas completo: devuelve la decisión de cada unidad (INLINE, " +
			"DELEGATE, PARALLEL, WAIT o REJECT) con su razón, los grupos que pueden ejecutarse a la vez y " +
			"el estado del presupuesto. Respeta dependencias, tope de concurrencia y tope de agentes. " +
			"NO inicia nada: es siempre una simulación, el runtime es quien ejecuta.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		PlanID string `json:"plan_id,omitempty" jsonschema:"Identificador del plan"`
		Tasks  []struct {
			ID               string   `json:"id"`
			Objective        string   `json:"objective"`
			TaskClass        string   `json:"task_class,omitempty"`
			Dependencies     []string `json:"dependencies,omitempty"`
			Files            []string `json:"files,omitempty"`
			ReadOnly         bool     `json:"read_only,omitempty"`
			Complexity       string   `json:"complexity,omitempty"`
			Risk             string   `json:"risk,omitempty"`
			ContextTokens    int      `json:"context_tokens,omitempty"`
			OutputTokens     int      `json:"output_tokens,omitempty"`
			NearlyFullParent bool     `json:"nearly_full_parent,omitempty"`
			CriticalPath     bool     `json:"critical_path,omitempty"`
			Optional         bool     `json:"optional,omitempty"`
		} `json:"tasks" jsonschema:"Unidades de trabajo del plan, con sus dependencias"`
		Resolved []string `json:"resolved,omitempty" jsonschema:"Tareas ya completadas"`

		Subagents       bool `json:"subagents,omitempty" jsonschema:"El runtime admite subagentes (ausente = NO)"`
		Parallel        bool `json:"parallel,omitempty" jsonschema:"El runtime admite ejecución paralela"`
		IsolatedContext bool `json:"isolated_context,omitempty" jsonschema:"El runtime aísla el contexto"`
		MaxParallel     int  `json:"max_parallel,omitempty" jsonschema:"Tope de concurrencia del runtime"`

		TotalTokens  int  `json:"total_tokens,omitempty" jsonschema:"Presupuesto total de tokens de la sesión"`
		MaxSubagents int  `json:"max_subagents,omitempty" jsonschema:"Tope de agentes delegados del plan"`
		PreferInline bool `json:"prefer_inline,omitempty" jsonschema:"Inclinar el desempate hacia inline"`

		AllowValidationReserve bool `json:"allow_validation_reserve,omitempty" jsonschema:"Autoriza explícitamente consumir la reserva de validación (FR-031)"`
	}) (*mcp.CallToolResult, any, error) {
		ajustes, reparto := politicaDesdeAjustes(deps)
		planReq := usecases.RoutePlanRequest{
			PlanID:   in.PlanID,
			Resolved: conjuntoDesdeLista(in.Resolved),
			Capabilities: domain.RuntimeCapabilities{
				Subagents:       in.Subagents,
				Parallel:        in.Parallel,
				IsolatedContext: in.IsolatedContext,
				MaxParallel:     in.MaxParallel,
			},
			Budget: domain.NewBudget(in.TotalTokens, reparto),
			Policy: combinarPolitica(domain.PolicyOverrides{
				MaxSubagents:           in.MaxSubagents,
				PreferInline:           in.PreferInline,
				AllowValidationReserve: in.AllowValidationReserve,
			}, ajustes),
			Root:    deps.Root,
			Project: deps.Project,
		}
		for _, t := range in.Tasks {
			if planReq.ContextMaterial == nil {
				planReq.ContextMaterial = make(map[string]string)
			}
			planReq.ContextMaterial[t.ID] = t.Objective + leerAlcance(t.Files)
			planReq.Units = append(planReq.Units, domain.WorkUnit{
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

		uc := usecases.NewRoutePlanUseCase(deps.TokenCounter, deps.SpecKitReader).WithEvidence(deps.OctopusRepo).WithMemoryRepository(deps.MemoryRepo)
		plan, err := uc.Route(planReq)
		if err != nil {
			return nil, nil, fmt.Errorf("enrutar plan: %w", err)
		}

		usecases.NewReportUseCase(deps.OctopusRepo).
			RecordPlan(deps.Project, plan, clasesDeLasUnidades(planReq.Units))

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: RenderRoutingPlan(plan)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: domain.ToolOctopusReport,
		Description: "Informa a Octopus del resultado real de una unidad ya ejecutada: consumo, duración y " +
			"calidad. Sirve para contrastar lo estimado con lo ocurrido y mejorar las estimaciones futuras. " +
			"Nunca falla: un reporte de una tarea sin decisión previa se ignora en silencio.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		TaskID        string `json:"task_id" jsonschema:"Identificador de la tarea reportada"`
		Route         string `json:"route,omitempty" jsonschema:"Ruta realmente ejecutada"`
		Status        string `json:"status,omitempty" jsonschema:"completed|failed|insufficient_context"`
		ContextTokens int    `json:"context_tokens,omitempty" jsonschema:"Tokens de contexto realmente consumidos"`
		OutputTokens  int    `json:"output_tokens,omitempty" jsonschema:"Tokens de salida realmente producidos"`
		DurationMS    int    `json:"duration_ms,omitempty" jsonschema:"Duración en milisegundos"`
		Quality       string `json:"quality,omitempty" jsonschema:"accepted|partial|rejected"`
		Retries       int    `json:"retries,omitempty" jsonschema:"Reintentos ya realizados para esta unidad"`
		Expansions    int    `json:"expansions,omitempty" jsonschema:"Ampliaciones de contexto ya realizadas"`
		ParentCanDoIt bool   `json:"parent_can_do_it,omitempty" jsonschema:"El agente principal puede asumir el trabajo"`

		Summary         string   `json:"summary,omitempty" jsonschema:"Resumen en prosa de lo que el subagente alcanzó a producir"`
		Evidence        []string `json:"evidence,omitempty" jsonschema:"Evidencia concreta recogida antes del fallo"`
		Artifacts       []string `json:"artifacts,omitempty" jsonschema:"Artefactos ya producidos (rutas, IDs) antes del fallo"`
		AffectedSymbols []string `json:"affected_symbols,omitempty" jsonschema:"Símbolos de código ya identificados como afectados"`
		Unresolved      []string `json:"unresolved,omitempty" jsonschema:"Preguntas o pasos que quedaron sin resolver"`
		Missing         []string `json:"missing,omitempty" jsonschema:"Qué le faltó al subagente (solo con status insufficient_context)"`
	}) (*mcp.CallToolResult, any, error) {
		reporte := domain.ExecutionReport{
			TaskID:        in.TaskID,
			Route:         domain.Route(in.Route),
			Status:        domain.ResultStatus(in.Status),
			ContextTokens: in.ContextTokens,
			OutputTokens:  in.OutputTokens,
			DurationMS:    in.DurationMS,
			Quality:       domain.Quality(in.Quality),
		}
		uc := usecases.NewReportUseCase(deps.OctopusRepo)
		texto := "Reporte registrado para " + in.TaskID
		if reporte.Status == domain.StatusFailed || reporte.Status == domain.StatusInsufficientContext {
			ajustes, _ := politicaDesdeAjustes(deps)
			decision := uc.HandleFailure(usecases.FailureRequest{
				Project: deps.Project, Report: reporte, Policy: ajustes,
				Result: domain.DelegatedResult{
					TaskID:          in.TaskID,
					Status:          reporte.Status,
					Summary:         in.Summary,
					Evidence:        in.Evidence,
					Artifacts:       in.Artifacts,
					AffectedSymbols: in.AffectedSymbols,
					Unresolved:      in.Unresolved,
					Missing:         in.Missing,
				},
				Attempts:      domain.AttemptState{Retries: in.Retries, Expansions: in.Expansions},
				ParentCanDoIt: in.ParentCanDoIt,
			})
			texto += "\nRecomendación: " + string(decision.Policy)
			if decision.ExtraContextTokens > 0 {
				texto += fmt.Sprintf(" (+%d tokens de contexto)", decision.ExtraContextTokens)
			}
			// FR-043: el resultado parcial tiene que llegar hasta la salida, no
			// solo hasta FailureDecision — antes se calculaba y se descartaba.
			if decision.PartialResult != nil {
				texto += "\n" + renderPartialResult(*decision.PartialResult)
			}
		} else {
			uc.Report(deps.Project, reporte)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: texto}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: domain.ToolOctopusStatus,
		Description: "Estado del enrutador: topes efectivos y agregados de telemetría — conteos por ruta, " +
			"consumo estimado frente a real, éxitos, fallos y ancho de paralelismo observado.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		uc := usecases.NewReportUseCase(deps.OctopusRepo)
		texto := RenderOctopusStatus(deps, uc.Stats(deps.Project))
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: texto}}}, nil, nil
	})
}
