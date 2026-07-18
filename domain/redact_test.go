package domain

import (
	"strings"
	"testing"
)

// TestRedactSecrets_DetectsKnownPatterns cubre la Historia de Usuario 2
// (specs/009-mitigacion-riesgos): un secreto reconocible pegado fuera de
// <private> debe quedar redactado igual.
func TestRedactSecrets_DetectsKnownPatterns(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantGone string // subcadena del secreto que NO debe sobrevivir
		wantTag  string // label esperado en el placeholder
	}{
		{
			name:     "clave de AWS",
			in:       "clave de ejemplo: AKIAIOSFODNN7EXAMPLE en el mensaje",
			wantGone: "AKIAIOSFODNN7EXAMPLE",
			wantTag:  "aws-key",
		},
		{
			name:     "token de GitHub",
			in:       "usa ghp_1234567890abcdefghijklmnopqrstuvwxyzABCD para autenticar",
			wantGone: "ghp_1234567890abcdefghijklmnopqrstuvwxyzABCD",
			wantTag:  "github-token",
		},
		{
			name:     "clave de proveedor de IA (Anthropic)",
			in:       "ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890",
			wantGone: "sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890",
			wantTag:  "ai-provider-key",
		},
		{
			name:     "token de Slack",
			in:       "webhook con xoxb-FAKE-TEST-TOKEN-NOT-REAL configurado",
			wantGone: "xoxb-FAKE-TEST-TOKEN-NOT-REAL",
			wantTag:  "slack-token",
		},
		{
			name:     "JWT",
			in:       "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantGone: "eyJzdWIiOiIxMjM0NTY3ODkwIn0",
			wantTag:  "jwt",
		},
		{
			name:     "bloque PEM de clave privada",
			in:       "config:\n-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA1234567890abcdef\n-----END RSA PRIVATE KEY-----\nresto",
			wantGone: "MIIEpAIBAAKCAQEA1234567890abcdef",
			wantTag:  "pem-private-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSecrets(tt.in)
			if strings.Contains(got, tt.wantGone) {
				t.Fatalf("el secreto no quedó redactado: %q", got)
			}
			if !strings.Contains(got, "[REDACTED:"+tt.wantTag+"]") {
				t.Fatalf("esperaba placeholder [REDACTED:%s] en %q", tt.wantTag, got)
			}
		})
	}
}

// TestRedactSecrets_NoFalsePositivesOnLegitimateContent confirma que
// contenido normal (sin patrones de secreto) se persiste intacto — un
// escáner demasiado agresivo degradaría memorias legítimas.
func TestRedactSecrets_NoFalsePositivesOnLegitimateContent(t *testing.T) {
	tests := []string{
		"decisión: usamos Fiber para el enrutamiento HTTP",
		"el bug estaba en la función ParseConfig, línea 42",
		"id de memoria: 12345, proyecto: go_memory",
		"hash sha256 corto: a1b2c3d4e5f6",
		"nombre de variable: skip_validation_flag",
	}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			if got := RedactSecrets(in); got != in {
				t.Fatalf("falso positivo: %q → %q", in, got)
			}
		})
	}
}

// TestRedactSecrets_ComposesWithRedactPrivate confirma que ambas funciones
// pueden encadenarse en cualquier orden sin interferir entre sí: un
// contenido con un secreto reconocido FUERA de <private> y otro secreto
// arbitrario DENTRO de <private> debe quedar limpio con cualquier orden de
// aplicación.
func TestRedactSecrets_ComposesWithRedactPrivate(t *testing.T) {
	in := "clave pública AKIAIOSFODNN7EXAMPLE, y aparte <private>token-interno-cualquiera</private> guardado"

	orderA := RedactSecrets(RedactPrivate(in))
	orderB := RedactPrivate(RedactSecrets(in))

	for _, got := range []string{orderA, orderB} {
		if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
			t.Fatalf("clave de AWS no redactada: %q", got)
		}
		if strings.Contains(got, "token-interno-cualquiera") {
			t.Fatalf("bloque <private> no redactado: %q", got)
		}
	}
}
