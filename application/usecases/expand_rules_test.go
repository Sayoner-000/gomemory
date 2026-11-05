package usecases

import (
	"context"
	"testing"

	"mem/domain"
)

type staticRules []domain.Rule

func (s staticRules) Load(context.Context, string) ([]domain.Rule, []domain.RuleLoadError) {
	return s, nil
}

func TestExpandRules_BuiltinBlocksAndApplyPatchFileRules(t *testing.T) {
	own, err := domain.ParseRule("env.md", []byte("---\nevent: file\nmatch: regex\npattern: '\\.env$'\naction: warn\n---\nno toques .env"))
	if err != nil {
		t.Fatal(err)
	}
	ev := ExpandRules{Source: staticRules(append(domain.BuiltinRules(), own))}
	res, err := ev.Evaluate(context.Background(), domain.RuleTargetBash, domain.ExpandSignals{Command: "rm -rf /tmp/x"})
	if err != nil || res.Decision != domain.DecisionBlock || res.Rule != "destructive-shell" {
		t.Fatalf("bash: %+v %v", res, err)
	}
	patch := "*** Begin Patch\n*** Update File: config/.env\n@@\n+X=1\n*** End Patch"
	res, _ = ev.Evaluate(context.Background(), domain.RuleTargetFile, domain.ExpandSignals{Command: patch})
	if res.Decision != domain.DecisionWarn || res.Rule != "env" {
		t.Fatalf("apply_patch de Codex: %+v", res)
	}
	res, _ = ev.Evaluate(context.Background(), domain.RuleTargetFile, domain.ExpandSignals{FilePath: "main.go", Content: "package main"})
	if res.Decision != domain.DecisionAllow {
		t.Fatalf("archivo inocuo: %+v", res)
	}
}

func TestExpandRules_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (ExpandRules{Source: staticRules(domain.BuiltinRules())}).Evaluate(ctx, domain.RuleTargetBash, domain.ExpandSignals{Command: "rm -rf /"}); err == nil {
		t.Fatal("con ctx cancelado debe devolver error")
	}
}

// HCD-027: ExpandRules pasa al dominio las extensiones que cubre el grafo.
func TestExpandRules_GraphCoverage(t *testing.T) {
	sig := domain.ExpandSignals{Command: `grep -rn Foo --include=*.go .`}
	con := ExpandRules{Source: staticRules(domain.BuiltinRules()), IndexedExtensions: func() []string { return []string{".go"} }}
	res, err := con.Evaluate(context.Background(), domain.RuleTargetBash, sig)
	if err != nil || res.Decision != domain.DecisionBlock || res.Rule != domain.GraphBeforeGrepRule {
		t.Fatalf("con grafo debe bloquear graph-before-grep: %+v %v", res, err)
	}
	sin := ExpandRules{Source: staticRules(domain.BuiltinRules())}
	if res, _ := sin.Evaluate(context.Background(), domain.RuleTargetBash, sig); res.Decision != domain.DecisionAllow {
		t.Fatalf("sin grafo debe permitir: %+v", res)
	}
}
