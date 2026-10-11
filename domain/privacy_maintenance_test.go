package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestSanitizeCandidate_Validate cubre la lista permitida de categoría y
// campo (spec 037, data-model "Candidato interno de saneamiento").
func TestSanitizeCandidate_Validate(t *testing.T) {
	ok := []SanitizeCandidate{
		{Category: SanitizeSession, RecordID: "s1", Project: "p", Field: "summary"},
		{Category: SanitizeRelation, RecordID: "7", Project: "p", Field: "reasoning"},
		{Category: SanitizeChannelError, RecordID: "opencode/user/turn", Project: "p", Field: "last_error"},
		{Category: SanitizeOriginal, RecordID: "abc123", Project: "p", Field: "content"},
		{Category: SanitizeMemoryReview, RecordID: "9", Project: "p", Field: "content"},
	}
	for _, c := range ok {
		if err := c.Validate("p"); err != nil {
			t.Errorf("Validate(%+v) = %v", c, err)
		}
	}
	bad := []SanitizeCandidate{
		{Category: "memories", RecordID: "1", Project: "p", Field: "content"},
		{Category: SanitizeSession, RecordID: "1", Project: "p", Field: "id"},
		{Category: SanitizeRelation, RecordID: "1", Project: "otro", Field: "reasoning"},
		{Category: SanitizeRelation, RecordID: "", Project: "p", Field: "reasoning"},
	}
	for _, c := range bad {
		if err := c.Validate("p"); !errors.Is(err, ErrValidation) {
			t.Errorf("Validate(%+v) = %v, quiero ErrValidation", c, err)
		}
	}
}

// TestNeedsSanitize distingue texto que cambia con la protección vigente del
// que ya está limpio, y detecta el cierre huérfano que dejó la regex anterior.
func TestNeedsSanitize(t *testing.T) {
	if !NeedsSanitize("a <private>x</private>") || !NeedsSanitize("clave AKIAIOSFODNN7EXAMPLE") {
		t.Error("texto protegido no detectado")
	}
	if NeedsSanitize("texto público") || NeedsSanitize(ProtectText("a <private>x")) {
		t.Error("texto ya protegido reportado")
	}
	if !HasOrphanPrivateClose("remanente Z</private> b") || HasOrphanPrivateClose("a <private>x</private> b") {
		t.Error("detección de cierre huérfano incorrecta")
	}
}

// TestSummarizeCandidates agrupa por categoría con ids ordenados y sin texto.
func TestSummarizeCandidates(t *testing.T) {
	got := SummarizeCandidates([]SanitizeCandidate{
		{Category: SanitizeSession, RecordID: "b", Text: "SECRETO_037"},
		{Category: SanitizeRelation, RecordID: "2"},
		{Category: SanitizeSession, RecordID: "a"},
	})
	if len(got) != 2 || got[0].Category != SanitizeRelation || got[1].Category != SanitizeSession {
		t.Fatalf("orden de categorías: %+v", got)
	}
	if got[1].AffectedCount != 2 || strings.Join(got[1].RecordIDs, ",") != "a,b" {
		t.Fatalf("conteo o ids: %+v", got[1])
	}
}

// TestSanitizeReport_JSONSinTexto: el informe serializado nunca contiene el
// texto de un candidato y respeta los campos del contrato.
func TestSanitizeReport_JSONSinTexto(t *testing.T) {
	c := SanitizeCandidate{Category: SanitizeSession, RecordID: "s1", Project: "p", Field: "summary", Text: "SECRETO_037"}
	r := NewSanitizeReport("p", SanitizeModePreview)
	r.Categories = SummarizeCandidates([]SanitizeCandidate{c})
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "SECRETO_037") {
		t.Fatalf("el informe revela texto: %s", s)
	}
	for _, key := range []string{`"schema_version":1`, `"mode":"preview"`, `"status":"preview"`, `"rows_changed":0`, `"external_copies":`, `"artifacts":[]`, `"pending":[]`} {
		if !strings.Contains(s, key) {
			t.Errorf("falta %s en %s", key, s)
		}
	}
	if b, _ := json.Marshal(c); strings.Contains(string(b), "SECRETO_037") {
		t.Fatalf("el candidato serializado revela texto: %s", b)
	}
}

// TestSanitizeError conserva el código y no expone causas crudas.
func TestSanitizeError(t *testing.T) {
	cause := errors.New("driver: SECRETO_037 database is locked")
	err := NewSanitizeError(SanitizeCodeBusy, cause)
	var se *SanitizeError
	if !errors.As(err, &se) || se.Code != SanitizeCodeBusy || !errors.Is(err, cause) {
		t.Fatalf("SanitizeError mal formado: %#v", err)
	}
	if strings.Contains(err.Error(), "SECRETO_037") {
		t.Fatalf("Error() revela la causa cruda: %v", err)
	}
}

// TestContainsProtected: un cierre huérfano (remanente reportado como
// memory_review) no es contenido protegido; una apertura o una credencial sí.
func TestContainsProtected(t *testing.T) {
	for s, want := range map[string]bool{
		"texto Z</private> fin":               false,
		"público":                             false,
		"a <Private>x":                        true,
		"a <private>x</private> b":            true,
		"clave AKIAIOSFODNN7EXAMPLE":          true,
		ProtectText("a <private>x</private>"): false,
	} {
		if got := ContainsProtected(s); got != want {
			t.Errorf("ContainsProtected(%q) = %v, quiero %v", s, got, want)
		}
	}
}
