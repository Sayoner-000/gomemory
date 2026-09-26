package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

	"mem/application/ports"
	"mem/application/usecases"
)

func TestRenderView_TodasLasPantallasMuestranContenido(t *testing.T) {
	mems := manyMemories(2)
	m := newTestModel(mems, 30)
	m.root = t.TempDir()
	m.settingsRepo = tuiSettingsStub{data: &ports.SettingsData{}}
	m.selected = mems[0]
	m.saveTitle = textinput.New()
	m.saveType = textinput.New()
	m.saveContent = textinput.New()
	m.saveFilepath = textinput.New()
	m.maintConfirm = textinput.New()
	m.importPath = textinput.New()
	m.docPath = textinput.New()
	m.editSettingInput = textinput.New()
	m.usageTaskInput = textinput.New()
	m.usageBudgetInput = textinput.New()
	m.dupGroups = []usecases.DuplicateGroup{{Type: mems[0].Type, Memories: mems, SuggestedKeepID: mems[0].ID}}
	m.docTemplates = map[string]string{}

	for _, tc := range []struct {
		name   string
		screen screen
	}{
		{"lista", screenList},
		{"detalle", screenDetail},
		{"guardar", screenSave},
		{"mantenimiento", screenMaintenance},
		{"confirmar mantenimiento", screenMaintenanceConfirm},
		{"configuración", screenConfig},
		{"importar", screenImport},
		{"optimizar", screenOptimize},
		{"detalle de duplicados", screenOptimizeDetail},
		{"confirmar duplicados", screenOptimizeConfirm},
		{"confirmar todos", screenOptimizeAllConfirm},
		{"editar ajuste", screenEditSetting},
		{"uso", screenUsage},
		{"documentos", screenDocs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m.screen = tc.screen
			if got := m.renderView(); strings.TrimSpace(got) == "" {
				t.Fatalf("pantalla %q vacía", tc.name)
			}
		})
	}
}
