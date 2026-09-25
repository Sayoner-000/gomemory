package usecases_test

import (
	"context"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type relojFijo struct{ t time.Time }

func (r relojFijo) Now() time.Time { return r.t }

// T065 — el ajuste adaptativo baja la agresividad solo con muestra suficiente,
// nunca la sube solo, y el nuevo escalón cambia los umbrales.
func TestAdaptiveLowersAggressiveness(t *testing.T) {
	ctx := context.Background()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	clk := relojFijo{time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	stats := persistence.NewCompressionStatsRepository(db, clk)
	tuning := persistence.NewCompressionTuningRepository(db, clk)
	meta := ports.OriginalMeta{Compressor: "code", ContentType: "code"}

	// 19 omisiones: muestra insuficiente aunque todo se recupere.
	_ = stats.Record(ctx, "p", ports.CompressionResult{Compressor: "code", ContentType: "code", Omissions: 19})
	for i := 0; i < 10; i++ {
		if lowered, _ := usecases.RecordRetrievalAndTune(ctx, stats, tuning, "p", meta, 0); lowered {
			t.Fatal("con 19 omisiones no debe ajustarse")
		}
	}

	// 20 omisiones nuevas y 5 recuperaciones más (25 %) → baja a 2.
	db2, _ := persistence.Init(t.TempDir())
	defer func() { _ = db2.Close() }()
	stats2 := persistence.NewCompressionStatsRepository(db2, clk)
	tuning2 := persistence.NewCompressionTuningRepository(db2, clk)
	_ = stats2.Record(ctx, "p", ports.CompressionResult{Compressor: "code", ContentType: "code", Omissions: 20})
	var lowered bool
	for i := 0; i < 5; i++ {
		lowered, _ = usecases.RecordRetrievalAndTune(ctx, stats2, tuning2, "p", meta, 0)
	}
	if !lowered {
		t.Fatal("25 % de recuperación sobre 20 omisiones debe bajar la agresividad")
	}
	a, _ := tuning2.Get(ctx, "p", "code")
	if a != 2 {
		t.Fatalf("agresividad esperada 2, obtenida %d", a)
	}
	list, _ := tuning2.List(ctx, "p")
	if len(list) != 1 || list[0].Reason == "" {
		t.Errorf("el ajuste debe registrar su motivo: %+v", list)
	}
	// Una recuperación más no vuelve a bajar: la tasa se mide desde el ajuste.
	if again, _ := usecases.RecordRetrievalAndTune(ctx, stats2, tuning2, "p", meta, 0); again {
		t.Error("no debe bajar dos veces por las mismas recuperaciones")
	}
	// El nuevo escalón conserva más.
	if domain.ThresholdsFor(a).CodeMinBodyLines <= domain.ThresholdsFor(domain.AggressivenessMax).CodeMinBodyLines {
		t.Error("el escalón 2 debe exigir cuerpos más largos para omitirlos")
	}
	_ = tuning2.Reset(ctx, "p", "code")
	if a, _ := tuning2.Get(ctx, "p", "code"); a != domain.AggressivenessMax {
		t.Error("Reset debe devolver la agresividad máxima")
	}
}
