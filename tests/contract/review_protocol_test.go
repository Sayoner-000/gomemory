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
	for _, tool := range []string{"review_start", "review_submit", "review_consensus", "review_status", "review_finalize"} {
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
