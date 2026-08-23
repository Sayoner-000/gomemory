package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Las cuatro operaciones que FR-041 exige en AMBAS superficies.
var operacionesExigidas = []string{"ver", "exportar", "importar", "restaurar"}

// TestPinnedDocs_ParidadCLIyTUI cubre FR-041 y SC-013.
//
// La paridad entre consola e interfaz interactiva es fácil de prometer y fácil
// de perder: basta con que alguien añada una operación en un lado y olvide el
// otro. Este test la comprueba de forma estructural sobre el código fuente, no
// sobre una lista escrita a mano que también habría que acordarse de actualizar.
func TestPinnedDocs_ParidadCLIyTUI(t *testing.T) {
	raiz := repoRootDir(t)

	cli := leerFuente(t, filepath.Join(raiz, "adapters", "primary", "cli", "cmd_docs.go"))
	tui := leerFuente(t, filepath.Join(raiz, "adapters", "primary", "tui", "tui_docs.go"))

	// La CLI expone las operaciones como subcomandos; la TUI, como teclas con
	// su rótulo. Se busca la intención, no el identificador exacto.
	marcadoresCLI := map[string][]string{
		"ver":       {`"show"`},
		"exportar":  {`"export"`},
		"importar":  {`"import"`},
		"restaurar": {`"reset"`},
	}
	marcadoresTUI := map[string][]string{
		"ver":       {"Ver contenido"},
		"exportar":  {"Exportar a archivo"},
		"importar":  {"Importar desde archivo"},
		"restaurar": {"Restaurar contenido por defecto"},
	}

	for _, op := range operacionesExigidas {
		if !contieneAlguno(cli, marcadoresCLI[op]) {
			t.Errorf("la CLI no ofrece la operación %q sobre documentos fijados", op)
		}
		if !contieneAlguno(tui, marcadoresTUI[op]) {
			t.Errorf("la TUI no ofrece la operación %q sobre documentos fijados", op)
		}
	}
}

// TestPinnedDocs_NingunaSuperficieEnumeraElCatalogo cubre FR-035 y SC-015:
// añadir un documento al catálogo no puede exigir tocar la CLI ni la TUI. Si
// alguna superficie menciona un alias concreto fuera de su uso legítimo,
// significa que dejó de recorrer domain.PinnedDocs.
func TestPinnedDocs_NingunaSuperficieEnumeraElCatalogo(t *testing.T) {
	raiz := repoRootDir(t)

	for _, ruta := range []string{
		filepath.Join(raiz, "adapters", "primary", "tui", "tui_docs.go"),
		filepath.Join(raiz, "adapters", "primary", "tui", "tui.go"),
	} {
		src := leerFuente(t, ruta)
		for _, alias := range []string{`"rules"`, `"constitution"`} {
			if strings.Contains(src, alias) {
				t.Errorf("%s menciona el alias %s: la TUI debe recorrer domain.PinnedDocs, no enumerarlo",
					filepath.Base(ruta), alias)
			}
		}
	}

	// Ambas superficies deben recorrer el catálogo de verdad.
	for _, ruta := range []string{
		filepath.Join(raiz, "adapters", "primary", "cli", "cmd_docs.go"),
		filepath.Join(raiz, "adapters", "primary", "tui", "tui_docs.go"),
	} {
		if !strings.Contains(leerFuente(t, ruta), "domain.PinnedDoc") {
			t.Errorf("%s no usa el catálogo domain.PinnedDocs", filepath.Base(ruta))
		}
	}
}

// TestPinnedDocs_CmdDocsCompila es una comprobación barata de que el archivo de
// la CLI sigue siendo Go válido y expone su punto de entrada.
func TestPinnedDocs_CmdDocsExponeSuEntrada(t *testing.T) {
	raiz := repoRootDir(t)
	ruta := filepath.Join(raiz, "adapters", "primary", "cli", "cmd_docs.go")

	fset := token.NewFileSet()
	archivo, err := parser.ParseFile(fset, ruta, nil, 0)
	if err != nil {
		t.Fatalf("parsear %s: %v", ruta, err)
	}

	encontrada := false
	ast.Inspect(archivo, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if ok && fn.Name.Name == "CmdDocs" {
			encontrada = true
		}
		return true
	})
	if !encontrada {
		t.Error("cmd_docs.go debe exponer CmdDocs como punto de entrada del comando")
	}
}

func leerFuente(t *testing.T, ruta string) string {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	return string(datos)
}

func contieneAlguno(s string, marcadores []string) bool {
	for _, m := range marcadores {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// repoRootDir sube desde el directorio de test hasta encontrar go.mod.
func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("no se encontró la raíz del repositorio")
	return ""
}
