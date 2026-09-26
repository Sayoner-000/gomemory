package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func TestUninstallPlanner_ProyectoIncluyeMemoriaYAplicaPasos(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	dataHome := filepath.Join(base, "data")
	t.Setenv("GOMEMORY_DATA_HOME", dataHome)
	memDir := filepath.Join(project, persistence.MemDir)
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatal(err)
	}
	storeDir, err := persistence.GlobalProjectDir(persistence.ProjectKey(project))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{memDir, storeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(storeDir, "mem.db"), []byte("memory"), 0o600); err != nil {
		t.Fatal(err)
	}
	pl := &uninstallPlanner{o: uninstallOptions{scope: domain.UninstallProject, target: project, memory: domain.MemoryDelete}, data: dataHome}
	if err := pl.plan(); err != nil {
		t.Fatal(err)
	}
	if len(pl.steps) < 3 || pl.steps[0].item.Category != domain.CategoryProjectFiles ||
		pl.steps[len(pl.steps)-1].item.Category != domain.CategoryMemory {
		t.Fatalf("inventario incompleto o desordenado: %+v", pl.steps)
	}
	for _, step := range pl.steps {
		if err := step.apply(); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{memDir, storeDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("no se retiró %s: %v", dir, err)
		}
	}
}

func TestUninstallPlanner_ConfiguracionGlobalConservaEntradasAjenas(t *testing.T) {
	home := t.TempDir()
	claude := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claude, []byte(`{"mcpServers":{"otro":{"command":"otro"},"gomemory":{"command":"mem"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	codex := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codex), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codex, []byte("[mcp_servers.otro]\ncommand = 'otro'\n[mcp_servers.gomemory]\ncommand = 'mem'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pl := &uninstallPlanner{home: home}
	pl.planGlobalAgentConfig()
	if len(pl.steps) < 2 {
		t.Fatalf("faltan entradas globales en el inventario: %+v", pl.steps)
	}
	for _, step := range pl.steps {
		if err := step.apply(); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{claude, codex} {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "otro") || strings.Contains(string(data), "gomemory") {
			t.Fatalf("retirada parcial incorrecta en %s: %v, %q", path, err, data)
		}
	}
}
