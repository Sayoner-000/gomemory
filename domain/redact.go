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
	// privateOpenLoose es una apertura con atributo sin '>' (R2-03): al
	// persistir cuenta como apertura; al escanear, no (R3-03).
	privateOpenLoose
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
		for j := i; j < len(s) && j < maxPrivateTagLen; j++ {
			switch s[j] {
			case '>':
				return privateOpen, j + 1
			case '<':
				return malformedPrivateOpen(s, i)
			}
		}
		return malformedPrivateOpen(s, i)
	}
	return notPrivateTag, 0
}

// privateAttr reconoce un atributo (`nombre=`) tras el espacio de la
// apertura.
var privateAttr = regexp.MustCompile(`^[ \t\r\n\f]+[A-Za-z_][A-Za-z0-9_.:-]*[ \t]*=`)

// malformedPrivateOpen decide una apertura sin '>': si lleva un atributo
// (`<private token=…`) quien escribe quiso marcar un secreto y la etiqueta
// falla cerrada, privada hasta su cierre o el final del texto (R2-03). Un
// "<private" suelto en prosa, sin atributo, sigue sin ser etiqueta.
func malformedPrivateOpen(s string, attrs int) (privateTag, int) {
	if privateAttr.MatchString(s[attrs:]) {
		return privateOpenLoose, attrs
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
func RedactPrivate(s string) string { return redactPrivate(s, false) }

// redactPrivateKeepLines es RedactPrivate pero deja en lugar de cada región
// eliminada tantos saltos de línea como tenía, para que los números de línea
// calculados después coincidan con el texto original (H-05 de 040).
func redactPrivateKeepLines(s string) string { return redactPrivate(s, true) }

// redactClosedPrivateKeepLines es la variante para detectar secretos (shield):
// solo quita regiones privadas bien formadas y cerradas. Una apertura mal
// formada o sin cierre no oculta el resto del texto al escáner (R3-03); el
// informe nunca incluye valores, así que escanearlo no los expone.
func redactClosedPrivateKeepLines(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		if kind, _ := matchPrivateTag(s[i:]); kind != privateOpen {
			continue
		}
		end := closedPrivateEnd(s, i)
		if end < 0 {
			continue
		}
		b.WriteString(s[last:i])
		b.WriteString(strings.Repeat("\n", strings.Count(s[i:end], "\n")))
		last = end
		i = end - 1
	}
	b.WriteString(s[last:])
	return b.String()
}

// closedPrivateEnd devuelve el final del cierre que equilibra la apertura
// bien formada de la posición i, o -1 si no se cierra.
func closedPrivateEnd(s string, i int) int {
	depth := 0
	for j := i; j < len(s); j++ {
		if s[j] != '<' {
			continue
		}
		switch kind, n := matchPrivateTag(s[j:]); kind {
		case privateOpen:
			depth++
			j += n - 1
		case privateClose:
			depth--
			if depth == 0 {
				return j + n
			}
			j += n - 1
		}
	}
	return -1
}

func redactPrivate(s string, keepLines bool) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	drop := func(seg string) {
		if keepLines {
			b.WriteString(strings.Repeat("\n", strings.Count(seg, "\n")))
		}
	}
	depth, last := 0, 0
	for i := 0; i < len(s); {
		if s[i] != '<' {
			i++
			continue
		}
		switch kind, n := matchPrivateTag(s[i:]); kind {
		case privateOpen, privateOpenLoose:
			if depth == 0 {
				b.WriteString(s[last:i])
			} else {
				drop(s[last:i])
			}
			drop(s[i : i+n])
			depth++
			i += n
			last = i
		case privateClose:
			if depth == 0 {
				b.WriteString(s[last:i])
			} else {
				drop(s[last:i])
				depth--
			}
			drop(s[i : i+n])
			i += n
			last = i
		default:
			i++
		}
	}
	if depth == 0 {
		b.WriteString(s[last:])
	} else {
		drop(s[last:])
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

// SecretHit identifies a secret without retaining its value.
type SecretHit struct {
	Category string
	Line     int
}

func FindSecrets(s string) []SecretHit {
	s = redactClosedPrivateKeepLines(s)
	var hits []SecretHit
	for _, p := range secretPatterns {
		for _, loc := range p.pattern.FindAllStringIndex(s, -1) {
			hits = append(hits, SecretHit{p.label, 1 + strings.Count(s[:loc[0]], "\n")})
		}
	}
	return hits
}

// ProtectText es la protección única de texto (spec 037, FR-001/FR-002):
// primero elimina regiones privadas y después oculta credenciales
// reconocidas. Escrituras y lecturas de campos sensibles deben pasar por
// aquí, no por una de las dos capas sueltas. Es idempotente.
func ProtectText(s string) string {
	return RedactSecrets(RedactPrivate(s))
}
