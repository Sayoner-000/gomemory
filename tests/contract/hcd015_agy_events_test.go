package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestHCD015AgyEventFixtures(t *testing.T) {
	b, err := os.ReadFile("../fixtures/hcd015/events.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version string                    `json:"agy_version"`
		Events  map[string]map[string]any `json:"events"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version == "" {
		t.Fatal("falta agy_version")
	}
	for _, name := range []string{"SessionStart", "PreInvocation", "PreToolUse", "PostToolUse", "Stop"} {
		event, ok := doc.Events[name]
		if !ok {
			t.Fatalf("falta el evento %s", name)
		}
		if id, _ := event["conversationId"].(string); id == "" {
			t.Errorf("%s sin conversationId", name)
		}
		if paths, _ := event["workspacePaths"].([]any); len(paths) == 0 {
			t.Errorf("%s sin workspacePaths", name)
		}
	}
	if _, leaked := doc.Events["PostToolUse"]["toolResponse"]; leaked {
		t.Error("PostToolUse no debe declarar una salida que el host no entrega")
	}
}
