package persistence

import (
	"context"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/application/ports"
)

// T068 (feature 035, FR-024): no ganar nada no es una degradación. Antes, las
// 276 salidas sin ganancia figuraban como «degradaciones» en mem pack savings.
func TestCompressionStats_SinGananciaNoEsDegradacion(t *testing.T) {
	db := openTestDB(t)
	r := NewCompressionStatsRepository(db, clock.SystemClock{})
	ctx := context.Background()
	_ = r.Record(ctx, "p", ports.CompressionResult{Compressor: "structural", ContentType: "unknown", FallbackReason: "no_gain", RawTokens: 10, Tokens: 10})
	_ = r.Record(ctx, "p", ports.CompressionResult{Compressor: "structural", ContentType: "unknown", FallbackReason: "literal_guard", RawTokens: 10, Tokens: 10})
	rows, err := r.Summary(ctx, "p")
	if err != nil || len(rows) != 1 {
		t.Fatalf("Summary: %v %+v", err, rows)
	}
	if rows[0].Fallbacks != 1 || rows[0].NoGains != 1 {
		t.Errorf("no_gain va a «sin ganancia» y literal_guard a «degradaciones»: %+v", rows[0])
	}
}

func TestCompressionStats_ColumnaNoGainsIdempotente(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		db, err := Init(dir)
		if err != nil {
			t.Fatalf("init %d: %v", i, err)
		}
		_ = db.Close()
	}
}
