package hcdfiles

import (
	"fmt"
	"mem/domain"
	"strings"
)

func ParseDelivery(data []byte) (domain.Delivery, error) {
	var out domain.Delivery
	var slice *domain.Slice
	var task *domain.Task
	section := ""
	lastResult := false
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		v := strings.TrimSpace(line)
		if strings.HasPrefix(v, "## VS-") || strings.HasPrefix(v, "### T-") {
			id, title, ok := parseHeading(v)
			if !ok {
				return out, domain.ErrValidation
			}
			if seen[id] {
				return out, domain.ErrValidation
			}
			seen[id] = true
			section = ""
			lastResult = false
			if strings.HasPrefix(id, "VS-") {
				out.Slices = append(out.Slices, domain.Slice{ID: id, Title: title})
				slice = &out.Slices[len(out.Slices)-1]
				task = nil
			} else {
				out.Tasks = append(out.Tasks, domain.Task{ID: id, Title: title})
				task = &out.Tasks[len(out.Tasks)-1]
				slice = nil
			}
			continue
		}
		if strings.HasPrefix(v, "#### ") {
			section = strings.TrimPrefix(v, "#### ")
			continue
		}
		key, value, ok := strings.Cut(v, ":")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.EqualFold(key, "Estado") || strings.EqualFold(key, "state") || strings.EqualFold(key, "status") {
			return out, domain.ErrValidation
		}
		if task != nil && !ok && lastResult && v != "" {
			task.Result += "\n" + v
			continue
		}
		if task != nil && ok {
			lastResult = key == "Resultado"
			switch key {
			case "Slice":
				task.SliceID = value
			case "Historias":
				task.Stories = splitValues(value)
			case "Criterios":
				task.Criteria = splitValues(value)
			case "Tipo":
				task.Type = value
			case "Dependencias":
				task.Dependencies = deliveryRefs(value)
			case "Reglas":
				task.Rules = splitValues(value)
			case "Resultado":
				task.Result = value
			case "Alcance":
				task.Scope = value
			case "Excluido":
				task.Excluded = value
			case "Verificación":
				task.Verification = value
			case "Evidencia esperada":
				task.ExpectedEvidence = value
			case "Desbloquea":
				task.Unlocks = splitValues(value)
			case "Justificación":
				task.Justification = value
			}
			continue
		}
		if slice != nil {
			if section != "" {
				var dst *string
				switch section {
				case "Comportamiento observable":
					dst = &slice.Observable
				case "Límites":
					dst = &slice.Limits
				case "Impacto arquitectónico conocido":
					dst = &slice.ArchitecturalImpact
				case "Verificación esperada":
					dst = &slice.Verification
				case "Incertidumbres":
					dst = &slice.Uncertainties
				}
				if dst != nil && v != "" {
					if *dst != "" {
						*dst += "\n"
					}
					*dst += v
				}
				continue
			}
			if ok {
				switch key {
				case "Historias":
					slice.Stories = splitValues(value)
				case "Criterios":
					slice.Criteria = splitValues(value)
				case "Dependencias":
					slice.Dependencies = deliveryRefs(value)
				}
			}
		}
	}
	return out, nil
}
func deliveryRefs(v string) []string {
	if strings.EqualFold(v, "ninguna") || strings.EqualFold(v, "ninguno") {
		return nil
	}
	return splitValues(v)
}

// renderRefs escribe «ninguna» para una lista vacía, el mismo valor que
// deliveryRefs acepta al leer, para que el archivo regenerado siga siendo válido.
func renderRefs(refs []string) string {
	if len(refs) == 0 {
		return "ninguna"
	}
	return strings.Join(refs, ", ")
}
func RenderDelivery(unitID string, d domain.Delivery) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Entrega %s\n\n", unitID)
	for _, s := range d.Slices {
		title := s.Title
		if title == "" {
			title = s.Observable
		}
		fmt.Fprintf(&b, "## %s. %s\nHistorias: %s\nCriterios: %s\nDependencias: %s\n", s.ID, title, strings.Join(s.Stories, ", "), strings.Join(s.Criteria, ", "), renderRefs(s.Dependencies))
		for _, f := range []struct{ k, v string }{{"Comportamiento observable", s.Observable}, {"Límites", s.Limits}, {"Impacto arquitectónico conocido", s.ArchitecturalImpact}, {"Verificación esperada", s.Verification}, {"Incertidumbres", s.Uncertainties}} {
			fmt.Fprintf(&b, "#### %s\n%s\n", f.k, f.v)
		}
		for _, t := range d.Tasks {
			if t.SliceID != s.ID {
				continue
			}
			fmt.Fprintf(&b, "\n### %s. %s\n", t.ID, t.Title)
			for _, f := range []struct{ k, v string }{{"Slice", t.SliceID}, {"Historias", strings.Join(t.Stories, ", ")}, {"Criterios", strings.Join(t.Criteria, ", ")}, {"Tipo", t.Type}, {"Dependencias", renderRefs(t.Dependencies)}, {"Reglas", strings.Join(t.Rules, ", ")}, {"Resultado", t.Result}, {"Alcance", t.Scope}, {"Excluido", t.Excluded}, {"Verificación", t.Verification}, {"Evidencia esperada", t.ExpectedEvidence}, {"Desbloquea", strings.Join(t.Unlocks, ", ")}, {"Justificación", t.Justification}} {
				fmt.Fprintf(&b, "%s: %s\n", f.k, f.v)
			}
		}
		b.WriteByte('\n')
	}
	return []byte(b.String())
}
