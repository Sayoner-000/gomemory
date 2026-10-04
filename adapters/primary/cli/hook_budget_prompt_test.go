package cli

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mem/domain"
)

func TestFitPromptSections_CuentaDocumentoReglaYSobreJSON(t *testing.T) {
	sections := []domain.HookSection{
		{Name: "plan", Priority: domain.HookPriorityPlan, Text: strings.Repeat("paso de plan con \"comillas\"\n", 430), Trimmable: true},
		{Name: "octopus", Priority: domain.HookPriorityOctopus, Text: strings.Repeat("regla de delegación\n", 85)},
		{Name: "nudge", Priority: domain.HookPriorityMemory, Text: "llama a save_memory"},
	}
	out := fitPromptSections(nil, dialectClaude, sections)
	if size := utf8.RuneCountInString(renderPromptContext(dialectClaude, out).stdout); size > domain.HookInlineContextMaxChars {
		t.Fatalf("stdout completo excede el canal: %d", size)
	}
	if !strings.Contains(out, "regla de delegación") || !strings.Contains(out, "llama a save_memory") {
		t.Fatalf("las secciones fijas deben conservarse: %q", out)
	}
	if !strings.Contains(out, domain.HookTrimNotice) {
		t.Fatal("el recorte del plan debe indicar cómo recuperar el resto")
	}
}
