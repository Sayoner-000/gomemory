package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"mem/store"
	"mem/types"

	memctx "mem/context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func cmdMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "Raíz absoluta del proyecto (evita depender del cwd del proceso que lo lanza)")
	fs.Parse(args)

	var root string
	if *rootFlag != "" {
		absRoot, err := filepath.Abs(*rootFlag)
		if err != nil {
			log.Fatalf("Error: --root inválido: %v", err)
		}
		if _, err := os.Stat(filepath.Join(absRoot, store.MemDir)); err != nil {
			log.Fatalf("Error: no existe %s en %s (ejecuta 'mem init' primero)", store.MemDir, absRoot)
		}
		root = absRoot
	} else {
		var err error
		root, err = store.FindRoot()
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
	}

	db, err := store.Open(root)
	if err != nil {
		log.Fatalf("Error al abrir DB: %v", err)
	}
	defer db.Close()

	project := filepath.Base(root)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gomemory",
		Version: "1.0.0",
	}, nil)

	registerTools(server, db, root, project)
	registerResources(server, db, root, project)

	log.Printf("MCP server iniciado para proyecto '%s'", project)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerTools(server *mcp.Server, db *sql.DB, root, project string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_memory",
		Description: "Guarda un aprendizaje, decisión o descubrimiento en la memoria del proyecto",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Title    string `json:"title" jsonschema:"Título descriptivo de la memoria"`
		Type     string `json:"type" jsonschema:"Tipo: learning|decision|architecture|bugfix|pattern|discovery"`
		Content  string `json:"content" jsonschema:"Contenido del aprendizaje"`
		Filepath string `json:"filepath,omitempty" jsonschema:"Archivo relacionado (opcional)"`
	}) (*mcp.CallToolResult, any, error) {
		memType := types.ValidMemoryType(in.Type)
		var sessionID string
		sess, _ := store.ActiveSession(db, project)
		if sess != nil {
			sessionID = sess.ID
		}
		mem := types.Memory{
			Project:   project,
			SessionID: sessionID,
			Type:      memType,
			Title:     in.Title,
			Content:   in.Content,
			Filepath:  in.Filepath,
		}
		id, err := store.InsertMemory(db, &mem)
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
		mems, err := store.SearchMemories(db, project, in.Query, limit)
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
		mems, err := store.ListMemories(db, project, limit)
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
		mems, err := store.SearchMemories(db, project, "", 200)
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
		active, _ := store.ActiveSession(db, project)
		if active != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("⚠ Ya hay una sesión activa desde %s", active.CreatedAt)}},
			}, nil, nil
		}
		sess, err := store.StartSession(db, project)
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
		sess, err := store.ActiveSession(db, project)
		if err != nil {
			return nil, nil, err
		}
		if sess == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No hay sesión activa para cerrar"}},
			}, nil, nil
		}
		if err := store.EndSession(db, sess.ID, in.Summary); err != nil {
			return nil, nil, fmt.Errorf("cerrar sesión: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("✓ Sesión %s finalizada", sess.ID[:8])}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_context",
		Description: "Obtiene el contexto completo del proyecto como markdown: arquitectura, decisiones, bugs, aprendizajes",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		builder := memctx.New(db, root, project)
		output, err := builder.Build()
		if err != nil {
			return nil, nil, fmt.Errorf("generar contexto: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: output}},
		}, nil, nil
	})
}

func registerResources(server *mcp.Server, db *sql.DB, root, project string) {
	builder := memctx.New(db, root, project)

	server.AddResource(&mcp.Resource{
		URI:         "mem://context",
		Name:        "Project Context",
		Description: "Contexto completo del proyecto en markdown",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		output, err := builder.Build()
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
		mems, err := store.SearchMemories(db, project, "", 200)
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
