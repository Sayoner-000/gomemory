package hcdfiles

import (
	"fmt"
	"strings"

	"mem/domain"
)

type Warning struct{ Code, Field, Message string }

func ParseDefinition(data []byte) (domain.Definition, []Warning, error) {
	var def domain.Definition
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	var story *domain.Story
	var criterion *domain.Criterion
	seen := map[string]bool{}
	var warnings []Warning
	known := map[string]bool{}
	for _, key := range []string{"ID", "Tipo", "Enfoque", "Implementador", "Playbook", "Problema", "Actor", "Resultado esperado", "Fallo observado", "Reproducción", "Causa raíz", "Comportamiento actual", "Cambio", "Compatibilidad", "Problema técnico", "Beneficiario", "Condición de mejora", "Incluido", "Excluido", "Restricciones", "Historias", "Verificación", "Condición", "Texto"} {
		known[key] = true
	}
	for lineNo, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			def.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			story, criterion = nil, nil
			continue
		}
		if strings.HasPrefix(trimmed, "### C-") || strings.HasPrefix(trimmed, "## US") {
			id, title, ok := parseHeading(trimmed)
			if !ok {
				continue
			}
			if seen[id] {
				return domain.Definition{}, warnings, fmt.Errorf("línea %d: ID duplicado %s", lineNo+1, id)
			}
			seen[id] = true
			if strings.HasPrefix(id, "US") {
				def.Stories = append(def.Stories, domain.Story{ID: id, Title: title, State: domain.StoryDraft})
				story = &def.Stories[len(def.Stories)-1]
				criterion = nil
			} else {
				def.Criteria = append(def.Criteria, domain.Criterion{ID: id, Title: title, State: domain.CriterionDefined})
				criterion = &def.Criteria[len(def.Criteria)-1]
				story = nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "#### ") {
			story = nil
			criterion = nil
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if strings.EqualFold(key, "Estado") || strings.EqualFold(key, "state") || strings.EqualFold(key, "status") {
			return domain.Definition{}, warnings, domain.ErrValidation
		}
		if !known[key] {
			warnings = append(warnings, Warning{Code: "ART-UNKNOWN-KEY", Field: key, Message: fmt.Sprintf("línea %d: clave desconocida %q", lineNo+1, key)})
		}
		switch {
		case story != nil:
			switch key {
			case "Actor":
				story.Actor = value
			case "Texto":
				story.Text = value
			}
		case criterion != nil:
			switch key {
			case "Historias":
				criterion.Stories = splitValues(value)
			case "Condición":
				criterion.Condition = value
			case "Verificación":
				criterion.Verification = domain.VerificationKind(value)
			}
		default:
			switch key {
			case "ID":
				def.ID = value
			case "Tipo":
				def.Kind = domain.Kind(value)
			case "Enfoque":
				def.Approach = value
			case "Implementador":
				def.Implementer = value
			case "Playbook":
				def.Playbook = value
			case "Problema":
				def.Problem = value
			case "Problema técnico":
				def.TechnicalProblem = value
			case "Actor":
				def.Actor = value
			case "Resultado esperado":
				def.ExpectedResult = value
			case "Incluido":
				def.Included = value
			case "Excluido":
				def.Excluded = value
			case "Restricciones":
				def.Restrictions = value
			case "Fallo observado":
				def.ObservedFailure = value
			case "Reproducción":
				def.Reproduction = value
			case "Causa raíz":
				def.RootCause = value
			case "Comportamiento actual":
				def.CurrentBehavior = value
			case "Cambio":
				def.Change = value
			case "Compatibilidad":
				def.Compatibility = value
			case "Beneficiario":
				def.Beneficiary = value
			case "Condición de mejora":
				def.ImprovementCondition = value
			}
		}
	}
	for _, check := range domain.ValidateDefinition(def, def.Kind) {
		warnings = append(warnings, Warning{Code: check.Code, Field: check.Field, Message: check.Message})
	}
	return def, warnings, nil
}

func parseHeading(line string) (id, title string, ok bool) {
	line = strings.TrimLeft(line, "# ")
	id, title, ok = strings.Cut(line, ".")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(id), strings.TrimSpace(title), true
}

func splitValues(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func RenderDefinition(def domain.Definition) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nID: %s\nTipo: %s\nEnfoque: %s\nImplementador: %s\nPlaybook: %s\n\n", def.Title, def.ID, def.Kind, def.Approach, def.Implementer, def.Playbook)
	fmt.Fprintf(&b, "## Intención\nProblema: %s\nActor: %s\nResultado esperado: %s\n\n", def.Problem, def.Actor, def.ExpectedResult)
	fmt.Fprintf(&b, "## Alcance\nIncluido: %s\nExcluido: %s\nRestricciones: %s\n\n", def.Included, def.Excluded, def.Restrictions)
	switch def.Kind {
	case domain.KindFix:
		fmt.Fprintf(&b, "## Particular del tipo\nFallo observado: %s\nReproducción: %s\nCausa raíz: %s\n\n", def.ObservedFailure, def.Reproduction, def.RootCause)
	case domain.KindEvolution:
		fmt.Fprintf(&b, "## Particular del tipo\nComportamiento actual: %s\nCambio: %s\nCompatibilidad: %s\n\n", def.CurrentBehavior, def.Change, def.Compatibility)
	case domain.KindTechnical:
		fmt.Fprintf(&b, "## Particular del tipo\nProblema técnico: %s\nBeneficiario: %s\nCondición de mejora: %s\n\n", def.TechnicalProblem, def.Beneficiary, def.ImprovementCondition)
	}
	for _, story := range def.Stories {
		fmt.Fprintf(&b, "## %s. %s\nActor: %s\nTexto: %s\n\n", story.ID, story.Title, story.Actor, story.Text)
		for _, criterion := range def.CriteriaForStory(story.ID) {
			fmt.Fprintf(&b, "### %s. %s\nHistorias: %s\nVerificación: %s\nCondición: %s\n\n", criterion.ID, criterion.Title, strings.Join(criterion.Stories, ", "), criterion.Verification, criterion.Condition)
		}
	}
	b.WriteString("## Decisiones pendientes\n- …\n\n## Referencias\n- …\n")
	return []byte(b.String())
}
