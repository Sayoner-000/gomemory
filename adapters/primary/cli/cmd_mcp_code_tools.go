package cli

import (
	"context"
	"fmt"
	"strings"

	"mem/application/usecases"
	"mem/domain"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerCodeTools expone el grafo de código indexado (Fase 1: solo Go, vía
// go/parser, sin cgo) como tools MCP separadas de las de memoria de
// registerTools. Todas son de solo lectura salvo index_project, que solo
// escribe en .memory/ — ninguna toca el código fuente del proyecto.
func registerCodeTools(server *mcp.Server, deps *Deps, root, project string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "index_project",
		Description: "Indexa (o reindexa) el código Go del proyecto en el grafo de símbolos: funciones, métodos, tipos, imports y llamadas",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Force bool `json:"force,omitempty" jsonschema:"Reindexar todos los archivos aunque no hayan cambiado (default false)"`
	}) (*mcp.CallToolResult, any, error) {
		ix := usecases.NewIndexer(deps.CodeGraphRepo, root, project)
		report, err := ix.IndexProject(in.Force)
		if err != nil {
			return nil, nil, fmt.Errorf("indexar proyecto: %w", err)
		}
		text := fmt.Sprintf(
			"✓ Índice actualizado en %s\nEscaneados: %d, parseados: %d, omitidos: %d, eliminados: %d\nNodos: %d, aristas: %d",
			report.Duration.Round(1e6), report.Scanned, report.Parsed, report.Skipped, report.Deleted, report.Nodes, report.Edges,
		)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_status",
		Description: "Muestra el tamaño del grafo de código indexado: archivos, símbolos, relaciones y paquetes principales",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		status, err := deps.CodeGraphRepo.Status(project)
		if err != nil {
			return nil, nil, fmt.Errorf("estado del grafo: %w", err)
		}
		if status.Files == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Sin código indexado todavía. Ejecuta index_project para empezar."}},
			}, nil, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Archivos: %d\nSímbolos: %d\nRelaciones: %d\n", status.Files, status.Nodes, status.Edges))
		if status.LastIndexedAt != "" {
			sb.WriteString(fmt.Sprintf("Última indexación: %s\n", status.LastIndexedAt))
		}
		if len(status.TopPackages) > 0 {
			sb.WriteString("\nPaquetes principales:\n")
			for _, p := range status.TopPackages {
				sb.WriteString(fmt.Sprintf("  %s: %d símbolos\n", p.Package, p.Symbols))
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_code",
		Description: "Busca símbolos de código (funciones, métodos, tipos) por nombre, firma o paquete",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Query string `json:"query" jsonschema:"Término de búsqueda"`
		Limit int    `json:"limit,omitempty" jsonschema:"Número máximo de resultados (default 10)"`
	}) (*mcp.CallToolResult, any, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 10
		}
		nodes, err := deps.CodeGraphRepo.SearchNodes(project, in.Query, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("buscar código: %w", err)
		}
		if len(nodes) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Sin resultados para: " + in.Query}},
			}, nil, nil
		}
		var sb strings.Builder
		for _, n := range nodes {
			sb.WriteString(formatCodeNodeLine(n))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_symbol",
		Description: "Obtiene la definición de un símbolo (función/método/tipo) por nombre, con sus callers y callees directos",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Name string `json:"name" jsonschema:"Nombre exacto del símbolo (ej. 'Foo' o 'Type.Method')"`
	}) (*mcp.CallToolResult, any, error) {
		nodes, err := deps.CodeGraphRepo.NodesByName(project, in.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("buscar símbolo: %w", err)
		}
		if len(nodes) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Símbolo %q no encontrado", in.Name)}},
			}, nil, nil
		}

		var sb strings.Builder
		for _, n := range nodes {
			sb.WriteString(formatCodeNodeDefinition(n))

			if n.Kind == domain.NodeFunction || n.Kind == domain.NodeMethod {
				callers, _, _ := deps.CodeGraphRepo.Neighbors(project, n.ID, domain.EdgeCalls, "in", 1)
				if len(callers) > 0 {
					sb.WriteString("  Llamado por:\n")
					for _, c := range callers {
						sb.WriteString("    " + formatCodeNodeLine(c))
					}
				}
				callees, _, _ := deps.CodeGraphRepo.Neighbors(project, n.ID, domain.EdgeCalls, "out", 1)
				if len(callees) > 0 {
					sb.WriteString("  Llama a:\n")
					for _, c := range callees {
						sb.WriteString("    " + formatCodeNodeLine(c))
					}
				}
			}
			sb.WriteString("\n")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_dependencies",
		Description: "Recorre el grafo de dependencias de un símbolo (llamadas o imports) hasta cierta profundidad",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Name      string `json:"name" jsonschema:"Nombre exacto del símbolo de partida"`
		Direction string `json:"direction,omitempty" jsonschema:"in|out|both (default out)"`
		Kind      string `json:"kind,omitempty" jsonschema:"calls|imports|defines (default calls)"`
		Depth     int    `json:"depth,omitempty" jsonschema:"Profundidad del recorrido, máximo 3 (default 1)"`
	}) (*mcp.CallToolResult, any, error) {
		nodes, err := deps.CodeGraphRepo.NodesByName(project, in.Name)
		if err != nil || len(nodes) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Símbolo %q no encontrado", in.Name)}},
			}, nil, nil
		}

		direction := in.Direction
		if direction == "" {
			direction = "out"
		}
		edgeKind, ok := domain.ValidCodeEdgeKind(in.Kind)
		if !ok {
			edgeKind = domain.EdgeCalls
		}
		depth := in.Depth
		if depth <= 0 {
			depth = 1
		}

		var sb strings.Builder
		for _, start := range nodes {
			neighbors, edges, err := deps.CodeGraphRepo.Neighbors(project, start.ID, edgeKind, direction, depth)
			if err != nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s (%s) — %d relaciones %s, profundidad %d:\n", start.Name, edgeKind, len(edges), direction, depth))
			for _, n := range neighbors {
				sb.WriteString("  " + formatCodeNodeLine(n))
			}
			sb.WriteString("\n")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
	})
}

func formatCodeNodeLine(n domain.CodeNode) string {
	loc := n.File
	if n.StartLine > 0 {
		loc = fmt.Sprintf("%s:%d", n.File, n.StartLine)
	}
	return fmt.Sprintf("[%d] %s %s (%s) — %s\n", n.ID, n.Kind, n.Name, n.Package, loc)
}

func formatCodeNodeDefinition(n domain.CodeNode) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%d] %s %s\n", n.ID, n.Kind, n.Name))
	if n.Package != "" {
		sb.WriteString(fmt.Sprintf("  Paquete: %s\n", n.Package))
	}
	if n.File != "" {
		loc := n.File
		if n.StartLine > 0 {
			loc = fmt.Sprintf("%s:%d-%d", n.File, n.StartLine, n.EndLine)
		}
		sb.WriteString(fmt.Sprintf("  Ubicación: %s\n", loc))
	}
	if n.Signature != "" {
		sb.WriteString(fmt.Sprintf("  Firma: %s\n", n.Signature))
	}
	return sb.String()
}
