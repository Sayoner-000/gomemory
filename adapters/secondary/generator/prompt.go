// Package generator implements ports.SocraticGenerator on top of the agents
// already installed and signed in on this machine (research R3): no new
// credentials, and always in a mode without tools or read-only, so generating
// questions can never carry out the task itself (FR-005).
package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// compactSchema is the embedded generation schema on a single line, so it can
// travel as one command-line argument.
func compactSchema() (string, error) {
	// Claude's validator does not register the 2020-12 meta-schema. Adapt only
	// the transport copy; domain and other harnesses keep the canonical schema.
	var schema map[string]json.RawMessage
	if err := json.Unmarshal([]byte(domain.SocraticaGenerationSchema), &schema); err != nil {
		return "", err
	}
	delete(schema, "$schema")
	b, err := json.Marshal(schema)
	return string(b), err
}

// Codex strict output requires every object property to be required. Optional
// canonical properties become nullable in its transport schema.
func codexSchema() (string, error) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(domain.SocraticaGenerationSchema), &schema); err != nil {
		return "", err
	}
	var strict func(map[string]any)
	strict = func(node map[string]any) {
		if props, ok := node["properties"].(map[string]any); ok {
			required := map[string]bool{}
			if list, ok := node["required"].([]any); ok {
				for _, key := range list {
					required[key.(string)] = true
				}
			}
			keys := make([]string, 0, len(props))
			for key, value := range props {
				child := value.(map[string]any)
				strict(child)
				if !required[key] {
					props[key] = map[string]any{"anyOf": []any{child, map[string]any{"type": "null"}}}
				}
				keys = append(keys, key)
			}
			sort.Strings(keys)
			node["required"] = keys
		}
		if items, ok := node["items"].(map[string]any); ok {
			strict(items)
		}
	}
	strict(schema)
	raw, err := json.Marshal(schema)
	return string(raw), err
}

// instructions builds the generation request. It mirrors the skill's steps 2
// and 3 and states the JSON contract; the model never executes the task.
func instructions(req ports.SocraticGenerationRequest, rewrite bool) string {
	f := req.Flow
	var b strings.Builder
	b.WriteString("Eres el generador de Socratica. NO ejecutes la tarea ni uses herramientas: solo devuelve un objeto JSON que cumpla el esquema.\n")
	if f.Mode == domain.ModeScrutiny {
		b.WriteString("Modo: escrutinio (/socratic-q). Examina supuestos, evidencia y alternativas sin defender por defecto la hipótesis inicial.\n")
	} else {
		b.WriteString("Modo: refinamiento (/socratica). Conserva la intención y la terminología del dominio.\n")
	}
	fmt.Fprintf(&b, "\nPrompt original (literal):\n<<<\n%s\n>>>\n", domain.ProtectText(f.OriginalPrompt))
	if strings.TrimSpace(req.Context) != "" {
		fmt.Fprintf(&b, "\nContexto relevante:\n%s\n", domain.ProtectText(req.Context))
	}
	if !rewrite {
		b.WriteString("\nDevuelve en `questions` entre 0 y 3 preguntas de alto valor informativo, solo si cambian el resultado " +
			"(audiencia, criterio de éxito, restricciones, formato, alcance). Cada una con 2 a 4 opciones cortas que no se solapen. " +
			"Si el prompt ya es claro, devuelve `questions` vacío.")
		if f.Mode == domain.ModeScrutiny {
			b.WriteString(" Incluye `scrutiny.restatement` y hasta tres `scrutiny.assumptions` con `kind` fact o hypothesis.")
		}
		b.WriteString("\n")
		return b.String()
	}
	if len(f.Questions) > 0 {
		b.WriteString("\nPreguntas y respuestas (una pregunta omitida no es una respuesta):\n")
		for i, q := range f.Questions {
			answer := "omitida"
			for _, a := range f.Answers {
				if a.QuestionIndex == i && !a.Skipped {
					answer = a.Choice
					if a.Custom != "" {
						answer = a.Custom
					}
				}
			}
			fmt.Fprintf(&b, "%d. %s → %s\n", i+1, domain.ProtectText(q.Text), domain.ProtectText(answer))
		}
	}
	b.WriteString("\nDevuelve en `proposed` una reescritura fiel en lenguaje llano que la persona podría haber escrito. " +
		"Integra solo lo respondido, no conviertas deseos opcionales en requisitos, usa [marcador] para lo esencial sin resolver " +
		"y lista en `additions` solo las adiciones materiales.")
	if f.Mode == domain.ModeScrutiny {
		b.WriteString(" Completa `scrutiny` con el mejor argumento a favor, la objeción más fuerte, alternativas, evidencia faltante y criterios de decisión; rotula lo no respaldado en `hypotheses`.")
	}
	b.WriteString("\n")
	return b.String()
}

// parseOutput accepts the agent's raw stdout. Claude's JSON envelope carries
// the object in structured_output (or as text in result); Codex prints the
// object itself.
func parseOutput(raw []byte) (domain.GenerationResult, error) {
	var envelope struct {
		Result           *string         `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
		IsError          bool            `json:"is_error"`
	}
	if json.Unmarshal(bytes.TrimSpace(raw), &envelope) == nil {
		if envelope.IsError {
			message := "el harness devolvió un error"
			if envelope.Result != nil {
				message = domain.ProtectText(*envelope.Result)
			}
			return domain.GenerationResult{}, fmt.Errorf("generación: %s", message)
		}
		if len(envelope.StructuredOutput) > 0 && string(envelope.StructuredOutput) != "null" {
			return domain.ParseGeneration(envelope.StructuredOutput)
		}
		if envelope.Result != nil {
			return domain.ParseGeneration([]byte(*envelope.Result))
		}
	}
	return domain.ParseGeneration(raw)
}
