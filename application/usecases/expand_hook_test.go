package usecases

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

type fakeRules struct {
	results map[string]RuleResult
	err     error
	calls   []string
}

func (f *fakeRules) Evaluate(_ context.Context, target string, _ domain.ExpandSignals) (RuleResult, error) {
	f.calls = append(f.calls, target)
	return f.results[target], f.err
}

type fakeShield struct {
	report  domain.ShieldReport
	scanned bool
}

func (f fakeShield) Scan(context.Context, domain.ExpandSignals) (domain.ShieldReport, bool, error) {
	return f.report, f.scanned, nil
}

type fakeInstincts struct {
	observed, folded int
	err              error
}

func (f *fakeInstincts) Observe(context.Context, domain.ExpandSignals) error {
	f.observed++
	return f.err
}
func (f *fakeInstincts) Fold(context.Context) error { f.folded++; return f.err }

func a(kind domain.ActionKind, target string) domain.SupertoolAction {
	return domain.SupertoolAction{Kind: kind, Target: target}
}

func TestExpandHook_NilDepsAreNoOps(t *testing.T) {
	actions := []domain.SupertoolAction{
		a(domain.ActionRulesEval, domain.RuleTargetBash), a(domain.ActionShieldScan, ""),
		a(domain.ActionInstinctsObserve, ""), a(domain.ActionInstinctsFold, ""),
	}
	out := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, actions, ExpandHookDeps{})
	if out.Decision != domain.DecisionAllow || len(out.Context) != 0 || len(out.Errors) != 0 || out.TimedOut {
		t.Fatalf("sin evaluadores debe permitir sin más: %+v", out)
	}
}

func TestExpandHook_SuggestionsPassThroughInOrder(t *testing.T) {
	actions := []domain.SupertoolAction{a(domain.ActionSuggest, domain.SkillVerifyLoop), a(domain.ActionSuggest, domain.SkillStrategicCompact)}
	out := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, actions, ExpandHookDeps{})
	if !reflect.DeepEqual(out.Suggestions, []string{domain.SkillVerifyLoop, domain.SkillStrategicCompact}) {
		t.Fatalf("sugerencias: %v", out.Suggestions)
	}
}

func TestExpandHook_BlockPrevailsOverWarn(t *testing.T) {
	rules := &fakeRules{results: map[string]RuleResult{
		domain.RuleTargetPrompt: {Decision: domain.DecisionWarn, Reason: "cuidado", Rule: "w"},
		domain.RuleTargetBash:   {Decision: domain.DecisionBlock, Reason: "no", Rule: "b"},
	}}
	actions := []domain.SupertoolAction{a(domain.ActionRulesEval, domain.RuleTargetPrompt), a(domain.ActionRulesEval, domain.RuleTargetBash)}
	out := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, actions, ExpandHookDeps{Rules: rules})
	if out.Decision != domain.DecisionBlock || out.Reason != "no" || out.Rule != "b" {
		t.Fatalf("block debe prevalecer: %+v", out)
	}
	if !reflect.DeepEqual(out.Fired, []string{"expand.warn:w", "expand.block:b"}) {
		t.Fatalf("activaciones: %v", out.Fired)
	}
}

func TestExpandHook_WarnAddsContext(t *testing.T) {
	rules := &fakeRules{results: map[string]RuleResult{domain.RuleTargetPrompt: {Decision: domain.DecisionWarn, Reason: "recuerda X", Rule: "r"}}}
	out := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, []domain.SupertoolAction{a(domain.ActionRulesEval, domain.RuleTargetPrompt)}, ExpandHookDeps{Rules: rules})
	if out.Decision != domain.DecisionWarn || !reflect.DeepEqual(out.Context, []string{"recuerda X"}) {
		t.Fatalf("%+v", out)
	}
}

func TestExpandHook_ErrorsAreRecordedNotFatal(t *testing.T) {
	rules := &fakeRules{err: errors.New("boom")}
	inst := &fakeInstincts{err: errors.New("db")}
	actions := []domain.SupertoolAction{a(domain.ActionRulesEval, domain.RuleTargetBash), a(domain.ActionInstinctsObserve, ""), a(domain.ActionSuggest, domain.SkillVerifyLoop)}
	out := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, actions, ExpandHookDeps{Rules: rules, Instincts: inst})
	if out.Decision != domain.DecisionAllow || len(out.Errors) != 2 || len(out.Suggestions) != 1 {
		t.Fatalf("los errores no deben cortar ni bloquear: %+v", out)
	}
}

func TestExpandHook_ShieldNoticeOnlyWhenFindings(t *testing.T) {
	act := []domain.SupertoolAction{a(domain.ActionShieldScan, "")}
	clean := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, act, ExpandHookDeps{Shield: fakeShield{scanned: true}})
	if len(clean.Context) != 0 {
		t.Fatalf("sin hallazgos no se emite nada: %+v", clean)
	}
	r := domain.ShieldReport{Findings: []domain.ShieldFinding{{Severity: "critical"}, {Severity: "medium"}}}
	out := ExpandHook{}.Run(context.Background(), domain.ExpandSignals{}, act, ExpandHookDeps{Shield: fakeShield{report: r, scanned: true}})
	if len(out.Context) != 1 || !strings.Contains(out.Context[0], "2 hallazgos") || !strings.Contains(out.Context[0], "1 critical") {
		t.Fatalf("aviso: %+v", out.Context)
	}
}

func TestExpandHook_StopsWhenContextExpires(t *testing.T) {
	// Un plazo ya vencido cancela al crearse: esperar a que venza uno de 1 ns
	// dependía de que el temporizador corriera a tiempo, y en Windows no siempre.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	rules := &fakeRules{results: map[string]RuleResult{domain.RuleTargetBash: {Decision: domain.DecisionBlock, Reason: "x", Rule: "r"}}}
	out := ExpandHook{}.Run(ctx, domain.ExpandSignals{}, []domain.SupertoolAction{a(domain.ActionRulesEval, domain.RuleTargetBash)}, ExpandHookDeps{Rules: rules})
	if !out.TimedOut || len(rules.calls) != 0 {
		t.Fatalf("con el plazo vencido no se evalúa nada: %+v %v", out, rules.calls)
	}
}
