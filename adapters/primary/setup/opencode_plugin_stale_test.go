package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPluginOpenCode_Desactualizado(t *testing.T) {
	previo := PluginFS
	t.Cleanup(func() { PluginFS = previo })
	PluginFS = fstest.MapFS{"plugin/opencode/gomemory.ts": &fstest.MapFile{Data: []byte("const BIN = \"{{BIN_PATH}}\";\nexport const nuevo = 1;\n")}}

	path := filepath.Join(t.TempDir(), "gomemory.ts")
	inspector := NewActivationInspector()
	if err := os.WriteFile(path, []byte("const BIN = \"/x/mem\";\nexport const nuevo = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := inspector.openCodePluginStale(path); p != "" {
		t.Fatalf("plugin vigente con otra ruta de binario: %s", p)
	}
	if err := os.WriteFile(path, []byte("const BIN = \"mem\";\nexport const viejo = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := inspector.openCodePluginStale(path); !strings.Contains(p, "mem setup-mcp --scope global") {
		t.Fatalf("problema = %q", p)
	}
}

func TestPluginOpenCode_V1CompatibleDesactualizado(t *testing.T) {
	previo := PluginFS
	t.Cleanup(func() { PluginFS = previo })
	PluginFS = fstest.MapFS{"plugin/opencode/gomemory.ts": &fstest.MapFile{Data: []byte("const BIN = \"{{BIN_PATH}}\";\nexport const nuevo = 1;\n")}}
	path := filepath.Join(t.TempDir(), "gomemory.ts")
	if err := os.WriteFile(path, []byte("export const GomemoryPlugin = 1;\nexport const viejo = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if forma := DetectPluginShape(path); forma != PluginShapeV1Only {
		t.Skipf("la prueba necesita un plugin solo v1, forma = %s", forma)
	}
	inspector := NewActivationInspector()
	inspector.openCodeVersion = func() (int, string) { return 1, "1.17.0" }
	if p := inspector.openCodePluginProblem(path); !strings.Contains(p, "desactualizado") {
		t.Fatalf("problema = %q", p)
	}
}
