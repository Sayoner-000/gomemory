package domain

import "regexp"

var privateBlock = regexp.MustCompile(`(?is)<private>.*?</private>`)

// RedactPrivate elimina cualquier bloque <private>...</private> (y su
// contenido) antes de persistir una memoria, para que secretos o tokens
// pegados por error nunca lleguen a la base de datos.
func RedactPrivate(s string) string {
	return privateBlock.ReplaceAllString(s, "")
}
