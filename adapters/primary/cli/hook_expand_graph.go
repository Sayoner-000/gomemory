package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// codeGraphExtensions son las extensiones de los lenguajes que indexa
// CodeGraph. La documentación y la configuración (.md, .json, .toml) quedan
// fuera: buscar en ellas con grep es legítimo (HCD-027, Q2).
var codeGraphExtensions = []string{
	".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".java", ".kt",
	".rs", ".rb", ".php", ".cs", ".c", ".h", ".cpp", ".hpp", ".cc", ".swift",
	".scala", ".dart", ".lua", ".ex", ".exs", ".vue", ".svelte",
}

// graphIndexedExtensions devuelve las extensiones que cubre un grafo de código
// del proyecto (HCD-027, Q2). Un índice .codegraph/ en la raíz cubre sus
// lenguajes; si no hay, el índice propio de gomemory con archivos cubre Go;
// sin ninguno, nada. ownFiles solo se consulta si hace falta: el hook de
// expand corre por una ruta ligera y abrir la base tiene coste.
func graphIndexedExtensions(root string, ownFiles func() int) []string {
	if info, err := os.Stat(filepath.Join(root, ".codegraph")); err == nil && info.IsDir() {
		return codeGraphExtensions
	}
	if ownFiles != nil && ownFiles() > 0 {
		return []string{".go"}
	}
	return nil
}

// coverageWalkLimit acota cuántas entradas mira graphCoverage para decidir si
// un directorio contiene código indexado: el hook corre con un plazo corto.
const coverageWalkLimit = 4000

// coverageSkipDirs no se recorren al buscar código: son dependencias,
// artefactos o metadatos, no código del proyecto.
var coverageSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".codegraph": true,
	"dist": true, "build": true, ".venv": true, "__pycache__": true, "target": true,
}

// newGraphCoverage devuelve la función que dice qué grafo cubre una ruta de un
// comando (domain.RuleInput.GraphCoverage). Cubre una ruta dentro de root el
// grafo del proyecto; fuera de root, el `.codegraph/` más cercano hacia arriba.
// Para un archivo devuelve las extensiones solo si es de una indexada; para un
// directorio, solo si contiene algún archivo indexado (recorrido acotado). Una
// ruta inexistente no está cubierta. Los resultados se recuerdan durante el
// hook.
func newGraphCoverage(root string, rootExts func() []string) func(string) []string {
	cache := map[string][]string{}
	var rootCache []string
	rootLoaded := false
	return func(p string) []string {
		abs := expandHome(p)
		if !filepath.IsAbs(abs) {
			var err error
			if abs, err = filepath.Abs(abs); err != nil {
				return nil
			}
		}
		abs = filepath.Clean(abs)
		if v, ok := cache[abs]; ok {
			return v
		}
		info, err := os.Stat(abs)
		if err != nil {
			cache[abs] = nil
			return nil
		}
		var exts []string
		if root != "" && withinDir(abs, root) {
			if !rootLoaded {
				rootCache, rootLoaded = rootExts(), true
			}
			exts = rootCache
		} else {
			start := abs
			if !info.IsDir() {
				start = filepath.Dir(abs)
			}
			if findUp(start, ".codegraph") {
				exts = codeGraphExtensions
			}
		}
		if len(exts) > 0 {
			if info.IsDir() {
				if !dirHasExt(abs, exts) {
					exts = nil
				}
			} else if !slices.Contains(exts, strings.ToLower(filepath.Ext(abs))) {
				exts = nil
			}
		}
		cache[abs] = exts
		return exts
	}
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func withinDir(p, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// findUp dice si name existe como directorio en dir o en algún ancestro.
func findUp(dir, name string) bool {
	for {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.IsDir() {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// dirHasExt dice si dir contiene algún archivo con una de exts, sin mirar más
// de coverageWalkLimit entradas.
func dirHasExt(dir string, exts []string) bool {
	seen, found := 0, false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		seen++
		if seen > coverageWalkLimit {
			return fs.SkipAll
		}
		if d.IsDir() {
			if p != dir && coverageSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if slices.Contains(exts, strings.ToLower(filepath.Ext(d.Name()))) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// ownIndexFiles cuenta los archivos del índice propio de gomemory, si el
// Deps lo trae; en la ruta ligera del hook no está y cuenta cero.
func ownIndexFiles(deps *Deps) func() int {
	return func() int {
		if deps == nil || deps.CodeGraphRepo == nil {
			return 0
		}
		st, err := deps.CodeGraphRepo.Status(deps.Project)
		if err != nil {
			return 0
		}
		return st.Files
	}
}
