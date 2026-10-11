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

// TestRedactPrivate_EstructuraDeEtiquetas cubre H-02 (spec 037, FR-003):
// anidamiento, mayúsculas, Unicode, apertura sin cierre y cierre aislado.
func TestRedactPrivate_EstructuraDeEtiquetas(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"sin etiquetas", "texto público ñandú 🚀", "texto público ñandú 🚀"},
		{"bloque simple", "a <private>X</private> b", "a  b"},
		{"bloque vacío", "a<private></private>b", "ab"},
		{"consecutivos", "a<private>X</private>b<private>Y</private>c", "abc"},
		{"anidado dos niveles", "a <private>X <private>Y</private> Z</private> b", "a  b"},
		{"anidado tres niveles", "a<private>1<private>2<private>3</private>4</private>5</private>b", "ab"},
		{"mayúsculas mixtas", "a <PRIVATE>X</Private> b", "a  b"},
		{"unicode alrededor y dentro", "ñ<private>ü 秘密 🚀</private>é", "ñé"},
		{"apertura sin cierre", "público <private>secreto hasta el final", "público "},
		{"anidado incompleto", "a <private>X <private>Y</private> sigue", "a "},
		{"cierre aislado", "antes </private> después", "antes  después"},
		{"cierre aislado y luego bloque", "a </private> b <private>X</private> c", "a  b  c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactPrivate(tt.in)
			if got != tt.want {
				t.Fatalf("RedactPrivate(%q) = %q, quiero %q", tt.in, got, tt.want)
			}
			if again := RedactPrivate(got); again != got {
				t.Fatalf("segunda aplicación no estable: %q -> %q", got, again)
			}
		})
	}
}

// TestProtectText_ComponeYEsIdempotente cubre FR-001/FR-003: una única
// composición de privacidad y credenciales, estable al reaplicarse.
func TestProtectText_ComponeYEsIdempotente(t *testing.T) {
	in := "público <Private>SECRETO_UNO <private>SECRETO_DOS</private></private> clave AKIAIOSFODNN7EXAMPLE <private>SECRETO_ABIERTO"
	got := ProtectText(in)
	for _, gone := range []string{"SECRETO_UNO", "SECRETO_DOS", "AKIAIOSFODNN7EXAMPLE", "SECRETO_ABIERTO", "<private", "</private"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(gone)) {
			t.Fatalf("ProtectText dejó %q en %q", gone, got)
		}
	}
	if !strings.HasPrefix(got, "público ") || !strings.Contains(got, "[REDACTED:aws-key]") {
		t.Fatalf("ProtectText perdió texto público o placeholder: %q", got)
	}
	if ProtectText(got) != got {
		t.Fatalf("ProtectText no es idempotente: %q", got)
	}
	public := "nada que ocultar · ñ 🚀"
	if ProtectText(public) != public {
		t.Fatalf("ProtectText alteró texto público")
	}
}

// TestRedactPrivate_VariantesDeEtiqueta cubre C-001 (acr_9ba278e2): una
// etiqueta con espacios o atributos sigue siendo una región privada; un
// nombre distinto (privateX, private-ish) no lo es.
func TestRedactPrivate_VariantesDeEtiqueta(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a <private >S</private> b", "a  b"},
		{"a <PRIVATE  >S</private > b", "a  b"},
		{"a <private\tmotivo=\"x\">S</private\n> b", "a  b"},
		{"a <private class='k'>S <private >T</private></private> b", "a  b"},
		{"a <private >S sin cierre", "a "},
		{"a </private  > b", "a  b"},
		{"a <privateX>no</privateX> b", "a <privateX>no</privateX> b"},
		{"a <private sin cerrar la etiqueta", "a <private sin cerrar la etiqueta"},
	}
	for _, tt := range tests {
		got := RedactPrivate(tt.in)
		if got != tt.want {
			t.Errorf("RedactPrivate(%q) = %q, quiero %q", tt.in, got, tt.want)
		}
		if RedactPrivate(got) != got {
			t.Errorf("no idempotente sobre %q", got)
		}
	}
	if !IsPrivateContent("x <private >S</private>") || !ContainsProtected("x <Private data-a='1'>S") {
		t.Error("detectores no reconocen la variante")
	}
	if !HasOrphanPrivateClose("Z</private > fin") {
		t.Error("cierre huérfano con espacio no detectado")
	}
}
