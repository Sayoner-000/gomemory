package cli

import (
	"fmt"
	"strings"

	"mem/application/usecases"
)

func formatGateNotice(res usecases.GateResult) string {
	if res.Updated || (res.Checked && len(res.Candidates) == 0) {
		return ""
	}
	if !res.Checked {
		return fmt.Sprintf("ℹ Comprobación de duplicados no realizada (%s): la memoria se guardó igual", res.Reason)
	}
	lines := make([]string, 0, len(res.Candidates)+1)
	for _, candidate := range res.Candidates {
		hint := fmt.Sprintf("si es el mismo tema, repite con topic_key o revisa get_memory %d", candidate.ID)
		if candidate.TopicKey != "" {
			hint = fmt.Sprintf("si es el mismo tema, repite con topic_key=%q o revisa get_memory %d", candidate.TopicKey, candidate.ID)
		}
		lines = append(lines, fmt.Sprintf("⚠ Posible duplicado de #%d «%s» (similitud título %.2f · contenido %.2f) — %s", candidate.ID, candidate.Title, candidate.TitleSim, candidate.BodySim, hint))
	}
	lines = append(lines, "  (similitud léxica de palabras: no afirma que sea el mismo tema)")
	return strings.Join(lines, "\n")
}
