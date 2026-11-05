package domain

import (
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
func evalGraphBeforeGrep(in RuleInput) (RuleAction, string) {
	if len(in.IndexedExtensions) == 0 {
		return RuleAllow, ""
	}
	indexed := map[string]bool{}
	for _, e := range in.IndexedExtensions {
		indexed[strings.ToLower(e)] = true
	}
	for _, c := range splitShell(in.Command, 0) {
		if c.Nested || c.PipeBack > 0 || len(c.Stdin) > 0 {
			continue
		}
		toks := unwrapCommand(stripEnvAssignments(c.Tokens))
		if len(toks) == 0 {
			continue
		}
		if tool, ext, ok := codeSearchTarget(toks, indexed); ok {
			return RuleBlock, "buscar código " + ext + " con " + tool + " cuando el grafo ya lo indexa; " +
				"usa codegraph_explore (o search_graph de codebase-memory-mcp, o search_code/get_symbol de gomemory), " +
				"que devuelve el código con sus llamadores y su radio de impacto"
		}
	}
	return RuleAllow, ""
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
