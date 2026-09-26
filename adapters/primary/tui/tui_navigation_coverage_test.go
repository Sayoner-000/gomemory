package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

	"mem/application/ports"
)

func navigationModel(t *testing.T) model {
	t.Helper()
	mems := manyMemories(2)
	m := newTestModel(mems, 30)
	m.root = t.TempDir()
	m.memRepo = &fakeMemRepo{mems: mems}
	m.maintenanceRepo = fakeMaintenanceRepo{}
	m.settingsRepo = tuiSettingsStub{data: &ports.SettingsData{}}
	m.saveTitle = textinput.New()
	m.saveType = textinput.New()
	m.saveContent = textinput.New()
	m.saveFilepath = textinput.New()
	m.maintConfirm = textinput.New()
	m.usageTaskInput = textinput.New()
	m.usageBudgetInput = textinput.New()
	return m
}

func TestList_AtajosAbrenPantallasYConservanSeleccion(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want screen
	}{
		{"enter", screenDetail},
		{"s", screenSave},
		{"m", screenMaintenance},
		{"c", screenConfig},
		{"o", screenOptimize},
		{"u", screenUsage},
	} {
		m := navigationModel(t)
		next, _ := m.updateList(keyMsg(tc.key))
		got := next.(model)
		if got.screen != tc.want {
			t.Fatalf("atajo %q abrió pantalla %d, esperaba %d", tc.key, got.screen, tc.want)
		}
	}

	m := navigationModel(t)
	next, _ := m.updateList(keyMsg("j"))
	m = next.(model)
	if m.listCursor != 1 {
		t.Fatalf("j: cursor=%d", m.listCursor)
	}
	next, _ = m.updateList(keyMsg("enter"))
	m = next.(model)
	if m.selected.ID != 2 {
		t.Fatalf("enter: memoria=%d", m.selected.ID)
	}
	next, _ = m.updateDetail(keyMsg("esc"))
	if next.(model).screen != screenList {
		t.Fatal("esc no volvió a la lista")
	}
}

func TestList_AutoApproveYFiltro(t *testing.T) {
	m := navigationModel(t)
	next, _ := m.updateList(keyMsg("a"))
	m = next.(model)
	if !m.autoApprove || !strings.Contains(m.statusMsg, "activado") {
		t.Fatalf("auto-approve no se activó: %+v", m)
	}
	next, _ = m.updateList(keyMsg("a"))
	m = next.(model)
	if m.autoApprove || !strings.Contains(m.statusMsg, "desactivado") {
		t.Fatalf("auto-approve no se desactivó: %+v", m)
	}
	next, _ = m.updateList(keyMsg("/"))
	m = next.(model)
	if !m.filtering {
		t.Fatal("/ no inició el filtro")
	}
	next, _ = m.updateList(keyMsg("enter"))
	if next.(model).filtering {
		t.Fatal("enter no terminó el filtro")
	}
}

func TestMaintenance_AccionesYPantallaDeConfirmacion(t *testing.T) {
	for _, tc := range []struct {
		cursor int
		want   screen
		act    string
	}{
		{0, screenMaintenanceConfirm, "purge"},
		{1, screenMaintenance, ""},
		{2, screenMaintenanceConfirm, "gc"},
		{3, screenMaintenance, ""},
	} {
		m := navigationModel(t)
		m.screen = screenMaintenance
		m.maintCursor = tc.cursor
		next, _ := m.updateMaintenance(keyMsg("enter"))
		got := next.(model)
		if got.screen != tc.want || got.maintAction != tc.act {
			t.Fatalf("acción %d: pantalla=%d acción=%q", tc.cursor, got.screen, got.maintAction)
		}
		if tc.cursor == 1 && !strings.Contains(got.statusMsg, "Compactado") {
			t.Fatalf("compactar no informó resultado: %q", got.statusMsg)
		}
	}
}
