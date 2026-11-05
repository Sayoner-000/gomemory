package tui

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"mem/application/ports"
	"mem/domain"
)

type tuiOptimizeMaintRepo struct {
	called  bool
	cleaned int64
}

func (m *tuiOptimizeMaintRepo) Stats(project string) (ports.StorageStats, error) {
	return ports.StorageStats{ProjectMemoryCount: 10, TotalMemoryCount: 20}, nil
}
func (m *tuiOptimizeMaintRepo) Purge(filter ports.PurgeFilter) (int64, error) { return 0, nil }
func (m *tuiOptimizeMaintRepo) Compact() (int64, int64, error)                { return 0, 0, nil }
func (m *tuiOptimizeMaintRepo) OptimizeStorage(ctx context.Context, project string, dryRun bool) (domain.StorageOptimizationReport, error) {
	m.called = true
	return domain.StorageOptimizationReport{
		Project:                project,
		DryRun:                 dryRun,
		OrphanRelationsCleaned: m.cleaned,
	}, nil
}

func TestTUIMaintenance_OptimizeOption(t *testing.T) {
	found := false
	for _, opt := range maintenanceOptions {
		if strings.Contains(strings.ToLower(opt), "optimizar") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("se esperaba una opción 'Optimizar' en maintenanceOptions, got: %v", maintenanceOptions)
	}

	m := model{
		screen:       screenMaintenance,
		project:      "demo-proj",
		maintCursor:  4, // Optimizar
		maintConfirm: textinput.New(),
		ready:        true,
	}

	updated, _ := m.updateMaintenance(keyMsg("enter"))
	got := updated.(model)

	if got.screen != screenMaintenanceConfirm {
		t.Fatalf("se esperaba screenMaintenanceConfirm, got=%v", got.screen)
	}
	if got.maintAction != "optimize" {
		t.Fatalf("se esperaba maintAction='optimize', got=%q", got.maintAction)
	}

	view := got.maintenanceConfirmView()
	if !strings.Contains(view, "Optimizar") {
		t.Errorf("se esperaba 'Optimizar' en maintenanceConfirmView, got:\n%s", view)
	}
}

func TestTUIMaintenance_ExecuteOptimize(t *testing.T) {
	maintRepo := &tuiOptimizeMaintRepo{cleaned: 5}
	ti := textinput.New()
	ti.SetValue("demo-proj")

	m := model{
		screen:          screenMaintenanceConfirm,
		project:         "demo-proj",
		maintAction:     "optimize",
		maintConfirm:    ti,
		maintenanceRepo: maintRepo,
		ready:           true,
	}

	updated, _ := m.updateMaintenanceConfirm(keyMsg("enter"))
	got := updated.(model)

	if !maintRepo.called {
		t.Errorf("se esperaba llamada a OptimizeStorage")
	}
	if got.screen != screenMaintenance {
		t.Errorf("se esperaba retorno a screenMaintenance, got=%v", got.screen)
	}
	if !strings.Contains(got.statusMsg, "Optimización") && !strings.Contains(got.statusMsg, "optimizado") {
		t.Errorf("se esperaba mensaje de optimización en statusMsg, got=%q", got.statusMsg)
	}
}
