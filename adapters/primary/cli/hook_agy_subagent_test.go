package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
)

// HCD-026: en agy el subagente (invoke_subagent) dispara SessionStart y
// PreInvocation con una conversación nueva y el prompt vacío, sin campo de
// padre (sondeo real, agy 1.5.0). Antes recibía session-start —que vacía el
// registro de entregas del raíz— y el bootstrap completo.

func agyEventJSON(t *testing.T, ev map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func agyProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, persistence.MemDir), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func callNames(plan agyPlan) []string {
	var out []string
	for _, c := range plan.calls {
		out = append(out, strings.Join(c.args, " "))
	}
	return out
}

// HCD-047: el PreToolUse(invoke_subagent) trae TypeName (sondeo real, agy
// 1.5.2). Un hijo de tipo splash arranca sin el aviso de subagente; los demás
// hijos lo conservan.
func TestAgySubagent_Splash(t *testing.T) {
	dir := agyProjectDir(t)
	now := time.Now()
	invoke := func(typeName string) []byte {
		return agyEventJSON(t, map[string]any{"conversationId": "root", "workspacePaths": []string{dir},
			"lastUserInput": "revisa", "toolCall": map[string]any{"name": "invoke_subagent", "args": map[string]any{"TypeName": typeName, "Role": "Revisor", "Prompt": "x"}}})
	}
	start := func(conv string) []byte {
		return agyEventJSON(t, map[string]any{"conversationId": conv, "workspacePaths": []string{dir}, "lastUserInput": ""})
	}

	classifyAgySubagent("PreToolUse", invoke("splash"), now)
	if !classifyAgySubagent("SessionStart", start("hijo-splash"), now.Add(time.Second)) {
		t.Fatal("el hijo splash sigue siendo un subagente")
	}
	if got := callNames(planAgyHookFor("SessionStart", start("hijo-splash"), true)); len(got) != 0 {
		t.Fatalf("el hijo splash no debe recibir nada de gomemory, se planificó %v", got)
	}

	classifyAgySubagent("PreToolUse", invoke("research"), now.Add(2*time.Second))
	if !classifyAgySubagent("SessionStart", start("hijo-normal"), now.Add(3*time.Second)) {
		t.Fatal("el hijo normal es un subagente")
	}
	if got := callNames(planAgyHookFor("SessionStart", start("hijo-normal"), true)); len(got) != 1 || !strings.HasPrefix(got[0], "subagent-start agy") {
		t.Fatalf("los demás hijos conservan el aviso de HCD-026, se planificó %v", got)
	}
}

func TestAgySubagent_SessionStartTrasInvokeSubagentEmiteSoloAviso(t *testing.T) {
	dir := agyProjectDir(t)
	now := time.Now()

	pre := agyEventJSON(t, map[string]any{"conversationId": "root", "workspacePaths": []string{dir},
		"lastUserInput": "delega", "toolCall": map[string]any{"name": "invoke_subagent"}})
	if classifyAgySubagent("PreToolUse", pre, now) {
		t.Fatal("el PreToolUse del raíz no es un subagente")
	}

	start := agyEventJSON(t, map[string]any{"conversationId": "child", "workspacePaths": []string{dir}, "lastUserInput": ""})
	if !classifyAgySubagent("SessionStart", start, now.Add(time.Second)) {
		t.Fatal("SessionStart con prompt vacío tras invoke_subagent debe clasificarse como subagente")
	}
	got := callNames(planAgyHookFor("SessionStart", start, true))
	if len(got) != 1 || !strings.HasPrefix(got[0], "subagent-start agy") {
		t.Fatalf("el subagente solo debe pedir el aviso mínimo; llamadas: %v", got)
	}

	inv := agyEventJSON(t, map[string]any{"conversationId": "child", "workspacePaths": []string{dir}, "invocationNum": 0, "lastUserInput": ""})
	if !classifyAgySubagent("PreInvocation", inv, now.Add(2*time.Second)) {
		t.Fatal("PreInvocation de la conversación hija debe seguir clasificada como subagente")
	}
	if got := callNames(planAgyHookFor("PreInvocation", inv, true)); len(got) != 0 {
		t.Fatalf("PreInvocation del subagente no debe inyectar el bootstrap; llamadas: %v", got)
	}
}

func TestAgySubagent_SessionStartSinMarcadorEsRaiz(t *testing.T) {
	dir := agyProjectDir(t)
	start := agyEventJSON(t, map[string]any{"conversationId": "root", "workspacePaths": []string{dir}, "lastUserInput": ""})
	if classifyAgySubagent("SessionStart", start, time.Now()) {
		t.Fatal("sin invoke_subagent previo, un SessionStart con prompt vacío es el agente raíz")
	}
	got := callNames(planAgyHookFor("SessionStart", start, false))
	if len(got) == 0 || !strings.HasPrefix(got[0], "session-start") {
		t.Fatalf("el raíz conserva session-start; llamadas: %v", got)
	}
}

func TestAgySubagent_MarcadorCaducadoEsRaiz(t *testing.T) {
	dir := agyProjectDir(t)
	now := time.Now()
	pre := agyEventJSON(t, map[string]any{"conversationId": "root", "workspacePaths": []string{dir},
		"toolCall": map[string]any{"name": "invoke_subagent"}})
	classifyAgySubagent("PreToolUse", pre, now)
	start := agyEventJSON(t, map[string]any{"conversationId": "late", "workspacePaths": []string{dir}, "lastUserInput": ""})
	if classifyAgySubagent("SessionStart", start, now.Add(agySubagentWindow+time.Second)) {
		t.Fatal("un marcador caducado no debe convertir en subagente a una sesión nueva")
	}
}

func TestAgySubagent_DosSubagentesConsumenUnMarcadorCadaUno(t *testing.T) {
	dir := agyProjectDir(t)
	now := time.Now()
	pre := agyEventJSON(t, map[string]any{"conversationId": "root", "workspacePaths": []string{dir},
		"toolCall": map[string]any{"name": "invoke_subagent"}})
	classifyAgySubagent("PreToolUse", pre, now)
	classifyAgySubagent("PreToolUse", pre, now)
	for i, id := range []string{"a", "b"} {
		start := agyEventJSON(t, map[string]any{"conversationId": id, "workspacePaths": []string{dir}, "lastUserInput": ""})
		if !classifyAgySubagent("SessionStart", start, now.Add(time.Second)) {
			t.Fatalf("subagente %d (%s) no clasificado", i, id)
		}
	}
	tercero := agyEventJSON(t, map[string]any{"conversationId": "c", "workspacePaths": []string{dir}, "lastUserInput": ""})
	if classifyAgySubagent("SessionStart", tercero, now.Add(time.Second)) {
		t.Fatal("cada invoke_subagent autoriza un solo subagente")
	}
}

func TestSubagentNoticeFor_AgyTextoPlanoSoloLectura(t *testing.T) {
	n := subagentNoticeFor("agy", true)
	if strings.Contains(n, "select:") || strings.Contains(n, "ToolSearch") {
		t.Errorf("agy no usa ToolSearch: %q", n)
	}
	for _, want := range []string{"search_memories", "get_memory", "search_code", "search_graph"} {
		if !strings.Contains(n, want) {
			t.Errorf("el aviso de agy debe nombrar %s: %q", want, n)
		}
	}
	for _, bad := range []string{"save_memory", "end_session", "start_session", "octopus"} {
		if strings.Contains(n, bad) {
			t.Errorf("el aviso de agy no debe nombrar %s: %q", bad, n)
		}
	}
}
