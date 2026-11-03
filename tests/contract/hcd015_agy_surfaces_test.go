package main

import (
	"os"
	"strings"
	"testing"
)

func TestHCD015AgentSurfaces(t *testing.T) {
	for path, wants := range map[string][]string{
		"../../installer/src/core.ts":     {"'agy'"},
		"../../README.md":                 {"mem install --agents agy", "does not compress its tool output automatically"},
		"../../docs/AGENT-INTEGRATION.md": {"mem hook agy <evento>", "no hay compresión automática de salidas"},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s no contiene %q", path, want)
			}
		}
	}
}
