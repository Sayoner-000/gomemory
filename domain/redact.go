package domain

import (
	"regexp"
	"strings"
)

const (
	privateOpenTag  = "<private"
	privateCloseTag = "</private"
	// maxPrivateTagLen acota la búsqueda del '>' de una etiqueta con
	// atributos: un "<private" suelto en prosa no convierte el resto del
	// texto en etiqueta.
	maxPrivateTagLen = 256
)

type privateTag int

const (
	notPrivateTag privateTag = iota
	privateOpen
	privateClose
)

// matchPrivateTag reconoce una etiqueta privada al inicio de s y devuelve su
// tipo y longitud. Sin distinguir mayúsculas (ASCII):
//   - apertura: "<private" seguido de ">" o de espacio en blanco y
//     atributos hasta ">" (sin "<" intermedio);
//   - cierre: "</private" con espacio en blanco opcional antes de ">".
//
// "<privateX>" u otro nombre no es una etiqueta privada (C-001,
// acr_9ba278e2: "<private >" y "</private >" se usaban para evadirla).
func matchPrivateTag(s string) (privateTag, int) {
	switch {
	case hasPrefixFold(s, privateCloseTag):
		i := len(privateCloseTag)
		for i < len(s) && isTagSpace(s[i]) {
			i++
		}
		if i < len(s) && s[i] == '>' {
			return privateClose, i + 1
		}
	case hasPrefixFold(s, privateOpenTag):
		i := len(privateOpenTag)
		if i < len(s) && s[i] == '>' {
			return privateOpen, i + 1
		}
		if i >= len(s) || !isTagSpace(s[i]) {
			return notPrivateTag, 0
		}
		for ; i < len(s) && i < maxPrivateTagLen; i++ {
			switch s[i] {
			case '>':
				return privateOpen, i + 1
			case '<':
				return notPrivateTag, 0
			}
		}
	}
	return notPrivateTag, 0
}

func isTagSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

// RedactPrivate elimina cualquier región <private>...</private> (y su
// contenido) antes de persistir o emitir texto, para que secretos o tokens
// pegados por error nunca lleguen a la base de datos ni a una salida.
//
// Recorre las etiquetas con profundidad (spec 037, FR-003): un bloque
// anidado se elimina completo, una apertura sin cierre es privada hasta el
// final del texto y un cierre aislado se descarta sin ocultar el texto
// público que le sigue. Las etiquetas se comparan en ASCII sin distinguir
// mayúsculas sobre el texto original, así los índices no se desplazan con
// Unicode; admiten espacios y atributos (ver matchPrivateTag). Es idempotente.
func RedactPrivate(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	depth, last := 0, 0
	for i := 0; i < len(s); {
		if s[i] != '<' {
			i++
			continue
		}
		switch kind, n := matchPrivateTag(s[i:]); kind {
		case privateOpen:
			if depth == 0 {
				b.WriteString(s[last:i])
			}
			depth++
			i += n
			last = i
		case privateClose:
			if depth == 0 {
				b.WriteString(s[last:i])
			} else {
				depth--
			}
			i += n
			last = i
		default:
			i++
		}
	}
	if depth == 0 {
		b.WriteString(s[last:])
	}
	return b.String()
}

// hasPrefixFold compara un prefijo ASCII en minúsculas sin distinguir
// mayúsculas, byte a byte: no aplica plegado Unicode, así ninguna runa no
// ASCII puede pasar por una letra de la etiqueta.
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// secretPatterns es la lista fija de patrones de secretos conocidos que
// RedactSecrets reemplaza como segunda capa de defensa, además de
// RedactPrivate (specs/009-mitigacion-riesgos, Historia de Usuario 2). No es
// configurable ni basada en heurísticas de entropía: solo reconoce formas de
// texto de proveedores concretos, para minimizar falsos positivos sobre
// memorias legítimas.
var secretPatterns = []struct {
	label   string
	pattern *regexp.Regexp
}{
	{"aws-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"github-token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"ai-provider-key", regexp.MustCompile(`sk-(ant-)?[A-Za-z0-9_-]{20,}`)},
	{"slack-token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]+`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
	{"pem-private-key", regexp.MustCompile(`(?s)-----BEGIN[^-]*PRIVATE KEY-----.*?-----END[^-]*PRIVATE KEY-----`)},
}

// RedactSecrets reemplaza patrones de secretos conocidos (claves de AWS,
// tokens de GitHub, claves de proveedores de IA, tokens de Slack, JWT y
// bloques de clave privada PEM) por un placeholder, antes de persistir una
// memoria. Complementa a RedactPrivate (no la reemplaza): cubre secretos
// pegados por error FUERA de un bloque <private>. Contenido que no matchea
// ningún patrón se devuelve intacto.
func RedactSecrets(s string) string {
	for _, p := range secretPatterns {
		s = p.pattern.ReplaceAllString(s, "[REDACTED:"+p.label+"]")
	}
	return s
}

// ProtectText es la protección única de texto (spec 037, FR-001/FR-002):
// primero elimina regiones privadas y después oculta credenciales
// reconocidas. Escrituras y lecturas de campos sensibles deben pasar por
// aquí, no por una de las dos capas sueltas. Es idempotente.
func ProtectText(s string) string {
	return RedactSecrets(RedactPrivate(s))
}
