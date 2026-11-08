package i18n

import (
	"regexp"
	"strings"
)

// Locale representa los metadatos de un idioma descubierto.
type Locale struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Default bool   `json:"default,omitempty"`
	Order   int    `json:"order,omitempty"`
}

var paramRegex = regexp.MustCompile(`%\{([a-zA-Z0-9_]+)\}`)

// Interpolate reemplaza patrones %{var} con los valores del mapa.
func Interpolate(pattern string, params map[string]string) string {
	if len(params) == 0 {
		return pattern
	}
	return paramRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "%{"), "}")
		if val, ok := params[key]; ok {
			return val
		}
		return match
	})
}
