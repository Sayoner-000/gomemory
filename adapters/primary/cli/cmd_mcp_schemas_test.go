package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/secondary/tokens"
	"mem/domain"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMeasurePublishedSchemas_CountsRealServer levanta el servidor MCP REAL
// sobre un transporte en memoria (sin stdio, sin proceso aparte) y mide lo
// que efectivamente publica — no una réplica escrita a mano de las 19
// llamadas mcp.AddTool, que el SDK no permite extraer de forma homogénea
// (research.md §3: cada una usa una struct anónima distinta como parámetro
// de tipo genérico).
func TestMeasurePublishedSchemas_CountsRealServer(t *testing.T) {
	deps := &Deps{Project: "proj"}

	schemaTokens, schemaOperations, err := measurePublishedSchemas(deps, "", "proj", tokens.ApproximateTokenCounter{})
	if err != nil {
		t.Fatalf("measurePublishedSchemas: %v", err)
	}

	// DERIVADO de domain.MCPAllTools(), no un número escrito a mano.
	//
	// Este total se subió a mano tres veces en una sola sesión (19 → 24 → 26 →
	// 27) según se añadían tools. Un número que hay que recordar actualizar es
	// un número que algún día no se actualiza, y entonces el test se relaja
	// «para que pase» en vez de avisar. Derivarlo convierte el recuento en la
	// invariante que de verdad importa: TODA tool declarada en el dominio está
	// efectivamente publicada por el servidor. Registrar una y olvidar
	// declararla —o al revés— ahora falla solo.
	esperadas := len(domain.MCPAllTools())
	if schemaOperations != esperadas {
		t.Fatalf("schemaOperations = %d, se esperaban %d (len(domain.MCPAllTools())): "+
			"una tool registrada sin declarar en el dominio, o declarada sin registrar",
			schemaOperations, esperadas)
	}
	if schemaTokens <= 0 {
		t.Fatalf("schemaTokens = %d, se esperaba un costo mayor que cero", schemaTokens)
	}
}

func TestReviewSubmitPublishedSchemaExplainsValidStatuses(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "gomemory-schema-probe", Version: "internal"}, nil)
	registerReviewTools(server, &Deps{Project: "proj"}, "proj")

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("conectar servidor MCP: %v", err)
	}
	defer func() { _ = serverSession.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "gomemory-schema-probe-client", Version: "internal"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("conectar cliente MCP: %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listar tools MCP: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "review_submit" {
			continue
		}
		published, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("serializar descriptor de review_submit: %v", err)
		}
		for _, expected := range []string{"success|failure", "no submitted, complete ni findings"} {
			if !strings.Contains(string(published), expected) {
				t.Errorf("el descriptor MCP publicado de review_submit no explica %q", expected)
			}
		}
		return
	}
	t.Fatal("review_submit no está publicada")
}

func TestReviewStartPublishedSchemaExplainsScopeArray(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "gomemory-schema-probe", Version: "internal"}, nil)
	registerReviewTools(server, &Deps{Project: "proj"}, "proj")

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("conectar servidor MCP: %v", err)
	}
	defer func() { _ = serverSession.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "gomemory-schema-probe-client", Version: "internal"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("conectar cliente MCP: %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listar tools MCP: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "review_start" {
			continue
		}
		published, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("serializar descriptor de review_start: %v", err)
		}
		for _, expected := range []string{"scope es opcional", "arreglo JSON", "[\\\".\\\"]"} {
			if !strings.Contains(string(published), expected) {
				t.Errorf("el descriptor MCP publicado de review_start no explica %q", expected)
			}
		}
		return
	}
	t.Fatal("review_start no está publicada")
}

// TestPublishedSchemasHaveNoNullUnions protege contra la regresión vista en
// OpenCode: ante {"type":["null","boolean"]} el modelo emitió
// `"fix_authorized": .` y review_start murió como JSON inválido. Recorre lo que
// el servidor REAL publica, así que cubre también las tools que se añadan.
func TestPublishedSchemasHaveNoNullUnions(t *testing.T) {
	deps := &Deps{Project: "proj"}
	server := mcp.NewServer(&mcp.Implementation{Name: "gomemory-schema-probe", Version: "internal"}, nil)
	registerTools(server, deps, "proj")
	registerCodeTools(server, deps, "", "proj")
	registerOctopusTools(server, deps)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("conectar servidor MCP: %v", err)
	}
	defer func() { _ = serverSession.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "gomemory-schema-probe-client", Version: "internal"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("conectar cliente MCP: %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listar tools MCP: %v", err)
	}
	for _, tool := range tools.Tools {
		data, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("serializar esquema de %s: %v", tool.Name, err)
		}
		var schema any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("decodificar esquema de %s: %v", tool.Name, err)
		}
		assertNoNullUnion(t, tool.Name, schema)
	}
}

func assertNoNullUnion(t *testing.T, path string, node any) {
	t.Helper()
	switch v := node.(type) {
	case map[string]any:
		if types, ok := v["type"].([]any); ok {
			t.Errorf("%s publica una unión de tipos %v; los modelos esperan un tipo simple", path, types)
		}
		for key, child := range v {
			assertNoNullUnion(t, path+"."+key, child)
		}
	case []any:
		for _, child := range v {
			assertNoNullUnion(t, path, child)
		}
	}
}

// TestDropNullUnions_RecorreComposicionYDefs: la unión con null también debe
// eliminarse cuando vive dentro de allOf/anyOf/oneOf o de $defs.
func TestDropNullUnions_RecorreComposicionYDefs(t *testing.T) {
	nullable := func() *jsonschema.Schema { return &jsonschema.Schema{Types: []string{"null", "string"}} }
	s := &jsonschema.Schema{
		Type:  "object",
		Defs:  map[string]*jsonschema.Schema{"d": nullable()},
		AllOf: []*jsonschema.Schema{nullable()},
		AnyOf: []*jsonschema.Schema{nullable()},
		OneOf: []*jsonschema.Schema{nullable()},
	}
	dropNullUnions(s)

	for nombre, sub := range map[string]*jsonschema.Schema{
		"$defs": s.Defs["d"], "allOf": s.AllOf[0], "anyOf": s.AnyOf[0], "oneOf": s.OneOf[0],
	} {
		if sub.Type != "string" || sub.Types != nil {
			t.Errorf("%s conserva la unión con null: type=%q types=%v", nombre, sub.Type, sub.Types)
		}
	}
}
