package domain

import (
	"regexp"
	"strings"
)

// learningHeaderPattern reconoce encabezados de nivel 2 o 3 que introducen
// una sección de aprendizajes, en español o en inglés, con o sin dos puntos
// finales, singular o plural, sin distinguir mayúsculas (feature 030, US3).
var learningHeaderPattern = regexp.MustCompile(
	`(?im)^#{2,3}\s+(?:Aprendizajes(?:\s+Clave)?|Key\s+Learnings?|Learnings?):?\s*$`,
)

// Los patrones se compilan una sola vez y se reutilizan en cada extracción.
var (
	learningNextHeaderPattern = regexp.MustCompile(`\n#{1,3} `)
	learningNumberedPattern   = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+(.+)`)
	learningBulletPattern     = regexp.MustCompile(`(?m)^\s*[-*]\s+(.+)`)
	learningBoldPattern       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	learningCodePattern       = regexp.MustCompile("`([^`]+)`")
	learningItalicPattern     = regexp.MustCompile(`\*([^*]+)\*`)
)

const (
	minLearningLength = 20
	minLearningWords  = 4
)

// ExtractLearnings extrae ítems de aprendizaje de un texto libre (el mensaje
// final de un subagente, feature 030 US3). Busca secciones como
// "## Key Learnings:" o "## Aprendizajes Clave" y extrae ítems numerados
// (1. texto) o de viñeta (- texto). Devuelve los aprendizajes de la ÚLTIMA
// sección que produzca ítems válidos (la más reciente si el texto tiene
// varias). Función pura, sin I/O.
func ExtractLearnings(text string) []string {
	matches := learningHeaderPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	for i := len(matches) - 1; i >= 0; i-- {
		sectionStart := matches[i][1]
		sectionText := text[sectionStart:]

		if nextHeader := learningNextHeaderPattern.FindStringIndex(sectionText); nextHeader != nil {
			sectionText = sectionText[:nextHeader[0]]
		}

		var learnings []string

		numbered := learningNumberedPattern.FindAllStringSubmatch(sectionText, -1)
		if len(numbered) > 0 {
			for _, m := range numbered {
				if cleaned := cleanLearningMarkdown(m[1]); isValidLearning(cleaned) {
					learnings = append(learnings, cleaned)
				}
			}
		}

		if len(learnings) == 0 {
			bullets := learningBulletPattern.FindAllStringSubmatch(sectionText, -1)
			for _, m := range bullets {
				if cleaned := cleanLearningMarkdown(m[1]); isValidLearning(cleaned) {
					learnings = append(learnings, cleaned)
				}
			}
		}

		if len(learnings) > 0 {
			return learnings
		}
	}

	return nil
}

// isValidLearning descarta ítems demasiado cortos: ni «corto» ni una sola
// palabra cuentan como aprendizaje capturable.
func isValidLearning(s string) bool {
	return len(s) >= minLearningLength && len(strings.Fields(s)) >= minLearningWords
}

// cleanLearningMarkdown quita el marcado básico (negrita, código inline,
// cursiva) y colapsa espacios, para que el ítem persistido sea texto plano
// legible.
func cleanLearningMarkdown(text string) string {
	text = learningBoldPattern.ReplaceAllString(text, "$1")
	text = learningCodePattern.ReplaceAllString(text, "$1")
	text = learningItalicPattern.ReplaceAllString(text, "$1")
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}
