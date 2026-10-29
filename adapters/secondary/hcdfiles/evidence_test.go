package hcdfiles

import (
	"errors"
	"mem/domain"
	"testing"
)

func TestParseEvidenceNamesMissingFields(t *testing.T) {
	// Formato libre con encabezados: ninguna clave reconocida salvo Resultado.
	_, e := ParseEvidence([]byte("# Evidencia\nResultado: pass\n## Procedimiento\ngo test\n"))
	var fields *domain.HCDValidationError
	if !errors.As(e, &fields) || !errors.Is(e, domain.ErrValidation) {
		t.Fatalf("err=%v", e)
	}
	got := map[string]bool{}
	for _, c := range fields.Checks {
		got[c.Field] = c.Code == "EVIDENCE-FIELDS"
	}
	if !got["Procedimiento"] || !got["Entorno"] || !got["Sujeto"] || len(got) != 3 {
		t.Fatalf("checks=%+v", fields.Checks)
	}
	v, e := ParseEvidence([]byte("Cubre: T-001, C-01\nResultado: pass\nProcedimiento: go test\nEntorno: darwin\nSujeto: tree:abc\n---\nok\n"))
	if e != nil || v.Result != "pass" || len(v.Covers) != 2 {
		t.Fatalf("valid evidence=%+v err=%v", v, e)
	}
}
