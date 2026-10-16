package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSocraticaFlow_RecorridoScrutiny (T030): el modo scrutiny conserva la
// síntesis, no asigna dispatch_id sin approve y deja la síntesis depurada en el
// historial.
func TestSocraticaFlow_RecorridoScrutiny(t *testing.T) {
	s := newSocraticaSession(t, t.TempDir())
	q := func(text string) map[string]any {
		return map[string]any{"text": text, "options": []string{"Sí", "No"}}
	}
	start := s.call(map[string]any{"action": "start", "client": "claude", "mode": "scrutiny", "prompt": "deberíamos migrar a SQLite en todos los entornos",
		"questions": []any{q("¿Hay datos de carga?"), q("¿Cuántos escritores?"), q("¿Réplicas?"), q("¿Coste?")}})
	id := start["flow_id"]
	if len(start["questions"].([]any)) > 3 {
		t.Fatal("más de tres preguntas", start["questions"])
	}
	s.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true})
	synthesis := map[string]any{
		"restatement":         "Se propone usar SQLite en todos los entornos.",
		"assumptions":         []any{map[string]any{"text": "Un solo escritor", "kind": "hypothesis"}, map[string]any{"text": "WAL activo", "kind": "fact"}},
		"strongest_support":   "Menos piezas que operar.",
		"strongest_objection": "Escrituras concurrentes <private>clave-interna</private>.",
		"hypotheses":          []any{"Hipótesis: la carga no crecerá"},
	}
	review := s.call(map[string]any{"action": "rewrite", "flow_id": id, "proposed": "Evalúa SQLite frente a [alternativa] con [criterio].", "scrutiny": synthesis})
	if review["phase"] != "review" {
		t.Fatal(review)
	}
	got := s.call(map[string]any{"action": "get", "flow_id": id})
	sc, _ := got["scrutiny"].(map[string]any)
	if sc["strongest_support"] != "Menos piezas que operar." || len(sc["assumptions"].([]any)) != 2 {
		t.Fatal(got["scrutiny"])
	}
	if got["dispatch_id"] != "" {
		t.Fatal("dispatch_id sin approve", got["dispatch_id"])
	}
	history := s.call(map[string]any{"action": "history", "client": "claude"})
	raw, _ := json.Marshal(history["history"])
	if !strings.Contains(string(raw), "Menos piezas que operar.") || strings.Contains(string(raw), "clave-interna") {
		t.Fatalf("historial sin síntesis depurada: %s", raw)
	}
}
