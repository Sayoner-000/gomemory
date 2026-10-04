package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// T022 (feature 035, US1, FR-004…FR-006): ninguna salida de hook supera el
// tope del canal de Claude Code (10 000 caracteres). Medido en vivo antes del
// cambio: session-start 17 777 y el primer prompt en modo plan 12 944; el host
// los guardaba en un archivo y el modelo solo veía una vista previa de 2 KB.

const hookInlineMax = 10000
const hookTrimNotice = "… contexto recortado: llama get_context() para el resto"

// seedLargeMemory llena el proyecto con memorias distintas tomadas del
// historial real de este repositorio, hasta que el contexto completo supera el
// tope (precondición verificada: sin ella la prueba no demostraría nada).
func seedLargeMemory(t *testing.T, s *lifecycleSandbox, p string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootContract(t), "tests", "contract", "testdata", "hook_budget", "context-snapshot.md"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l = strings.TrimSpace(strings.TrimLeft(l, "-#>* ")); len(l) > 60 {
			lines = append(lines, l)
		}
	}
	tipos := []string{"decision", "bugfix", "pattern", "learning", "architecture"}
	for i := 0; i < 90; i++ {
		l := lines[i%len(lines)]
		title := fmt.Sprintf("Hallazgo %03d: %s", i, firstRunes(l, 50))
		content := fmt.Sprintf("%s. Caso %d: %s", l, i, strings.Repeat(fmt.Sprintf("detalle-%d ", i), 12))
		if r := s.Run("", p, "save", "-t", title, "-y", tipos[i%len(tipos)], content); r.ExitCode != 0 {
			t.Fatalf("save %d: %s %s", i, r.Stdout, r.Stderr)
		}
	}
	full := s.Run("", p, "context", "--full")
	if n := utf8.RuneCountInString(full.Stdout); n <= hookInlineMax {
		t.Fatalf("precondición: el contexto completo debe superar el tope (%d runas)", n)
	}
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// injectedText devuelve lo que llega al modelo: additionalContext si la salida
// es el sobre JSON del protocolo de hooks, o el texto tal cual si no.
func injectedText(t *testing.T, out string) string {
	t.Helper()
	var env struct {
		HSO struct {
			Ctx string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("salida JSON inválida: %v\n%s", err, firstRunes(out, 300))
		}
		return env.HSO.Ctx
	}
	return out
}

func TestHookBudget_SessionStartCabeEnElTope(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("budget-start")
	seedLargeMemory(t, s, p)
	s.Setenv("CLAUDE_PROJECT_DIR", p)

	r := s.RunInput("", p, `{"session_id":"budget-1","source":"startup"}`, "hook", "session-start")
	ctx := injectedText(t, r.Stdout)
	if n := utf8.RuneCountInString(ctx); n > hookInlineMax {
		t.Fatalf("session-start supera el tope: %d runas", n)
	}
	if !strings.HasSuffix(strings.TrimSpace(ctx), hookTrimNotice) {
		t.Errorf("un contexto recortado debe terminar con el aviso para pedir el resto: …%q", lastRunes(ctx, 120))
	}
	if doc := s.Run("", p, "doctor"); !strings.Contains(doc.Stdout, "budget_trimmed") {
		t.Errorf("el recorte debe quedar registrado y visible en mem doctor:\n%s", doc.Stdout)
	}
}

func TestHookBudget_PrimerPromptEnModoPlanCabeEnElTope(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("budget-plan")
	seedLargeMemory(t, s, p)
	mustWrite(t, filepath.Join(p, ".memory", "settings.json"), `{"octopus_enabled":true}`)

	r := s.RunInput("", p, `{"session_id":"budget-2","prompt":"planifica la feature","permission_mode":"plan"}`, "hook", "user-prompt-submit")
	ctx := injectedText(t, r.Stdout)
	if n := utf8.RuneCountInString(ctx); n > hookInlineMax {
		t.Fatalf("el primer prompt supera el tope: %d runas", n)
	}
	if !strings.HasPrefix(ctx, "PRIMERA ACCIÓN") || !strings.Contains(ctx, "select:mcp__gomemory__get_context") {
		t.Errorf("la carga de tools debe ir completa y primero: %q", firstRunes(ctx, 200))
	}
	for _, want := range []string{"JUEZ IMPARCIAL", "PRIVACIDAD", "OCTOPUS AAR — REGLA OBLIGATORIA DE DELEGACIÓN"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("falta %q: lo crítico nunca se recorta", want)
		}
	}
	if !strings.Contains(ctx, "Descomposición Atómica") {
		t.Error("el documento de plan debe estar presente (recortado si hace falta)")
	}
}

func lastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return string(r)
}
