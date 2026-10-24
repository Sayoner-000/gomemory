package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	LastSeen     string   `json:"last_seen,omitempty"`
}

func InstinctConfidence(obs, corrections int) float64 {
	if obs < 1 {
		obs = 1
	}
	return math.Min(.9, .3+.1*math.Floor(math.Log2(float64(obs)))+.1*float64(corrections))
}

var observationWord = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// trivialCommands son lecturas y navegación que el agente repite sin que
// formen un hábito: no aportan nada al contexto y se excluyen de la clave.
var trivialCommands = map[string]bool{
	"cat": true, "sed": true, "grep": true, "rg": true, "ls": true, "head": true,
	"tail": true, "find": true, "echo": true, "cd": true, "wc": true, "pwd": true,
	"sqlite3": true,
}

func trivialCommand(toks []string) bool {
	bin := strings.ToLower(filepath.Base(toks[0]))
	return trivialCommands[bin] || (bin == "python3" && len(toks) > 1 && toks[1] == "-c")
}

// observationParts devuelve binario y subcomando del primer comando
// significativo: salta asignaciones VAR=valor, sudo y comandos triviales.
func observationParts(cmd string) []string {
	for _, toks := range SplitShellCommands(ProtectText(cmd)) {
		toks = stripEnvAssignments(toks)
		if len(toks) == 0 || trivialCommand(toks) {
			continue
		}
		parts := []string{strings.ToLower(filepath.Base(toks[0]))}
		if !observationWord.MatchString(parts[0]) {
			return nil
		}
		if len(toks) > 1 && !strings.HasPrefix(toks[1], "-") {
			sub := strings.ToLower(toks[1])
			if observationWord.MatchString(sub) {
				parts = append(parts, sub)
			}
		}
		return parts
	}
	return nil
}

// ObservationKey deriva la clave de un comando ya protegido: un token que
// parece credencial (p. ej. `sk-…`) nunca llega a la clave ni al slug que
// se persiste y se comparte entre proyectos (B-001 del ACR de 040).
func ObservationKey(cmd string) string {
	parts := observationParts(cmd)
	if len(parts) == 0 {
		return ""
	}
	return TruncateRunes("cmd:"+strings.Join(parts, "-"), 80)
}

// ObservationLabel es la forma legible de la clave («go test»).
func ObservationLabel(cmd string) string {
	return strings.Join(observationParts(cmd), " ")
}

// StaleInstinct indica que una fila de comando se guardó con un criterio de
// claves anterior: su muestra ya no produce su slug (asignaciones, cd o un
// comando trivial). Se deja de inyectar sin borrar la fila.
func StaleInstinct(i Instinct) bool {
	return i.Kind == "cmd" && ObservationKey(i.Description) != i.Slug
}

// ContextLine es la línea de la sección ## Instincts: el patrón con sus
// estadísticas, nunca la muestra cruda de una ejecución.
func (i Instinct) ContextLine() string {
	if i.Kind != "cmd" {
		return fmt.Sprintf("- corrección: %s (%.1f)", i.Description, i.Confidence)
	}
	label := ObservationLabel(i.Description)
	if label == "" {
		label = strings.TrimPrefix(i.Slug, "cmd:")
	}
	usage := fmt.Sprintf("%d usos", i.Observations)
	if i.Scope == "global" {
		usage = fmt.Sprintf("%d proyectos", len(i.Projects))
	}
	line := fmt.Sprintf("- `%s` · %s", label, usage)
	if len(i.LastSeen) >= 10 {
		line += " · último " + i.LastSeen[:10]
	}
	return fmt.Sprintf("%s (%.1f)", line, i.Confidence)
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
