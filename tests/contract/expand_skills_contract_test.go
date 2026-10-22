package main

import (
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandSkillsContract(t *testing.T) {
	base := filepath.Join("..", "..", "infrastructure", "templates", "expand")
	if _, err := os.Stat(filepath.Join(base, "NOTICE")); err != nil {
		t.Error(err)
	}
	required := map[string][]string{"verify-loop": {"search_memories", "save_memory", "review_start"}, "search-first": {"search_memories", "search_code", "get_symbol", "save_memory"}, "iterative-retrieval": {"pack_build", "pack_retrieve", "octopus_route_task"}, "strategic-compact": {"save_session_summary"}, "instincts": {"save_memory"}}
	for name, tools := range required {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(base, name, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			body := string(raw)
			if errs := domain.ValidateSkillBody(name, body, domain.AllMCPTools()); len(errs) > 0 {
				t.Fatal(errs)
			}
			for _, tool := range tools {
				if !strings.Contains(body, "`"+tool+"`") {
					t.Errorf("missing %s", tool)
				}
			}
		})
	}
}
