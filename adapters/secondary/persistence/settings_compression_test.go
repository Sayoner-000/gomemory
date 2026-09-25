package persistence

import (
	"testing"

	"mem/application/ports"
)

// T011 — Los campos de compresión sobreviven a la escritura de CUALQUIER otro
// ajuste. SettingsRepository.Write reconstruye Settings desde SettingsData: un
// campo que falte en uno de los dos mapeos se borra en silencio (ya le pasó a
// la política de revisión).
func TestSettingsCompression_SobreviveAOtroWrite(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()

	s := repo.Read(root)
	s.ContextCompressionLevel = "max"
	s.ToolOutputCompression = true
	s.ConciseOutputDirective = true
	s.CompressionMinTokens = 80
	s.CompressionOriginalsTTLDays = 3
	s.CompressionOriginalsMaxMB = 64
	s.CompressionAdaptiveThresholdPct = 30
	if err := repo.Write(root, s); err != nil {
		t.Fatal(err)
	}

	// Otra preferencia cualquiera, escrita después.
	s2 := repo.Read(root)
	s2.AutoApprove = !s2.AutoApprove
	if err := repo.Write(root, s2); err != nil {
		t.Fatal(err)
	}

	got := repo.Read(root)
	want := ports.SettingsData{
		ContextCompressionLevel:         "max",
		ToolOutputCompression:           true,
		ConciseOutputDirective:          true,
		CompressionMinTokens:            80,
		CompressionOriginalsTTLDays:     3,
		CompressionOriginalsMaxMB:       64,
		CompressionAdaptiveThresholdPct: 30,
	}
	if got.ContextCompressionLevel != want.ContextCompressionLevel ||
		got.ToolOutputCompression != want.ToolOutputCompression ||
		got.ConciseOutputDirective != want.ConciseOutputDirective ||
		got.CompressionMinTokens != want.CompressionMinTokens ||
		got.CompressionOriginalsTTLDays != want.CompressionOriginalsTTLDays ||
		got.CompressionOriginalsMaxMB != want.CompressionOriginalsMaxMB ||
		got.CompressionAdaptiveThresholdPct != want.CompressionAdaptiveThresholdPct {
		t.Errorf("ajustes de compresión perdidos tras otro Write: %+v", got)
	}
}

// Sin la clave en settings.json, el nivel llega vacío: el llamador aplica
// domain.ParseCompressionLevel (structural por defecto en instalaciones previas).
func TestSettingsCompression_AusenteEsVacio(t *testing.T) {
	if got := NewSettingsRepository().Read(t.TempDir()).ContextCompressionLevel; got != "" {
		t.Errorf("nivel ausente debe leerse vacío, obtenido %q", got)
	}
}
