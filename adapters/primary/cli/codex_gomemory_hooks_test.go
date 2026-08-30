package cli

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"mem/adapters/primary/setup"
)

// configConHooksAjenos reproduce el estado real de una máquina tras la
// consolidación de la v2.12.0: hooks de terceros presentes, y ni uno de
// gomemory.
const configConHooksAjenos = `[mcp_servers.gomemory]
command = "mem"
args = ["mcp"]

[features]
hooks = true

[hooks]
[[hooks.SessionStart]]
matcher = "startup|resume|clear|compact"

[[hooks.SessionStart.hooks]]
type = "command"
command = "echo aviso de otro proveedor"

[[hooks.Stop]]
[[hooks.Stop.hooks]]
type = "command"
command = "bash /Users/alguien/.codex/otro-agente.sh"
timeout = 10
`

func decodeTOML(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("TOML inválido: %v\n%s", err, data)
	}
	return doc
}

// TestEnsureCodexGomemoryHooks_RegistraElCicloCompleto cubre el hueco medido:
// Codex recibía registro MCP pero ningún hook, así que gomemory no ejercía su
// ciclo — sin inyección al arrancar y sin checkpoint al cerrar el turno.
func TestEnsureCodexGomemoryHooks_RegistraElCicloCompleto(t *testing.T) {
	got, añadidos, err := ensureCodexGomemoryHooks([]byte(configConHooksAjenos), "mem")
	if err != nil {
		t.Fatalf("ensureCodexGomemoryHooks: %v", err)
	}
	// Derivado de la tabla, no de un literal: con el número escrito a mano este
	// test se rompía al crecer el ciclo y obligaba a editarlo cada vez, que es
	// como una aserción acaba relajándose «para que pase». Ahora la tabla es la
	// única fuente y añadir un enganche no exige tocar aquí.
	esperados := setup.CodexGomemoryHooks()
	if añadidos != len(esperados) {
		t.Fatalf("se esperaban %d enganches añadidos, got %d", len(esperados), añadidos)
	}

	texto := string(got)
	for _, h := range esperados {
		sub := "hook " + h.Sub
		if !strings.Contains(texto, sub) {
			t.Errorf("falta el enganche %q en el resultado:\n%s", sub, texto)
		}
	}

	// Nada ajeno puede perderse: es el archivo de configuración de la persona.
	if !strings.Contains(texto, "echo aviso de otro proveedor") ||
		!strings.Contains(texto, "otro-agente.sh") {
		t.Errorf("los hooks ajenos deben preservarse íntegros:\n%s", texto)
	}
	if !strings.Contains(texto, "[mcp_servers.gomemory]") {
		t.Errorf("el registro MCP no puede perderse al escribir hooks:\n%s", texto)
	}
	decodeTOML(t, got)
}

// TestEnsureCodexGomemoryHooks_EsIdempotente: reinstalar no puede duplicar el
// ciclo. Un hook duplicado se ejecuta dos veces por turno y produce checkpoints
// repetidos, que es justo el defecto que se acaba de corregir aguas arriba.
func TestEnsureCodexGomemoryHooks_EsIdempotente(t *testing.T) {
	primera, _, err := ensureCodexGomemoryHooks([]byte(configConHooksAjenos), "mem")
	if err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	segunda, añadidos, err := ensureCodexGomemoryHooks(primera, "mem")
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if añadidos != 0 {
		t.Fatalf("reaplicar no debe añadir nada, añadió %d", añadidos)
	}
	if string(segunda) != string(primera) {
		t.Fatalf("reaplicar no debe cambiar el archivo:\n--- primera ---\n%s\n--- segunda ---\n%s", primera, segunda)
	}
	if strings.Count(string(segunda), "hook turn-end") != 1 {
		t.Fatalf("el enganche de cierre de turno quedó duplicado:\n%s", segunda)
	}
}

// TestEnsureCodexGomemoryHooks_ReconoceOtraRutaDelBinario: quien instaló con una
// ruta absoluta ya tiene el ciclo enganchado; reinstalar con "mem" no debe
// añadir un segundo hook equivalente.
func TestEnsureCodexGomemoryHooks_ReconoceOtraRutaDelBinario(t *testing.T) {
	conRutaAbsoluta, _, err := ensureCodexGomemoryHooks([]byte(configConHooksAjenos), "/opt/bin/mem")
	if err != nil {
		t.Fatalf("instalación con ruta absoluta: %v", err)
	}
	_, añadidos, err := ensureCodexGomemoryHooks(conRutaAbsoluta, "mem")
	if err != nil {
		t.Fatalf("reinstalación: %v", err)
	}
	if añadidos != 0 {
		t.Fatalf("el ciclo ya estaba enganchado; no debe volver a añadirse (%d)", añadidos)
	}
}

// TestEnsureCodexGomemoryHooks_ActualizaElDialectoHeredado fija la migración
// desde v2.16.6: el mismo subcomando sin --emit=text emitía el sobre JSON de
// Claude y Codex descartaba la inyección de turn-end.
func TestEnsureCodexGomemoryHooks_ActualizaElDialectoHeredado(t *testing.T) {
	const heredado = `[features]
hooks = true

[hooks]
[[hooks.Stop]]
[[hooks.Stop.hooks]]
type = "command"
command = "mem hook turn-end"
`

	got, cambios, err := ensureCodexGomemoryHooks([]byte(heredado), "mem")
	if err != nil {
		t.Fatalf("ensureCodexGomemoryHooks: %v", err)
	}
	if esperados := len(setup.CodexGomemoryHooks()); cambios != esperados {
		t.Fatalf("se esperaba actualizar turn-end y añadir %d hooks del ciclo, got %d", esperados-1, cambios)
	}
	texto := string(got)
	if strings.Count(texto, "hook turn-end") != 1 {
		t.Fatalf("turn-end debe actualizarse sin duplicarse:\n%s", texto)
	}
	if !strings.Contains(texto, "command = 'mem hook turn-end --emit=text'") &&
		!strings.Contains(texto, `command = "mem hook turn-end --emit=text"`) {
		t.Fatalf("turn-end debe usar el dialecto plano:\n%s", texto)
	}
	if !setup.CodexHookPresente(decodeTOML(t, got)["hooks"].(map[string]any), setup.CodexGomemoryHooks()[2]) {
		t.Fatalf("el hook actualizado debe satisfacer el diagnóstico de setup:\n%s", got)
	}
	segunda, cambios, err := ensureCodexGomemoryHooks(got, "mem")
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if cambios != 0 || string(segunda) != string(got) {
		t.Fatalf("la migración corregida debe ser idempotente; cambios=%d\n%s", cambios, segunda)
	}
}

// TestEnsureCodexGomemoryHooks_ActivaLaBandera: escribir hooks sin
// `[features] hooks = true` deja un ciclo presente y muerto — el fallo
// silencioso más caro, porque el archivo se ve correcto.
func TestEnsureCodexGomemoryHooks_ActivaLaBandera(t *testing.T) {
	casos := map[string]string{
		"sin tabla features":          "[mcp_servers.gomemory]\ncommand = \"mem\"\n",
		"features sin la clave":       "[features]\notra_cosa = true\n",
		"features con la clave":       "[features]\nhooks = false\n",
		"archivo completamente vacío": "",
	}
	for nombre, entrada := range casos {
		t.Run(nombre, func(t *testing.T) {
			got, _, err := ensureCodexGomemoryHooks([]byte(entrada), "mem")
			if err != nil {
				t.Fatalf("ensureCodexGomemoryHooks: %v", err)
			}
			doc := decodeTOML(t, got)
			features, _ := doc["features"].(map[string]any)
			if activo, _ := features["hooks"].(bool); !activo {
				t.Fatalf("features.hooks debe quedar en true:\n%s", got)
			}
		})
	}
}

// TestHooksDeCodexQueInyectanTextoPidenElDialectoPlano fija la regla que el propio
// campo Emit documenta y que turn-end incumplía.
//
// Codex toma el stdout del hook COMO CONTEXTO TAL CUAL. Un hook que inyecta texto
// al modelo y se registra sin Emit emite el sobre JSON de Claude Code, que Codex no
// reconoce: rechaza la salida entera y el mensaje no llega nunca. `turn-end` inyecta
// el refuerzo de preferencias y estaba registrado sin pedir el dialecto plano; el
// fallo era silencioso salvo por un error del runtime ajeno a este repositorio.
func TestHooksDeCodexQueInyectanTextoPidenElDialectoPlano(t *testing.T) {
	// Subcomandos cuyo stdout va dirigido al MODELO. Si añades uno, decide de qué
	// lado está: si inyecta texto, va aquí y necesita Emit.
	inyectanTexto := map[string]bool{
		"user-prompt-submit": true,
		"turn-end":           true,
	}
	vistos := map[string]bool{}
	for _, hook := range setup.CodexGomemoryHooks() {
		vistos[hook.Sub] = true
		if !inyectanTexto[hook.Sub] {
			continue
		}
		if hook.Emit != "text" {
			t.Errorf("el hook %q inyecta texto al modelo y se registró con Emit=%q: "+
				"Codex recibiría el sobre JSON de Claude Code", hook.Sub, hook.Emit)
		}
	}
	for sub := range inyectanTexto {
		if !vistos[sub] {
			t.Errorf("el hook %q ya no está en la tabla de Codex: revisa esta lista", sub)
		}
	}
}

// TestEnsureCodexGomemoryHooks_NormalizaElDialectoSobrante cubre la dirección
// contraria a ActualizaElDialectoHeredado, que quedó sin cubrir.
//
// La reconciliación era asimétrica: sustituía el comando cuando FALTABA el
// --emit, pero no cuando SOBRABA. Un hook cuyo dialecto se retire de la tabla
// —una vuelta atrás, o un subcomando que deje de inyectar texto— dejaba de
// reconocerse como presente y se añadía otra vez, así que toda instalación ya
// migrada acababa ejecutando ese hook dos veces por evento. Es el mismo defecto
// que la migración vino a cerrar, en el sentido opuesto.
func TestEnsureCodexGomemoryHooks_NormalizaElDialectoSobrante(t *testing.T) {
	const sobrante = `[features]
hooks = true

[hooks]
[[hooks.SessionStart]]
matcher = "startup|resume|clear"
[[hooks.SessionStart.hooks]]
type = "command"
command = "mem hook session-start --emit=text"
`

	got, _, err := ensureCodexGomemoryHooks([]byte(sobrante), "mem")
	if err != nil {
		t.Fatalf("ensureCodexGomemoryHooks: %v", err)
	}
	texto := string(got)
	if n := strings.Count(texto, "hook session-start"); n != 1 {
		t.Fatalf("session-start se duplicó (%d apariciones):\n%s", n, texto)
	}
	if strings.Contains(texto, "hook session-start --emit") {
		t.Fatalf("session-start no declara dialecto: el --emit sobrante debe retirarse:\n%s", texto)
	}

	segunda, cambios, err := ensureCodexGomemoryHooks(got, "mem")
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if cambios != 0 || string(segunda) != string(got) {
		t.Fatalf("la normalización debe ser idempotente; cambios=%d\n%s", cambios, segunda)
	}
}

// TestEnsureCodexGomemoryHooks_ConservaLaRutaDelBinarioAlReconciliar protege la
// promesa que CodexHookPresente lleva documentada desde su origen: un hook de
// gomemory se reconoce por el subcomando, NO por la ruta del binario.
//
// Al reconciliar el dialecto había que reescribir el comando, y reescribirlo
// entero sustituía además la ruta que el usuario tenía instalada. Quien apunta a
// una ruta absoluta lo hace justamente porque `mem` no está en el PATH que ve
// Codex: cambiarla por el comando por defecto deja el hook registrado, visible y
// muerto. Solo el dialecto es de gomemory; la ruta es del usuario.
func TestEnsureCodexGomemoryHooks_ConservaLaRutaDelBinarioAlReconciliar(t *testing.T) {
	const rutaAbsoluta = `[features]
hooks = true

[hooks]
[[hooks.Stop]]
[[hooks.Stop.hooks]]
type = "command"
command = "/usr/local/bin/mem hook turn-end"
`

	got, _, err := ensureCodexGomemoryHooks([]byte(rutaAbsoluta), "mem")
	if err != nil {
		t.Fatalf("ensureCodexGomemoryHooks: %v", err)
	}
	texto := string(got)
	if n := strings.Count(texto, "hook turn-end"); n != 1 {
		t.Fatalf("turn-end se duplicó (%d apariciones):\n%s", n, texto)
	}
	if !strings.Contains(texto, "/usr/local/bin/mem hook turn-end --emit=text") {
		t.Fatalf("la reconciliación debe añadir el dialecto SIN tocar la ruta del binario:\n%s", texto)
	}
}
