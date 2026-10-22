package domain

import (
	"strings"
	"testing"
	"time"
)

// El guardarraíl no debe entrar en pánico ni tardar de forma desmedida con
// entradas arbitrarias, incluidas sustituciones y heredocs sin cerrar.
func FuzzDestructiveShell(f *testing.F) {
	for _, seed := range []string{"rm -rf x", "echo $(rm -rf x", "bash <<EOF\nrm", "su -c", "find -exec", "env -S", "eval", "`", "$'\\x", "timeout", "git push +", "a | sh", "((", "runuser -u"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		EvaluateRules(RuleEventBash, RuleInput{Command: cmd}, BuiltinRules())
	})
}

// Sustituciones anidadas sin fin: se bloquea en tiempo acotado.
func TestDeepSubstitutionBoundedAndBlocked(t *testing.T) {
	cmd := strings.Repeat("echo $(", 20000) + "ls" + strings.Repeat(")", 20000)
	start := time.Now()
	v := EvaluateRules(RuleEventBash, RuleInput{Command: cmd}, BuiltinRules())
	if v.Action != RuleBlock {
		t.Fatalf("anidamiento excesivo debe bloquearse: %+v", v)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("demasiado lento: %v", d)
	}
	if v := EvaluateRules(RuleEventBash, RuleInput{Command: strings.Repeat("echo $(", 5) + "ls" + strings.Repeat(")", 5)}, BuiltinRules()); v.Action != RuleAllow {
		t.Fatalf("anidamiento razonable permite: %+v", v)
	}
}
