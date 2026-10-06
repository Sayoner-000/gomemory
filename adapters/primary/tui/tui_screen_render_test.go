package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"mem/application/ports"
	"mem/application/usecases"
)

func TestRenderView_TodasLasPantallasMuestranContenido(t *testing.T) {
	defer applyTheme()
	for _, theme := range []string{"dark", "light", "matrix"} {
		t.Run(theme, func(t *testing.T) {
			t.Setenv("GOMEMORY_THEME", theme)
			applyTheme()
			renderAllScreens(t)
		})
	}
}

func renderAllScreens(t *testing.T) {
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
			for _, size := range [][2]int{{40, 12}, {60, 18}, {80, 24}, {120, 40}, {200, 60}} {
				t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
					updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
					resized := updated.(model)
					got := resized.renderView()
					if strings.Contains(got, footerBoundary) {
						t.Fatal("la vista expone metadatos internos")
					}
					if strings.TrimSpace(got) == "" {
						t.Fatalf("pantalla %q vacía", tc.name)
					}
					if lipgloss.Width(got) > size[0] || lipgloss.Height(got) > size[1] {
						t.Fatalf("pantalla %q mide %dx%d para terminal %dx%d", tc.name, lipgloss.Width(got), lipgloss.Height(got), size[0], size[1])
					}
				})
			}
		})
	}
}
