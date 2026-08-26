package cli

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
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
	if añadidos != 3 {
		t.Fatalf("se esperaban 3 enganches añadidos, got %d", añadidos)
	}

	texto := string(got)
	for _, sub := range []string{"hook session-start", "hook post-compact", "hook turn-end"} {
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
