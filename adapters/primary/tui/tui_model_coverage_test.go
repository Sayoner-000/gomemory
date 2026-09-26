package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"mem/application/ports"
)

func TestInitialModel_PreparaFormularioYDespachaEventos(t *testing.T) {
	mems := manyMemories(2)
	m := initialModel(&fakeMemRepo{mems: mems}, nil, tuiSettingsStub{data: &ports.SettingsData{}}, fakeMaintenanceRepo{}, nil, t.TempDir(), "demo", UsageDeps{})
	if len(m.memories) != 2 || m.saveType.Value() != "learning" || m.screen != screenList {
		t.Fatalf("modelo inicial incompleto: memorias=%d tipo=%q pantalla=%d", len(m.memories), m.saveType.Value(), m.screen)
	}
	if m.Init() == nil {
		t.Fatal("Init debe programar el cursor")
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(model)
	if !m.ready || m.width != 100 || m.height != 30 {
		t.Fatalf("tamaño de terminal ignorado: %+v", m)
	}
	_ = m.View()
	next, _ = m.Update(keyMsg("enter"))
	m = next.(model)
	if m.screen != screenDetail {
		t.Fatalf("enter no abrió detalle: %d", m.screen)
	}
	next, _ = m.Update(keyMsg("esc"))
	if next.(model).screen != screenList {
		t.Fatal("esc no volvió a lista")
	}
}
