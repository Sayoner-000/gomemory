package setup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Formas posibles del plugin gomemory instalado (feature 032). OpenCode 2.x
// solo carga la forma dual; 1.x carga las dos.
const (
	PluginShapeDual    = "dual"
	PluginShapeV1Only  = "v1-only"
	PluginShapeMissing = "missing"
)

// OpenCodeInstallStatus resume lo que `mem doctor` necesita para explicar por
// qué OpenCode muestra un error de plugin y cómo repararlo.
type OpenCodeInstallStatus struct {
	Version           string
	Major             int
	PluginPath        string
	PluginShape       string
	ForeignV1Plugins  []string
	StaleTestArtifact bool
}

// Compatible informa si el plugin instalado carga en la versión detectada.
// Solo la forma dual es segura sin versión conocida: un plugin solo v1 carga
// en 1.x, pero OpenCode 2.x lo rechaza, y el doctor puede no ver el binario
// (fuera del PATH, o instalado como app).
func (s OpenCodeInstallStatus) Compatible() bool {
	switch s.PluginShape {
	case PluginShapeDual:
		return true
	case PluginShapeV1Only:
		return s.Major == 1
	default:
		return false
	}
}

var openCodeVersionRe = regexp.MustCompile(`\bv?(\d+)\.(\d+)\.(\d+)\b`)

// ParseOpenCodeVersion extrae la versión de `opencode --version`: la 2.x
// imprime "opencode v2.0.16" y la 1.x solo "1.18.32".
func ParseOpenCodeVersion(out string) (int, string) {
	m := openCodeVersionRe.FindStringSubmatch(out)
	if m == nil {
		return 0, ""
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, ""
	}
	return major, m[1] + "." + m[2] + "." + m[3]
}

// DetectPluginShape clasifica el plugin por su texto, sin ejecutarlo: la forma
// dual declara el export por defecto con setup y server.
func DetectPluginShape(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginShapeMissing
	}
	texto := string(data)
	if strings.Contains(texto, "export default {") && strings.Contains(texto, "setup:") && strings.Contains(texto, "server:") {
		return PluginShapeDual
	}
	return PluginShapeV1Only
}

// ForeignV1Plugins lista los plugins ajenos (.ts/.js) sin export por defecto
// en la carpeta de plugins, que OpenCode 2.x rechaza al cargar. Es heurística
// textual: no se ejecuta código de terceros. En 1.x esa forma es válida.
func ForeignV1Plugins(dir string, major int) []string {
	if major < 2 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "gomemory.ts" || strings.Contains(name, ".test.") {
			continue
		}
		if ext := filepath.Ext(name); ext != ".ts" && ext != ".js" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || strings.Contains(string(data), "export default") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// openCodeVersionCommand es el comando que consulta la versión; las pruebas
// lo reemplazan para no depender de un OpenCode instalado.
var openCodeVersionCommand = func(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "opencode", "--version").Output()
}

// detectOpenCodeVersion ejecuta `opencode --version` con timeout. Si el
// binario no está o no responde, devuelve (0, "").
func detectOpenCodeVersion() (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := openCodeVersionCommand(ctx)
	if err != nil {
		return 0, ""
	}
	return ParseOpenCodeVersion(string(out))
}

// InspectOpenCode reúne el estado de OpenCode en esta máquina. Si `opencode`
// no está o no responde, la versión queda vacía y el resto se informa igual.
func InspectOpenCode() OpenCodeInstallStatus {
	var st OpenCodeInstallStatus
	st.Major, st.Version = detectOpenCodeVersion()
	home, err := os.UserHomeDir()
	if err != nil {
		st.PluginShape = PluginShapeMissing
		return st
	}
	dir := filepath.Join(home, ".config", "opencode", "plugins")
	st.PluginPath = filepath.Join(dir, "gomemory.ts")
	st.PluginShape = DetectPluginShape(st.PluginPath)
	st.ForeignV1Plugins = ForeignV1Plugins(dir, st.Major)
	if _, err := os.Stat(filepath.Join(dir, "gomemory.test.mjs")); err == nil {
		st.StaleTestArtifact = true
	}
	return st
}
