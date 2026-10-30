package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func TestHCD001ReductionAccounting(t *testing.T) {
	repo := newContextPackTestDeps(t)
	for _, m := range []domain.Memory{
		{Project: "account", Type: domain.Learning, Title: "sesion", Content: "sesion usa un reloj monotónico"},
		{Project: "account", Type: domain.Preference, Title: "sesion larga", Content: strings.Repeat("sesion detallada  ", 300)},
	} {
		if _, err := repo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"sesion", "zzsincoincidenciazz"} {
		pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{Project: "account", Task: query, MaxTokens: 100, Compression: ports.CompressionStructural})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(pack.Stats)
		if err != nil {
			t.Fatal(err)
		}
		var stats map[string]any
		if err := json.Unmarshal(data, &stats); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"SelectionSavedTokens", "CompressionSavedTokens", "DeltaSavedTokens", "MetadataTokens", "RecoveryTokens", "CounterKind"} {
			if _, ok := stats[key]; !ok {
				t.Errorf("métrica atribuible ausente: %s", key)
			}
		}
		if selection, ok := stats["SelectionSavedTokens"].(float64); ok {
			compression := stats["CompressionSavedTokens"].(float64)
			if int(selection+compression) != pack.Stats.SavedTokens {
				t.Fatal("reducciones superpuestas o no conservadas")
			}
		}
	}
}
