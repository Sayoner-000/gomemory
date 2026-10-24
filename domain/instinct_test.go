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

// La clave sale del primer comando significativo: se saltan asignaciones,
// sudo, cd y los comandos de lectura triviales.
func TestObservationKeySkipsNoiseAndTrivialCommands(t *testing.T) {
	for cmd, want := range map[string]string{
		"S=/tmp/x/scratchpad":                 "",
		"S=/tmp/x/scratchpad && go build ./x": "cmd:go-build",
		"W=/tmp/win; GOOS=windows go test ./": "cmd:go-test",
		"cd /repo && go test ./...":           "cmd:go-test",
		"sudo docker ps":                      "cmd:docker-ps",
		"sudo -u app go test ./...":           "cmd:go-test",
		"sudo -E go test":                     "cmd:go-test",
		"sudo --user=app -g ops docker ps":    "cmd:docker-ps",
		"grep -rn foo . | head -5":            "",
		"cd x && sed -n '1,9p' a.go":          "",
		"cat a; ls b; wc -l c; pwd":           "",
		"rg foo && git diff":                  "cmd:git-diff",
		"python3 -c 'print(1)'":               "",
		"python3 scripts/coverage.py":         "cmd:python3",
		"sqlite3 -readonly x.db 'select 1'":   "",
		"echo hi; find . -name x; tail -f l":  "",
	} {
		if got := ObservationKey(cmd); got != want {
			t.Errorf("ObservationKey(%q) = %q, want %q", cmd, got, want)
		}
	}
	if got := ObservationLabel("cd /repo && go test ./... -run X"); got != "go test" {
		t.Errorf("ObservationLabel = %q", got)
	}
}

// Una fila guardada con el criterio de claves anterior ya no se inyecta.
func TestStaleInstinct(t *testing.T) {
	for _, tc := range []struct {
		i    Instinct
		want bool
	}{
		{Instinct{Slug: "cmd:go-test", Kind: "cmd", Description: "go test ./tests/contract/ -run X"}, false},
		{Instinct{Slug: "cmd:scratchpad", Kind: "cmd", Description: "S=/private/tmp/scratchpad && go build"}, true},
		{Instinct{Slug: "cmd:sed", Kind: "cmd", Description: "sed -n '1300,1380p' a.go"}, true},
		{Instinct{Slug: "cmd:python3", Kind: "cmd", Description: "python3 -c 'import json'"}, true},
		{Instinct{Slug: "correction:abc", Kind: "correction", Description: "nunca uses force"}, false},
	} {
		if got := StaleInstinct(tc.i); got != tc.want {
			t.Errorf("StaleInstinct(%s) = %v, want %v", tc.i.Slug, got, tc.want)
		}
	}
}

func TestInstinctContextLine(t *testing.T) {
	for _, tc := range []struct {
		i    Instinct
		want string
	}{
		{Instinct{Slug: "cmd:go-test", Kind: "cmd", Description: "go test ./... -run X", Observations: 40, Confidence: .8, LastSeen: "2026-10-24T06:00:00Z", Scope: "project"},
			"- `go test` · 40 usos · último 2026-10-24 (0.8)"},
		{Instinct{Slug: "cmd:go-test", Kind: "cmd", Description: "go test ./...", Projects: []string{"a", "b"}, Confidence: .7, LastSeen: "2026-10-20T00:00:00Z", Scope: "global"},
			"- `go test` · 2 proyectos · último 2026-10-20 (0.7)"},
		{Instinct{Slug: "correction:abc", Kind: "correction", Description: "nunca uses force", Confidence: .7},
			"- corrección: nunca uses force (0.7)"},
	} {
		if got := tc.i.ContextLine(); got != tc.want {
			t.Errorf("ContextLine = %q, want %q", got, tc.want)
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
