package cli

import (
	"strings"
	"testing"

	"mem/application/usecases"
)

func TestFormatGateNotice(t *testing.T) {
	if got := formatGateNotice(usecases.GateResult{Checked: true}); got != "" {
		t.Fatalf("empty notice = %q", got)
	}
	if got := formatGateNotice(usecases.GateResult{Checked: true, Updated: true}); got != "" {
		t.Fatalf("upsert notice = %q", got)
	}
	got := formatGateNotice(usecases.GateResult{Checked: true, Candidates: []usecases.NearDuplicate{{ID: 7, Title: "tema", TopicKey: "clave", TitleSim: 1, BodySim: .5}}})
	for _, want := range []string{"⚠ Posible duplicado de #7", "topic_key=\"clave\"", "similitud léxica"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q missing from %q", want, got)
		}
	}
	if got := formatGateNotice(usecases.GateResult{Reason: "caída"}); !strings.Contains(got, "Comprobación de duplicados no realizada") {
		t.Fatalf("failure notice = %q", got)
	}
}
