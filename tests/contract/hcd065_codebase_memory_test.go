package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// HCD-065: codegraph es el único grafo de código externo de gomemory. Ningún
// archivo vigente nombra al proveedor retirado (codebase-memory-mcp, su
// adaptador codebasememory ni sus scripts cbm-*), salvo lo que es registro
// histórico, datos medidos o la prueba de que un ajuste heredado se ignora.
var vestigioCodebaseMemory = regexp.MustCompile(`(?i)codebase[-_]?memory|\bcbm-[a-z]`)

// excepcionesHCD065 son rutas (archivo, o prefijo de directorio terminado en
// "/") que pueden conservar el término. Cada una debe existir y, si es un
// archivo, seguir conteniéndolo: una excepción que ya no aplica se retira.
var excepcionesHCD065 = []string{
	// Registro histórico.
	"specs/",
	"hcd/",
	"CHANGELOG.md",
	"docs/audits/",
	"docs/lessons.md",
	"tasks/todo.md",
	// Datos medidos o capturados: cambiarlos falsea mediciones o migraciones.
	"tests/testdata/compression_corpus/",
	"tests/integration/testdata/golden_structural/",
	"tests/contract/testdata/",
	"tests/fixtures/038/protocol-v8.md",
	// Compatibilidad con settings.json heredados (Q1).
	"adapters/secondary/codegraph/providers_test.go",
	"adapters/secondary/persistence/settings_test.go",
	"adapters/primary/setup/activation_inspect_test.go",
	"tests/integration/code_graph_multi_provider_test.go",
	// Esta misma prueba.
	"tests/contract/hcd065_codebase_memory_test.go",
}

func repoRootHCD065(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func exceptuadoHCD065(path string) bool {
	for _, e := range excepcionesHCD065 {
		if path == e || (strings.HasSuffix(e, "/") && strings.HasPrefix(path, e)) {
			return true
		}
	}
	return false
}

// areaHCD065 clasifica un archivo vigente en el área que lo limpia.
func areaHCD065(path string) string {
	switch {
	case strings.HasPrefix(path, "infrastructure/templates/"), strings.HasPrefix(path, "skills/"), strings.HasPrefix(path, "plugin/"):
		return "plantillas"
	case strings.HasPrefix(path, "scripts/"):
		return "scripts"
	case strings.HasSuffix(path, "_test.go"), strings.HasPrefix(path, "tests/"):
		return "tests"
	case strings.HasSuffix(path, ".go"):
		return "produccion"
	default:
		return "docs"
	}
}

func TestHCD065_SinVestigiosDeCodebaseMemory(t *testing.T) {
	root := repoRootHCD065(t)
	out, err := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		t.Skipf("sin git no se puede enumerar el repositorio: %v", err)
	}
	hallazgos := map[string][]string{}
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" || exceptuadoHCD065(path) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			continue // borrado en el árbol de trabajo
		}
		for n, line := range strings.Split(string(data), "\n") {
			if vestigioCodebaseMemory.MatchString(line) {
				area := areaHCD065(path)
				hallazgos[area] = append(hallazgos[area], path+":"+strconv.Itoa(n+1))
			}
		}
	}
	for _, area := range []string{"plantillas", "produccion", "tests", "scripts", "docs"} {
		t.Run(area, func(t *testing.T) {
			for _, path := range hallazgos[area] {
				t.Errorf("%s nombra al proveedor retirado codebase-memory-mcp", path)
			}
		})
	}
	t.Run("excepciones", func(t *testing.T) {
		for _, e := range excepcionesHCD065 {
			full := filepath.Join(root, e)
			info, err := os.Stat(full)
			if err != nil {
				t.Errorf("excepción %s ya no existe: retírala de la lista", e)
				continue
			}
			if info.IsDir() {
				continue
			}
			data, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			if !vestigioCodebaseMemory.Match(data) {
				t.Errorf("excepción %s ya no contiene el término: retírala de la lista", e)
			}
		}
	})
}
