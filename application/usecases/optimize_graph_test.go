package usecases_test

import (
	"context"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type mockMaintenanceRepo struct {
	report domain.StorageOptimizationReport
	called bool
}

func (m *mockMaintenanceRepo) Stats(project string) (ports.StorageStats, error) {
	return ports.StorageStats{}, nil
}
func (m *mockMaintenanceRepo) Purge(filter ports.PurgeFilter) (int64, error) { return 0, nil }
func (m *mockMaintenanceRepo) Compact() (int64, int64, error)                { return 0, 0, nil }
func (m *mockMaintenanceRepo) OptimizeStorage(ctx context.Context, project string, dryRun bool) (domain.StorageOptimizationReport, error) {
	m.called = true
	m.report.Project = project
	m.report.DryRun = dryRun
	return m.report, nil
}

func TestOptimizeGraph_UseCase(t *testing.T) {
	mockRepo := &mockMaintenanceRepo{
		report: domain.StorageOptimizationReport{
			OrphanRelationsCleaned:   3,
			DuplicateRelationsMerged: 2,
			SelfRelationsCleaned:     1,
			OrphanCodeNodesCleaned:   4,
			OrphanCodeEdgesCleaned:   5,
			FTSOptimized:             true,
			DatabaseOptimized:        true,
		},
	}

	uc := usecases.NewOptimizeGraph(mockRepo, nil)
	out, err := uc.Execute(context.Background(), usecases.OptimizeGraphInput{
		Project: "proj",
		DryRun:  true,
		Target:  "internal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mockRepo.called {
		t.Errorf("se esperaba llamada a OptimizeStorage")
	}
	if !out.StorageReport.DryRun {
		t.Errorf("se esperaba DryRun = true")
	}
	if out.StorageReport.OrphanRelationsCleaned != 3 {
		t.Errorf("got %d orphan relations, want 3", out.StorageReport.OrphanRelationsCleaned)
	}
	if out.StorageReport.TotalCleaned() != 15 {
		t.Errorf("TotalCleaned = %d, want 15", out.StorageReport.TotalCleaned())
	}
}

func TestOptimizeGraph_InvalidTarget(t *testing.T) {
	mockRepo := &mockMaintenanceRepo{}
	uc := usecases.NewOptimizeGraph(mockRepo, nil)

	for _, invalid := range []string{"typo", "internla", "invalid", "123"} {
		_, err := uc.Execute(context.Background(), usecases.OptimizeGraphInput{
			Project: "proj",
			Target:  invalid,
		})
		if err == nil {
			t.Errorf("target %q debió devolver error", invalid)
		}
		if mockRepo.called {
			t.Errorf("target inválido %q no debió llamar al repositorio", invalid)
		}
	}

	// Vacío y válidos deben pasar
	for _, valid := range []string{"", "all", "internal", "external"} {
		mockRepo.called = false
		_, err := uc.Execute(context.Background(), usecases.OptimizeGraphInput{
			Project: "proj",
			Target:  valid,
		})
		if err != nil {
			t.Errorf("target válido %q falló: %v", valid, err)
		}
	}
}
