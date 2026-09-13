package domain

import (
	"fmt"
	"regexp"
)

// impactAnnotationFormat y impactAnnotationPattern describen la misma nota:
// uno para escribirla y otro para reconocerla. El test de ida y vuelta de
// StripImpactAnnotation los mantiene alineados.
//
// El símbolo es un nombre del grafo de código: sin espacios, pero puede llevar
// corchetes (Cache[T]). Reconocerlo como \S+ acepta esos nombres y sigue
// rechazando un texto citado, que sí tiene espacios.
const impactAnnotationFormat = "\n\n[impacto: %s es un hotspot con %d llamadores directos]"

var impactAnnotationPattern = regexp.MustCompile(`\n\n\[impacto: \S+ es un hotspot con \d+ llamadores directos\]$`)

// ImpactAnnotation es la nota que se anexa al contenido de una memoria anclada a
// un hotspot. Vive en el dominio para que quien la escribe (la persistencia) y
// quien la descuenta (el gate de duplicados) compartan un solo formato.
func ImpactAnnotation(symbol string, fanIn int) string {
	return fmt.Sprintf(impactAnnotationFormat, symbol, fanIn)
}

// StripImpactAnnotation quita esa nota del final del contenido solo si tiene su
// forma exacta: un texto citado que se le parezca conserva su cola.
func StripImpactAnnotation(content string) string {
	if loc := impactAnnotationPattern.FindStringIndex(content); loc != nil {
		return content[:loc[0]]
	}
	return content
}
