package usecases

import (
	"strings"
	"testing"
	"time"

	"mem/domain"
)

// HCD-021: la Evidencia se lee en la hora del host, no en UTC sin zona.
func TestHCDEvidence_HoraLocal(t *testing.T) {
	prev := time.Local
	time.Local = time.FixedZone("UTC-5", -5*3600)
	t.Cleanup(func() { time.Local = prev })

	ev := domain.Evidence{Tasks: []string{"1"}, Kind: domain.EvidenceManual, CreatedAt: time.Date(2026, 11, 4, 0, 42, 0, 0, time.UTC)}
	out := renderEvidence([]domain.Evidence{ev}, "", "")
	if !strings.Contains(out, "- 2026-11-03 19:42 -05:00 · tarea 1") {
		t.Fatalf("la hora debe ir en la zona local con su desfase:\n%s", out)
	}
}
