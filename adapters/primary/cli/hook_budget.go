package cli

import (
	"fmt"
	"unicode/utf8"

	"mem/domain"
)

// fitHookSections ajusta una salida de hook al tope del canal
// (domain.FitHookOutput) y registra cada recorte como evento de guarda
// (FR-028), para que una regresión de tamaño se vea en mem doctor y no solo
// cuando el agente ya trabajó con una vista previa truncada.
func fitHookSections(deps *Deps, agent string, sections []domain.HookSection) (string, bool) {
	out, trims := domain.FitHookOutput(sections, domain.HookInlineContextMaxChars)
	for _, tr := range trims {
		recordGuard(deps, agent, domain.GuardBudgetTrimmed, fmt.Sprintf("%s orig=%d,emit=%d", tr.Name, tr.OrigRunes, tr.EmitRunes))
	}
	return out, len(trims) > 0
}

// fitPromptSections cuenta también el sobre JSON y sus escapes: el límite del
// host se aplica a stdout completo, no solo a additionalContext.
func fitPromptSections(deps *Deps, dialect hookDialect, sections []domain.HookSection) string {
	budget := domain.HookInlineContextMaxChars
	for budget > 0 {
		out, trims := domain.FitHookOutput(sections, budget)
		size := utf8.RuneCountInString(renderPromptContext(dialect, out).stdout)
		if size <= domain.HookInlineContextMaxChars {
			for _, tr := range trims {
				recordGuard(deps, agentOfDialect(dialect), domain.GuardBudgetTrimmed,
					fmt.Sprintf("%s orig=%d,emit=%d", tr.Name, tr.OrigRunes, tr.EmitRunes))
			}
			return out
		}
		budget -= size - domain.HookInlineContextMaxChars + 64
	}
	return domain.HookTrimNotice
}
