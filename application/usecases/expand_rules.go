package usecases

import (
	"context"

	"mem/application/ports"
	"mem/domain"
)

// ExpandRules implementa RulesEvaluator sobre las reglas que entrega el
// puerto (feature 040, US1): construye la entrada de cada evento a partir
// de las señales y aplica domain.EvaluateRules.
type ExpandRules struct {
	Source ports.RuleSourcePort
	Root   string
	// IndexedExtensions devuelve las extensiones que cubre un grafo de código
	// del proyecto (HCD-027). nil o vacío: sin grafo, graph-before-grep no
	// actúa. Se consulta solo en el evento bash.
	IndexedExtensions func() []string
}

func (e ExpandRules) Evaluate(ctx context.Context, target string, sig domain.ExpandSignals) (RuleResult, error) {
	rules, _ := e.Source.Load(ctx, e.Root)
	if err := ctx.Err(); err != nil {
		return RuleResult{}, err
	}
	in := domain.RuleInput{Command: sig.Command, Content: sig.Content, Prompt: sig.Prompt, LastMessage: sig.LastMessage}
	if target == domain.RuleTargetBash && e.IndexedExtensions != nil {
		in.IndexedExtensions = e.IndexedExtensions()
	}
	if target == domain.RuleTargetFile {
		if sig.FilePath != "" {
			in.FilePaths = append(in.FilePaths, sig.FilePath)
		}
		// Codex (apply_patch) manda el parche como command: las rutas salen
		// de sus cabeceras y el contenido de las líneas añadidas.
		if paths, body := domain.ParseApplyPatch(sig.Command); len(paths) > 0 {
			in.FilePaths = append(in.FilePaths, paths...)
			if in.Content == "" {
				in.Content = body
			}
		}
	}
	v := domain.EvaluateRules(domain.RuleEvent(target), in, rules)
	res := RuleResult{Decision: domain.DecisionAllow, Reason: v.Reason, Rule: v.Rule}
	switch v.Action {
	case domain.RuleBlock:
		res.Decision = domain.DecisionBlock
	case domain.RuleWarn:
		res.Decision = domain.DecisionWarn
	}
	return res, nil
}
