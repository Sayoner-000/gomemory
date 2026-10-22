package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Observation es una observación de instincts en la cola (feature 040,
// research R12/R18). ID es un UUIDv7: el plegado es idempotente por
// observación, así que una ya plegada nunca vuelve a contar aunque el
// proceso muera antes de borrarla de la cola. Sample está redactado.
type Observation struct {
	ID     string    `json:"id"`
	T      time.Time `json:"t"`
	Kind   string    `json:"kind"`
	Key    string    `json:"key"`
	Sample string    `json:"sample,omitempty"`
}

type Instinct struct {
	Slug         string   `json:"slug"`
	Kind         string   `json:"kind"`
	Description  string   `json:"description"`
	Observations int      `json:"observations"`
	Corrections  int      `json:"corrections"`
	Confidence   float64  `json:"confidence"`
	Scope        string   `json:"scope"`
	Status       string   `json:"status"`
	Projects     []string `json:"projects,omitempty"`
}

func InstinctConfidence(obs, corrections int) float64 {
	if obs < 1 {
		obs = 1
	}
	return math.Min(.9, .3+.1*math.Floor(math.Log2(float64(obs)))+.1*float64(corrections))
}

var observationWord = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ObservationKey deriva la clave de un comando ya protegido: un token que
// parece credencial (p. ej. `sk-…`) nunca llega a la clave ni al slug que
// se persiste y se comparte entre proyectos (B-001 del ACR de 040).
func ObservationKey(cmd string) string {
	commands := SplitShellCommands(ProtectText(cmd))
	if len(commands) == 0 || len(commands[0]) == 0 {
		return ""
	}
	tokens := commands[0]
	parts := []string{strings.ToLower(filepath.Base(tokens[0]))}
	if !observationWord.MatchString(parts[0]) {
		return ""
	}
	if len(tokens) > 1 && !strings.HasPrefix(tokens[1], "-") {
		sub := strings.ToLower(tokens[1])
		if observationWord.MatchString(sub) {
			parts = append(parts, sub)
		}
	}
	return TruncateRunes("cmd:"+strings.Join(parts, "-"), 80)
}

// ProtectObservationKey sanea una clave o un slug ya guardados: si contiene
// una credencial reconocida, se reduce a la parte anterior a ella, que es la
// misma clave que daría hoy ObservationKey. Las claves se guardan en
// minúsculas, así que también se busca sobre su versión en mayúsculas (el
// patrón de AWS solo reconoce AKIA…); son ASCII y los índices coinciden. Sin
// nada que proteger, devuelve la clave intacta.
func ProtectObservationKey(key string) string {
	cut := -1
	for _, variant := range []string{key, strings.ToUpper(key)} {
		if i := strings.Index(ProtectText(variant), "[REDACTED"); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut < 0 {
		return key
	}
	prefix, rest, ok := strings.Cut(strings.TrimRight(key[:cut], "-"), ":")
	if ok && rest != "" && observationWord.MatchString(rest) {
		return prefix + ":" + rest
	}
	if ok && observationWord.MatchString(prefix) {
		return prefix + ":redacted"
	}
	return "redacted"
}

var correctionStart = regexp.MustCompile(`(?i)^(?:no,?\s+usa\b|usa\s+.+\s+en\s+(?:vez|lugar)\s+de\b|siempre\s+|nunca\s+|no\s+uses\b|don't\s+|do not\s+|always\s+|never\s+)`)

func CorrectionKey(prompt string) string {
	clean := strings.TrimSpace(strings.ToLower(RedactSecrets(RedactPrivate(prompt))))
	if !correctionStart.MatchString(clean) {
		return ""
	}
	clean = strings.Join(strings.Fields(clean), " ")
	hash := sha256.Sum256([]byte(clean))
	return "correction:" + hex.EncodeToString(hash[:8])
}

func CanSetInstinctStatus(from, to string) bool {
	return (from == "proposed" && (to == "confirmed" || to == "rejected")) || (from == to && (to == "confirmed" || to == "rejected"))
}

func TruncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}
