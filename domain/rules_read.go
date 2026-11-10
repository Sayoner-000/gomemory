package domain

import (
	"path"
	"strings"
)

// NativeReadOverShellRule es la regla de serie que exige leer código con la
// herramienta nativa del runtime (Read en Claude Code, read en OpenCode) o con
// el grafo, en lugar de volcarlo por shell con cat, sed -n, head o awk. Es la
// mitad de lectura de native-edit-over-scripts (HCD-030).
const NativeReadOverShellRule = "native-read-over-shell"

// shellReaders son los comandos que vuelcan el contenido de un archivo. sed y
// awk también editan, pero esa forma (-i) ya la bloquea native-edit-over-scripts.
var shellReaders = map[string]bool{
	"cat": true, "bat": true, "batcat": true, "less": true, "more": true,
	"head": true, "tail": true, "nl": true, "view": true,
	"sed": true, "gsed": true, "awk": true, "gawk": true, "mawk": true,
}

// inlineInterpreters ejecutan un script dado en la línea (-c, -e) o por la
// entrada (`-`); los argumentos que siguen al script son su argv.
var inlineInterpreters = map[string]bool{
	"python": true, "python3": true, "node": true, "nodejs": true, "bun": true,
	"deno": true, "ruby": true, "perl": true,
}

// inlineScriptArgs devuelve los argumentos de un intérprete que ejecuta un
// script inline, sin el script: son las rutas que el script recibe en argv.
// ok=false si el intérprete ejecuta un archivo de script, que no es lectura
// inline.
func inlineScriptArgs(args []string) (rest []string, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-":
			ok = true
		case a == "-c" || a == "-e" || a == "-E" || a == "-p" || a == "--eval" || a == "--print":
			ok = true
			i++ // el script
		case isShortFlags(a) && (strings.HasSuffix(a, "e") || strings.HasSuffix(a, "E")):
			ok = true // perl -ne 'script'
			i++
		case strings.HasPrefix(a, "-"):
		default:
			if !ok {
				return nil, false // `python3 tool.py …`: script en archivo
			}
			rest = append(rest, a)
		}
	}
	return rest, ok
}

// evalNativeReadOverShell bloquea leer un archivo de código por shell en los
// runtimes que tienen lectura nativa y avisa en los demás (Codex lee por
// shell: no tiene otra vía). Cuenta tanto los lectores (cat, sed -n, head…)
// como un intérprete inline que recibe la ruta de código en argv. Un lector
// que recibe una tubería filtra salida y se deja pasar; un heredoc no exime:
// solo actúa si el comando nombra un archivo de código.
func evalNativeReadOverShell(in RuleInput) (RuleAction, string) {
	for _, c := range splitShell(in.Command, 0) {
		if c.Nested || c.PipeBack > 0 {
			continue
		}
		toks := unwrapCommand(stripEnvAssignments(c.Tokens))
		if len(toks) == 0 {
			continue
		}
		name := path.Base(toks[0])
		args := toks[1:]
		switch {
		case shellReaders[name]:
		case inlineInterpreters[name]:
			var ok bool
			if args, ok = inlineScriptArgs(args); !ok {
				continue
			}
		default:
			continue
		}
		for _, a := range args {
			if strings.HasPrefix(a, "-") {
				continue
			}
			ext, ok := isCodeFile(a)
			if !ok {
				continue
			}
			if !in.HasNativeReadTools() {
				return RuleWarn, "leer código " + ext + " con " + name + " vuelca el archivo sin contexto; " +
					"para entender un símbolo usa codegraph_explore o get_symbol/search_code de gomemory, " +
					"que devuelven su código con llamadores y radio de impacto"
			}
			return RuleBlock, "leer código " + ext + " con " + name + " en lugar de la herramienta nativa; " +
				"usa " + in.nativeReadTool() + " para un archivo concreto, o codegraph_explore " +
				"(get_symbol/search_code de gomemory) para entender un símbolo con sus llamadores"
		}
	}
	return RuleAllow, ""
}
