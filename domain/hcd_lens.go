package domain

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// Lentes deterministas de analyze (HCD-034): detectan sin modelo lo que una
// lente de revisión encontraría. Son funciones puras; la existencia de las
// rutas la consulta el caso de uso a través del port.

var (
	backtickSpan = regexp.MustCompile("`([^`\n]+)`")
	// lineSuffix acepta «:12», «:12:3», rangos «:152-159» y listas «:160,190».
	lineSuffix = regexp.MustCompile(`:\d+([-,]\d+)*(:\d+)?$`)
)

// pathExtensions son las extensiones que bastan para reconocer un archivo sin
// barra («README.md»). Sin ellas, «HCDState.Analyze» parecería un archivo.
var pathExtensions = map[string]bool{
	".go": true, ".md": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".java": true, ".kt": true, ".rs": true, ".json": true, ".yaml": true,
	".yml": true, ".toml": true, ".sql": true, ".sh": true, ".html": true, ".css": true,
	".mod": true, ".sum": true, ".txt": true, ".golden": true, ".proto": true, ".tmpl": true,
}

// pathLike reconoce un token que nombra un archivo o directorio: con una
// extensión conocida, con «./» delante, con «/» final o con dos barras o más.
// «CLI/MCP» o «n/a» no cuentan.
func pathLike(token string) bool {
	token = lineSuffix.ReplaceAllString(token, "")
	if token == "" || strings.ContainsAny(token, " *?<>{}$|\\") || strings.Contains(token, "://") || strings.Contains(token, "...") {
		return false
	}
	return pathExtensions[path.Ext(token)] || strings.HasPrefix(token, "./") || strings.HasSuffix(token, "/") || strings.Count(token, "/") >= 2
}

// CitedPaths lista, ordenadas y sin repetir, las rutas relativas citadas entre
// comillas invertidas en el documento. Omite la Evidencia (la escribe el
// motor), los comentarios HTML, las rutas absolutas y las que salen de la raíz.
func CitedPaths(doc UnitDoc) []string {
	seen := map[string]bool{}
	for name, body := range doc.Sections {
		if name == SectionEvidence || name == SectionNext {
			continue
		}
		addCitedPaths(seen, body)
	}
	return sortedKeys(seen)
}

// addCitedPaths añade a seen las rutas relativas citadas entre comillas
// invertidas en text, sin sufijo de línea.
func addCitedPaths(seen map[string]bool, text string) {
	for _, m := range backtickSpan.FindAllStringSubmatch(stripComments(text), -1) {
		token := strings.TrimSpace(m[1])
		// Las rutas absolutas y las del HOME («~/…») no son del repositorio.
		if !pathLike(token) || strings.HasPrefix(token, "/") || strings.HasPrefix(token, "~") {
			continue
		}
		clean := path.Clean(lineSuffix.ReplaceAllString(token, ""))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			continue
		}
		seen[clean] = true
	}
}

func sortedKeys(seen map[string]bool) []string {
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// TaskNamesFile informa si el texto o la comprobación de la tarea nombran un
// archivo o directorio.
func TaskNamesFile(t Task) bool {
	for _, field := range strings.Fields(t.Text + " " + t.Check) {
		if pathLike(strings.TrimRight(strings.Trim(field, "`()[],;«»\"'"), ".:")) {
			return true
		}
	}
	return false
}

var cheapestFiller = map[string]bool{
	"ninguna": true, "ninguno": true, "no hay": true, "n/a": true, "na": true,
	"no aplica": true, "nada": true, "-": true, "no": true, "none": true,
}

// IsCheapestFiller informa si la Alternativa más barata es una negación
// escueta, sin motivo (Q2). Vacío o «pendiente» lo cubre DOC-MISSING.
func IsCheapestFiller(text string) bool {
	s := strings.ToLower(strings.TrimSpace(stripComments(text)))
	s = strings.TrimSpace(strings.TrimRight(s, ".!"))
	return cheapestFiller[s]
}
