package usecases

import (
	"context"
	"fmt"

	"mem/domain"
)

// RuleResult es el veredicto de las reglas para un evento.
type RuleResult struct {
	Decision domain.ExpandDecision
	Reason   string
	Rule     string
}

// RulesEvaluator evalúa las reglas (de serie y propias) sobre la entrada
// de un evento; target es domain.RuleTarget*.
type RulesEvaluator interface {
	Evaluate(ctx context.Context, target string, sig domain.ExpandSignals) (RuleResult, error)
}

// ShieldRunner escanea la configuración de los agentes. scanned es false si
// no hizo falta escanear (la huella no cambió).
type ShieldRunner interface {
	Scan(ctx context.Context, sig domain.ExpandSignals) (report domain.ShieldReport, scanned bool, err error)
}

// InstinctObserver observa eventos y pliega la cola de instincts.
type InstinctObserver interface {
	Observe(ctx context.Context, sig domain.ExpandSignals) error
	Fold(ctx context.Context) error
}

// ExpandHookDeps son los evaluadores de cada supertool. Todos son
// opcionales: nil equivale a «esta supertool todavía no está enchufada» y
// su acción no hace nada.
type ExpandHookDeps struct {
	Rules     RulesEvaluator
	Shield    ShieldRunner
	Instincts InstinctObserver
}

// ExpandHook ejecuta las acciones que el router decidió para un evento
// (feature 040). No decide nada por sí mismo ni hace E/S directa: delega en
// los evaluadores inyectados, que reciben ctx y deben respetar su plazo.
type ExpandHook struct{}

// Run ejecuta actions en orden y agrega el resultado. Un block prevalece
// sobre un warn. Una acción que falla se anota en Errors sin interrumpir
// las demás; si el plazo vence, se marca TimedOut y se detiene.
func (ExpandHook) Run(ctx context.Context, sig domain.ExpandSignals, actions []domain.SupertoolAction, deps ExpandHookDeps) domain.ExpandOutcome {
	out := domain.ExpandOutcome{Decision: domain.DecisionAllow}
	for _, a := range actions {
		if ctx.Err() != nil {
			out.TimedOut = true
			return out
		}
		switch a.Kind {
		case domain.ActionRulesEval:
			if deps.Rules == nil {
				continue
			}
			res, err := deps.Rules.Evaluate(ctx, a.Target, sig)
			if err != nil {
				out.Errors = append(out.Errors, string(a.Kind))
				continue
			}
			applyRuleResult(&out, res)
		case domain.ActionShieldScan:
			if deps.Shield == nil {
				continue
			}
			report, scanned, err := deps.Shield.Scan(ctx, sig)
			if err != nil {
				out.Errors = append(out.Errors, string(a.Kind))
				continue
			}
			if scanned && len(report.Findings) > 0 {
				out.Context = append(out.Context, ShieldNotice(report))
				out.Fired = append(out.Fired, "expand.shield")
			}
		case domain.ActionSuggest:
			out.Suggestions = append(out.Suggestions, a.Target)
		case domain.ActionInstinctsObserve:
			if deps.Instincts != nil && deps.Instincts.Observe(ctx, sig) != nil {
				out.Errors = append(out.Errors, string(a.Kind))
			}
		case domain.ActionInstinctsFold:
			if deps.Instincts != nil && deps.Instincts.Fold(ctx) != nil {
				out.Errors = append(out.Errors, string(a.Kind))
			}
		}
	}
	if ctx.Err() != nil {
		out.TimedOut = true
	}
	return out
}

func applyRuleResult(out *domain.ExpandOutcome, res RuleResult) {
	switch res.Decision {
	case domain.DecisionBlock:
		if out.Decision != domain.DecisionBlock {
			out.Decision, out.Reason, out.Rule = domain.DecisionBlock, res.Reason, res.Rule
			out.Fired = append(out.Fired, "expand.block:"+res.Rule)
		}
	case domain.DecisionWarn:
		if out.Decision == domain.DecisionAllow {
			out.Decision = domain.DecisionWarn
		}
		out.Context = append(out.Context, res.Reason)
		out.Fired = append(out.Fired, "expand.warn:"+res.Rule)
	}
}

// ShieldNotice resume un informe con hallazgos sin repetir su contenido.
func ShieldNotice(r domain.ShieldReport) string {
	count := map[string]int{}
	for _, f := range r.Findings {
		count[f.Severity]++
	}
	return fmt.Sprintf("shield: %d hallazgos en la configuración de los agentes (%d critical, %d high, %d medium). Revísalos con `mem doctor --shield`.",
		len(r.Findings), count["critical"], count["high"], count["medium"])
}
