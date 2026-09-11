package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"mem/application/ports"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTool registra una tool como mcp.AddTool, pero publica un esquema de
// entrada sin uniones con "null". El SDK infiere slices y punteros como
// {"type":["null","array"]} / ["null","boolean"], y hay modelos (visto en
// OpenCode con review_start.scope y review_start.fix_authorized) que ante esa
// unión emiten un valor vacío — `"fix_authorized": .` — y la llamada muere
// como JSON inválido antes de llegar al handler. Todos esos campos son
// opcionales (omitempty): omitirlos es la forma de "no valor", no enviar null.
// El esquema se fija en la tool, así que el SDK valida contra el mismo que
// publica.
func addTool[In, Out any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
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
}

// measurePublishedSchemas mide el costo en tokens de los descriptores de
// operación que gomemory PUBLICA (feature 020, FR-007). No hay forma de
// extraer esos descriptores replicando a mano las llamadas mcp.AddTool: cada
// una usa una struct anónima distinta como parámetro de tipo genérico, y el
// esquema JSON lo infiere el SDK, no está escrito en ningún literal
// recorrible (research.md §3). En vez de eso, se levanta el servidor REAL
// sobre un transporte en memoria (sin stdio, sin proceso aparte) y se
// serializa lo que él mismo responde a ListTools — así la medición sigue
// siendo correcta cuando se añada una operación nueva, sin tocar este código.
//
// Solo mide TOOLS (root, project) — no resources: la cuenta de "19
// operaciones publicadas" verificada en research.md §3 se refiere a
// mcp.AddTool, que es lo que un cliente ve como capacidad invocable.
func measurePublishedSchemas(deps *Deps, root, project string, counter ports.TokenCounter) (schemaTokens, schemaOperations int, err error) {
	server := mcp.NewServer(&mcp.Implementation{Name: "gomemory-schema-probe", Version: "internal"}, nil)
	registerTools(server, deps, project)
	registerCodeTools(server, deps, root, project)

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = serverSession.Close() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "gomemory-schema-probe-client", Version: "internal"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = clientSession.Close() }()

	result, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		return 0, 0, err
	}

	total := 0
	for _, tool := range result.Tools {
		data, err := json.Marshal(tool)
		if err != nil {
			continue // best-effort: un descriptor que no serializa no debe tumbar la medición completa
		}
		total += counter.Count(string(data))
	}
	return total, len(result.Tools), nil
}
