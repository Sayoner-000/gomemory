package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewMCPToolContracts(t *testing.T) {
	path := filepath.Join(repoRootContract(t), "adapters", "primary", "cli", "cmd_mcp_review_tools.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer contrato de tools de revisión: %v", err)
	}
	source := string(data)
	for _, tool := range []string{
		"review_start", "review_submit", "review_consensus", "review_status",
		"review_finalize", "review_fix_record", "review_rejudge", "review_promote_memory",
	} {
		if !strings.Contains(source, `"`+tool+`"`) {
			t.Errorf("falta la tool MCP %q", tool)
		}
	}
	// El nombre del parámetro es el contrato; `,omitempty` es una decisión de
	// esquema sobre si es obligatorio, y cambiarla no rompe a nadie. Comparar la
	// etiqueta literal ataba el contrato a esa decisión: hacer opcional
	// `unmatched` —un consenso sin hallazgos sueltos es un caso legítimo, y
	// exigirlo obligaba al agente a mandar una lista vacía— hacía fallar este
	// test sin que ningún consumidor se viera afectado.
	for _, parameter := range []string{
		"target_type", "revision", "digest",
		"review_id", "reviewer", "target_digest",
		"matches", "unmatched",
		// Funcionalidad 028: el re-juicio pasa a ser por revisor —con el mapa
		// plano anterior era imposible exigir corroboración independiente— y la
		// autorización de corrección se declara explícitamente.
		"judgments", "fix_authorized",
	} {
		if !strings.Contains(source, `json:"`+parameter+`"`) &&
			!strings.Contains(source, `json:"`+parameter+`,`) {
			t.Errorf("falta el parámetro de contrato %q", parameter)
		}
	}

	finalizeStart := strings.Index(source, `"review_finalize"`)
	if finalizeStart < 0 {
		t.Fatal("no se encontró review_finalize")
	}
	finalizeBlock := source[finalizeStart:]
	if next := strings.Index(finalizeBlock[1:], "mcp.AddTool"); next >= 0 {
		finalizeBlock = finalizeBlock[:next+1]
	}
	if strings.Contains(finalizeBlock, `json:"verdict"`) {
		t.Fatal("review_finalize no puede aceptar verdict como entrada")
	}
}

// TestReviewStatusExponeElLinaje cubre FR-022, FR-023 y SC-006: un auditor debe
// reconstruir el recorrido completo de cualquier hallazgo con UNA sola consulta.
//
// review_status devolvía cuatro campos —review_id, status, round, verdict— mientras
// el contrato de la 027 ya prometía "resumen de hallazgos por estado".
func TestReviewStatusExponeElLinaje(t *testing.T) {
	path := filepath.Join(repoRootContract(t), "adapters", "primary", "cli", "cmd_mcp_review_tools.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, campo := range []string{
		"fix_authorized", "original_digest", "current_digest",
		"by_status", "by_severity", "by_rejudgment",
		"addressed_by_round", "rejudgments", "aggregate_state",
		"source_finding_ids", "fix_rounds",
	} {
		if !strings.Contains(source, `"`+campo+`"`) {
			t.Errorf("review_status no expone %q, exigido por el contrato de auditoría", campo)
		}
	}
}

// TestReviewFinalizeMetricasDelContrato comprueba que las ocho métricas publicadas
// existen con su nombre snake_case. La verificación de la serialización REAL vive en
// adapters/primary/cli/cmd_mcp_review_metrics_test.go: este test solo evita que un
// campo desaparezca del DTO.
func TestReviewFinalizeMetricasDelContrato(t *testing.T) {
	path := filepath.Join(repoRootContract(t), "adapters", "primary", "cli", "cmd_mcp_review_tools.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, metrica := range []string{
		"duration", "findings_total", "findings_confirmed", "findings_suspect",
		"contradictions", "fix_rounds", "memory_promoted", "memory_deduplicated",
	} {
		if !strings.Contains(source, `json:"`+metrica+`"`) {
			t.Errorf("falta la métrica %q del contrato publicado", metrica)
		}
	}
}
