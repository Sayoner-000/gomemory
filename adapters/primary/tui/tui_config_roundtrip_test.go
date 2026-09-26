package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

	"mem/application/ports"
	"mem/domain"
)

func TestConfig_TogglesVuelvenAlEstadoInicial(t *testing.T) {
	rows := []int{
		0, 1, 4, 5, configRowAtomicPlan, configRowPlanGuard,
		configRowOctopus, configRowCompactAgentNotice, configRowToolOutput,
		configRowConcise,
	}
	for _, row := range rows {
		data := &ports.SettingsData{}
		m := newTestModel(nil, 30)
		m.root = t.TempDir()
		m.screen = screenConfig
		m.settingsRepo = tuiSettingsStub{data: data}
		m.configCursor = row
		for i := 0; i < 2; i++ {
			next, _ := m.updateConfig(keyMsg("enter"))
			m = next.(model)
			if m.screen != screenConfig || strings.TrimSpace(m.statusMsg) == "" {
				t.Fatalf("fila %d pasada %d: no informó el cambio", row, i+1)
			}
		}
	}
}

func TestConfig_EntradasAbrenLaPantallaCorrecta(t *testing.T) {
	for _, tc := range []struct {
		row  int
		want screen
	}{
		{3, screenImport},
		{configRowEditBudget, screenEditSetting},
		{configRowEditCompactThreshold, screenEditSetting},
		{configRowEditDedupDays, screenEditSetting},
		{configRowDocsBase, screenDocs},
	} {
		m := newTestModel(nil, 30)
		m.root = t.TempDir()
		m.screen = screenConfig
		m.settingsRepo = tuiSettingsStub{data: &ports.SettingsData{}}
		m.importPath = textinput.New()
		m.editSettingInput = textinput.New()
		m.configCursor = tc.row
		next, _ := m.updateConfig(keyMsg("enter"))
		got := next.(model)
		if got.screen != tc.want {
			t.Fatalf("fila %d: pantalla %d; esperaba %d", tc.row, got.screen, tc.want)
		}
	}

	data := &ports.SettingsData{}
	m := newTestModel(nil, 30)
	m.root = t.TempDir()
	m.screen = screenConfig
	m.settingsRepo = tuiSettingsStub{data: data}
	m.configCursor = configRowCompressionLevel
	for _, want := range []string{domain.CompressionLevelMax, domain.CompressionLevelNone, domain.CompressionLevelStructural} {
		next, _ := m.updateConfig(keyMsg("enter"))
		m = next.(model)
		if data.ContextCompressionLevel != want {
			t.Fatalf("nivel %q; esperaba %q", data.ContextCompressionLevel, want)
		}
	}
}
