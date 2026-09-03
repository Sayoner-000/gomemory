package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mem/adapters/secondary/persistence"
)

// buildMemBinary compila el binario mem una sola vez por corrida de tests y
// devuelve su ruta. hookSessionStart/hookUserPromptSubmit terminan con
// os.Exit(0), así que probarlas requiere un subproceso real (llamarlas
// in-process mataría el binario de test).
var (
	memBinOnce sync.Once
	memBinPath string
	memBinErr  error
)

func buildMemBinary(t *testing.T) string {
	t.Helper()
	memBinOnce.Do(func() {
		// os.MkdirTemp (no t.TempDir()): debe sobrevivir a todos los tests del
		// binario, no solo al que dispara el sync.Once.
		dir, err := os.MkdirTemp("", "gomemory-test-bin-*")
		if err != nil {
			memBinErr = err
			return
		}
		memBinPath = filepath.Join(dir, "mem-test-bin")
		cmd := exec.Command("go", "build", "-o", memBinPath, "./infrastructure")
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			memBinErr = err
			t.Logf("go build output: %s", out)
		}
	})
	if memBinErr != nil {
		t.Fatalf("build mem binary: %v", memBinErr)
	}
	return memBinPath
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// tests/integration -> raíz del repo
	return filepath.Join(wd, "..", "..")
}

// dirDeProyecto devuelve un directorio temporal para usar como raíz de un
// proyecto de prueba, con limpieza BEST-EFFORT.
//
// Existe para cerrar una carrera real, y benigna, que hacía fallar tests al
// azar sin que ninguna aserción fallara: al invocar el binario, el proveedor de
// grafo lanza `mem code-refresh` como proceso DESACOPLADO
// (codebasememory.Provider.MaybeRefresh) que escribe el snapshot dentro de
// `.memory/` de forma asíncrona. Si el test termina antes que ese hijo, la
// escritura compite con la limpieza y t.TempDir() aborta el test con
// "TempDir RemoveAll cleanup: directory not empty" — un test distinto cada vez,
// ~1 de cada 4 corridas.
//
// La escritura tardía no es un defecto del producto: el refresco en segundo
// plano es deliberado y no debe bloquear el hot path. Lo inadecuado era que la
// limpieza del directorio temporal tratara esa carrera como un fallo del test.
// Con os.RemoveAll ignorando el error, el sistema operativo se encarga del
// resto y la aserción del test vuelve a ser lo único que decide si pasa.
//
// Se descartó apagar el grafo con code_graph_disabled=true: cambia el texto que
// los hooks emiten (la lista de tools del grafo) y rompería
// TestHookSubagentStart_BootstrapVaEnAdditionalContext, que sí lo ejerce.
func dirDeProyecto(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gomemory-proyecto-*")
	if err != nil {
		t.Fatalf("crear directorio de proyecto: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func runHook(t *testing.T, bin, dir, event string) string {
	t.Helper()
	cmd := exec.Command(bin, "hook", event)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("mem hook %s: %v (%s)", event, err, out.String())
	}
	return out.String()
}

// TestHookMarkerResetsPerSession verifica el fix del bug: el recordatorio de
// protocolo (marker .session-tools-injected) debía inyectarse una sola vez en
// toda la vida del proyecto porque nada lo borraba. Ahora session-start lo
// resetea, así que debe re-inyectarse en el primer prompt de CADA sesión.
func TestHookMarkerResetsPerSession(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	marker := filepath.Join(target, ".memory", ".session-tools-injected")

	// Sesión 1: session-start, luego dos prompts.
	runHook(t, bin, target, "session-start")
	first := runHook(t, bin, target, "user-prompt-submit")
	if !bytes.Contains([]byte(first), []byte("additionalContext")) {
		t.Fatalf("primer prompt de la sesión debía inyectar additionalContext, got: %s", first)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker debía existir tras el primer prompt: %v", err)
	}

	// Desde la feature 019 (Historia 2), el segundo prompt SÍ puede llevar
	// additionalContext: el recordatorio de modo plan de una línea se emite en
	// cada turno (FR-003), a diferencia del bootstrap completo. Lo que este
	// test protege es que el BOOTSTRAP (ToolSearch + memoryProtocolReminder)
	// no se reinyecte — eso sigue gateado por el marker.
	second := runHook(t, bin, target, "user-prompt-submit")
	if bytes.Contains([]byte(second), []byte("PRIMERA ACCIÓN")) {
		t.Fatalf("segundo prompt de la misma sesión NO debía reinyectar el bootstrap completo, got: %s", second)
	}

	// Sesión 2: session-start debe resetear el marker.
	runHook(t, bin, target, "session-start")
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("marker debía eliminarse al iniciar una nueva sesión")
	}

	third := runHook(t, bin, target, "user-prompt-submit")
	if !bytes.Contains([]byte(third), []byte("additionalContext")) {
		t.Fatalf("primer prompt de la NUEVA sesión debía re-inyectar el recordatorio, got: %s", third)
	}
}

// TestHookSessionEndResetsMarker cubre el caso defensivo: cerrar sesión
// también debe resetear el marker (compactación/cierre sin un session-start
// intermedio no debe dejar el recordatorio inyectado para siempre).
func TestHookSessionEndResetsMarker(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	marker := filepath.Join(target, ".memory", ".session-tools-injected")

	runHook(t, bin, target, "session-start")
	runHook(t, bin, target, "user-prompt-submit")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker debía existir tras el primer prompt: %v", err)
	}

	runHook(t, bin, target, "session-end")
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("marker debía eliminarse al cerrar sesión")
	}
}

// TestHookUserPromptSubmit_BootstrapVaEnAdditionalContext cubre el bug real:
// Claude Code muestra el campo top-level "systemMessage" SOLO al humano en la
// terminal — nunca lo inyecta en el contexto del modelo (confirmado contra la
// documentación oficial de hooks). El bootstrap de ToolSearch que fuerza la
// carga de gomemory + codebase-memory-mcp vivía en "systemMessage", así que el
// agente jamás ejecutaba el ToolSearch forzado: la instrucción "PRIMERA ACCIÓN"
// nunca llegó a su contexto en ninguna sesión, solo se mostraba en la UI del
// usuario. Este test falla si el bootstrap vuelve a viajar por ese campo.
func TestHookUserPromptSubmit_BootstrapVaEnAdditionalContext(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	runHook(t, bin, target, "session-start")
	out := runHook(t, bin, target, "user-prompt-submit")

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida del hook no es JSON válido: %v\n%s", err, out)
	}

	if _, present := payload["systemMessage"]; present {
		t.Fatalf("el bootstrap de ToolSearch NO debe viajar en systemMessage (el modelo nunca lo ve): %s", out)
	}

	hso, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("esperaba hookSpecificOutput en la salida: %s", out)
	}
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "select:") || !strings.Contains(ctx, "mcp__gomemory__") {
		t.Fatalf("additionalContext debía incluir el select: de ToolSearch con las tools de gomemory: %q", ctx)
	}
}

// TestHookUserPromptSubmit_NudgeDeGuardadoVaEnAdditionalContext cubre el mismo
// bug para el recordatorio de guardado (prompts subsiguientes de la sesión):
// saveNudgeMessage está dirigido al agente en segunda persona ("llama a
// save_memory ahora"), así que debe viajar en additionalContext, no en
// systemMessage.
func TestHookUserPromptSubmit_NudgeDeGuardadoVaEnAdditionalContext(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	runHook(t, bin, target, "session-start")
	runHook(t, bin, target, "user-prompt-submit") // primer prompt: pone el marker

	// Empuja la sesión al pasado para superar el umbral de "overdue" (15 min)
	// sin esperar en tiempo real, y limpia el debounce por si el propio hook ya
	// escribió un timestamp de nudge.
	if _, err := db.Exec("UPDATE sessions SET created_at = datetime(created_at, '-1000 seconds')"); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
	db.Close()
	os.Remove(filepath.Join(target, ".memory", ".last-nudge"))

	out := runHook(t, bin, target, "user-prompt-submit") // segundo prompt: rama del nudge

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida del hook no es JSON válido: %v\n%s", err, out)
	}
	if _, present := payload["systemMessage"]; present {
		t.Fatalf("el recordatorio de guardado NO debe viajar en systemMessage (el modelo nunca lo ve): %s", out)
	}
	hso, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("esperaba hookSpecificOutput con el recordatorio de guardado: %s", out)
	}
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "save_memory") {
		t.Fatalf("additionalContext debía incluir el recordatorio de save_memory: %q", ctx)
	}
}

// TestHookSubagentStart_BootstrapVaEnAdditionalContext cubre el gap real: un
// subagente (tool Task, p. ej. tipo Explore) arranca en un contexto aislado que
// NUNCA pasa por session-start ni por el primer prompt de user-prompt-submit,
// así que sin este hook no recibía el bootstrap de ToolSearch ni el
// recordatorio del protocolo — el subagente no sabía que codebase-memory-mcp
// existía y recurría a grep/glob manuales para explorar código. No requiere
// session-start previo: un subagente puede arrancar sin que la sesión
// principal haya corrido ese hook en este processo de test.
func TestHookSubagentStart_BootstrapVaEnAdditionalContext(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	out := runHook(t, bin, target, "subagent-start")

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("salida del hook no es JSON válido: %v\n%s", err, out)
	}
	if _, present := payload["systemMessage"]; present {
		t.Fatalf("el bootstrap de subagente NO debe viajar en systemMessage (el modelo nunca lo ve): %s", out)
	}
	hso, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("esperaba hookSpecificOutput en la salida de subagent-start: %s", out)
	}
	if hso["hookEventName"] != "SubagentStart" {
		t.Errorf("hookEventName debía ser SubagentStart, got %v", hso["hookEventName"])
	}
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "select:") || !strings.Contains(ctx, "mcp__codebase-memory-mcp__") {
		t.Fatalf("additionalContext del subagente debía incluir el select: con codebase-memory-mcp: %q", ctx)
	}
}

func TestHookOctopusEncendido_InyectaLaPoliticaAlAgenteRaizYSubagente(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	settings := filepath.Join(target, ".memory", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"octopus_enabled":true}`), 0o600); err != nil {
		t.Fatalf("activar Octopus: %v", err)
	}

	for _, event := range []string{"user-prompt-submit", "subagent-start"} {
		out := runHook(t, bin, target, event)
		var payload map[string]any
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("%s debe devolver JSON: %v\\n%s", event, err, out)
		}
		hso, _ := payload["hookSpecificOutput"].(map[string]any)
		ctx, _ := hso["additionalContext"].(string)
		for _, want := range []string{"OCTOPUS AAR — REGLA OBLIGATORIA DE DELEGACIÓN", "mcp__gomemory__octopus_route_task", "DELEGATE es la única autorización"} {
			if !strings.Contains(ctx, want) {
				t.Errorf("%s debe inyectar %q: %q", event, want, ctx)
			}
		}
	}
}

// TestHookOctopusEncendido_SiguePresenteEnTurnosPosteriores es la regresión
// de ACR 029, hallazgo C-002: el bootstrap completo de user-prompt-submit solo
// se emite una vez por sesión (protegido por el marker), así que si la regla
// de Octopus viviera solo ahí, activar el módulo a mitad de sesión no le
// llegaría nunca al agente raíz hasta reiniciar o borrar el marcador. El
// segundo turno (marker ya escrito por el primero) debe seguir incluyéndola.
func TestHookOctopusEncendido_SiguePresenteEnTurnosPosteriores(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	settings := filepath.Join(target, ".memory", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"octopus_enabled":true}`), 0o600); err != nil {
		t.Fatalf("activar Octopus: %v", err)
	}

	runHook(t, bin, target, "user-prompt-submit") // primer turno: escribe el marker

	out := runHook(t, bin, target, "user-prompt-submit") // segundo turno
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("user-prompt-submit debe devolver JSON: %v\n%s", err, out)
	}
	hso, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hso["additionalContext"].(string)
	if !strings.Contains(ctx, "OCTOPUS AAR — REGLA OBLIGATORIA DE DELEGACIÓN") {
		t.Errorf("el segundo turno también debe incluir la regla de Octopus: %q", ctx)
	}
	// ACR 029, hallazgo C-001: sin vía de hook hacia un subagente de Codex, la
	// única forma de que la reciba es que el agente raíz la copie a mano.
	if !strings.Contains(ctx, "codex exec") {
		t.Errorf("debe instruir la propagación manual para subagentes sin hooks propios: %q", ctx)
	}
}

// TestHookNudge_IncluyeCompactNudge cubre un gap real de OpenCode: su plugin
// invoca `mem hook turn-end` en cada session.idle pero descarta la salida (solo
// la usa para grabar el checkpoint), y session.idle de todos modos ocurre
// DESPUÉS de que el modelo ya respondió — no hay forma de inyectarle contenido
// a esa respuesta ya emitida. El compact-nudge y el refuerzo de preferencias
// (que en Claude Code viajan por el hook Stop) nunca llegaban al modelo en
// OpenCode. Este test verifica que `mem hook nudge` —el único punto de OpenCode
// que sí llega al modelo, invocado en cada turno vía chat.system.transform—
// ahora también los incluye.
func TestHookNudge_IncluyeCompactNudge(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	// Huella por encima del umbral por defecto (48000): la huella se persiste
	// en un archivo plano, así que se puede simular sin pasar por el servidor MCP.
	if err := os.WriteFile(filepath.Join(target, ".memory", ".footprint"), []byte("60000"), 0644); err != nil {
		t.Fatalf("escribir footprint: %v", err)
	}

	out := runHook(t, bin, target, "nudge")
	if strings.Contains(out, "systemMessage") || strings.Contains(out, "{") {
		t.Fatalf("mem hook nudge debe imprimir texto plano, no JSON: %q", out)
	}
	if !strings.Contains(strings.ToLower(out), "compact") {
		t.Fatalf("esperaba el recordatorio de compactar en la salida de nudge: %q", out)
	}
}
