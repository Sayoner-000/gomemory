package main

import (
	"os"
	"strings"
	"testing"
)

// TestPluginOpenCode_MarcadorSoloDondeSeSustituye protege una regresión sutil:
// InstallPlugin sustituye {{BIN_PATH}} con un reemplazo de texto plano sobre
// TODO el archivo. Cualquier aparición del marcador fuera de la línea que
// realmente define el binario —por ejemplo, un comentario que lo mencione— se
// sustituye también y deja texto sin sentido en el plugin ya instalado.
//
// La regla, por tanto: el marcador aparece exactamente una vez, y en la línea
// de la constante.
func TestPluginOpenCode_MarcadorSoloDondeSeSustituye(t *testing.T) {
	const ruta = "../../infrastructure/plugin/opencode/gomemory.ts"
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}

	const marcador = "{{BIN_PATH}}"
	if n := strings.Count(string(datos), marcador); n != 1 {
		t.Fatalf("%s aparece %d veces en el plugin; debe aparecer exactamente 1 (solo en la constante BIN)", marcador, n)
	}

	for _, linea := range strings.Split(string(datos), "\n") {
		if !strings.Contains(linea, marcador) {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(linea), "const BIN") {
			t.Errorf("el marcador está fuera de la constante BIN, en: %q", strings.TrimSpace(linea))
		}
	}
}
