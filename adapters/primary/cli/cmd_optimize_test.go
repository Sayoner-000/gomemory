package cli

import (
	"context"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

type optimizeTestMaintRepo struct {
	report domain.StorageOptimizationReport
	called bool
}

func (m *optimizeTestMaintRepo) Stats(project string) (ports.StorageStats, error) {
	return ports.StorageStats{}, nil
}
func (m *optimizeTestMaintRepo) Purge(filter ports.PurgeFilter) (int64, error) { return 0, nil }
func (m *optimizeTestMaintRepo) Compact() (int64, int64, error)                { return 0, 0, nil }
func (m *optimizeTestMaintRepo) OptimizeStorage(ctx context.Context, project string, dryRun bool) (domain.StorageOptimizationReport, error) {
	m.called = true
	m.report.Project = project
	m.report.DryRun = dryRun
	return m.report, nil
}

type optimizeTestProjectRepo struct {
	root string
}

func (p *optimizeTestProjectRepo) FindRoot() (string, error)   { return p.root, nil }
func (p *optimizeTestProjectRepo) Key(root string) string      { return "test-proj" }
func (p *optimizeTestProjectRepo) EnsureDir(root string) error { return nil }
func (p *optimizeTestProjectRepo) MemDir() string              { return "" }
func (p *optimizeTestProjectRepo) DbPath(root string) string   { return "" }
func (p *optimizeTestProjectRepo) Init(root string) error      { return nil }
func (p *optimizeTestProjectRepo) MigrateLegacy(root string, force bool) (bool, error) {
	return false, nil
}

func TestCmdOptimize_Help(t *testing.T) {
	out := captureStdout(t, func() {
		CmdOptimize(nil, []string{"--help"})
	})
	if !strings.Contains(out, "mem optimize") {
		t.Errorf("se esperaba ayuda de mem optimize, got:\n%s", out)
	}
}

func TestCmdOptimize_DryRun(t *testing.T) {
	maint := &optimizeTestMaintRepo{
		report: domain.StorageOptimizationReport{
			OrphanRelationsCleaned:   2,
			DuplicateRelationsMerged: 1,
			OrphanCodeNodesCleaned:   3,
			OrphanCodeEdgesCleaned:   4,
		},
	}
	deps := &Deps{
		ProjectRepo:     &optimizeTestProjectRepo{root: t.TempDir()},
		MaintenanceRepo: maint,
	}

	out := captureStdout(t, func() {
		CmdOptimize(deps, []string{"--dry-run"})
	})

	if !maint.called {
		t.Errorf("se esperaba llamada a OptimizeStorage")
	}
	if !maint.report.DryRun {
		t.Errorf("se esperaba DryRun = true")
	}
	if !strings.Contains(out, "simulación") && !strings.Contains(out, "dry-run") {
		t.Errorf("se esperaba indicación de simulación/dry-run en la salida, got:\n%s", out)
	}
}

func TestCmdOptimize_YesApplies(t *testing.T) {
	maint := &optimizeTestMaintRepo{
		report: domain.StorageOptimizationReport{
			OrphanRelationsCleaned:   1,
			DuplicateRelationsMerged: 1,
			FTSOptimized:             true,
			DatabaseOptimized:        true,
		},
	}
	deps := &Deps{
		ProjectRepo:     &optimizeTestProjectRepo{root: t.TempDir()},
		MaintenanceRepo: maint,
	}

	out := captureStdout(t, func() {
		CmdOptimize(deps, []string{"--yes"})
	})

	if !maint.called {
		t.Errorf("se esperaba llamada a OptimizeStorage")
	}
	if maint.report.DryRun {
		t.Errorf("se esperaba DryRun = false con --yes")
	}
	if !strings.Contains(out, "optimizado") && !strings.Contains(out, "relaciones") {
		t.Errorf("se esperaba reporte de optimización en salida, got:\n%s", out)
	}
}

type mockCodeProviderMaintainer struct {
	name      string
	available bool
	cleaned   int
}

func (m *mockCodeProviderMaintainer) Name() string { return m.name }
func (m *mockCodeProviderMaintainer) Snapshot() domain.CodeProviderSnapshot {
	return domain.CodeProviderSnapshot{Available: m.available, Provider: m.name}
}
func (m *mockCodeProviderMaintainer) MaybeRefresh() {}
func (m *mockCodeProviderMaintainer) ImpactFor(string) (domain.CodeImpactAnnotation, bool) {
	return domain.CodeImpactAnnotation{}, false
}
func (m *mockCodeProviderMaintainer) Optimize(ctx context.Context, dryRun bool, reindex bool) (domain.ExternalProviderOptimizeReport, error) {
	return domain.ExternalProviderOptimizeReport{
		Provider:         m.name,
		Available:        m.available,
		SnapshotsCleaned: m.cleaned,
		Reindexed:        reindex,
		Nodes:            100,
		Edges:            200,
		StatusSummary:    "activo",
	}, nil
}

func TestCmdOptimize_ExternalProviders(t *testing.T) {
	mockProvider := &mockCodeProviderMaintainer{
		name:      "codebase-memory-mcp",
		available: true,
		cleaned:   2,
	}
	deps := &Deps{
		ProjectRepo:   &optimizeTestProjectRepo{root: t.TempDir()},
		CodeProviders: []ports.CodeGraphProvider{mockProvider},
	}

	out := captureStdout(t, func() {
		CmdOptimize(deps, []string{"--yes", "--target=external", "--reindex-external"})
	})

	if !strings.Contains(out, "codebase-memory-mcp") {
		t.Errorf("se esperaba mención al proveedor externo en salida, got:\n%s", out)
	}
	if !strings.Contains(out, "snapshot(s) huérfano(s) purgado(s)") {
		t.Errorf("se esperaba reporte de snapshots purgados, got:\n%s", out)
	}
	if !strings.Contains(out, "reindexado completo") {
		t.Errorf("se esperaba reporte de reindexado completo, got:\n%s", out)
	}
}
