package cli

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"mem/application/usecases"
)

// TestReviewMetricsDTO_CoincideConElContrato compara las claves REALMENTE
// serializadas con las ocho del contrato publicado en
// specs/027-adversarial-consensus-review/contracts/mcp-tools.md.
//
// Es un test de serialización y no un escaneo del código fuente a propósito: el
// defecto original era exactamente que el struct de la capa de aplicación no tenía
// etiquetas JSON, así que la respuesta salía en PascalCase y omitía tres campos. Un
// test que busca cadenas en el archivo no ve eso; uno que hace Marshal, sí (FR-024,
// SC-007).
func TestReviewMetricsDTO_CoincideConElContrato(t *testing.T) {
	esperadas := []string{
		"contradictions", "duration", "findings_confirmed", "findings_suspect",
		"findings_total", "fix_rounds", "memory_deduplicated", "memory_promoted",
	}

	data, err := json.Marshal(nuevasMetricasDTO(usecases.ReviewMetrics{
		Duration: 412, FindingsTotal: 4, FindingsConfirmed: 2, FindingsSuspect: 1,
		Contradictions: 0, FixRounds: 1, MemoryPromoted: 0, MemoryDeduplicated: 0,
	}))
	if err != nil {
		t.Fatal(err)
	}
	var salida map[string]any
	if err := json.Unmarshal(data, &salida); err != nil {
		t.Fatal(err)
	}

	claves := make([]string, 0, len(salida))
	for clave := range salida {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	if !reflect.DeepEqual(claves, esperadas) {
		t.Fatalf("claves de metrics = %v\nse esperaban       = %v", claves, esperadas)
	}

	if salida["duration"].(float64) != 412 {
		t.Errorf("duration no llegó al JSON: %v", salida["duration"])
	}
	// Ningún campo puede omitirse cuando vale cero: un consumidor que lea
	// `contradictions` ausente no sabe si son cero o si la métrica no existe.
	for _, clave := range esperadas {
		if _, ok := salida[clave]; !ok {
			t.Errorf("la métrica %q desaparece cuando vale cero", clave)
		}
	}
}
