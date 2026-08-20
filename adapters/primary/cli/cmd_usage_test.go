package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/domain"
)

// usageSettingsStub aísla el ajuste de ventana de referencia sin tocar el
// doble compartido fakeSettingsRepo de cmd_plan_context_test.go (Principio
// III: los tests existentes son intocables).
type usageSettingsStub struct {
	windowTokens int
}

func (s usageSettingsStub) Read(string) ports.SettingsData {
	return ports.SettingsData{UsageWindowTokens: s.windowTokens}
}
func (s usageSettingsStub) Write(string, ports.SettingsData) error { return nil }
func (s usageSettingsStub) ApplyAutoApprove(string, ports.SettingsData) {}

func newUsageTestDeps(t *testing.T, windowTokens int) (*Deps, func()) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	deps := &Deps{
		Root:         root,
		Project:      "proj",
		UsageRepo:    persistence.NewUsageRepository(db),
		SessionRepo:  persistence.NewSessionRepository(db),
		SettingsRepo: usageSettingsStub{windowTokens: windowTokens},
		TokenCounter: tokens.ApproximateTokenCounter{},
	}
	return deps, func() { db.Close() }
}

func TestCmdUsage_HeaderDeclaresApproximateCounting(t *testing.T) {
	deps, closeDB := newUsageTestDeps(t, 0)
	defer closeDB()
	deps.UsageRepo.Record(domain.UsageRecord{
		Project: "proj", SessionID: "sess-1", Operation: domain.OpSaveMemory,
		Channel: "cli", BaselineTokens: 10, EmittedTokens: 10,
	})

	out := captureStdout(t, func() { CmdUsage(deps, []string{"--session", "sess-1"}) })

	if !strings.Contains(strings.ToLower(out), "aproxima") {
		t.Fatalf("la cabecera debe declarar el método de conteo aproximado, got:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "comparable") {
		t.Fatalf("la cabecera debe declarar que las cifras son comparables contra sí mismas, got:\n%s", out)
	}
}

func TestCmdUsage_NoWindow_OmitsPercentageLine(t *testing.T) {
	deps, closeDB := newUsageTestDeps(t, 0) // ventana en su valor por defecto: 0
	defer closeDB()
	deps.UsageRepo.Record(domain.UsageRecord{
		Project: "proj", SessionID: "sess-1", Operation: domain.OpBuildContext,
		Channel: "cli", BaselineTokens: 1000, EmittedTokens: 400,
	})

	out := captureStdout(t, func() { CmdUsage(deps, []string{"--session", "sess-1"}) })

	if strings.Contains(strings.ToLower(out), "ventana") {
		t.Fatalf("con usage_window_tokens=0 no debe aparecer ninguna línea de ventana, got:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "estimado") {
		t.Fatalf("con usage_window_tokens=0 nada debe rotularse estimado, got:\n%s", out)
	}
}

func TestCmdUsage_WithWindow_ShowsEstimatedLine(t *testing.T) {
	deps, closeDB := newUsageTestDeps(t, 200000)
	defer closeDB()
	deps.UsageRepo.Record(domain.UsageRecord{
		Project: "proj", SessionID: "sess-1", Operation: domain.OpBuildContext,
		Channel: "cli", BaselineTokens: 1000, EmittedTokens: 400,
	})

	out := captureStdout(t, func() { CmdUsage(deps, []string{"--session", "sess-1"}) })

	if !strings.Contains(strings.ToLower(out), "estimado") {
		t.Fatalf("con usage_window_tokens>0 debe aparecer la línea rotulada (estimado), got:\n%s", out)
	}
}

func TestCmdUsage_JSON_MatchesContractGuarantees(t *testing.T) {
	deps, closeDB := newUsageTestDeps(t, 0)
	defer closeDB()
	deps.UsageRepo.Record(domain.UsageRecord{
		Project: "proj", SessionID: "sess-1", Operation: domain.OpBuildContext,
		Channel: "mcp", BaselineTokens: 6000, EmittedTokens: 3500,
	})
	deps.UsageRepo.Record(domain.UsageRecord{
		Project: "proj", SessionID: "sess-1", Operation: domain.OpSaveMemory,
		Channel: "cli", BaselineTokens: 320, EmittedTokens: 310,
	})

	out := captureStdout(t, func() { CmdUsage(deps, []string{"--session", "sess-1", "--json"}) })

	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("--json no produjo JSON válido: %v\nsalida:\n%s", err, out)
	}
	baseline := int(report["baseline_tokens"].(float64))
	saved := int(report["saved_tokens"].(float64))
	emitted := int(report["emitted_tokens"].(float64))
	if baseline-saved != emitted {
		t.Fatalf("G1 rota: baseline(%d) - saved(%d) != emitted(%d)", baseline, saved, emitted)
	}
	if report["window_ratio"] != nil {
		t.Fatalf("window_ratio debe ser null con ventana 0, got %v", report["window_ratio"])
	}
	if _, ok := report["counting_method"]; !ok {
		t.Fatalf("falta counting_method en la salida JSON")
	}
}
