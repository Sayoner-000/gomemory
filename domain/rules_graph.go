package domain

import (
	"maps"
	"path"
	"strings"
)

// GraphBeforeGrepRule es el nombre de la regla de serie que exige consultar el
// código con el grafo en lugar de buscarlo por shell (HCD-027).
const GraphBeforeGrepRule = "graph-before-grep"

// searchTypeExtensions traduce los tipos de rg (-t go) y ag/ack (--go) a la
// extensión que indexan los grafos.
var searchTypeExtensions = map[string]string{
	"go": ".go", "ts": ".ts", "typescript": ".ts", "js": ".js", "javascript": ".js",
	"py": ".py", "python": ".py", "java": ".java", "rust": ".rs", "ruby": ".rb",
	"php": ".php", "kotlin": ".kt", "swift": ".swift", "cs": ".cs", "csharp": ".cs",
	"c": ".c", "cpp": ".cpp", "yaml": ".yaml",
}

// evalGraphBeforeGrep bloquea una búsqueda de código por shell cuando apunta,
// por filtro o por ruta, a una extensión que un grafo del proyecto indexa
// (HCD-027, Q1 y Q2). Decide sobre tokens del lexer, como destructive-shell:
// `echo "grep …"` no es un grep. Un comando que lee de una tubería filtra
// salida, no busca código, y se deja pasar.
//
// La cobertura sale de dos fuentes: el grafo del proyecto de la sesión
// (IndexedExtensions) y el que cubre cada ruta que el comando busca
// (GraphCoverage), para que un agente en el proyecto A no lea por shell el
// código indexado del proyecto B. Una búsqueda recursiva sin filtro sobre un
// directorio con código indexado también cuenta: en los runtimes con búsqueda
// nativa se bloquea; en los demás, se avisa.
func evalGraphBeforeGrep(in RuleInput) (RuleAction, string) {
	base := map[string]bool{}
	for _, e := range in.IndexedExtensions {
		base[strings.ToLower(e)] = true
	}
	if len(base) == 0 && in.GraphCoverage == nil {
		return RuleAllow, ""
	}
	dir := ""
	for _, c := range splitShell(in.Command, 0) {
		if c.Nested {
			continue
		}
		toks := unwrapCommand(stripEnvAssignments(c.Tokens))
		if len(toks) == 0 {
			continue
		}
		// `cd X; grep -rn foo adapters`: las rutas relativas siguientes se
		// resuelven contra X.
		if toks[0] == "cd" {
			if len(toks) > 1 {
				dir = joinShellPath(dir, toks[1])
			}
			continue
		}
		if c.PipeBack > 0 {
			continue
		}
		name := path.Base(toks[0])
		paths, implicit := searchPaths(name, toks[1:])
		// Un heredoc o here-string solo convierte la búsqueda en un filtro
		// de esa entrada si no hay rutas explícitas (ADM 3).
		if len(c.Stdin) > 0 && (len(paths) == 0 || implicit) {
			continue
		}
		// `git -C repo grep …` resuelve sus rutas contra repo (ADM 4).
		from := dir
		if name == "git" {
			from = gitCDir(dir, toks[1:])
		}
		// Las extensiones del proyecto de la sesión solo valen si alguna ruta
		// es relativa a él; para rutas externas manda su propia cobertura
		// (ADM 1). Sin GraphCoverage no hay otra fuente y se usa la base.
		indexed := map[string]bool{}
		useBase := in.GraphCoverage == nil || len(paths) == 0
		var covered []string
		for _, p := range paths {
			full := joinShellPath(from, p)
			if !externalPath(full) {
				useBase = true
			}
			if in.GraphCoverage == nil {
				continue
			}
			exts := in.GraphCoverage(full)
			if len(exts) == 0 {
				continue
			}
			covered = append(covered, full)
			for _, e := range exts {
				indexed[strings.ToLower(e)] = true
			}
		}
		if useBase {
			maps.Copy(indexed, base)
		}
		if len(indexed) == 0 {
			continue
		}
		if tool, ext, ok := codeSearchTarget(toks, indexed); ok {
			return RuleBlock, "buscar código " + ext + " con " + tool + " cuando el grafo ya lo indexa; " +
				"usa codegraph_explore (o search_graph de codebase-memory-mcp, o search_code/get_symbol de gomemory), " +
				"que devuelve el código con sus llamadores y su radio de impacto"
		}
		if len(covered) > 0 && recursiveSearch(name, toks[1:]) && !hasFileFilter(toks[1:]) {
			tool := name
			if name == "git" {
				tool = "git grep"
			}
			if !in.HasNativeReadTools() {
				return RuleWarn, "buscar con " + tool + " en " + covered[0] + ", que un grafo de código indexa; " +
					"para código usa codegraph_explore o search_code/get_symbol de gomemory"
			}
			return RuleBlock, "buscar con " + tool + " en " + covered[0] + ", que un grafo de código indexa; " +
				"usa codegraph_explore (o search_code/get_symbol de gomemory) para el código, " +
				"o la herramienta nativa " + in.nativeGrepTool() + " para texto"
		}
	}
	return RuleAllow, ""
}

// joinShellPath resuelve p contra el directorio de un `cd` previo de la misma
// línea. Sin cd, o con p absoluta o con ~, p queda igual.
func joinShellPath(dir, p string) string {
	p = strings.Trim(p, `'"`)
	if dir == "" || p == "" || absolutePath(p) || strings.HasPrefix(p, "~") {
		return p
	}
	return path.Join(dir, p)
}

// absolutePath reconoce rutas absolutas de Unix y de Windows: con letra de
// unidad (`C:/x`, `C:\x`) o UNC (`\\srv\share`).
func absolutePath(p string) bool {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\\`) {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') &&
		((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z'))
}

// externalPath dice si una ruta ya resuelta apunta fuera del directorio de la
// sesión: absoluta (también de Windows), del home o que sube por encima de él.
func externalPath(p string) bool {
	return absolutePath(p) || strings.HasPrefix(p, "~") || p == ".." || strings.HasPrefix(p, "../")
}

// gitCDir aplica las opciones `-C dir` de git (acumulables, como en git) al
// directorio de partida.
func gitCDir(dir string, args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-C" && i+1 < len(args) {
			dir = joinShellPath(dir, args[i+1])
			i++
			continue
		}
		if a == "-c" || a == "--git-dir" || a == "--work-tree" || a == "--namespace" || a == "--super-prefix" || a == "--config-env" {
			i++
			continue
		}
		if !strings.HasPrefix(a, "-") {
			break // el subcomando
		}
	}
	return dir
}

// searchValueFlags son las opciones de grep/rg/ag/ack/git grep que llevan su
// valor en el token siguiente: ese valor no es ni el patrón ni una ruta.
var searchValueFlags = map[string]bool{
	"-A": true, "-B": true, "-C": true, "-m": true, "-g": true, "-t": true, "-T": true,
	"-G": true, "-d": true, "-D": true, "-M": true, "-j": true,
	"--glob": true, "--iglob": true, "--type": true, "--type-not": true, "--max-count": true,
	"--include": true, "--exclude": true, "--exclude-dir": true, "--context": true,
	"--after-context": true, "--before-context": true, "--max-depth": true, "--threads": true,
	"--file-search-regex": true, "--ignore-dir": true,
}

// searchPaths devuelve las rutas que busca un comando de búsqueda: los
// argumentos posicionales tras el patrón. Sin ruta, rg/ag/ack/git grep y un
// grep recursivo buscan en el directorio actual, y implicit lo indica. find
// busca en sus rutas iniciales, antes de la primera expresión.
func searchPaths(name string, args []string) (paths []string, implicit bool) {
	switch name {
	case "grep", "egrep", "fgrep", "rg", "ag", "ack":
	case "git":
		sub, rest := gitSubcommandArgs(args)
		if sub != "grep" {
			return nil, false
		}
		args = rest
	case "find":
		var out []string
		for _, a := range args {
			if strings.HasPrefix(a, "-") || a == "(" || a == "!" {
				break
			}
			out = append(out, a)
		}
		if len(out) == 0 {
			return []string{"."}, true
		}
		return out, false
	default:
		return nil, false
	}
	patternPending := true
	afterDashDash := false
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if afterDashDash || !strings.HasPrefix(a, "-") || a == "-" {
			if patternPending && !afterDashDash {
				patternPending = false
				continue
			}
			if a != "-" {
				out = append(out, a)
			}
			continue
		}
		switch {
		case a == "--":
			afterDashDash, patternPending = true, false
		case a == "-e" || a == "-f" || a == "--regexp" || a == "--file":
			patternPending = false
			i++
		case strings.HasPrefix(a, "--regexp=") || strings.HasPrefix(a, "--file="):
			patternPending = false
		case searchValueFlags[a]:
			i++
		}
	}
	if len(out) == 0 && recursiveSearch(name, args) {
		return []string{"."}, true
	}
	return out, false
}

// hasFileFilter dice si la búsqueda declara qué archivos mira (`--include`,
// `-g`, `-t`, `--md`…). Si el filtro apuntara a código indexado,
// codeSearchTarget ya habría bloqueado: lo que queda es una búsqueda de texto
// en archivos que el grafo no cubre, y se deja pasar.
func hasFileFilter(args []string) bool {
	for i, a := range args {
		if a == "--" {
			return false
		}
		for _, f := range []string{"--include", "-g", "--glob", "--iglob", "-t", "--type", "-G", "--file-search-regex"} {
			if v, matched, _ := flagValue(args, i, []string{f}); matched && v != "" {
				return true
			}
		}
		if t, ok := strings.CutPrefix(a, "--"); ok {
			if _, known := searchTypeExtensions[strings.ToLower(t)]; known {
				return true
			}
		}
	}
	return false
}

// recursiveSearch dice si el buscador recorre directorios: rg, ag, ack y git
// grep siempre; grep solo con -r/-R/--recursive/-d recurse.
func recursiveSearch(name string, args []string) bool {
	switch name {
	case "rg", "ag", "ack", "git":
		return true
	case "grep", "egrep", "fgrep":
		for i, a := range args {
			switch {
			case a == "--":
				return false
			case a == "--recursive" || a == "--dereference-recursive" || a == "--directories=recurse":
				return true
			case a == "-d" && i+1 < len(args) && args[i+1] == "recurse":
				return true
			case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR"):
				return true
			}
		}
	}
	return false
}

// codeSearchTarget reconoce grep/egrep/fgrep, rg, ag, ack, git grep y find, y
// devuelve la primera extensión indexada a la que apuntan.
func codeSearchTarget(toks []string, indexed map[string]bool) (tool, ext string, ok bool) {
	name := path.Base(toks[0])
	args := toks[1:]
	switch name {
	case "grep", "egrep", "fgrep":
		ext, ok = grepTarget(args, indexed, []string{"--include"}, nil, true)
	case "rg":
		ext, ok = grepTarget(args, indexed, []string{"-g", "--glob", "--iglob"}, []string{"-t", "--type"}, true)
	case "ag", "ack":
		ext, ok = typeFlagTarget(args, indexed)
		if !ok {
			ext, ok = grepTarget(args, indexed, []string{"-G", "--file-search-regex"}, nil, true)
		}
	case "git":
		sub, rest := gitSubcommandArgs(args)
		if sub != "grep" {
			return "", "", false
		}
		name = "git grep"
		ext, ok = grepTarget(rest, indexed, nil, nil, true)
	case "find":
		ext, ok = findTarget(args, indexed)
	default:
		return "", "", false
	}
	return name, ext, ok
}

// grepTarget recorre los argumentos de un buscador. globFlags llevan un glob
// (`--include=*.go`, `-g '*.go'`), typeFlags un tipo (`-t go`). El primer
// argumento posicional es el patrón salvo que se haya dado con -e/-f.
func grepTarget(args []string, indexed map[string]bool, globFlags, typeFlags []string, firstIsPattern bool) (string, bool) {
	patternPending := firstIsPattern
	afterDashDash := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if afterDashDash || !strings.HasPrefix(a, "-") || a == "-" {
			if patternPending && !afterDashDash {
				patternPending = false
				continue
			}
			if e, ok := indexedExt(a, indexed); ok {
				return e, true
			}
			continue
		}
		if a == "--" {
			// Tras `--` (git grep, rg) solo quedan rutas: el patrón ya pasó o
			// se dio con -e.
			afterDashDash = true
			patternPending = false
			continue
		}
		if a == "-e" || a == "-f" || a == "--regexp" || a == "--file" {
			patternPending = false
			i++
			continue
		}
		if strings.HasPrefix(a, "--regexp=") || strings.HasPrefix(a, "--file=") {
			patternPending = false
			continue
		}
		if v, matched, consumed := flagValue(args, i, globFlags); matched {
			i += consumed
			if e, ok := indexedExt(v, indexed); ok {
				return e, true
			}
			continue
		}
		if v, matched, consumed := flagValue(args, i, typeFlags); matched {
			i += consumed
			if e, ok := searchTypeExtensions[strings.ToLower(v)]; ok && indexed[e] {
				return e, true
			}
		}
	}
	return "", false
}

// typeFlagTarget reconoce los tipos de ag/ack escritos como opción (`--go`).
func typeFlagTarget(args []string, indexed map[string]bool) (string, bool) {
	for _, a := range args {
		if t, ok := strings.CutPrefix(a, "--"); ok {
			if e, known := searchTypeExtensions[strings.ToLower(t)]; known && indexed[e] {
				return e, true
			}
		}
	}
	return "", false
}

// findTarget mira los filtros de nombre de find; sin ellos, find no busca
// código por extensión y se deja pasar.
func findTarget(args []string, indexed map[string]bool) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "-name", "-iname", "-path", "-ipath", "-wholename", "-iwholename", "-regex", "-iregex":
			if e, ok := indexedExt(args[i+1], indexed); ok {
				return e, true
			}
		}
	}
	return "", false
}

// flagValue lee el valor de una opción en sus formas `-g v`, `-gv`,
// `--glob v` y `--glob=v`. consumed es cuántos argumentos extra ocupó.
func flagValue(args []string, i int, flags []string) (value string, matched bool, consumed int) {
	a := args[i]
	for _, f := range flags {
		switch {
		case a == f:
			if i+1 < len(args) {
				return args[i+1], true, 1
			}
			return "", true, 0
		case strings.HasPrefix(f, "--") && strings.HasPrefix(a, f+"="):
			return a[len(f)+1:], true, 0
		case !strings.HasPrefix(f, "--") && len(a) > len(f) && strings.HasPrefix(a, f):
			return a[len(f):], true, 0
		}
	}
	return "", false, 0
}

// indexedExt dice si un glob o ruta termina en una extensión indexada.
// Admite un nivel de llaves: `*.{go,ts}`.
func indexedExt(v string, indexed map[string]bool) (string, bool) {
	v = strings.ToLower(strings.Trim(v, `'"`))
	candidates := []string{v}
	if i, j := strings.Index(v, "{"), strings.LastIndex(v, "}"); i >= 0 && j > i {
		candidates = nil
		for _, alt := range strings.Split(v[i+1:j], ",") {
			candidates = append(candidates, v[:i]+alt+v[j+1:])
		}
	}
	for _, c := range candidates {
		c = strings.TrimRight(c, "$)")
		if e := path.Ext(c); e != "" && indexed[e] {
			return e, true
		}
	}
	return "", false
}
