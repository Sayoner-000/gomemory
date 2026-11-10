package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHCD057DefinitionReview(t *testing.T) {
	h := newHCDBinary(t)

	// 1. Carril fast funciona de punta a punta sin exigir approve ni review
	rFast := h.run(true, "start", "--kind", "fix", "--lane", "fast", "--title", "Fast Lane Fix", "--why", "fix directo", "--intent", "Corregir defecto directo")
	h.expect(rFast, "define", "hcd-define")
	docFast := rFast.DocPath
	h.edit(docFast, hcdCommonFill...)
	h.expect(h.run(true, "show"), "implement", "hcd-implement")
	// Verificar rojo y verde en carril fast
	_ = h.run(true, "verify", "--task", "1", "--expect-fail", "--", "git grep -q fixed -- save.go")
	h.write("save.go", "package save // fixed\n")
	_ = h.run(true, "verify", "--task", "1", "--", "git grep -q fixed -- save.go")
	rClose := h.run(true, "close", "--quote", "apruebo")
	if rClose.State != "closed" || rClose.Stage != "closed" {
		t.Fatalf("fast close failed: %+v", rClose)
	}

	// 2. Ceremonia completa con examen socrático, decisiones, informes y caducidad
	rFull := h.run(true, "start", "--kind", "evolution", "--lane", "full", "--title", "Full Ceremony Review", "--why", "requiere examen y splash", "--intent", "Probar integracion completa HCD-057")
	h.expect(rFull, "define", "hcd-define")
	docFull := rFull.DocPath

	// Llenar secciones requeridas de la definición
	h.edit(docFull,
		"## Reproducción\npendiente", "",
		"<!-- /fix -->\n", "",
		"<!-- fix -->\n", "",
		"Incluido: pendiente", "Incluido: guardado",
		"Excluido: pendiente", "Excluido: exportación",
		"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nnada",
		"## Alternativa más barata\npendiente", "## Alternativa más barata\nninguna",
		"### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Historia\nComo usuario quiero guardar para persistir\n- C-01: Dado x, cuando y, entonces z",
	)

	// 3. Sin examen socrático, permanece en define con skill hcd-define
	rShow := h.run(true, "show")
	h.expect(rShow, "define", "hcd-define")
	rev1 := rShow.DefinitionRev
	if rev1 == "" {
		t.Fatal("expected valid definition revision")
	}

	// Intento de aprobar sin examen socrático es rechazado (sin escrituras parciales)
	rBadApprove := h.run(false, "approve", "--quote", "apruebo")
	if rBadApprove.Error.Code != "not_allowed" {
		t.Fatalf("expected not_allowed, got %+v", rBadApprove)
	}

	// 4. Registrar examen socrático con preguntas
	socraticFile := filepath.Join(h.root, "exam.json")
	socraticPayload := `{
		"summary": "Examen socratico de supuestos, alternativas y criterios verificado",
		"questions": [
			{
				"text": "¿El almacenamiento garantiza persistencia atómica?",
				"answer": "Sí, SQLite opera en modo WAL con transacciones ACID.",
				"status": "answered"
			},
			{
				"text": "¿Se evaluó almacenar en memoria volatil?",
				"status": "omitted"
			}
		]
	}`
	if err := os.WriteFile(socraticFile, []byte(socraticPayload), 0o644); err != nil {
		t.Fatal(err)
	}
	rExam := h.run(true, "review", "--kind", "socratic", "--action", "complete", "--revision", rev1, "--operation-id", "op-soc-int-1", "--input", socraticFile)
	h.expect(rExam, "approve", "hcd-approve")

	// Reanudación idempotente del mismo examen
	rExamReplay := h.run(true, "review", "--kind", "socratic", "--action", "complete", "--revision", rev1, "--operation-id", "op-soc-int-1", "--input", socraticFile)
	if !rExamReplay.OK {
		t.Fatalf("replay of socratic exam failed: %+v", rExamReplay)
	}

	// 5. Aprobación rechazada sin decisión splash
	rBadSplash := h.run(false, "approve", "--quote", "apruebo")
	if rBadSplash.Error.Code != "not_allowed" || !strings.Contains(rBadSplash.Error.Message, "splash") {
		t.Fatalf("expected splash decision required, got %+v", rBadSplash)
	}

	// 6. Aceptar splash, registrar intento en curso, fallo e indisponibilidad
	_ = h.run(true, "review", "--kind", "splash", "--action", "accept", "--revision", rev1, "--operation-id", "op-splash-acc", "--quote", "acepto la revisión splash")

	// Intento en curso att-1
	_ = h.run(true, "review", "--kind", "splash", "--action", "start", "--revision", rev1, "--operation-id", "op-splash-s1", "--attempt", "att-1")

	// Aprobar con intento en curso sigue rechazado
	if r := h.run(false, "approve", "--quote", "apruebo"); r.Error.Code != "not_allowed" {
		t.Fatalf("expected not_allowed while splash in progress, got %+v", r)
	}

	// Registrar fallo de att-1
	_ = h.run(true, "review", "--kind", "splash", "--action", "fail", "--revision", rev1, "--operation-id", "op-splash-f1", "--attempt", "att-1", "--reason", "timeout")

	// Aprobar con intento fallido sigue rechazado
	if r := h.run(false, "approve", "--quote", "apruebo"); r.Error.Code != "not_allowed" {
		t.Fatalf("expected not_allowed after fail, got %+v", r)
	}

	// Nuevo intento att-2 y reporte con Blocker B1
	_ = h.run(true, "review", "--kind", "splash", "--action", "start", "--revision", rev1, "--operation-id", "op-splash-s2", "--attempt", "att-2")
	splashReportFile := filepath.Join(h.root, "report.json")
	reportPayload := map[string]any{
		"attempt_id": "att-2",
		"summary":    "Informe splash completo",
		"findings": []map[string]any{
			{
				"id":       "B1",
				"severity": "Blocker",
				"quote":    "Incluido: guardado",
				"problem":  "Bloqueo sin mitigacion explicita",
			},
			{
				"id":       "M1",
				"severity": "Major",
				"problem":  "Aviso informativo",
			},
		},
	}
	reportData, _ := json.Marshal(reportPayload)
	_ = os.WriteFile(splashReportFile, reportData, 0o644)
	_ = h.run(true, "review", "--kind", "splash", "--action", "complete", "--revision", rev1, "--operation-id", "op-splash-c2", "--input", splashReportFile)

	// Aprobar con Blocker B1 no descartado sigue rechazado
	if r := h.run(false, "approve", "--quote", "apruebo"); r.Error.Code != "not_allowed" || !strings.Contains(r.Error.Message, "B1") {
		t.Fatalf("expected blocker B1 to prevent approval, got %+v", r)
	}

	// Descartar Blocker B1 de forma explícita
	_ = h.run(true, "review", "--kind", "splash", "--action", "dismiss", "--revision", rev1, "--operation-id", "op-splash-d1", "--finding", "B1", "--quote", "descarto B1 por justificacion")

	// Ahora aprobar sí tiene éxito
	rApprove := h.run(true, "approve", "--quote", "apruebo")
	if !rApprove.Approved || rApprove.Stage != "slice" {
		t.Fatalf("expected approved and stage slice, got %+v", rApprove)
	}

	// 7. Caducidad por edición: cambiar definición invalida aprobación y revisiones
	h.edit(docFull, "Incluido: guardado", "Incluido: guardado e historial")
	rAfterEdit := h.run(true, "show")
	h.expect(rAfterEdit, "define", "hcd-define")
	rev2 := rAfterEdit.DefinitionRev
	if rev2 == rev1 {
		t.Fatal("expected new definition revision after editorial change")
	}
	revData := rAfterEdit.Data["definition_review"].(map[string]any)
	if revData["socratic"] != "expired" || revData["splash"] != "expired" {
		t.Fatalf("expected expired status for both reviews, got %+v", revData)
	}

	// 8. En la nueva revisión, examen socrático y omisión explícita de splash
	_ = h.run(true, "review", "--kind", "socratic", "--action", "complete", "--revision", rev2, "--operation-id", "op-soc-int-2", "--input", socraticFile)
	_ = h.run(true, "review", "--kind", "splash", "--action", "skip", "--revision", rev2, "--operation-id", "op-splash-skip-2", "--quote", "omito splash en revision 2")

	// Aprobar la nueva revisión
	rApprove2 := h.run(true, "approve", "--quote", "apruebo")
	if !rApprove2.Approved || rApprove2.Stage != "slice" {
		t.Fatalf("expected approved rev2 and stage slice, got %+v", rApprove2)
	}

	// 9. Verificar conservación de aprobaciones históricas al consultar historial
	rHist := h.run(true, "history", "Full Ceremony Review")
	if !rHist.OK {
		t.Fatalf("history failed: %+v", rHist)
	}
}
