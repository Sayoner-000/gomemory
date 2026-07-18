package domain

import "regexp"

var privateBlock = regexp.MustCompile(`(?is)<private>.*?</private>`)

// RedactPrivate elimina cualquier bloque <private>...</private> (y su
// contenido) antes de persistir una memoria, para que secretos o tokens
// pegados por error nunca lleguen a la base de datos.
func RedactPrivate(s string) string {
	return privateBlock.ReplaceAllString(s, "")
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
