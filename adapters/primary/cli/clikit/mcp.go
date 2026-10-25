package clikit

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddTool registra una tool como mcp.AddTool, pero publica un esquema de
// entrada sin uniones con "null". El SDK infiere slices y punteros como
// {"type":["null","array"]} / ["null","boolean"], y hay modelos (visto en
// OpenCode con review_start.scope y review_start.fix_authorized) que ante esa
// unión emiten un valor vacío — `"fix_authorized": .` — y la llamada muere
// como JSON inválido antes de llegar al handler. Todos esos campos son
// opcionales (omitempty): omitirlos es la forma de "no valor", no enviar null.
// El esquema se fija en la tool, así que el SDK valida contra el mismo que
// publica.
func AddTool[In, Out any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	if tool.InputSchema == nil {
		schema, err := jsonschema.For[In](nil)
		if err != nil {
			panic(fmt.Sprintf("AddTool: tool %q: %v", tool.Name, err)) // mismo contrato que mcp.AddTool
		}
		dropNullUnions(schema)
		tool.InputSchema = schema
	}
	mcp.AddTool(server, tool, handler)
}

// dropNullUnions reemplaza recursivamente ["null", X] por el tipo simple X.
func dropNullUnions(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) == 2 && s.Types[0] == "null" {
		s.Type, s.Types = s.Types[1], nil
	}
	for _, prop := range s.Properties {
		dropNullUnions(prop)
	}
	dropNullUnions(s.Items)
	dropNullUnions(s.AdditionalProperties)
	// Composición y definiciones: hoy el SDK no las genera, pero si un tipo
	// de entrada llegara a producirlas la unión con null quedaría escondida ahí.
	for _, def := range s.Defs {
		dropNullUnions(def)
	}
	for _, group := range [][]*jsonschema.Schema{s.AllOf, s.AnyOf, s.OneOf} {
		for _, sub := range group {
			dropNullUnions(sub)
		}
	}
}

// ToolResult devuelve value como respuesta JSON de una tool.
func ToolResult(value any) (*mcp.CallToolResult, any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, value, nil
}
