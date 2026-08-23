package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// leerJSON devuelve el contenido de un archivo JSON como mapa, fallando el test
// si no se puede leer o interpretar.
func leerJSON(t *testing.T, ruta string) map[string]any {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(datos, &cfg); err != nil {
		t.Fatalf("interpretar %s: %v", ruta, err)
	}
	return cfg
}

// escribirOpenCodeJSON deja un opencode.json con el esquema REAL que produce la
// instalación vigente: clave "mcp" (no "mcpServers") y clave "permission".
func escribirOpenCodeJSON(t *testing.T, dir string, extra map[string]any) string {
	t.Helper()
	cfg := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"gomemory": map[string]any{
				"command": []any{"mem", "mcp"},
				"enabled": true,
				"type":    "local",
			},
		},
		"permission": map[string]any{
			"gomemory_*":             "allow",
			"gomemory_forget_memory": "ask",
		},
	}
	for k, v := range extra {
		cfg[k] = v
	}
	datos, _ := json.MarshalIndent(cfg, "", "  ")
	ruta := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(ruta, datos, 0o644); err != nil {
		t.Fatalf("escribir opencode.json: %v", err)
	}
	return ruta
}

// TestRemoveMCPEntries_RetiraGomemoryDeOpenCode cubre el defecto de raíz: la
// tabla de configuraciones solo listaba `.opencode.json` (esquema legado con
// "mcpServers") y el borrador solo sabía leer "mcpServers", así que la entrada
// del `opencode.json` vigente sobrevivía a toda desinstalación.
func TestRemoveMCPEntries_RetiraGomemoryDeOpenCode(t *testing.T) {
	dir := t.TempDir()
	ruta := escribirOpenCodeJSON(t, dir, nil)

	captureStdout(t, func() { removeMCPEntries(dir) })

	cfg := leerJSON(t, ruta)
	mcp, _ := cfg["mcp"].(map[string]any)
	if _, sigue := mcp["gomemory"]; sigue {
		t.Error("la entrada gomemory sobrevivió al uninstall en opencode.json")
	}
}

// TestRemoveMCPEntries_PreservaOtrosServidoresDeOpenCode: la limpieza es
// conservadora con lo ajeno, igual que del lado de Claude Code.
func TestRemoveMCPEntries_PreservaOtrosServidoresDeOpenCode(t *testing.T) {
	dir := t.TempDir()
	ruta := escribirOpenCodeJSON(t, dir, map[string]any{
		"mcp": map[string]any{
			"gomemory": map[string]any{"type": "local"},
			"context7": map[string]any{"type": "remote"},
		},
	})

	captureStdout(t, func() { removeMCPEntries(dir) })

	cfg := leerJSON(t, ruta)
	mcp, _ := cfg["mcp"].(map[string]any)
	if _, sigue := mcp["gomemory"]; sigue {
		t.Error("la entrada gomemory sobrevivió al uninstall")
	}
	if _, ok := mcp["context7"]; !ok {
		t.Error("se perdió un servidor ajeno: la limpieza debe tocar solo gomemory")
	}
}

// TestRemoveMCPEntries_SigueLimpiandoElEsquemaLegado: el `.opencode.json` con
// "mcpServers" que dejaron versiones anteriores debe seguir limpiándose.
func TestRemoveMCPEntries_SigueLimpiandoElEsquemaLegado(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, ".opencode.json")
	datos, _ := json.MarshalIndent(map[string]any{
		"mcpServers": map[string]any{"gomemory": map[string]any{"command": "mem"}},
	}, "", "  ")
	os.WriteFile(ruta, datos, 0o644)

	captureStdout(t, func() { removeMCPEntries(dir) })

	cfg := leerJSON(t, ruta)
	ms, _ := cfg["mcpServers"].(map[string]any)
	if _, sigue := ms["gomemory"]; sigue {
		t.Error("la entrada legada en .opencode.json sobrevivió al uninstall")
	}
}

// TestRemoveOpenCodeArtifacts_LimpiaPermisosDelProyecto verifica que se retiran
// los permisos que la instalación escribe en el opencode.json del proyecto, sin
// tocar los que la persona declaró por su cuenta.
func TestRemoveOpenCodeArtifacts_LimpiaPermisosDelProyecto(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	ruta := escribirOpenCodeJSON(t, dir, map[string]any{
		"permission": map[string]any{
			"gomemory_*":             "allow",
			"gomemory_forget_memory": "ask",
			"bash":                   "ask", // permiso propio de la persona
		},
	})

	captureStdout(t, func() { removeOpenCodeArtifacts(dir) })

	cfg := leerJSON(t, ruta)
	perm, _ := cfg["permission"].(map[string]any)
	for _, clave := range []string{"gomemory_*", "gomemory_forget_memory"} {
		if _, sigue := perm[clave]; sigue {
			t.Errorf("el permiso %q sobrevivió al uninstall", clave)
		}
	}
	if _, ok := perm["bash"]; !ok {
		t.Error("se perdió un permiso ajeno: la limpieza debe tocar solo los de gomemory")
	}
}

// TestRemoveOpenCodeArtifacts_NoTocaElAmbitoDeUsuario fija la regresión que
// importa: una desinstalación dirigida a un proyecto NO puede borrar el plugin
// de ámbito de usuario, porque lo comparten todos los proyectos.
//
// Sin esta garantía, las pruebas de integración que ejercen CmdUninstall sobre
// un directorio temporal —y que no aíslan HOME— borran el plugin real de quien
// ejecute la batería. Ocurrió.
func TestRemoveOpenCodeArtifacts_NoTocaElAmbitoDeUsuario(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	escribirOpenCodeJSON(t, dir, nil)
	plugin := filepath.Join(home, ".config", "opencode", "plugins", "gomemory.ts")
	os.MkdirAll(filepath.Dir(plugin), 0o755)
	os.WriteFile(plugin, []byte("// plugin"), 0o644)

	salida := captureStdout(t, func() { removeOpenCodeArtifacts(dir) })

	if _, err := os.Stat(plugin); err != nil {
		t.Fatal("el plugin de ámbito de usuario fue eliminado por una desinstalación de proyecto")
	}
	if !strings.Contains(salida, "no se elimina") {
		t.Errorf("debe informarse que el plugin queda en su sitio y por qué:\n%s", salida)
	}
}

// TestRemoveOpenCodeArtifacts_SinArtefactosNoFalla: idempotente y silenciosa
// sobre un proyecto donde OpenCode nunca se instaló.
func TestRemoveOpenCodeArtifacts_SinArtefactosNoFalla(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	salida := captureStdout(t, func() { removeOpenCodeArtifacts(dir) })

	if strings.Contains(salida, "⚠️") {
		t.Errorf("no debería advertir sobre artefactos inexistentes:\n%s", salida)
	}
}

// TestRemoveNativeWrappers_RetiraLosDeAmbosAgentes cierra FR-008 para los
// envoltorios nativos.
//
// La verificación de dominio decía que la desinstalación los cubría, y la
// batería estaba en verde. La validación contra el binario mostró lo contrario:
// sobrevivían los cuatro. Es el caso literal de que «verde en tests» no es
// «funciona» — el contrato describía la intención y nadie comprobaba el efecto.
func TestRemoveNativeWrappers_RetiraLosDeAmbosAgentes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	generados := []string{
		filepath.Join(".claude", "skills", "atomic-decomposition", "SKILL.md"),
		filepath.Join(".claude", "skills", "constitution", "SKILL.md"),
		filepath.Join(".opencode", "commands", "atomic-decomposition.md"),
		filepath.Join(".opencode", "commands", "constitution.md"),
	}
	// Artefactos ajenos que DEBEN sobrevivir: una habilidad de otra herramienta
	// y un comando propio de la persona.
	ajenos := []string{
		filepath.Join(".claude", "skills", "speckit-plan", "SKILL.md"),
		filepath.Join(".opencode", "commands", "mi-comando.md"),
	}
	for _, rel := range append(append([]string{}, generados...), ajenos...) {
		ruta := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(ruta), 0o755)
		os.WriteFile(ruta, []byte("contenido"), 0o644)
	}

	captureStdout(t, func() { removeNativeWrappers(dir) })

	for _, rel := range generados {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Errorf("el envoltorio %q sobrevivió a la desinstalación", rel)
		}
	}
	for _, rel := range ajenos {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("se eliminó %q, que no es de gomemory", rel)
		}
	}
}

// registroFalso captura lo que los hooks de canal anotan, sin base de datos.
type registroFalso struct {
	firedAgent, firedKind string
	errKind, errMsg       string
}

func (r *registroFalso) RecordFired(a, s, k string) error {
	r.firedAgent, r.firedKind = a, k
	return nil
}
func (r *registroFalso) RecordError(a, s, k, m string) error {
	r.errKind, r.errMsg = k, m
	return nil
}
func (r *registroFalso) Last(a, s, k string) (time.Time, string, bool) {
	return time.Time{}, "", false
}
func (r *registroFalso) SessionsSince(t time.Time) int { return 0 }

// TestHookChannelActivity_LeeLosArgumentosEnSuPosicion fija el desfase que
// tuvo esta implementación: args[0] es el propio subcomando, así que leer el
// agente desde args[0] anotaba "channel-error" como nombre de agente y el
// rastro nunca aparecía en el informe.
func TestHookChannelActivity_LeeLosArgumentosEnSuPosicion(t *testing.T) {
	reg := &registroFalso{}
	deps := &Deps{ChannelActivity: reg}

	CmdHook(deps, []string{"channel-fired", "opencode", "user", "plan_entry"})
	if reg.firedAgent != "opencode" || reg.firedKind != "plan_entry" {
		t.Errorf("channel-fired anotó agente=%q canal=%q", reg.firedAgent, reg.firedKind)
	}

	CmdHook(deps, []string{"channel-error", "opencode", "user", "plan_entry", "la operación ya no existe"})
	if reg.errKind != "plan_entry" || reg.errMsg != "la operación ya no existe" {
		t.Errorf("channel-error anotó canal=%q msg=%q", reg.errKind, reg.errMsg)
	}
}
