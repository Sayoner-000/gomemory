package cli

import (
	"os"
	"path/filepath"
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
