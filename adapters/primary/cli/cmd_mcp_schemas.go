package cli

import (
	"context"
	"encoding/json"

	"mem/application/ports"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "gomemory-schema-probe-client", Version: "internal"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return 0, 0, err
	}
	defer clientSession.Close()

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
