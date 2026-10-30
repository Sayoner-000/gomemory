package usecases_test

import (
	"encoding/json"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
)

func TestHCD001NoMatch(t *testing.T) {
	pack, err := usecases.BuildContextPack(newContextPackTestDeps(t), compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{Project: "empty", Task: "zzsincoincidenciazz", MaxTokens: 4000, Renderer: hcd001JSONRenderer{}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(pack)
	var wire map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["MatchStatus"] != "no_match" || len(pack.Items) != 0 {
		t.Fatalf("ausencia no explícita: %s", b)
	}
}
