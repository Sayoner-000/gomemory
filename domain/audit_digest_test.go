package domain

import (
	"os"
	"strings"
	"testing"
)

// HCD-046: cada informe de docs/audits tiene una ficha con sus hallazgos.

func readAudit(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../docs/audits/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func findingByID(r AuditReport, id string) (AuditFinding, bool) {
	for _, f := range r.Findings {
		if f.ID == id {
			return f, true
		}
	}
	return AuditFinding{}, false
}

func TestParseAuditReport(t *testing.T) {
	t.Run("defensiva con estado de cierre", func(t *testing.T) {
		r := ParseAuditReport("docs/audits/2026-10-11-defensive-qa.md", readAudit(t, "2026-10-11-defensive-qa.md"))
		if r.Title != "Auditoría defensiva multilenguaje — gomemory" || r.Date != "2026-10-11" {
			t.Fatalf("cabecera = %q %q", r.Title, r.Date)
		}
		if len(r.Findings) != 9 {
			t.Fatalf("hallazgos = %d, quiero 9: %+v", len(r.Findings), r.Findings)
		}
		f, ok := findingByID(r, "H-01")
		if !ok || f.Severity != "Alta" || f.Problem != "Resúmenes, relaciones y errores de canal omiten la redacción" || !strings.Contains(f.Status, "Cerrado") {
			t.Fatalf("H-01 = %+v", f)
		}
	})
	t.Run("ACR con veredicto y cierre en unidades", func(t *testing.T) {
		r := ParseAuditReport("docs/audits/2026-11-06-acr-adm-project.md", readAudit(t, "2026-11-06-acr-adm-project.md"))
		if !strings.Contains(r.Verdict, "APPROVED") {
			t.Fatalf("veredicto = %q", r.Verdict)
		}
		if len(r.Findings) != 3 {
			t.Fatalf("hallazgos = %+v", r.Findings)
		}
		f, ok := findingByID(r, "H-01")
		if !ok || f.Severity != "Alta" || !strings.Contains(f.Status, "HCD-043") {
			t.Fatalf("H-01 = %+v", f)
		}
		if f, _ := findingByID(r, "H-02"); f.Severity != "Media" {
			t.Fatalf("H-02 = %+v", f)
		}
		d := RenderAuditDigest(r)
		for _, want := range []string{"- H-01 [Alta] Recortar antes de redactar pierde el límite privado → HCD-043", "Veredicto: "} {
			if !strings.Contains(d.Content, want) {
				t.Errorf("la ficha no contiene %q:\n%s", want, d.Content)
			}
		}
		if d.Title != "Auditoría 2026-11-06 · Informe ACR + ADM del proyecto" {
			t.Errorf("título = %q", d.Title)
		}
		if !containsString(d.Files, "adapters/secondary/hcdfiles/workspace.go") {
			t.Errorf("archivos = %v", d.Files)
		}
		if len([]rune(d.Content)) > HCDDigestMaxContent {
			t.Errorf("la ficha supera el tope: %d", len([]rune(d.Content)))
		}
	})
	t.Run("informe sin tabla", func(t *testing.T) {
		r := ParseAuditReport("docs/audits/2026-01-01-nota.md", []byte("# Nota de revisión\n\nResumen corto del informe.\n\nOtro párrafo.\n"))
		d := RenderAuditDigest(r)
		if len(r.Findings) != 0 || r.Summary != "Resumen corto del informe." || !strings.Contains(d.Content, "Resumen: Resumen corto del informe.") {
			t.Fatalf("informe sin tabla = %+v\n%s", r, d.Content)
		}
	})
	if AuditDigestTopic("docs/audits/2026-11-06-acr-adm-project.md") != "audit/2026-11-06-acr-adm-project.md" {
		t.Fatalf("topic = %q", AuditDigestTopic("docs/audits/2026-11-06-acr-adm-project.md"))
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
