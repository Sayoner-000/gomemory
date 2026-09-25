package usecases

import (
	"context"
	"strings"
	"testing"
)

type memBlocks struct{ seen map[string]bool }

func (m *memBlocks) Seen(_ context.Context, hs []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, h := range hs {
		if m.seen[h] {
			out[h] = true
		}
	}
	return out, nil
}
func (m *memBlocks) Mark(_ context.Context, hs []string) error {
	for _, h := range hs {
		m.seen[h] = true
	}
	return nil
}
func (m *memBlocks) Reset(context.Context) error { m.seen = map[string]bool{}; return nil }

const docDelta = "# Memoria del Proyecto\n\n## Reglas de trabajo (memoria fijada)\n\n1. regla fija\n\n" +
	"## Decisiones Técnicas\n\n- **A**: uno\n- **B**: dos\n- **C**: tres\n\n## Bugfixes\n\n- **D**: cuatro\n  → `x.go`\n\n"

func TestSessionDeltaContext(t *testing.T) {
	ctx := context.Background()
	d := SessionDelta{Blocks: &memBlocks{seen: map[string]bool{}}}
	if got := d.ApplyContext(ctx, docDelta); got != docDelta {
		t.Fatal("la primera entrega debe ir completa")
	}
	second := d.ApplyContext(ctx, docDelta)
	if !strings.Contains(second, "3 entradas ya entregadas") || !strings.Contains(second, "- **D** ⟦ya entregado⟧") {
		t.Errorf("segunda entrega sin delta:\n%s", second)
	}
	if !strings.Contains(second, "1. regla fija") {
		t.Error("las reglas fijadas nunca se sustituyen")
	}
	if strings.Contains(second, "x.go") {
		t.Error("la línea de continuación pertenece a la entrada ya entregada")
	}
	changed := strings.Replace(docDelta, "- **B**: dos", "- **B**: dos (editada)", 1)
	third := d.ApplyContext(ctx, changed)
	if !strings.Contains(third, "- **B**: dos (editada)") || !strings.Contains(third, "- **A** ⟦ya entregado⟧") {
		t.Errorf("una entrada cambiada debe viajar completa y romper la serie:\n%s", third)
	}
}

func TestSessionDeltaSearchAndWhole(t *testing.T) {
	ctx := context.Background()
	d := SessionDelta{Blocks: &memBlocks{seen: map[string]bool{}}}
	res := "[1] decision | Título uno\n  extracto uno\n\n[2] bugfix | Título dos\n  extracto dos\n\n"
	_ = d.ApplySearch(ctx, res)
	again := d.ApplySearch(ctx, res)
	if !strings.Contains(again, "[1] decision | Título uno ⟦ya entregado⟧") || strings.Contains(again, "extracto uno") {
		t.Errorf("resultado repetido sin delta:\n%s", again)
	}
	if got := d.ApplyWhole(ctx, "método", "aviso"); got != "método" {
		t.Error("el método debe entregarse completo la primera vez")
	}
	if got := d.ApplyWhole(ctx, "método", "aviso"); got != "aviso" {
		t.Error("el método repetido debe sustituirse por el aviso")
	}
	if got := (SessionDelta{}).ApplyContext(ctx, docDelta); got != docDelta {
		t.Error("sin registro, el documento se entrega completo")
	}
}
