package cli

import (
	"fmt"

	"mem/adapters/primary/setup"
)

// inspectOpenCode es la fuente del estado de OpenCode; las pruebas la
// reemplazan para no depender de un OpenCode instalado.
var inspectOpenCode = setup.InspectOpenCode

// openCodeRemedy es la reinstalación que deja el plugin dual en su sitio.
const openCodeRemedy = "mem setup-mcp --scope global --agents opencode"

// doctorOpenCodeJSON es la sección OpenCode de `mem doctor --json`.
type doctorOpenCodeJSON struct {
	Version           string   `json:"version"`
	PluginShape       string   `json:"plugin_shape"`
	Compatible        bool     `json:"compatible"`
	ForeignV1Plugins  []string `json:"foreign_v1_plugins"`
	StaleTestArtifact bool     `json:"stale_test_artifact"`
	Remedio           string   `json:"remedio,omitempty"`
}

// openCodeRelevant: sin OpenCode ni plugin no hay nada que informar.
func openCodeRelevant(st setup.OpenCodeInstallStatus) bool {
	return st.Version != "" || st.PluginShape != setup.PluginShapeMissing
}

// openCodeProblem: el plugin de gomemory está instalado y no carga en la
// versión detectada (feature 032). Los plugins ajenos no cuentan: no los
// gestiona gomemory.
func openCodeProblem(st setup.OpenCodeInstallStatus) bool {
	return st.PluginShape != setup.PluginShapeMissing && !st.Compatible()
}

func openCodeJSON(st setup.OpenCodeInstallStatus) *doctorOpenCodeJSON {
	if !openCodeRelevant(st) {
		return nil
	}
	out := &doctorOpenCodeJSON{
		Version: st.Version, PluginShape: st.PluginShape, Compatible: st.Compatible(),
		ForeignV1Plugins: st.ForeignV1Plugins, StaleTestArtifact: st.StaleTestArtifact,
	}
	if out.ForeignV1Plugins == nil {
		out.ForeignV1Plugins = []string{}
	}
	if openCodeProblem(st) || st.StaleTestArtifact {
		out.Remedio = openCodeRemedy
	}
	return out
}

// printDoctorOpenCode pinta la sección OpenCode (contrato C-4 de
// specs/032-opencode-v2-plugin-compat).
func printDoctorOpenCode(st setup.OpenCodeInstallStatus) {
	if !openCodeRelevant(st) {
		return
	}
	version := "versión no detectada"
	if st.Version != "" {
		version = "v" + st.Version + " detectado"
	}
	var plugin string
	switch st.PluginShape {
	case setup.PluginShapeDual:
		plugin = "dual (v1+v2) ✅"
	case setup.PluginShapeV1Only:
		switch {
		case st.Compatible():
			plugin = "solo v1 ✅ (actualízalo antes de pasar a OpenCode 2.x: " + openCodeRemedy + ")"
		case st.Major == 0:
			plugin = "solo v1 ❌ (OpenCode 2.x no lo carga) → ejecuta `" + openCodeRemedy + "`"
		default:
			plugin = "solo v1 ❌ → ejecuta `" + openCodeRemedy + "`"
		}
	default:
		plugin = "no instalado"
	}
	fmt.Printf("\nOpenCode: %s · plugin gomemory: %s\n", version, plugin)
	for _, p := range st.ForeignV1Plugins {
		fmt.Printf("⚠️  plugin ajeno con forma v1 en OpenCode 2.x: ~/.config/opencode/plugins/%s (no es de gomemory; actualízalo en su proyecto)\n", p)
	}
	if st.StaleTestArtifact {
		fmt.Printf("🧹 artefacto de pruebas en la carpeta de plugins: gomemory.test.mjs (se retira al reinstalar: %s)\n", openCodeRemedy)
	}
}
