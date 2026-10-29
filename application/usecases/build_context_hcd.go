package usecases

import (
	"fmt"
	"mem/domain"
	"strings"
)

func formatHCDActiveContext(c domain.HCDActiveContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## HCD activo\n\n- **%s · %s · %s · «%s»** — paso `%s` · `%s`\n", c.Unit.ID, c.Unit.Kind, c.Unit.Lane, domain.ProtectText(c.Unit.Title), c.Stage, c.Unit.DocPath)
	if len(c.Unverified) > 0 {
		fmt.Fprintf(&b, "- Tareas sin verificar: %s\n", strings.Join(c.Unverified, ", "))
	}
	fmt.Fprintf(&b, "- Siguiente: `/%s` — %s\n", c.NextAction.Skill, c.NextAction.Why)
	b.WriteString("- Retomar: `/hcd` o `$hcd`\n\n")
	return b.String()
}
