package cli

import (
	"strings"
	"testing"
)

// FR-005: el protocolo se refiere al binario del PATH (`mem …`). `./mem` solo
// aparece una vez, como alternativa para instalaciones sin binario global.
func TestIntegrationBlock_UsaElBinarioDelPath(t *testing.T) {
	b := buildIntegrationBlock()
	if n := strings.Count(b, "./mem"); n != 1 {
		t.Errorf("./mem aparece %d veces; solo debe quedar la nota de alternativa", n)
	}
	if !strings.Contains(b, "si `mem` no está en el PATH, usa `./mem`") {
		t.Error("falta la nota de alternativa para instalaciones sin global")
	}
	for _, cmd := range []string{"`mem save", "`mem search", "`mem context`", "`mem plan-context`", "`mem session start"} {
		if !strings.Contains(b, cmd) {
			t.Errorf("falta %s en el protocolo", cmd)
		}
	}
}

// Los mensajes de install tampoco invitan a usar una copia local.
func TestInstallNextSteps_UsaElBinarioDelPath(t *testing.T) {
	for _, line := range installNextSteps() {
		if strings.Contains(line, "./mem") {
			t.Errorf("paso siguiente con ./mem: %q", line)
		}
	}
}
