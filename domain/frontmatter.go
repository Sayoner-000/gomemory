package domain

import "strings"

// ParseFrontmatter separa una cabecera YAML mínima (`---` … `---`) del
// cuerpo de un Markdown (feature 040: reglas de expand y skills). Solo
// admite pares `clave: valor` de una línea, con valores opcionalmente entre
// comillas simples o dobles y comentarios `#` fuera de comillas; es todo lo
// que necesitan las reglas y las skills, sin añadir una dependencia YAML.
// ok es false si no hay cabecera completa.
func ParseFrontmatter(raw []byte) (fields map[string]string, body string, ok bool) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, "", false
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, "", false
	}
	header := rest[:end]
	after := rest[end+len("\n---"):]
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		after = after[nl+1:]
	} else {
		after = ""
	}
	fields = map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		key, value, found := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !found || key == "" || strings.HasPrefix(key, "#") {
			continue
		}
		fields[key] = frontmatterValue(strings.TrimSpace(value))
	}
	return fields, strings.TrimSpace(after), true
}

func frontmatterValue(v string) string {
	if len(v) >= 1 && (v[0] == '\'' || v[0] == '"') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : end+1]
		}
		return v[1:]
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
