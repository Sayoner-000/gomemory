package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// La constitución (§3) exige que las reglas de dependencia entre capas se
// verifiquen en CI. Hasta ahora se cumplían solo por disciplina: nada impedía
// que un caso de uso importara persistence o que un adaptador primario sumara
// otro secundario (hallazgo H-01 de la evaluación 043).
//
// El grafo se obtiene parseando solo las importaciones del código de
// producción, sin lanzar `go list`: no depende del toolchain en PATH y une
// las variantes por plataforma (_unix/_windows), que es lo más estricto.

// primaryToSecondary son los cruces de adaptador primario a secundario que
// existen hoy, cada uno con su motivo. Añadir uno nuevo hace fallar la prueba:
// o se justifica aquí, o la dependencia va por un port. Quitar el uso hace
// fallar también, para que la lista solo pueda encoger.
var primaryToSecondary = map[string]string{
	"adapters/primary/cli -> adapters/secondary/atomicfs":      "escritura atómica de archivos de configuración del proyecto",
	"adapters/primary/cli -> adapters/secondary/persistence":   "rutas del almacén (MemDir, DataHome) y operaciones entre proyectos de install/uninstall/doctor",
	"adapters/primary/cli -> adapters/secondary/release":       "adaptador por defecto del aviso de versión cuando Deps no lo trae",
	"adapters/primary/setup -> adapters/secondary/configpath":  "resolución de rutas de configuración de cada agente",
	"adapters/primary/setup -> adapters/secondary/persistence": "lectura de settings del proyecto al cablear hooks",
}

func TestLayerRules_CodigoDeProduccion(t *testing.T) {
	graph := productionImports(t, repoRootDir(t))
	if len(graph) == 0 {
		t.Fatal("no se encontró ningún paquete de producción: el recorrido está roto")
	}
	for _, v := range layerViolations(graph, primaryToSecondary) {
		t.Error(v)
	}
}

// TestLayerRules_DetectaViolaciones prueba la regla con un grafo sintético:
// sin esto, una regla que nunca falla pasaría por buena.
func TestLayerRules_DetectaViolaciones(t *testing.T) {
	graph := map[string][]string{
		"domain":                      {"application/ports"},
		"application/ports":           {"domain"},
		"application/usecases":        {"adapters/secondary/persistence"},
		"adapters/primary/tui":        {"adapters/secondary/generator"},
		"adapters/secondary/launcher": {"adapters/primary/cli", "infrastructure"},
		"adapters/primary/cli/review": {"adapters/primary/cli", "adapters/primary/cli/socratica", "adapters/primary/cli/clikit"},
		"adapters/primary/cli/clikit": {"adapters/primary/cli/review"},
	}
	allowed := map[string]string{"adapters/primary/cli -> adapters/secondary/release": "obsoleta"}

	got := strings.Join(layerViolations(graph, allowed), "\n")
	for _, want := range []string{
		"L1: domain importa application/ports",
		"L2: application/usecases importa adapters/secondary/persistence",
		"L3: adapters/primary/tui importa adapters/secondary/generator",
		"L3: adapters/secondary/launcher importa adapters/primary/cli",
		"L3: adapters/secondary/launcher importa infrastructure",
		"excepción obsoleta: adapters/primary/cli -> adapters/secondary/release",
		"L3: adapters/primary/cli/review importa adapters/primary/cli (",
		"L3: adapters/primary/cli/review importa adapters/primary/cli/socratica",
		"L3: adapters/primary/cli/clikit importa adapters/primary/cli/review",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta la violación %q en:\n%s", want, got)
		}
	}
	if strings.Contains(got, "importa adapters/primary/cli/clikit") {
		t.Errorf("un slice puede importar clikit:\n%s", got)
	}
	if n := len(layerViolations(map[string][]string{"application/usecases": {"domain", "application/ports"}}, nil)); n != 0 {
		t.Errorf("un grafo correcto no debe tener violaciones, tiene %d", n)
	}
}

// layerViolations aplica L1–L3 a un grafo paquete → importaciones internas,
// con las rutas sin el prefijo del módulo.
func layerViolations(graph map[string][]string, allowed map[string]string) []string {
	var out []string
	used := map[string]bool{}
	for _, pkg := range slices.Sorted(maps.Keys(graph)) {
		for _, imp := range graph[pkg] {
			edge := pkg + " -> " + imp
			switch {
			case pkg == "domain":
				out = append(out, "L1: "+pkg+" importa "+imp+" (domain solo depende de la librería estándar)")
			case strings.HasPrefix(pkg, "application/") && !allowedInApplication(pkg, imp):
				out = append(out, "L2: "+pkg+" importa "+imp+" (application solo depende de domain y de sus ports)")
			case strings.HasPrefix(pkg, "adapters/") && imp == "infrastructure":
				out = append(out, "L3: "+pkg+" importa "+imp+" (el composition root no se importa)")
			case strings.HasPrefix(pkg, cliPkg+"/") && strings.HasPrefix(imp+"/", cliPkg+"/") && !cliSubpackageMayImport(pkg, imp):
				out = append(out, "L3: "+pkg+" importa "+imp+" (un slice de cli/ solo comparte clikit, y clikit no importa ningún paquete de cli/)")
			case strings.HasPrefix(pkg, "adapters/secondary/") && strings.HasPrefix(imp, "adapters/primary/"):
				out = append(out, "L3: "+pkg+" importa "+imp+" (un adaptador secundario no depende de uno primario)")
			case strings.HasPrefix(pkg, "adapters/primary/") && strings.HasPrefix(imp, "adapters/secondary/"):
				if _, ok := allowed[edge]; ok {
					used[edge] = true
				} else {
					out = append(out, "L3: "+pkg+" importa "+imp+" (justifícalo en primaryToSecondary o usa un port)")
				}
			}
		}
	}
	for _, edge := range slices.Sorted(maps.Keys(allowed)) {
		if !used[edge] {
			out = append(out, "excepción obsoleta: "+edge+" (ya no se usa; quítala de primaryToSecondary)")
		}
	}
	return out
}

// cliPkg agrupa las entradas de la CLI. Cada slice que se extrae vive en
// cli/<slice> y comparte con el resto solo cli/clikit: si un slice importara
// cli u otro slice, volvería a acoplarse a todo el paquete (paso 4 de 043).
const cliPkg = "adapters/primary/cli"

// cliSubpackageMayImport decide una importación entre paquetes de cli/ hecha
// desde un subpaquete.
func cliSubpackageMayImport(pkg, imp string) bool {
	kit := cliPkg + "/clikit"
	if imp == cliPkg || pkg == kit || strings.HasPrefix(pkg, kit+"/") {
		return false
	}
	return imp == kit || cliSlice(imp) == cliSlice(pkg)
}

// cliSlice devuelve el primer segmento bajo cli/.
func cliSlice(pkg string) string {
	rest := strings.TrimPrefix(pkg, cliPkg+"/")
	slice, _, _ := strings.Cut(rest, "/")
	return slice
}

func allowedInApplication(pkg, imp string) bool {
	if imp == "domain" {
		return true
	}
	return pkg != "application/ports" && imp == "application/ports"
}

// productionImports recorre el repositorio y devuelve, por paquete, las
// importaciones del propio módulo que hace su código de producción.
func productionImports(t *testing.T, root string) map[string][]string {
	t.Helper()
	module := modulePath(t, root)
	sets := map[string]map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(rel)
		if sets[pkg] == nil {
			sets[pkg] = map[string]bool{}
		}
		for _, spec := range file.Imports {
			imp, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if internal, ok := strings.CutPrefix(imp, module+"/"); ok && internal != pkg {
				sets[pkg][internal] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorrer el código: %v", err)
	}
	graph := make(map[string][]string, len(sets))
	for pkg, imps := range sets {
		graph[pkg] = slices.Sorted(maps.Keys(imps))
	}
	return graph
}

func modulePath(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("leer go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(mod)
		}
	}
	t.Fatal("go.mod sin directiva module")
	return ""
}
