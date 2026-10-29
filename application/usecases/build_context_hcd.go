package usecases

import (
	"fmt"
	"mem/domain"
	"strings"
)

func formatHCDActiveContext(c domain.HCDActiveContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## HCD activo\n\n- **%s · %s · «%s»** — paso `%s`, estado `%s`, rev `%s`\n- Siguiente: `/%s` — %s\n", c.Unit.ID, c.Unit.Kind, domain.ProtectText(c.Unit.Title), c.Stage, c.Unit.State, c.Unit.DefinitionRev, c.NextAction.Skill, c.NextAction.Why)
	for i, v := range c.Clarifications {
		if i == 3 {
			break
		}
		fmt.Fprintf(&b, "- Aclaración %s: %s (bloqueante: %t)\n", v.ID, domain.ProtectText(v.Question), v.Blocking)
	}
	start := max(0, len(c.Decisions)-3)
	for _, d := range c.Decisions[start:] {
		fmt.Fprintf(&b, "- Decisión %s %s: %s (rev %s)\n", d.ID, d.Status, domain.ProtectText(d.Summary), d.Revision)
	}
	for i, d := range c.Drift {
		if i == 5 {
			break
		}
		fmt.Fprintf(&b, "- Desvío: %s\n", d)
	}
	b.WriteString("- Recuperar todo: `/hcd` o `$hcd`\n\n")
	return b.String()
}
