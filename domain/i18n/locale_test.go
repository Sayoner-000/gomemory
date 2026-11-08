package i18n

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	tests := []struct {
		pattern  string
		params   map[string]string
		expected string
	}{
		{
			pattern:  "Hola %{name}",
			params:   map[string]string{"name": "Mundo"},
			expected: "Hola Mundo",
		},
		{
			pattern:  "Sin cambios",
			params:   nil,
			expected: "Sin cambios",
		},
		{
			pattern:  "Código %{code} en %{name}",
			params:   map[string]string{"code": "es", "name": "Español"},
			expected: "Código es en Español",
		},
		{
			pattern:  "Falta %{missing}",
			params:   map[string]string{"other": "val"},
			expected: "Falta %{missing}",
		},
	}

	for _, tt := range tests {
		got := Interpolate(tt.pattern, tt.params)
		if got != tt.expected {
			t.Errorf("Interpolate(%q, %v) = %q; want %q", tt.pattern, tt.params, got, tt.expected)
		}
	}
}
