package domain

import "testing"

func TestUsageRecord_Saved(t *testing.T) {
	r := UsageRecord{BaselineTokens: 100, EmittedTokens: 40}
	if got := r.Saved(); got != 60 {
		t.Fatalf("Saved() = %d, want 60", got)
	}
}

func TestUsageReport_Saved(t *testing.T) {
	rep := UsageReport{BaselineTokens: 8120, EmittedTokens: 5310}
	if got := rep.Saved(); got != 2810 {
		t.Fatalf("Saved() = %d, want 2810", got)
	}
	if got := rep.BaselineTokens - rep.Saved(); got != rep.EmittedTokens {
		t.Fatalf("baseline - saved = %d, want emitted %d", got, rep.EmittedTokens)
	}
}

func TestUsageReport_ReductionRatio(t *testing.T) {
	rep := UsageReport{BaselineTokens: 1000, EmittedTokens: 600}
	if got := rep.ReductionRatio(); got != 0.4 {
		t.Fatalf("ReductionRatio() = %v, want 0.4", got)
	}
}

func TestUsageReport_ReductionRatio_ZeroBaseline(t *testing.T) {
	rep := UsageReport{BaselineTokens: 0, EmittedTokens: 0}
	if got := rep.ReductionRatio(); got != 0 {
		t.Fatalf("ReductionRatio() con baseline 0 = %v, want 0 (no NaN, no división por cero)", got)
	}
}

func TestUsageReport_WindowRatio_NoWindow(t *testing.T) {
	rep := UsageReport{BaselineTokens: 1000, EmittedTokens: 600, WindowTokens: 0}
	if _, ok := rep.WindowRatio(); ok {
		t.Fatalf("WindowRatio() con WindowTokens=0 debe ser inválido (ok=false)")
	}
}

func TestUsageReport_WindowRatio_WithWindow(t *testing.T) {
	rep := UsageReport{BaselineTokens: 1000, EmittedTokens: 600, WindowTokens: 200000}
	ratio, ok := rep.WindowRatio()
	if !ok {
		t.Fatalf("WindowRatio() con WindowTokens>0 debe ser válido")
	}
	want := 400.0 / 200000.0
	if ratio != want {
		t.Fatalf("WindowRatio() = %v, want %v", ratio, want)
	}
}

func TestUsageReport_WindowRatio_CanExceedOne(t *testing.T) {
	rep := UsageReport{BaselineTokens: 1000, EmittedTokens: 100, WindowTokens: 10}
	ratio, ok := rep.WindowRatio()
	if !ok {
		t.Fatalf("WindowRatio() debe ser válido")
	}
	if ratio <= 1.0 {
		t.Fatalf("WindowRatio() = %v, se esperaba > 1.0 sin recorte (caso borde de la spec)", ratio)
	}
}

func TestUsageBucket_ExistsWithExpectedFields(t *testing.T) {
	b := UsageBucket{Key: "build_context", Calls: 1, BaselineTokens: 6000, EmittedTokens: 3500}
	if b.Key != "build_context" || b.Calls != 1 {
		t.Fatalf("UsageBucket no conserva sus campos: %+v", b)
	}
}

func TestUsageOperationConstants(t *testing.T) {
	ops := []string{
		OpBuildContext, OpSearchMemories, OpListMemories, OpGetMemory,
		OpBuildPack, OpCompressPack, OpPlanContext, OpSaveMemory, OpOther,
	}
	seen := map[string]bool{}
	for _, op := range ops {
		if op == "" {
			t.Fatalf("una constante de operación está vacía")
		}
		if seen[op] {
			t.Fatalf("operación duplicada: %s", op)
		}
		seen[op] = true
	}
}
