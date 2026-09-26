package main

import (
	"strings"
	"testing"
)

// Los hooks son comandos públicos invocados por agentes en procesos separados.
// Se ejercitan con el binario instalado y un proyecto aislado para incluir sus
// rutas reales de inicialización, sesión y recuperación en la cobertura.
func TestCLIHookWorkflows_CicloDeSesionYEventos(t *testing.T) {
	s := newLifecycleSandbox(t)
	project := s.Project("hooks")
	if res := s.Run("", project, "init"); res.ExitCode != 0 {
		t.Fatalf("init: %+v", res)
	}
	events := []struct {
		args  []string
		input string
	}{
		{[]string{"session-start"}, `{}`},
		{[]string{"user-prompt-submit"}, `{"prompt":"Necesito una recomendación de arquitectura"}`},
		{[]string{"plan-entered"}, `{}`},
		{[]string{"plan-entered"}, `{}`},
		{[]string{"plan-approved"}, `{"plan":"Implementar la mejora acordada"}`},
		{[]string{"turn-end"}, `{}`},
		{[]string{"subagent-start"}, `{}`},
		{[]string{"subagent-stop"}, `{"agent_id":"prueba"}`},
		{[]string{"pre-compact"}, `{}`},
		{[]string{"post-compact"}, `{}`},
		{[]string{"compaction-context"}, `{}`},
		{[]string{"compact-summary"}, `{"summary":"Trabajo completado en sandbox"}`},
		{[]string{"nudge"}, `{}`},
		{[]string{"prompt"}, `{}`},
		{[]string{"agent-notice"}, `{}`},
		{[]string{"session-end"}, `{"summary":"Sesión de prueba completada"}`},
	}
	for _, event := range events {
		t.Run(strings.Join(event.args, "_"), func(t *testing.T) {
			res := s.RunInput("", project, event.input, append([]string{"hook"}, event.args...)...)
			if res.ExitCode != 0 {
				t.Fatalf("hook %v: %+v", event.args, res)
			}
		})
	}
}
