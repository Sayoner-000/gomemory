package cli

import (
	"context"
	"fmt"
	"log"
	"strings"

	"mem/application/usecases"
	"mem/domain"
	"mem/version"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CmdMCP arranca el servidor MCP stdio. root y project ya vienen resueltos
// por el composition root (infrastructure.NewContainer, vía
// resolveRootForCommand) — respeta --root si se pasó, o el cwd (git root o
// cwd si no hay .git) en caso contrario. No se re-resuelven aquí: hacerlo
// llevaría a un desajuste entre la base de datos ya abierta en deps y el
// "project" usado para filtrar, si --root difiere del cwd real.
func CmdMCP(deps *Deps, args []string) {
	root := deps.Root
	project := deps.Project

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gomemory",
		Version: version.Version,
	}, nil)

	registerTools(server, deps, project)
	registerCodeTools(server, deps, root, project)
	registerResources(server, deps, project)

	// Auto-start session on MCP server start (best-effort, no debe romper el server)
	if active, _ := deps.SessionRepo.Active(project); active == nil {
		if sess, err := deps.SessionRepo.Start(project); err == nil {
			log.Printf("Sesión auto-iniciada (id=%s) para proyecto '%s'", sess.ID[:8], project)
		}
	}

	log.Printf("MCP server iniciado para proyecto '%s'", project)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerTools(server *mcp.Server, deps *Deps, project string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_memory",
		Description: "Guarda un aprendizaje, decisión o descubrimiento en la memoria del proyecto",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Title    string `json:"title" jsonschema:"Título descriptivo de la memoria"`
		Type     string `json:"type" jsonschema:"Tipo: learning|decision|architecture|bugfix|pattern|discovery|preference"`
		Content  string `json:"content" jsonschema:"Contenido del aprendizaje"`
		Filepath string `json:"filepath,omitempty" jsonschema:"Archivo relacionado (opcional)"`
	}) (*mcp.CallToolResult, any, error) {
		memType := domain.ValidMemoryType(in.Type)
		var sessionID string
		sess, _ := deps.SessionRepo.Active(project)
		if sess != nil {
			sessionID = sess.ID
		}
		mem := domain.Memory{
			Project:   project,
			SessionID: sessionID,
			Type:      memType,
			Title:     in.Title,
			Content:   in.Content,
			Filepath:  in.Filepath,
		}
		id, err := deps.MemoryRepo.Insert(&mem)
		if err != nil {
			return nil, nil, fmt.Errorf("guardar memoria: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("✓ Memoria guardada (id=%d)", id)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_memories",
		Description: "Busca en todas las memorias del proyecto por texto",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Query string `json:"query" jsonschema:"Término de búsqueda"`
		Limit int    `json:"limit,omitempty" jsonschema:"Número máximo de resultados (default 10)"`
	}) (*mcp.CallToolResult, any, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 10
		}
		mems, err := deps.MemoryRepo.Search(project, in.Query, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("buscar: %w", err)
		}
		if len(mems) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Sin resultados para: " + in.Query}},
			}, nil, nil
		}
		var sb strings.Builder
		for _, m := range mems {
			sb.WriteString(fmt.Sprintf("[%d] %s | %s\n  %s\n\n", m.ID, m.Type, m.Title, m.Content))
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_memories",
		Description: "Lista las memorias más recientes del proyecto",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Limit int `json:"limit,omitempty" jsonschema:"Número máximo (default 20)"`
	}) (*mcp.CallToolResult, any, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		mems, err := deps.MemoryRepo.List(project, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("listar: %w", err)
		}
		if len(mems) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No hay memorias guardadas."}},
			}, nil, nil
		}
		var sb strings.Builder
		for _, m := range mems {
			content := m.Content
			if len(content) > 77 {
				content = content[:77] + "..."
			}
			sb.WriteString(fmt.Sprintf("[%d] %s | %s\n  %s\n\n", m.ID, m.Type, m.Title, content))
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_memory",
		Description: "Obtiene una memoria específica por su ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ID int `json:"id" jsonschema:"ID de la memoria"`
	}) (*mcp.CallToolResult, any, error) {
		mems, err := deps.MemoryRepo.List(project, 200)
		if err != nil {
			return nil, nil, err
		}
		for _, m := range mems {
			if m.ID == int64(in.ID) {
				sessionInfo := ""
				if m.SessionID != "" {
					sessionInfo = fmt.Sprintf("\nSesión: %s", m.SessionID[:8])
				}
				fileInfo := ""
				if m.Filepath != "" {
					fileInfo = fmt.Sprintf("\nArchivo: %s", m.Filepath)
				}
				text := fmt.Sprintf("ID: %d\nTipo: %s\nTítulo: %s\nFecha: %s%s%s\n\n%s",
					m.ID, m.Type, m.Title, m.CreatedAt, sessionInfo, fileInfo, m.Content)
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: text}},
				}, nil, nil
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Memoria %d no encontrada", in.ID)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "start_session",
		Description: "Inicia una nueva sesión de trabajo. Las próximas memorias se asociarán a esta sesión.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		active, _ := deps.SessionRepo.Active(project)
		if active != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("⚠ Ya hay una sesión activa desde %s", active.CreatedAt)}},
			}, nil, nil
		}
		sess, err := deps.SessionRepo.Start(project)
		if err != nil {
			return nil, nil, fmt.Errorf("iniciar sesión: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("✓ Sesión iniciada: %s", sess.ID[:8])}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "end_session",
		Description: "Finaliza la sesión de trabajo activa con un resumen",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Summary string `json:"summary,omitempty" jsonschema:"Resumen de lo realizado en la sesión"`
	}) (*mcp.CallToolResult, any, error) {
		sess, err := deps.SessionRepo.Active(project)
		if err != nil {
			return nil, nil, err
		}
		if sess == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No hay sesión activa para cerrar"}},
			}, nil, nil
		}
		if err := deps.SessionRepo.End(sess.ID, in.Summary); err != nil {
			return nil, nil, fmt.Errorf("cerrar sesión: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("✓ Sesión %s finalizada", sess.ID[:8])}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "forget_memory",
		Description: "Borra una memoria específica del proyecto por su ID (irreversible)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		ID int `json:"id" jsonschema:"ID de la memoria a borrar"`
	}) (*mcp.CallToolResult, any, error) {
		deleted, err := deps.MemoryRepo.Delete(project, int64(in.ID))
		if err != nil {
			return nil, nil, fmt.Errorf("borrar memoria: %w", err)
		}
		if !deleted {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Memoria %d no encontrada", in.ID)}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("✓ Memoria %d eliminada", in.ID)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "judge_memories",
		Description: "Actúa como juez imparcial entre dos memorias que se contradicen: relee el código/archivo " +
			"fuente actual para verificar cuál refleja los hechos reales (no asumas que la más reciente gana) y " +
			"registra el veredicto",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		IDA        int     `json:"id_a" jsonschema:"ID de la primera memoria"`
		IDB        int     `json:"id_b" jsonschema:"ID de la segunda memoria"`
		Verdict    string  `json:"verdict" jsonschema:"Veredicto: related|compatible|scoped|conflicts_with|supersedes|not_conflict"`
		Confidence float64 `json:"confidence,omitempty" jsonschema:"Confianza del veredicto (0.0-1.0, default 1.0)"`
		Reasoning  string  `json:"reasoning" jsonschema:"Por qué se llegó a este veredicto — obligatorio, debe citar los hechos verificados"`
	}) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Reasoning) == "" {
			return nil, nil, fmt.Errorf("reasoning es obligatorio: explica qué hechos verificaste para llegar a este veredicto")
		}
		confidence := in.Confidence
		if confidence <= 0 {
			confidence = 1.0
		}

		mems, err := deps.MemoryRepo.List(project, 200)
		if err != nil {
			return nil, nil, fmt.Errorf("listar memorias: %w", err)
		}
		foundA, foundB := false, false
		for _, m := range mems {
			if m.ID == int64(in.IDA) {
				foundA = true
			}
			if m.ID == int64(in.IDB) {
				foundB = true
			}
		}
		if !foundA {
			return nil, nil, fmt.Errorf("memoria %d no encontrada", in.IDA)
		}
		if !foundB {
			return nil, nil, fmt.Errorf("memoria %d no encontrada", in.IDB)
		}

		relType := domain.ValidRelationType(in.Verdict)
		rel, updated, err := usecases.RecordVerdict(deps.RelationRepo, project, int64(in.IDA), int64(in.IDB), relType, confidence, in.Reasoning)
		if err != nil {
			return nil, nil, err
		}
		verb := "guardado"
		if updated {
			verb = "actualizado"
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("✓ Veredicto %s (id=%d): %d ↔ %d → %s", verb, rel.ID, in.IDA, in.IDB, relType)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_context",
		Description: "Obtiene el contexto completo del proyecto como markdown: arquitectura, decisiones, bugs, aprendizajes",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		output, err := deps.ContextBuilder.Build()
		if err != nil {
			return nil, nil, fmt.Errorf("generar contexto: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: output}},
		}, nil, nil
	})
}

func registerResources(server *mcp.Server, deps *Deps, project string) {
	server.AddResource(&mcp.Resource{
		URI:         "mem://context",
		Name:        "Project Context",
		Description: "Contexto completo del proyecto en markdown",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		output, err := deps.ContextBuilder.Build()
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "mem://context", Text: output, MIMEType: "text/markdown"},
			},
		}, nil
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "mem://memory/{id}",
		Name:        "Memory by ID",
		Description: "Obtiene una memoria específica por su ID numérico",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		idStr := strings.TrimPrefix(req.Params.URI, "mem://memory/")
		var id int
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			return nil, fmt.Errorf("id inválido: %s", idStr)
		}
		mems, err := deps.MemoryRepo.List(project, 200)
		if err != nil {
			return nil, err
		}
		for _, m := range mems {
			if m.ID == int64(id) {
				text := fmt.Sprintf("ID: %d\nTipo: %s\nTítulo: %s\nFecha: %s\n\n%s", m.ID, m.Type, m.Title, m.CreatedAt, m.Content)
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{URI: req.Params.URI, Text: text},
					},
				}, nil
			}
		}
		return nil, fmt.Errorf("memoria %d no encontrada", id)
	})
}
