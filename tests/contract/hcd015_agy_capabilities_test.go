package main

import (
	"strings"
	"testing"

	"mem/domain"
)

func TestHCD015AgyCapabilities(t *testing.T) {
	var agy *domain.AgentCapability
	for i := range domain.KnownAgents {
		if domain.KnownAgents[i].Name == "agy" {
			agy = &domain.KnownAgents[i]
		}
	}
	if agy == nil {
		t.Fatal("agy no figura en domain.KnownAgents")
	}
	if !agy.Scopes[domain.ScopeUser] || agy.Scopes[domain.ScopeProject] {
		t.Errorf("agy se instala solo en ámbito de usuario: %v", agy.Scopes)
	}
	if !agy.HasLevel(domain.AgentLevelEntry) || agy.GuardUnavailableReason == "" {
		t.Error("agy debe declarar la entrada y el motivo de no tener guardián de plan")
	}
	for _, capability := range domain.AllCompactionCapabilities() {
		if strings.TrimSpace(agy.CompactionUnavailable[capability]) == "" && !agy.Compaction[capability] {
			t.Errorf("capacidad %s sin declarar", capability)
		}
	}
	if !strings.Contains(agy.CompactionUnavailable[domain.CompactionPostCompactChannel], "compactación") {
		t.Error("falta el motivo de no avisar tras compactar")
	}
}
