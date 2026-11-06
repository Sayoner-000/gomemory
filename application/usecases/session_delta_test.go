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
	if !strings.Contains(second, DeliveredRecoveryHint) || strings.Contains(second, "get_memory(id)") {
		t.Errorf("los marcadores deben llevar a una vía de recuperación real (full=true):\n%s", second)
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

func TestHCDActiveContextDeltaDeduplicates(t *testing.T) {
	d := SessionDelta{Blocks: &memBlocks{seen: map[string]bool{}}}
	doc := "# Memoria del Proyecto\n\n## HCD activo\n\n- **HCD-001 · fix · fast · «x»** — paso `define`\n- Siguiente: `/hcd-define`\n\n"
	d.ApplyContext(context.Background(), doc)
	second := d.ApplyContext(context.Background(), doc)
	if strings.Contains(second, "paso `define`") {
		t.Fatalf("unchanged HCD context repeated: %s", second)
	}
	third := d.ApplyContext(context.Background(), strings.ReplaceAll(doc, "paso `define`", "paso `implement`"))
	if !strings.Contains(third, "paso `implement`") {
		t.Fatalf("changed HCD context omitted: %s", third)
	}
}

// HCD-037 — un arranque recortado solo da por entregadas las entradas que el
// agente vio completas: la que el recorte corta a medias (aquí D pierde su
// línea de continuación) y las que quedan fuera viajan enteras después.
func TestSessionDelta_MarkVisible(t *testing.T) {
	ctx := context.Background()
	d := SessionDelta{Blocks: &memBlocks{seen: map[string]bool{}}}
	visible := docDelta[:strings.Index(docDelta, "  → `x.go`")]
	d.MarkVisible(ctx, docDelta, visible)

	got := d.ApplyContext(ctx, docDelta)
	if !strings.Contains(got, "3 entradas ya entregadas") {
		t.Errorf("A, B y C se vieron completas y deben sustituirse:\n%s", got)
	}
	if !strings.Contains(got, "- **D**: cuatro\n  → `x.go`") {
		t.Errorf("D llegó cortada: no puede darse por entregada:\n%s", got)
	}

	// Con compresión max, lo emitido tiene los espacios colapsados: la entrada
	// con continuación sangrada llegó completa aunque no sea literal.
	sangrado := "## Decisiones Técnicas\n\n- **E**: cinco\n   sigue   aquí\n"
	comprimido := "## Decisiones Técnicas\n\n- **E**: cinco\n sigue aquí"
	conSangria := SessionDelta{Blocks: &memBlocks{seen: map[string]bool{}}}
	conSangria.MarkVisible(ctx, sangrado, comprimido)
	if got := conSangria.ApplyContext(ctx, sangrado); !strings.Contains(got, "- **E** ⟦ya entregado⟧") {
		t.Errorf("la compresión de espacios no impide reconocer lo entregado:\n%s", got)
	}

	vacio := SessionDelta{Blocks: &memBlocks{seen: map[string]bool{}}}
	vacio.MarkVisible(ctx, docDelta, "")
	if got := vacio.ApplyContext(ctx, docDelta); got != docDelta {
		t.Errorf("sin texto visible no se anota nada:\n%s", got)
	}
}
