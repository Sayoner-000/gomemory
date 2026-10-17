package domain

import (
	_ "embed"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

//go:embed socratica_generation.schema.json
var SocraticaGenerationSchema string

type GenerationResult struct {
	Questions    []Question         `json:"questions"`
	Proposed     string             `json:"proposed"`
	Additions    []string           `json:"additions"`
	Placeholders []string           `json:"placeholders"`
	Scrutiny     *ScrutinySynthesis `json:"scrutiny,omitempty"`
}
type RewriteResult = GenerationResult

var ErrInvalidGeneration = errors.New("invalid socratica generation")

// ErrGenerationTimeout marks a generation that ran out of time. With
// ErrInvalidGeneration it is the only transient kind the adapters emit.
var ErrGenerationTimeout = errors.New("tiempo de generación agotado")

// RetryableGenerationFailure reports whether a persisted generation error is
// one of the transient kinds the adapters emit with these stable texts. Any
// other failure, including a provider message phrased in a new way, fails
// closed: retrying it unchanged would only repeat the same failure (H-03).
func RetryableGenerationFailure(raw string) bool {
	return strings.Contains(raw, ErrGenerationTimeout.Error()) || strings.Contains(raw, ErrInvalidGeneration.Error())
}

func ParseGeneration(raw []byte) (GenerationResult, error) {
	var result GenerationResult
	// Decoder identifies the first balanced object, including braces inside strings.
	start := strings.IndexByte(string(raw), '{')
	if start < 0 {
		return result, ErrInvalidGeneration
	}
	if err := json.NewDecoder(strings.NewReader(string(raw[start:]))).Decode(&result); err != nil {
		return GenerationResult{}, ErrInvalidGeneration
	}
	questions := make([]Question, 0, 3)
	for _, q := range result.Questions {
		if len(questions) == 3 {
			break
		}
		if strings.TrimSpace(q.Text) == "" || utf8.RuneCountInString(q.Text) > 200 {
			return GenerationResult{}, ErrInvalidGeneration
		}
		if len(q.Options) < 2 {
			continue
		}
		if len(q.Options) > 4 {
			q.Options = q.Options[:4]
		}
		for _, o := range q.Options {
			if strings.TrimSpace(o) == "" || utf8.RuneCountInString(o) > 60 {
				return GenerationResult{}, ErrInvalidGeneration
			}
		}
		questions = append(questions, q)
	}
	result.Questions = questions
	if s := result.Scrutiny; s != nil {
		if len(s.Assumptions) > 3 {
			s.Assumptions = s.Assumptions[:3]
		}
		for _, a := range s.Assumptions {
			if a.Kind != "fact" && a.Kind != "hypothesis" {
				return GenerationResult{}, ErrInvalidGeneration
			}
		}
	}
	return result, nil
}
func WordCount(text string) int { return len(strings.Fields(text)) }
