package clikit

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

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
