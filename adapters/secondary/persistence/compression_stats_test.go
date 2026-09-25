package persistence

import (
	"context"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// T064 — acumulados por compresor, sin contenido.
func TestCompressionStats(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	repo := NewCompressionStatsRepository(db, clk)
	r := ports.CompressionResult{Compressor: "log", ContentType: "log", RawTokens: 1000, StructuralTokens: 900, Tokens: 100, Omissions: 50, LatencyMicros: 300}
	_ = repo.Record(ctx, "p", r)
	r.FallbackReason = "no_gain"
	_ = repo.Record(ctx, "p", r)
	_ = repo.RecordRetrieval(ctx, "p", "log", "log")
	_ = repo.Record(ctx, "p", ports.CompressionResult{Compressor: "json", ContentType: "json", RawTokens: 10, Tokens: 5})

	sum, err := repo.Summary(ctx, "p")
	if err != nil || len(sum) != 2 {
		t.Fatalf("Summary: %v %+v", err, sum)
	}
	var logRow ports.CompressorStats
	for _, s := range sum {
		if s.Compressor == "log" {
			logRow = s
		}
	}
	if logRow.Uses != 2 || logRow.RawTokens != 2000 || logRow.FinalTokens != 200 || logRow.Omissions != 100 ||
		logRow.Retrievals != 1 || logRow.Fallbacks != 1 || logRow.LatencyMicrosTotal != 600 || logRow.StructuralTokens != 1800 {
		t.Errorf("acumulado inesperado: %+v", logRow)
	}
	// Solo cifras: ninguna columna de texto libre.
	cols, _ := db.Query(`SELECT name, type FROM pragma_table_info('compression_stats')`)
	defer func() { _ = cols.Close() }()
	for cols.Next() {
		var name, typ string
		_ = cols.Scan(&name, &typ)
		if typ == "TEXT" && name != "project" && name != "compressor" && name != "content_type" && name != "updated_at" {
			t.Errorf("columna de texto inesperada: %s", name)
		}
	}
}

func TestCompressionTuningRepo(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	stats := NewCompressionStatsRepository(db, clk)
	tuning := NewCompressionTuningRepository(db, clk)
	if a, _ := tuning.Get(ctx, "p", "code"); a != domain.AggressivenessMax {
		t.Fatalf("sin fila debe ser máxima: %d", a)
	}
	_ = stats.Record(ctx, "p", ports.CompressionResult{Compressor: "code", ContentType: "code", Omissions: 30})
	_ = tuning.Lower(ctx, "p", "code", "prueba")
	if a, _ := tuning.Get(ctx, "p", "code"); a != 2 {
		t.Errorf("Lower debe bajar a 2: %d", a)
	}
	if om, _, _ := tuning.SinceLastAdjustment(ctx, "p", "code"); om != 0 {
		t.Errorf("tras ajustar, la línea base debe absorber las omisiones previas: %d", om)
	}
	list, _ := tuning.List(ctx, "p")
	if len(list) != 1 || list[0].Reason != "prueba" {
		t.Errorf("List: %+v", list)
	}
	_ = tuning.Reset(ctx, "p", "")
	if a, _ := tuning.Get(ctx, "p", "code"); a != domain.AggressivenessMax {
		t.Error("Reset debe volver a la máxima")
	}
}
