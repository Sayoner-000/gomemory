package main

import (
	"testing"

	"mem/domain"
)

// TestCompactionCapabilities_INV_C1 verifica la invariante INV-C1 (feature
// 030, data-model.md §2): para cada agente del registro y cada una de las 6
// capacidades de compactación, exactamente una de dos se cumple —la
// capacidad está declarada disponible, o tiene un motivo explícito por el
// que no se usa. Una ausencia sin motivo es indistinguible de un olvido, y
// eso fue justo el defecto que originó el registro de canales (ver
// domain/channel_matrix.go).
func TestCompactionCapabilities_INV_C1(t *testing.T) {
	todas := domain.AllCompactionCapabilities()
	if len(todas) != 6 {
		t.Fatalf("se esperaban 6 capacidades (C1-C6), got %d: %v", len(todas), todas)
	}

	for _, agente := range domain.KnownAgents {
		for _, cap := range todas {
			disponible := agente.Compaction[cap]
			motivo := agente.CompactionUnavailable[cap]
			if disponible && motivo != "" {
				t.Errorf("%s/%s: no puede estar disponible Y tener motivo de ausencia a la vez", agente.Name, cap)
			}
			if !disponible && motivo == "" {
				t.Errorf("%s/%s: ausente sin motivo — declarar disponible o el motivo por el que no se usa", agente.Name, cap)
			}
		}
	}
}

// TestCompactionCapabilities_ValoresConocidos fija los valores de la tabla
// R1 de research.md, verificados contra documentación oficial y los
// binarios/tipos instalados: claude no usa C1 (decisión R12, cubierta por
// C2+C6), codex no ofrece C1 ni C6 y opencode cubre las seis.
func TestCompactionCapabilities_ValoresConocidos(t *testing.T) {
	claude, ok := domain.AgentByName("claude")
	if !ok {
		t.Fatal("claude debe estar en el registro")
	}
	if claude.Compaction[domain.CompactionPreCompactInput] {
		t.Error("claude no debe usar C1 (research.md R12: C2+C6 cubren la misma necesidad)")
	}
	if claude.CompactionUnavailable[domain.CompactionPreCompactInput] == "" {
		t.Error("claude debe declarar el motivo de no usar C1")
	}
	if !claude.Compaction[domain.CompactionPostCompactChannel] {
		t.Error("claude debe declarar C2 (SessionStart matcher=compact)")
	}
	if !claude.Compaction[domain.CompactionSummaryInput] {
		t.Error("claude debe declarar C6 (PostCompact.compact_summary)")
	}
	if !claude.Compaction[domain.CompactionSubagentFinalText] {
		t.Error("claude debe declarar C3 (SubagentStop.last_assistant_message)")
	}

	codex, ok := domain.AgentByName("codex")
	if !ok {
		t.Fatal("codex debe estar en el registro")
	}
	if codex.Compaction[domain.CompactionPreCompactInput] {
		t.Error("codex no ofrece C1 (PreCompact sin aporte de instrucciones)")
	}
	if codex.CompactionUnavailable[domain.CompactionPreCompactInput] == "" {
		t.Error("codex debe declarar el motivo de no ofrecer C1")
	}
	if codex.Compaction[domain.CompactionSummaryInput] {
		t.Error("codex no ofrece C6 (PostCompact sin compact_summary)")
	}
	if codex.CompactionUnavailable[domain.CompactionSummaryInput] == "" {
		t.Error("codex debe declarar el motivo de no ofrecer C6")
	}

	// OpenCode cubre las 6 capacidades (complemento gomemory.ts): INV-C1 solo
	// comprueba la coherencia disponible/motivo, no que siga declarándolas.
	opencode, ok := domain.AgentByName("opencode")
	if !ok {
		t.Fatal("opencode debe estar en el registro")
	}
	for _, cap := range domain.AllCompactionCapabilities() {
		if !opencode.Compaction[cap] {
			t.Errorf("opencode debe declarar %s", cap)
		}
	}
	if len(opencode.CompactionUnavailable) != 0 {
		t.Errorf("opencode no debe declarar motivos de ausencia, got %v", opencode.CompactionUnavailable)
	}
}
