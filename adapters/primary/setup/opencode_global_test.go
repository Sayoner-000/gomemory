package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

// homeTemporal aísla el HOME para que openCodeGlobalConfigPath resuelva dentro
// del directorio de la prueba y no toque la configuración real de la máquina.
func homeTemporal(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func escribirJSON(t *testing.T, ruta string, contenido map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	datos, err := json.MarshalIndent(contenido, "", "  ")
	if err != nil {
		t.Fatalf("serializar: %v", err)
	}
	if err := os.WriteFile(ruta, datos, 0o644); err != nil {
		t.Fatalf("escribir: %v", err)
	}
}

func leerJSON(t *testing.T, ruta string) map[string]interface{} {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(datos, &cfg); err != nil {
		t.Fatalf("parsear %s: %v", ruta, err)
	}
	return cfg
}

// TestInstallOpenCode_RegistraEnScopeUsuarioYRetiraElArchivoDeProyecto cubre la
// migración completa: el registro del servidor y los permisos viven en
// ~/.config/opencode/opencode.json, y el archivo de proyecto —que solo
// duplicaba lo que el merge de OpenCode ya resuelve— desaparece.
func TestInstallOpenCode_RegistraEnScopeUsuarioYRetiraElArchivoDeProyecto(t *testing.T) {
	home := homeTemporal(t)
	root := t.TempDir()
	escribirJSON(t, filepath.Join(root, "opencode.json"), map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]interface{}{
			"gomemory": map[string]interface{}{"type": "local"},
		},
		"permission": map[string]interface{}{
			"gomemory_*": "allow",
		},
	})

	// PluginFS lo inyecta el composition root (embed en infrastructure); el
	// test monta un FS de juguete con la misma forma para ejercitar la copia.
	fuente := t.TempDir()
	dir := filepath.Join(fuente, "plugin", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gomemory.ts"),
		[]byte("const BIN = \"{{BIN_PATH}}\";\nexport const GomemoryPlugin = {};\n"), 0o644); err != nil {
		t.Fatalf("escribir plugin: %v", err)
	}
	PluginFS = os.DirFS(fuente)
	t.Cleanup(func() { PluginFS = nil })

	ref := AgentRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}
	if err := InstallOpenCode(root, ref); err != nil {
		t.Fatalf("install: %v", err)
	}

	global := leerJSON(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	mcp, _ := global["mcp"].(map[string]interface{})
	if mcp == nil || mcp["gomemory"] == nil {
		t.Errorf("el scope usuario debía registrar gomemory:\n%s", global["mcp"])
	}
	perm, _ := global["permission"].(map[string]interface{})
	if perm == nil || perm["gomemory_*"] != "allow" {
		t.Errorf("el scope usuario debía pre-aprobar las tools:\n%v", perm)
	}

	if _, err := os.Stat(filepath.Join(root, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("el opencode.json de proyecto debía eliminarse; stat devolvió %v", err)
	}
}

// TestCleanupProjectOpenCodeRegistration_ConservaLoAjeno: si el archivo guarda
// configuración propia de la persona, se reescribe sin nuestras claves y se
// conserva.
func TestCleanupProjectOpenCodeRegistration_ConservaLoAjeno(t *testing.T) {
	root := t.TempDir()
	ruta := filepath.Join(root, "opencode.json")
	escribirJSON(t, ruta, map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"model":   "qwen2.5-coder:7b",
		"mcp": map[string]interface{}{
			"gomemory": map[string]interface{}{"type": "local"},
			"otro-mcp": map[string]interface{}{"type": "local"},
		},
		"permission": map[string]interface{}{
			"gomemory_*":             "allow",
			"gomemory_forget_memory": "ask",
			"otra_tool":              "allow",
		},
	})

	cleanupProjectOpenCodeRegistration(root)

	cfg := leerJSON(t, ruta)
	if cfg["model"] != "qwen2.5-coder:7b" {
		t.Errorf("la config ajena debía conservarse: %v", cfg)
	}
	mcp := cfg["mcp"].(map[string]interface{})
	if _, sigue := mcp["gomemory"]; sigue {
		t.Error("el registro de gomemory debía retirarse del proyecto")
	}
	if _, sigue := mcp["otro-mcp"]; !sigue {
		t.Error("el registro ajeno debía conservarse")
	}
	perm := cfg["permission"].(map[string]interface{})
	if len(perm) != 1 || perm["otra_tool"] != "allow" {
		t.Errorf("solo la permisión ajena debía quedar: %v", perm)
	}
}

// TestCleanupProjectOpenCodeRegistration_SinNuestroNoToca: un archivo sin
// rastro de gomemory no se modifica ni se borra.
func TestCleanupProjectOpenCodeRegistration_SinNuestroNoToca(t *testing.T) {
	root := t.TempDir()
	ruta := filepath.Join(root, "opencode.json")
	escribirJSON(t, ruta, map[string]interface{}{"model": "otro"})

	cleanupProjectOpenCodeRegistration(root)

	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("el archivo ajeno debía conservarse intacto: %v", err)
	}
}

// TestOpenCodeGlobalRegistrationExists: el inspector de activación necesita
// distinguir registrado / sin registro / JSON ilegible en el scope usuario.
func TestOpenCodeGlobalRegistrationExists(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido string
		existe    bool
	}{
		{"registrado", `{"mcp":{"gomemory":{"command":["mem","mcp"]}}}`, true},
		{"sin registro", `{"mcp":{"otro":{"command":["x"]}}}`, false},
		{"json ilegible", `{no soy json`, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			home := homeTemporal(t)
			dir := filepath.Join(home, ".config", "opencode")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(c.contenido), 0o644); err != nil {
				t.Fatalf("escribir: %v", err)
			}

			if got := OpenCodeGlobalRegistrationExists(); got != c.existe {
				t.Errorf("esperaba %v, obtuve %v", c.existe, got)
			}
		})
	}
}

// TestInspectOpenCodeServerConfig: el canal nuevo reporta ok con registro y
// missing sin él — es la evidencia que sostiene la migración al scope usuario.
func TestInspectOpenCodeServerConfig(t *testing.T) {
	inspector := NewActivationInspector()

	home := homeTemporal(t)
	ch := buscarCanal(inspector, t.TempDir())
	if ch == nil {
		t.Fatal("el canal server_config de opencode·usuario debía existir siempre, nunca omitirse")
	}
	if ch.State != domain.StateMissing {
		t.Errorf("sin config global esperaba missing, obtuve %s (%s)", ch.State, ch.Detail)
	}

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	escribirJSON(t, filepath.Join(dir, "opencode.json"), map[string]interface{}{
		"mcp": map[string]interface{}{
			"gomemory": map[string]interface{}{"command": []string{"mem", "mcp"}},
		},
	})

	var encontrado bool
	for _, c := range inspector.Inspect(t.TempDir()) {
		if c.Agent == "opencode" && c.Scope == domain.ScopeUser && c.Kind == domain.KindServerConfig {
			encontrado = true
			if c.State != domain.StateOK {
				t.Errorf("con registro global esperaba ok, obtuve %s (%s)", c.State, c.Detail)
			}
		}
	}
	if !encontrado {
		t.Error("el canal server_config de opencode·usuario debía aparecer en el informe")
	}
}

// buscarCanal devuelve el canal opencode·user·server_config del informe, o nil.
func buscarCanal(inspector *ActivationInspector, root string) *domain.ActivationChannel {
	for _, c := range inspector.Inspect(root) {
		if c.Agent == "opencode" && c.Scope == domain.ScopeUser && c.Kind == domain.KindServerConfig {
			copia := c
			return &copia
		}
	}
	return nil
}
