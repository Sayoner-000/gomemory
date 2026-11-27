package domain

import "regexp"

// impactAnnotationPattern reconoce la nota de impacto que InsertMemory anexaba
// al contenido de una memoria anclada a un hotspot, cuando un grafo externo
// anterior daba fan-in. La anotación ya no se escribe (HCD-069),
// pero las memorias antiguas la conservan y el gate de duplicados debe seguir
// descontándola.
//
// El símbolo es un nombre del grafo de código: sin espacios, pero puede llevar
// corchetes (Cache[T]). Reconocerlo como \S+ acepta esos nombres y sigue
// rechazando un texto citado, que sí tiene espacios.
var impactAnnotationPattern = regexp.MustCompile(`\n\n\[impacto: \S+ es un hotspot con \d+ llamadores directos\]$`)

// StripImpactAnnotation quita esa nota del final del contenido solo si tiene su
// forma exacta: un texto citado que se le parezca conserva su cola.
func StripImpactAnnotation(content string) string {
	if loc := impactAnnotationPattern.FindStringIndex(content); loc != nil {
		return content[:loc[0]]
	}
	return content
}
