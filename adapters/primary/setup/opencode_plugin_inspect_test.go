package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseOpenCodeVersion(t *testing.T) {
	casos := []struct {
		salida  string
		major   int
		version string
	}{
		{"opencode v2.0.16\n", 2, "2.0.16"},
		{"1.18.32\n", 1, "1.18.32"},
		{"v1.17.0", 1, "1.17.0"},
		{"", 0, ""},
		{"Error: @opencode/cli's postinstall script was not run.", 0, ""},
	}
	for _, c := range casos {
		major, version := ParseOpenCodeVersion(c.salida)
		if major != c.major || version != c.version {
			t.Errorf("ParseOpenCodeVersion(%q) = (%d, %q); want (%d, %q)", c.salida, major, version, c.major, c.version)
		}
	}
}

func TestDetectPluginShape(t *testing.T) {
	dir := t.TempDir()
	escribir := func(nombre, texto string) string {
		ruta := filepath.Join(dir, nombre)
		if err := os.WriteFile(ruta, []byte(texto), 0o644); err != nil {
			t.Fatalf("escribir %s: %v", nombre, err)
		}
		return ruta
	}
	dual := escribir("dual.ts", "export const GomemoryPlugin = async () => ({});\nexport default {\n  id: \"gomemory\",\n  setup: createV2Setup(execFileExec),\n  server: GomemoryPlugin,\n};\n")
	v1 := escribir("v1.ts", "export const GomemoryPlugin: Plugin = async ({ $, directory, client }) => {\n  return {};\n};\n")
	casos := map[string]string{dual: PluginShapeDual, v1: PluginShapeV1Only, filepath.Join(dir, "no-existe.ts"): PluginShapeMissing}
	for ruta, want := range casos {
		if got := DetectPluginShape(ruta); got != want {
			t.Errorf("DetectPluginShape(%s) = %q; want %q", filepath.Base(ruta), got, want)
		}
	}
}

func TestForeignV1Plugins(t *testing.T) {
	dir := t.TempDir()
	archivos := map[string]string{
		"cbm-augment.ts":    "export const CbmAugment = async () => ({});\n",
		"ok.ts":             "export default { id: \"ok\", setup() {} };\n",
		"gomemory.ts":       "export const GomemoryPlugin = async () => ({});\n",
		"gomemory.test.mjs": "import { test } from 'node:test';\n",
		"notas.md":          "# no es un plugin\n",
	}
	for n, texto := range archivos {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(texto), 0o644); err != nil {
			t.Fatalf("escribir %s: %v", n, err)
		}
	}
	if got := ForeignV1Plugins(dir, 2); !reflect.DeepEqual(got, []string{"cbm-augment.ts"}) {
		t.Errorf("ForeignV1Plugins(v2) = %v; want [cbm-augment.ts]", got)
	}
	if got := ForeignV1Plugins(dir, 1); len(got) != 0 {
		t.Errorf("en OpenCode 1.x los plugins v1 son válidos; got %v", got)
	}
}

// TestCompatible_VersionDesconocida: sin versión detectada (OpenCode fuera del
// PATH del doctor, o instalado como app) no se puede afirmar que un plugin
// solo v1 cargue; OpenCode 2.x lo rechaza. Solo la forma dual es segura.
func TestCompatible_VersionDesconocida(t *testing.T) {
	casos := []struct {
		st   OpenCodeInstallStatus
		want bool
	}{
		{OpenCodeInstallStatus{Major: 0, PluginShape: PluginShapeV1Only}, false},
		{OpenCodeInstallStatus{Major: 1, PluginShape: PluginShapeV1Only}, true},
		{OpenCodeInstallStatus{Major: 2, PluginShape: PluginShapeV1Only}, false},
		{OpenCodeInstallStatus{Major: 0, PluginShape: PluginShapeDual}, true},
		{OpenCodeInstallStatus{Major: 2, PluginShape: PluginShapeMissing}, false},
	}
	for _, c := range casos {
		if got := c.st.Compatible(); got != c.want {
			t.Errorf("Compatible(major=%d, shape=%s) = %v; want %v", c.st.Major, c.st.PluginShape, got, c.want)
		}
	}
}
