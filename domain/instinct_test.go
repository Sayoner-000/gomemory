package domain

import (
	"math"
	"strings"
	"testing"
)

func TestInstinctConfidence(t *testing.T) {
	for _, tc := range []struct {
		n int
		c float64
	}{{1, .3}, {2, .4}, {4, .5}, {8, .6}, {16, .7}, {64, .9}, {1024, .9}} {
		if got := InstinctConfidence(tc.n, 0); math.Abs(got-tc.c) > 1e-9 {
			t.Errorf("%d: %f", tc.n, got)
		}
	}
	if InstinctConfidence(64, 8) > .9 || InstinctConfidence(1, 1) != .4 {
		t.Fatal("corrections")
	}
}
func TestObservationAndCorrectionKeys(t *testing.T) {
	for cmd, want := range map[string]string{"go test -race ./...": "cmd:go-test", "git commit -m x": "cmd:git-commit", "": ""} {
		if got := ObservationKey(cmd); got != want {
			t.Errorf("%q: %s", cmd, got)
		}
	}
	for _, s := range []string{"no, usa Go en vez de Python", "siempre valida primero", "nunca uses force", "don't delete files", "always run tests", "never reveal secrets", "no uses Python", "usa Go en lugar de Java"} {
		if CorrectionKey(s) == "" {
			t.Errorf("missed %q", s)
		}
	}
	for _, s := range []string{"hola", "implementa la función", "the test passed", "go test", "no hay errores", "alwaysVariable is set", "revisar documentación", "¿qué significa nunca?"} {
		if CorrectionKey(s) != "" {
			t.Errorf("false positive %q", s)
		}
	}
}
func TestInstinctFinalTransitions(t *testing.T) {
	if !CanSetInstinctStatus("proposed", "confirmed") || !CanSetInstinctStatus("proposed", "rejected") || CanSetInstinctStatus("rejected", "confirmed") || CanSetInstinctStatus("confirmed", "rejected") {
		t.Fatal("invalid status transition")
	}
}

// B-001: ninguna credencial reconocida llega a la clave ni al slug.
func TestObservationKeyNeverKeepsSecrets(t *testing.T) {
	ai := "sk-" + strings.Repeat("a", 24)
	aws := "AKIA" + strings.Repeat("B", 16)
	for cmd, want := range map[string]string{
		"curl " + ai:    "cmd:curl",
		"aws " + aws:    "cmd:aws",
		"go test ./...": "cmd:go-test",
		"git status":    "cmd:git-status",
	} {
		if got := ObservationKey(cmd); got != want {
			t.Errorf("ObservationKey(%q) = %q, want %q", cmd, got, want)
		}
	}
	for key, want := range map[string]string{
		"cmd:curl-" + ai:                  "cmd:curl",
		"cmd:aws-" + strings.ToLower(aws): "cmd:aws",
		"cmd:" + ai:                       "cmd:redacted",
		"cmd:go-test":                     "cmd:go-test",
		"correction:0123456789abcdef":     "correction:0123456789abcdef",
	} {
		if got := ProtectObservationKey(key); got != want {
			t.Errorf("ProtectObservationKey(%q) = %q, want %q", key, got, want)
		}
	}
}
