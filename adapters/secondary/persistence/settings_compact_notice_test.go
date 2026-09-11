package persistence

import "testing"

// TestDefaultSettings_CompactAgentNoticeApagadaPorDefecto cubre FR-014/FR-015
// (feature 030, US4): el aviso al agente es opt-in.
func TestDefaultSettings_CompactAgentNoticeApagadaPorDefecto(t *testing.T) {
	if DefaultSettings().CompactAgentNotice {
		t.Error("CompactAgentNotice debía ser false por defecto")
	}
}

// TestSettingsRepository_CompactAgentNotice_RoundTrip cubre el roundtrip
// write/read a través del repositorio (mismo patrón que
// TestSettingsRepository_ContextFields_RoundTrip), incluida la
// retrocompatibilidad de un settings.json sin la clave nueva.
func TestSettingsRepository_CompactAgentNotice_RoundTrip(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()

	s := repo.Read(root)
	if s.CompactAgentNotice {
		t.Fatal("un settings.json sin la clave debía deserializar a false")
	}
	s.CompactAgentNotice = true
	if err := repo.Write(root, s); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := repo.Read(root)
	if !got.CompactAgentNotice {
		t.Error("CompactAgentNotice=true debería sobrevivir un roundtrip write/read")
	}
}

// TestReadSettings_CompactAgentNotice_ClaveJSON verifica el nombre exacto de
// la clave JSON, para que la TUI y las integraciones lo usen sin adivinarlo.
func TestReadSettings_CompactAgentNotice_ClaveJSON(t *testing.T) {
	root := t.TempDir()
	writeRawSettings(t, root, map[string]any{"compact_agent_notice": true})

	if got := ReadSettings(root).CompactAgentNotice; !got {
		t.Errorf("CompactAgentNotice = %v, se esperaba true", got)
	}
}
