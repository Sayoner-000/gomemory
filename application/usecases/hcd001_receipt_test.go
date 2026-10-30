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

func TestHCD001DeliveryReceipt(t *testing.T) {
	repo := newContextPackTestDeps(t)
	m := domain.Memory{Project: "receipt", Type: domain.Learning, Title: "recibo", Content: strings.Repeat("recibo detallado ", 100), Filepath: "<private>ruta-sensible</private>"}
	if _, err := repo.Insert(&m); err != nil {
		t.Fatal(err)
	}
	pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{Project: "receipt", Task: "recibo", MaxTokens: 1, Compression: ports.CompressionStructural})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	receipts, ok := result["Receipts"].([]any)
	if !ok || len(receipts) != 1 {
		t.Fatalf("recibo de omisión ausente: %s", data)
	}
	r := receipts[0].(map[string]any)
	for _, key := range []string{"ID", "Source", "OriginalSHA256", "DeliveredSHA256", "Status", "Reason", "Refs", "Recovery"} {
		if _, ok := r[key]; !ok {
			t.Errorf("campo ausente: %s", key)
		}
	}
	if r["Status"] != "omitted" || r["Reason"] != "budget" {
		t.Fatalf("causa no atribuible: %v", r)
	}
	if strings.Contains(string(data), "ruta-sensible") {
		t.Fatal("recibo filtra ruta privada")
	}
}
