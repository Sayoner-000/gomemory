package domain

import "strings"

// Extract devuelve un extracto compacto de s para mostrarlo en contextos
// acotados (progressive disclosure, capa 1). Reglas, en orden:
//   - maxChars <= 0 o s vacío → s recortado (sin límite).
//   - s ya cabe en maxChars (en runas) → intacto.
//   - si la primera oración (hasta ". ") cabe en maxChars → esa oración.
//   - en otro caso, trunca a maxChars sin cortar palabra y añade "…".
//
// Es una función pura (sin I/O): la comparten Builder.Build, search_memories y
// list_memories para que el acotado sea idéntico por cualquier vía. El límite se
// mide en runas para no partir caracteres multibyte (acentos del español).
func Extract(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if maxChars <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}

	// Primera oración, si cabe entera en el techo.
	if i := strings.Index(s, ". "); i >= 0 {
		sentence := strings.TrimSpace(s[:i+1])
		if sentence != "" && len([]rune(sentence)) <= maxChars {
			return sentence
		}
	}

	// Truncado por límite de palabra.
	cut := string(r[:maxChars])
	if idx := strings.LastIndexAny(cut, " \n\t"); idx > 0 {
		cut = cut[:idx]
	}
	return strings.TrimRight(cut, " \n\t.,;:") + "…"
}
