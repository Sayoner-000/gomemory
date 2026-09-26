package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FR-005 (feature 034): el brazo extensor de spec-kit usa el `mem` del PATH y
// solo recurre a ./mem sin global. Antes buscaba ./mem primero y ejecutaba
// copias por proyecto que se quedaban en versiones viejas.
func TestSpeckitExtensionScripts_PrefierenElMemDelPath(t *testing.T) {
	base := filepath.Join("..", "..", "..", "infrastructure", "templates", "gomemory-context", "extension", "scripts")
	cases := []struct {
		file, path, local string
	}{
		{filepath.Join(base, "bash", "update-gomemory-context.sh"), "command -v mem", `"$PROJECT_ROOT/mem"`},
		{filepath.Join(base, "powershell", "update-gomemory-context.ps1"), "Get-Command 'mem'", "Join-Path $ProjectRoot $candidate"},
	}
	for _, c := range cases {
		data, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatalf("leer %s: %v", c.file, err)
		}
		s := string(data)
		ip, il := strings.Index(s, c.path), strings.Index(s, c.local)
		if ip < 0 || il < 0 {
			t.Fatalf("%s: no encontré la búsqueda por PATH (%d) o la local (%d)", c.file, ip, il)
		}
		if ip > il {
			t.Errorf("%s: debe buscar `mem` en el PATH antes que la copia local", filepath.Base(c.file))
		}
	}
}
