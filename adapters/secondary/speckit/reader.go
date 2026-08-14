// Package speckit implementa ports.SpecKitReader: lectura directa de
// archivos (sin caché, sin base de datos) de los artefactos que ya produce
// Spec Kit en este proyecto (feature 015, research.md §1).
package speckit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"mem/application/ports"
	"mem/domain"
)

// minWordLength descarta palabras cortas (artículos, preposiciones) al medir
// solapamiento entre una línea de un artefacto y la tarea — mismo criterio
// de "señal mínima" que detect_duplicates.go usa para sus propios tokens.
const minWordLength = 4

type featureFile struct {
	FeatureDirectory string `json:"feature_directory"`
}

// Reader implementa ports.SpecKitReader.
type Reader struct{}

var _ ports.SpecKitReader = Reader{}

func (Reader) ActiveFeature(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, ".specify", "feature.json"))
	if err != nil {
		// Proyecto sin Spec Kit inicializado no es un error (FR-015, edge case
		// del spec): BuildContextPack sigue con las demás fuentes habilitadas.
		return "", nil
	}
	var ff featureFile
	if err := json.Unmarshal(data, &ff); err != nil {
		return "", nil
	}
	return strings.TrimPrefix(ff.FeatureDirectory, "specs/"), nil
}

func (Reader) Read(root, feature, task string) (domain.SpecKitFeatureContext, error) {
	ctx := domain.SpecKitFeatureContext{Feature: feature}
	if feature == "" {
		return ctx, nil
	}
	featureDir := filepath.Join(root, "specs", feature)
	words := taskWords(task)

	ctx.Requirements = relevantLines(filepath.Join(featureDir, "spec.md"), "- **FR-", words)
	ctx.Decisions = relevantLines(filepath.Join(featureDir, "research.md"), "**Decisión", words)
	ctx.TaskDependencies = relevantLines(filepath.Join(featureDir, "tasks.md"), "- [ ] T", words)
	ctx.Constraints = relevantConstraintLines(filepath.Join(root, ".specify", "memory", "constitution.md"), words)

	return ctx, nil
}

// taskWords tokeniza task en palabras en minúscula de al menos
// minWordLength runas — la misma señal de solapamiento léxico simple que ya
// usa detect_duplicates.go, aplicada aquí para filtrar líneas de artefactos
// en vez de agrupar memorias.
func taskWords(task string) []string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(task), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len([]rune(w)) >= minWordLength {
			words = append(words, w)
		}
	}
	return words
}

// relevantLines lee path (si existe) y devuelve las líneas que empiezan con
// prefix y comparten al menos una palabra con taskWords. Un archivo ausente
// no es error: la feature puede no tener ese artefacto todavía.
func relevantLines(path, prefix string, taskWords []string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		if hasOverlap(trimmed, taskWords) {
			out = append(out, trimmed)
		}
	}
	return out
}

// relevantConstraintLines aplica el mismo filtro de relevancia a
// constitution.md, sobre líneas que expresan una obligación ("DEBE"/"MUST"),
// consistentes con la convención de escritura de este proyecto.
func relevantConstraintLines(path string, taskWords []string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !(strings.Contains(trimmed, "DEBE") || strings.Contains(trimmed, "MUST")) {
			continue
		}
		if hasOverlap(trimmed, taskWords) {
			out = append(out, trimmed)
		}
	}
	return out
}

func hasOverlap(line string, taskWords []string) bool {
	lower := strings.ToLower(line)
	for _, w := range taskWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}
