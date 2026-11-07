package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func TestHCD001DeliveryReceipt(t *testing.T) {
	// Desde HCD-044 un filepath privado se rechaza al guardar; solo puede
	// llegar en una fila histórica: se guarda por la vía normal y se degrada.
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := persistence.NewMemoryRepository(db)
	m := domain.Memory{Project: "receipt", Type: domain.Learning, Title: "recibo", Content: strings.Repeat("recibo detallado ", 100), Filepath: "ruta-publica"}
	id, err := repo.Insert(&m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE memories SET filepath = '<private>ruta-sensible</private>' WHERE id = ?`, id); err != nil {
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
