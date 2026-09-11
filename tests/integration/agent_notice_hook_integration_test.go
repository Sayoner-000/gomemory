package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// setUpAgentNoticeProject prepara un proyecto con la huella por encima del
// umbral (48000, el default) y opcionalmente compact_agent_notice activado.
func setUpAgentNoticeProject(t *testing.T, agentNoticeOn bool) string {
	t.Helper()
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	memDir := filepath.Join(target, ".memory")
	if err := os.WriteFile(filepath.Join(memDir, ".footprint"), []byte("60000"), 0644); err != nil {
		t.Fatalf("escribir footprint: %v", err)
	}
	if agentNoticeOn {
		settings := map[string]any{"compact_agent_notice": true}
		data, _ := json.Marshal(settings)
		if err := os.WriteFile(filepath.Join(memDir, "settings.json"), data, 0644); err != nil {
			t.Fatalf("escribir settings: %v", err)
		}
	}
	return target
}

// turnEndBaselineSpec008 es la salida de `mem hook turn-end` con huella 60000,
// grabada con el binario de e49437b (anterior a la feature 030). Es literal a
// propósito: compararla contra la constante del código sería tautológico y no
// detectaría un cambio de texto o de formato.
const turnEndBaselineSpec008 = `RECORDATORIO DE MEMORIA: la memoria persistente ya aportó bastante ` +
	`contexto a esta sesión. Para abaratar los próximos turnos, considera compactar el ` +
	`contexto de la conversación. Si prefieres seguir así, ignora este recordatorio.`

// TestHookTurnEnd_OpcionApagadaSalidaIdentica cubre FR-015: con la opción
// apagada (default), la salida de turn-end debe ser idéntica en los tres
// dialectos a la que produce la rama vigente (spec 008) con la misma huella.
func TestHookTurnEnd_OpcionApagadaSalidaIdentica(t *testing.T) {
	bin := buildMemBinary(t)

	jsonBaseline := `{"systemMessage":"` + turnEndBaselineSpec008 + `"}`
	baselines := map[string]string{
		"claude": jsonBaseline,
		"json":   jsonBaseline,
		"text":   turnEndBaselineSpec008,
	}

	for _, dialecto := range []string{"claude", "json", "text"} {
		target := setUpAgentNoticeProject(t, false)
		args := []string{"hook", "turn-end"}
		if dialecto != "claude" {
			args = append(args, "--emit="+dialecto)
		}
		out := runHookWithStdinArgs(t, bin, target, args, `{}`)

		if out != baselines[dialecto] {
			t.Errorf("[%s] la salida debía ser idéntica a la línea base de spec 008:\n got: %q\nwant: %q", dialecto, out, baselines[dialecto])
		}
		if _, err := os.Stat(filepath.Join(target, ".memory", ".pending-agent-notice")); err == nil {
			t.Errorf("[%s] con la opción apagada no debía crearse el archivo pendiente", dialecto)
		}
	}
}

// TestHookTurnEnd_ClaudeEmiteSobreCombinado cubre que, con la opción
// activada, claude reciba AMBOS mensajes en un único objeto: systemMessage
// (persona) y hookSpecificOutput.additionalContext (agente), sin decision ni
// continue.
func TestHookTurnEnd_ClaudeEmiteSobreCombinado(t *testing.T) {
	bin := buildMemBinary(t)
	target := setUpAgentNoticeProject(t, true)

	out := runHookWithStdin(t, bin, target, "turn-end", `{}`)

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida no es JSON válido: %v\n%s", err, out)
	}
	if _, present := payload["decision"]; present {
		t.Error("no debe incluir 'decision': nunca debe bloquear ni prolongar el turno")
	}
	if _, present := payload["continue"]; present {
		t.Error("no debe incluir 'continue'")
	}
	sysMsg, _ := payload["systemMessage"].(string)
	if !strings.Contains(strings.ToLower(sysMsg), "compact") {
		t.Errorf("systemMessage debía traer el aviso a la persona: %q", sysMsg)
	}
	hso, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("esperaba hookSpecificOutput con el aviso al agente: %s", out)
	}
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "AVISO DE MEMORIA") {
		t.Errorf("additionalContext debía traer el aviso al agente: %q", ctx)
	}
}

// TestHookTurnEnd_JSONYTextCreanElArchivoPendiente cubre que, en dialectos sin
// canal doble, solo se emite el aviso a la persona y el aviso al agente queda
// pendiente en disco para el turno siguiente.
func TestHookTurnEnd_JSONYTextCreanElArchivoPendiente(t *testing.T) {
	bin := buildMemBinary(t)

	for _, dialecto := range []string{"json", "text"} {
		target := setUpAgentNoticeProject(t, true)
		out := runHookWithStdinArgs(t, bin, target, []string{"hook", "turn-end", "--emit=" + dialecto}, `{}`)

		if strings.Contains(out, "AVISO DE MEMORIA") {
			t.Errorf("[%s] el aviso al agente NO debía viajar en la salida de este turno: %q", dialecto, out)
		}
		if !strings.Contains(strings.ToLower(out), "compact") {
			t.Errorf("[%s] debía seguir el aviso a la persona: %q", dialecto, out)
		}
		if _, err := os.Stat(filepath.Join(target, ".memory", ".pending-agent-notice")); err != nil {
			t.Errorf("[%s] debía crear el archivo .pending-agent-notice: %v", dialecto, err)
		}
	}
}

// TestHookUserPromptSubmit_ConsumeElAvisoPendienteUnaSolaVez cubre que el
// siguiente user-prompt-submit anteponga el aviso y lo consuma (no se repite
// en el turno de después).
func TestHookUserPromptSubmit_ConsumeElAvisoPendienteUnaSolaVez(t *testing.T) {
	bin := buildMemBinary(t)
	target := setUpAgentNoticeProject(t, true)

	// Flujo real: primer prompt (crea el marker de bootstrap), fin de ese
	// turno (crea el aviso pendiente), segundo prompt (debe consumirlo). El
	// aviso pendiente solo puede llegar tras un turn-end, que en la práctica
	// nunca ocurre antes del primer user-prompt-submit de la sesión.
	runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit"}, `{}`)
	runHookWithStdinArgs(t, bin, target, []string{"hook", "turn-end", "--emit=json"}, `{}`)
	pending := filepath.Join(target, ".memory", ".pending-agent-notice")
	if _, err := os.Stat(pending); err != nil {
		t.Fatalf("precondición: debía existir el archivo pendiente: %v", err)
	}

	first := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit"}, `{}`)
	if !strings.Contains(first, "AVISO DE MEMORIA") {
		t.Fatalf("el primer user-prompt-submit tras el aviso debía incluirlo: %q", first)
	}
	if _, err := os.Stat(pending); err == nil {
		t.Error("el archivo pendiente debía borrarse al consumirse")
	}

	second := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit"}, `{}`)
	if strings.Contains(second, "AVISO DE MEMORIA") {
		t.Errorf("un segundo turno NO debía repetir el aviso ya consumido: %q", second)
	}
}

// TestHookAgentNotice_EmiteYConsume cubre el subcomando dedicado para
// integraciones que no consumen el JSON de user-prompt-submit (OpenCode).
func TestHookAgentNotice_EmiteYConsume(t *testing.T) {
	bin := buildMemBinary(t)
	target := setUpAgentNoticeProject(t, true)

	runHookWithStdinArgs(t, bin, target, []string{"hook", "turn-end", "--emit=json"}, `{}`)

	first := runHook(t, bin, target, "agent-notice")
	if !strings.Contains(first, "AVISO DE MEMORIA") {
		t.Fatalf("agent-notice debía emitir el aviso pendiente: %q", first)
	}
	second := runHook(t, bin, target, "agent-notice")
	if strings.TrimSpace(second) != "" {
		t.Errorf("una segunda llamada no debía emitir nada (ya consumido): %q", second)
	}
}

// TestHookPostCompact_BorraElAvisoPendiente cubre que compactar limpia
// cualquier aviso pendiente que no se haya consumido.
func TestHookPostCompact_BorraElAvisoPendiente(t *testing.T) {
	bin := buildMemBinary(t)
	target := setUpAgentNoticeProject(t, true)

	runHookWithStdinArgs(t, bin, target, []string{"hook", "turn-end", "--emit=json"}, `{}`)
	pending := filepath.Join(target, ".memory", ".pending-agent-notice")
	if _, err := os.Stat(pending); err != nil {
		t.Fatalf("precondición: debía existir el archivo pendiente: %v", err)
	}

	runHook(t, bin, target, "post-compact")
	if _, err := os.Stat(pending); err == nil {
		t.Error("post-compact debía borrar el aviso pendiente")
	}
}
