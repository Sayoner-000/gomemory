package cli

import (
	"strings"
	"testing"

	"mem/domain"
)

// TestPrintDoctorCompaction_ListaCapacidadesPorAgenteInstalado cubre la Fase
// 7 (feature 030): `mem doctor` debe listar, por cada agente con al menos un
// canal instalado (no missing/not_applicable), las 6 capacidades de
// compactación con "sí" o el motivo declarado de ausencia.
func TestPrintDoctorCompaction_ListaCapacidadesPorAgenteInstalado(t *testing.T) {
	report := domain.CoverageReport{
		Channels: []domain.ActivationChannel{
			{Arm: domain.ArmGomemory, Agent: "claude", Scope: domain.ScopeUser, Kind: domain.KindPlanEntry, State: domain.StateOK},
			{Arm: domain.ArmGomemory, Agent: "opencode", Scope: domain.ScopeUser, Kind: domain.KindPlanEntry, State: domain.StateMissing},
		},
	}

	out := captureStdout(t, func() { printDoctorCompaction(report) })

	if !strings.Contains(out, "claude") {
		t.Fatalf("debía listar claude (tiene un canal OK): %s", out)
	}
	if strings.Contains(out, "opencode") {
		t.Errorf("NO debía listar opencode (su único canal está missing): %s", out)
	}

	claude, _ := domain.AgentByName("claude")
	for _, cap := range domain.AllCompactionCapabilities() {
		if claude.Compaction[cap] {
			if !strings.Contains(out, "sí") {
				t.Errorf("capacidad disponible debía marcarse con 'sí': %s", out)
			}
		} else if !strings.Contains(out, claude.CompactionUnavailable[cap]) {
			t.Errorf("capacidad no disponible debía mostrar su motivo (%q): %s", claude.CompactionUnavailable[cap], out)
		}
	}
}

// TestPrintDoctorCompaction_SinAgentesInstaladosNoImprimeNada evita una
// sección vacía y confusa cuando el diagnóstico no encontró ningún agente
// activo.
func TestPrintDoctorCompaction_SinAgentesInstaladosNoImprimeNada(t *testing.T) {
	report := domain.CoverageReport{}
	out := captureStdout(t, func() { printDoctorCompaction(report) })
	if strings.TrimSpace(out) != "" {
		t.Errorf("sin agentes instalados no debía imprimir nada, got %q", out)
	}
}
