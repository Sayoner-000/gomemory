package persistence

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/compression/native"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// C-003 (acr_961a1676) — en un documento mixto (prosa con un bloque JSON), las
// omisiones del bloque y sus recuperaciones se cuentan con la misma clave, así
// el ajuste adaptativo (FR-027) puede decidir sobre ese tipo.
func TestAdaptiveTuningWithMixedDocument(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	store := NewOriginalStoreRepository(db, clk, 7, 64)
	stats := NewCompressionStatsRepository(db, clk)
	tuning := NewCompressionTuningRepository(db, clk)
	engine := native.NewEngine(store, stats, tuning, "p")

	var sb strings.Builder
	sb.WriteString("Resultado de la consulta al servicio de pedidos del día.\n\n```json\n[")
	for i := 0; i < 60; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":%d,"status":200,"region":"eu-west","ok":true}`, i)
	}
	sb.WriteString("]\n```\n\nFin del informe de pedidos.\n")

	res, err := engine.Compress(sb.String(), ports.CompressionOptions{Level: ports.CompressionMax})
	if err != nil || !res.Compressed || res.ContentType != string(domain.ContentMixed) {
		t.Fatalf("se esperaba compresión de un documento mixto: %v %+v", err, res)
	}
	if res.Omissions < domain.CompressionAdaptiveMinOmissions || len(res.Refs) != 1 {
		t.Fatalf("muestra insuficiente para el ajuste: omisiones=%d refs=%v", res.Omissions, res.Refs)
	}

	_, meta, found, err := store.Get(ctx, res.Refs[0])
	if err != nil || !found || meta.ContentType != string(domain.ContentJSON) {
		t.Fatalf("original: found=%v meta=%+v err=%v", found, meta, err)
	}
	om, _, err := tuning.SinceLastAdjustment(ctx, "p", meta.ContentType)
	if err != nil || om != res.Omissions {
		t.Fatalf("las omisiones del bloque deben contarse con su tipo: %d, se esperaban %d (%v)", om, res.Omissions, err)
	}

	lowered := false
	for i := 0; i < res.Omissions && !lowered; i++ {
		lowered, err = usecases.RecordRetrievalAndTune(ctx, stats, tuning, "p", meta, 0.2)
		if err != nil {
			t.Fatal(err)
		}
	}
	if a, _ := tuning.Get(ctx, "p", meta.ContentType); !lowered || a != domain.AggressivenessMax-1 {
		t.Errorf("con recuperación por encima del umbral la agresividad debe bajar: lowered=%v a=%d", lowered, a)
	}

	// El total de omisiones no se duplica entre la fila del documento y la del bloque.
	sum, _ := stats.Summary(ctx, "p")
	total := 0
	for _, s := range sum {
		total += s.Omissions
	}
	if total != res.Omissions {
		t.Errorf("total de omisiones = %d, se esperaba %d: %+v", total, res.Omissions, sum)
	}
}
