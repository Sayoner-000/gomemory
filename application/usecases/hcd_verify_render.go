package usecases

import (
	"fmt"
	"mem/domain"
	"sort"
	"strings"
)

func RenderHCDVerification(u domain.Unit, r HCDVerification) []byte {
	var b strings.Builder
	verdict := "FAIL"
	if r.Verdict.CanClose {
		verdict = "PASS"
	}
	for _, c := range r.Verdict.Causes {
		if c.Code == "BLOCK-CRITICAL" {
			verdict = "BLOCKED"
		}
	}
	fmt.Fprintf(&b, "<!-- GENERADO por mem hcd verify; no editar -->\n# Verificación %s\n\nResultado: %s\nDefinición: %s\nEntrega: %s\n\n## Protocolo\nEnfoque: %s\nPlaybook: %s\nConstitución: %s\n\n## Trazabilidad\nTareas: %d; implementadas: %d; verificadas: %d\n\n## Criterios\n", u.ID, verdict, u.DefinitionRev, u.DeliveryRev, u.Approach, u.PlaybookRev, u.ConstitutionRev, r.Total, r.Implemented, r.Verified)
	keys := make([]string, 0, len(r.Criteria))
	for key := range r.Criteria {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "- %s: %s\n", key, r.Criteria[key])
	}
	b.WriteString("\n## Evidencias\n")
	keys = nil
	for key := range r.Evidence {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "- %s: %s\n", key, r.Evidence[key])
	}
	b.WriteString("\n## Aprobaciones\n")
	for _, a := range r.Approvals {
		fmt.Fprintf(&b, "- %s: %s, rev %s, origen %s, cliente %s, cita «%s»\n", a.ID, a.Object, a.Revision, a.Origin, a.Client, domain.ProtectText(a.Quote))
	}
	b.WriteString("\n## Excepciones\n")
	b.WriteString("\n## Bloqueos\n")
	for _, block := range r.Blocks {
		fmt.Fprintf(&b, "- %s: %s (%s)\n", block.ID, domain.ProtectText(block.Reason), block.Status)
	}
	b.WriteString("\n## Desvíos\n")
	for _, drift := range r.Drift {
		fmt.Fprintf(&b, "- %s\n", drift)
	}
	b.WriteString("\n## Acción requerida\n")
	if r.Verdict.CanClose {
		b.WriteString("Solicitar aceptación explícita de la revisión; PASS no cierra la unidad.\n")
	} else {
		for _, c := range r.Verdict.Causes {
			fmt.Fprintf(&b, "- %s · %s: %s\n", c.Code, c.Field, c.Message)
		}
	}
	return []byte(domain.ProtectText(b.String()))
}
