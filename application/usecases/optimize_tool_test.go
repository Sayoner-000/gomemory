package usecases_test

import (
	"testing"

	"mem/adapters/secondary/compression"
	"mem/application/usecases"
)

func TestOptimizeToolDescription_ShortensDescriptionKeepsNameAndSchemaIdentical(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	tool := usecases.ToolDescriptor{
		Name: "search_repositories",
		Description: "This tool provides a comprehensive mechanism that allows users to search repositories.\n\n" +
			"This tool provides a comprehensive mechanism that allows users to search repositories.",
		Schema: schema,
	}

	optimized, err := usecases.OptimizeToolDescription(tool, compression.StructuralCompressor{})
	if err != nil {
		t.Fatalf("OptimizeToolDescription: %v", err)
	}

	if optimized.Name != tool.Name {
		t.Errorf("Name = %q, se esperaba idéntico a %q", optimized.Name, tool.Name)
	}
	if string(optimized.Schema) != string(tool.Schema) {
		t.Errorf("Schema cambió: %q != %q", optimized.Schema, tool.Schema)
	}
	if len(optimized.Description) >= len(tool.Description) {
		t.Errorf("Description no se acortó: %d >= %d caracteres", len(optimized.Description), len(tool.Description))
	}
}

func TestOptimizeToolDescription_EmptyDescription_NoError(t *testing.T) {
	tool := usecases.ToolDescriptor{Name: "noop_tool", Schema: []byte(`{}`)}
	optimized, err := usecases.OptimizeToolDescription(tool, compression.StructuralCompressor{})
	if err != nil {
		t.Fatalf("OptimizeToolDescription: %v", err)
	}
	if optimized.Name != "noop_tool" || string(optimized.Schema) != "{}" {
		t.Errorf("Name/Schema no deberían cambiar con Description vacía: %+v", optimized)
	}
}
