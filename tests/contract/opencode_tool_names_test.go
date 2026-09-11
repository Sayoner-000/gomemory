package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestOpenCodeToolConstantsEmparejanNombreYValor: gomemory.ts declara los
// nombres de tool con destructuración posicional (const [T_A, T_B] = ["a",
// "b"]). Si se añade, quita o reordena un elemento solo en un lado, cada
// constante siguiente apunta a otra tool sin que nada falle (ya pasó: un valor
// huérfano en el bloque principal). TestOpenCodeProtocolNombraTodasLasToolsPrefijadas
// solo comprueba que cada string exista; este test fija el emparejamiento:
// T_<X> ↔ "gomemory_<x>" y T_EXT_<X> ↔ "codebase-memory-mcp_<x>".
func TestOpenCodeToolConstantsEmparejanNombreYValor(t *testing.T) {
	ruta := filepath.Join(repoRootContract(t), "infrastructure", "plugin", "opencode", "gomemory.ts")
	contenido, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}

	bloques := regexp.MustCompile(`(?s)const \[([^\]]*)\]\s*=\s*\[([^\]]*)\];`).FindAllStringSubmatch(string(contenido), -1)
	if len(bloques) < 5 {
		t.Fatalf("esperaba al menos 5 bloques de constantes de tool, encontré %d: ¿cambió el formato de gomemory.ts?", len(bloques))
	}

	literal := regexp.MustCompile(`"([^"]*)"`)
	for _, b := range bloques {
		var nombres []string
		for _, n := range strings.Split(b[1], ",") {
			if n = strings.TrimSpace(n); n != "" {
				nombres = append(nombres, n)
			}
		}
		var valores []string
		for _, m := range literal.FindAllStringSubmatch(b[2], -1) {
			valores = append(valores, m[1])
		}
		if len(nombres) != len(valores) {
			t.Errorf("bloque desparejado: %d constantes y %d valores\nconstantes: %v\nvalores: %v", len(nombres), len(valores), nombres, valores)
			continue
		}
		for i, nombre := range nombres {
			esperado := "gomemory_" + strings.ToLower(strings.TrimPrefix(nombre, "T_"))
			if ext, ok := strings.CutPrefix(nombre, "T_EXT_"); ok {
				esperado = "codebase-memory-mcp_" + strings.ToLower(ext)
			}
			if valores[i] != esperado {
				t.Errorf("%s = %q, esperaba %q", nombre, valores[i], esperado)
			}
		}
	}
}
