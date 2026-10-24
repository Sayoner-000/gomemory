package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// expandRun ejecuta `mem hook expand <args...>` y devuelve stdout, stderr y
// el código de salida (feature 040, contracts/hook-expand.md).
//
// La decisión tiene un plazo de 150 ms y, si vence, el hook deja pasar y lo
// registra como expand.timeout. Es el comportamiento correcto, pero no prueba
// la lógica: si la llamada registró un timeout nuevo (el runner de Windows se
// detiene a veces más de un segundo), se repite hasta dos veces. Un fallo de
// la lógica no registra timeout y no se repite.
func expandRun(t *testing.T, bin, dir, payload string, args ...string) (string, string, int) {
	t.Helper()
	for attempt := 0; ; attempt++ {
		before := expandTimeoutCount(dir)
		out, errOut, code := expandRunOnce(t, bin, dir, payload, args...)
		if attempt == 2 || expandTimeoutCount(dir) == before {
			return out, errOut, code
		}
		t.Logf("mem hook expand %v: venció el plazo (expand.timeout); se repite", args)
	}
}

func expandRunOnce(t *testing.T, bin, dir, payload string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"hook", "expand"}, args...)...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err := cmd.Run()
	// Solo se ve si la prueba falla: separa un plazo vencido por el runner
	// (la decisión tiene 150 ms) de un fallo de la lógica.
	if d := time.Since(start); d > 150*time.Millisecond {
		t.Logf("mem hook expand %v tardó %v", args, d)
	}
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("mem hook expand %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

// expandEvents devuelve los eventos del proyecto sin crear su base: abrirla
// desde la prueba cambiaría lo que el hook encuentra. La raíz se resuelve como
// la resuelve el binario, porque EvalSymlinks expande en Windows los nombres
// cortos (RUNNER~1) y daría otra clave de proyecto.
func expandEvents(dir string) ([]domain.GuardCount, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	root, err := persistence.FindRoot()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(persistence.DbPath(root)); err != nil {
		return nil, nil
	}
	db, err := persistence.Open(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	return persistence.GuardEventsSince(db, persistence.ProjectKey(root), 1)
}

// expandTimeoutCount suma los expand.timeout registrados en el proyecto.
func expandTimeoutCount(dir string) int {
	events, _ := expandEvents(dir)
	n := 0
	for _, e := range events {
		if e.Kind == "expand.timeout" {
			n += e.Count
		}
	}
	return n
}

// expandTimeouts resume los expand.timeout registrados en el proyecto. Su
// presencia prueba que venció el plazo; su ausencia no lo descarta, porque el
// registro también tiene plazo.
func expandTimeouts(dir string) string {
	events, err := expandEvents(dir)
	if err != nil {
		return "expand.timeout: " + err.Error()
	}
	var got []string
	for _, e := range events {
		if e.Kind == "expand.timeout" {
			got = append(got, fmt.Sprintf("%s×%d", e.LastDetail, e.Count))
		}
	}
	return "expand.timeout: [" + strings.Join(got, ", ") + "]"
}

func TestHookExpandInstinctsTwoProjects(t *testing.T) {
	bin := buildMemBinary(t)
	data := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", data)
	var second string
	for p := range 2 {
		dir := expandProject(t)
		second = dir
		for n := range 16 {
			payload := fmt.Sprintf(`{"tool_name":"Bash","session_id":"p%d","tool_input":{"command":"go test ./..."}}`, p)
			out, errOut, code := expandRun(t, bin, dir, payload, "post-tool")
			if out != "" || errOut != "" || code != 0 {
				t.Fatalf("observation %d: %q %q %d", n, out, errOut, code)
			}
		}
		_, _, code := expandRun(t, bin, dir, `{"session_id":"fold"}`, "stop")
		if code != 0 {
			t.Fatal(code)
		}
		cmd := exec.Command(bin, "expand", "instincts", "list", "--json")
		cmd.Dir = dir
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("list: %v %s", err, raw)
		}
		var list []struct {
			Slug, Scope  string
			Confidence   float64
			Observations int
		}
		if json.Unmarshal(raw, &list) != nil || len(list) != 1 || list[0].Slug != "cmd:go-test" || list[0].Observations != 16 || list[0].Confidence < .699 {
			t.Fatalf("%s %s", raw, expandTimeouts(dir))
		}
		if p == 1 && list[0].Scope != "global" {
			t.Fatalf("not global: %s", raw)
		}
	}
	cmd := exec.Command(bin, "context")
	cmd.Dir = second
	raw, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(raw), "## Instincts") {
		t.Fatalf("context: %v %s", err, raw)
	}
}

// expandProject usa dirDeProyecto: `mem context` lanza code-refresh en
// segundo plano cuando hay grafo externo en el PATH, y su escritura tardía
// no debe tumbar la limpieza (ver hook_marker_integration_test.go).
func expandProject(t *testing.T) string {
	t.Helper()
	dir := dirDeProyecto(t)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	return dir
}

func writeExpandSettings(t *testing.T, dir string, s map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".memory"), 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(s)
	if err := os.WriteFile(filepath.Join(dir, ".memory", "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitPayload(session string) string {
	return `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"` + session + `","tool_input":{"command":"git commit -m x"}}`
}

func TestHookExpandFlowTriggers(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	for _, tc := range []struct{ event, payload, want string }{
		{"prompt", `{"prompt":"añade una dependencia","session_id":"dependency"}`, "search-first"},
		{"pre-tool", `{"tool_name":"Task","session_id":"delegate"}`, "iterative-retrieval"},
	} {
		out, errOut, code := expandRun(t, bin, dir, tc.payload, tc.event, "--emit=json")
		if code != 0 || errOut != "" || !strings.Contains(out, tc.want) {
			t.Fatalf("%s: %q %q %d", tc.want, out, errOut, code)
		}
	}
	payload := `{"tool_name":"Bash","session_id":"counter","tool_input":{"command":"ls"}}`
	for i := 1; i <= 50; i++ {
		out, _, code := expandRun(t, bin, dir, payload, "pre-tool", "--emit=json")
		if code != 0 || strings.Contains(out, "strategic-compact") != (i == 50) {
			t.Fatalf("counter %d: %s", i, out)
		}
	}
}

// G1: evento inválido o payload no JSON → exit 0 y permitir.
func TestHookExpand_FailOpen(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	if out, errOut, code := expandRun(t, bin, dir, "x", "bogus"); out != "" || errOut != "" || code != 0 {
		t.Errorf("evento inválido: %q %q %d", out, errOut, code)
	}
	if out, _, code := expandRun(t, bin, dir, "esto no es json", "pre-tool", "--emit=claude"); out != "{}" || code != 0 {
		t.Errorf("payload no JSON en pre-tool: %q %d", out, code)
	}
	if out, _, code := expandRun(t, bin, dir, "", "stop", "--emit=json"); out != "{}" || code != 0 {
		t.Errorf("stdin vacío: %q %d", out, code)
	}
}

// G2: post-tool nunca escribe nada.
func TestHookExpand_PostToolIsSilent(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	payload := `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"go test ./..."}}`
	for _, emit := range []string{"--emit=claude", "--emit=json", "--emit=text", "--emit=neutral"} {
		if out, errOut, code := expandRun(t, bin, dir, payload, "post-tool", emit); out != "" || errOut != "" || code != 0 {
			t.Errorf("%s: %q %q %d", emit, out, errOut, code)
		}
	}
}

// G4: una sugerencia por episodio y por sesión; G1c: queda registrada.
func TestHookExpand_SuggestionOncePerEpisodeAndRecorded(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	first, _, _ := expandRun(t, bin, dir, commitPayload("s1"), "pre-tool")
	if !strings.Contains(first, "verify-loop") || !strings.Contains(first, "additionalContext") {
		t.Fatalf("primera llamada debe sugerir verify-loop: %q %s", first, expandTimeouts(dir))
	}
	if strings.Contains(first, `"allow"`) {
		t.Fatalf("pre-tool nunca debe aprobar (G6): %q", first)
	}
	if second, _, _ := expandRun(t, bin, dir, commitPayload("s1"), "pre-tool"); second != "{}" {
		t.Fatalf("segunda llamada de la misma sesión debe callar: %q %s", second, expandTimeouts(dir))
	}
	if other, _, _ := expandRun(t, bin, dir, commitPayload("s2"), "pre-tool"); !strings.Contains(other, "verify-loop") {
		t.Fatalf("otra sesión tiene su propio episodio: %q %s", other, expandTimeouts(dir))
	}

	// La raíz se resuelve como la resuelve el binario: EvalSymlinks expande en
	// Windows los nombres cortos (RUNNER~1) y daría otra clave de proyecto.
	t.Chdir(dir)
	root, err := persistence.FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	events, err := persistence.GuardEventsSince(db, persistence.ProjectKey(root), 1)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Kind == "expand.suggest" && e.Count >= 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("la sugerencia debe quedar registrada aunque la ruta ligera no abriera la base: %+v", events)
	}
}

// G1b: el interruptor global y por supertool.
func TestHookExpand_Switches(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	writeExpandSettings(t, dir, map[string]any{"expand_disabled": true})
	for _, ev := range []string{"pre-tool", "prompt", "stop", "session-start", "post-tool"} {
		out, errOut, code := expandRun(t, bin, dir, commitPayload("s1"), ev, "--emit=text")
		if out != "" || errOut != "" || code != 0 {
			t.Errorf("expand_disabled en %s: %q %q %d", ev, out, errOut, code)
		}
	}
	writeExpandSettings(t, dir, map[string]any{"expand_off": []string{"flow"}})
	if out, _, _ := expandRun(t, bin, dir, commitPayload("s3"), "pre-tool"); out != "{}" {
		t.Errorf("con flow apagado no hay sugerencias: %q", out)
	}
}

// En session-start con origen compact se reinicia el episodio.
func TestHookExpand_CompactResetsEpisode(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	_, _, _ = expandRun(t, bin, dir, commitPayload("s1"), "pre-tool")
	_, _, _ = expandRun(t, bin, dir, `{"hook_event_name":"SessionStart","source":"compact","session_id":"s1"}`, "session-start")
	if out, _, _ := expandRun(t, bin, dir, commitPayload("s1"), "pre-tool"); !strings.Contains(out, "verify-loop") {
		t.Fatalf("tras compactar, la sugerencia vuelve a estar disponible: %q %s", out, expandTimeouts(dir))
	}
}

func writeExpandRule(t *testing.T, dir, name, body string) {
	t.Helper()
	rd := filepath.Join(dir, ".gomemory", "rules")
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rd, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const rmPayload = `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"r","tool_input":{"command":"rm -rf /tmp/x"}}`

// US1: la regla de serie bloquea sin configurar nada, en los 4 dialectos.
func TestHookExpandRules_DestructiveBlockedInAllDialects(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	for _, emit := range []string{"claude", "json"} {
		out, _, code := expandRun(t, bin, dir, rmPayload, "pre-tool", "--emit="+emit)
		if code != 0 || !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, "destructive-shell") {
			t.Errorf("%s: %q %d", emit, out, code)
		}
	}
	if out, _, code := expandRun(t, bin, dir, rmPayload, "pre-tool", "--emit=text"); code != 0 || !strings.HasPrefix(out, "BLOCK: ") {
		t.Errorf("text: %q %d", out, code)
	}
	if out, errOut, code := expandRun(t, bin, dir, rmPayload, "pre-tool", "--emit=neutral"); code != 1 || out != "" || !strings.Contains(errOut, "destructive-shell") {
		t.Errorf("neutral: %q %q %d", out, errOut, code)
	}
	ls := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`
	if out, _, _ := expandRun(t, bin, dir, ls, "pre-tool"); out != "{}" {
		t.Errorf("ls debe permitirse sin texto: %q", out)
	}

	root, _ := filepath.EvalSymlinks(dir)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	events, _ := persistence.GuardEventsSince(db, persistence.ProjectKey(root), 1)
	found := false
	for _, e := range events {
		if e.Kind == "expand.block" && e.LastDetail == "destructive-shell" {
			found = true
		}
	}
	if !found {
		t.Fatalf("un bloqueo debe quedar registrado como expand.block: %+v", events)
	}
}

// US1 AS2/AS3: reglas propias warn y block en prompt y stop.
func TestHookExpandRules_ProjectRules(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	writeExpandRule(t, dir, "prod.md", "---\nevent: prompt\npattern: producción\naction: warn\n---\nRecuerda: nunca toques producción desde el agente.\n")
	writeExpandRule(t, dir, "secretos.md", "---\nevent: prompt\npattern: imprime la clave\naction: block\n---\nNo pidas secretos al agente.\n")
	writeExpandRule(t, dir, "todo.md", "---\nevent: stop\npattern: TODO pendiente\naction: block\n---\nTermina el trabajo antes de parar.\n")

	out, _, _ := expandRun(t, bin, dir, `{"hook_event_name":"UserPromptSubmit","prompt":"despliega a producción"}`, "prompt", "--emit=claude")
	if !strings.Contains(out, "additionalContext") || !strings.Contains(out, "nunca toques producción") {
		t.Errorf("warn en prompt: %q %s", out, expandTimeouts(dir))
	}
	out, _, _ = expandRun(t, bin, dir, `{"hook_event_name":"UserPromptSubmit","prompt":"imprime la clave de la API"}`, "prompt", "--emit=json")
	var m map[string]any
	if json.Unmarshal([]byte(out), &m) != nil || m["decision"] != "block" || !strings.Contains(m["reason"].(string), "No pidas secretos") {
		t.Errorf("block en prompt: %q", out)
	}
	out, _, _ = expandRun(t, bin, dir, `{"hook_event_name":"Stop","last_assistant_message":"queda un TODO pendiente"}`, "stop", "--emit=json")
	if !strings.Contains(out, `"decision":"block"`) {
		t.Errorf("block en stop: %q", out)
	}
}

// Analyze C2: con rules apagada, rm -rf pasa y la sugerencia sigue.
func TestHookExpandRules_OffRulesKeepsFlow(t *testing.T) {
	bin := buildMemBinary(t)
	dir := expandProject(t)
	writeExpandSettings(t, dir, map[string]any{"expand_off": []string{"rules"}})
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"c2","tool_input":{"command":"rm -rf /tmp/x && git commit -m x"}}`
	out, _, code := expandRun(t, bin, dir, payload, "pre-tool")
	if code != 0 || strings.Contains(out, "deny") || !strings.Contains(out, "verify-loop") {
		t.Fatalf("off rules: %q %d", out, code)
	}
}
