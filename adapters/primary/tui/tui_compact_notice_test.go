package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"mem/application/ports"
)

// TestConfigScreen_CompactAgentNoticeApareceYAlternaPersiste cubre US4
// (feature 030): la fila «Aviso de compactación al agente» debe aparecer en
// la pantalla de Configuración, junto al umbral, y alternarla debe persistir
// compact_agent_notice.
func TestConfigScreen_CompactAgentNoticeApareceYAlternaPersiste(t *testing.T) {
	data := &ports.SettingsData{}
	m := model{
		screen:       screenConfig,
		settingsRepo: tuiSettingsStub{data: data},
		configCursor: configRowCompactAgentNotice,
		width:        100,
		height:       40,
		ready:        true,
	}

	if !strings.Contains(ansi.Strip(m.configView()), "Aviso de compactación al agente: OFF") {
		t.Fatal("la vista debía mostrar la fila apagada por defecto")
	}

	updated, _ := m.updateConfig(keyMsg("enter"))
	if !data.CompactAgentNotice {
		t.Fatal("al confirmar sobre el interruptor debe activarse")
	}

	m2 := updated.(model)
	if !strings.Contains(ansi.Strip(m2.configView()), "Aviso de compactación al agente: ON") {
		t.Error("la vista debía reflejar el nuevo estado")
	}

	m2.configCursor = configRowCompactAgentNotice
	if _, _ = m2.updateConfig(keyMsg("enter")); data.CompactAgentNotice {
		t.Fatal("al confirmar de nuevo debe apagarse")
	}
}
