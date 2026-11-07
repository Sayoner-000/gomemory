package cli

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerHistoryTools expone el historial de decisiones (HCD-046): las
// fichas de las unidades HCD y de las auditorías, en orden cronológico. Solo
// lectura; el backfill vive en la CLI (`mem hcd index`), que pide --apply.
func registerHistoryTools(server *mcp.Server, deps *Deps) {
	addTool(server, &mcp.Tool{
		Name:        "hcd_history",
		Description: "Historial de decisiones: fichas de unidades HCD y auditorías en orden cronológico (fecha, título, id). Filtra por texto o por archivo enlazado; ábrelas con get_memory.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Text string `json:"text,omitempty" jsonschema:"Texto a buscar en título y contenido (sin distinguir mayúsculas)"`
		File string `json:"file,omitempty" jsonschema:"Ruta relativa de un archivo: solo las fichas que lo citan"`
	}) (*mcp.CallToolResult, any, error) {
		if deps.HistoryQuery == nil {
			return nil, nil, errors.New("el historial de decisiones no está conectado")
		}
		items, err := deps.HistoryQuery.List(in.Text, in.File)
		if err != nil {
			return nil, nil, err
		}
		data, err := json.Marshal(items)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
	})
}
