package domain

import (
	"regexp"
	"strings"
	"unicode"
)

// shellCommand es un subcomando con lo que recibe por la entrada estándar:
// los cuerpos de sus heredocs y, si le llega una tubería, a cuántas
// posiciones atrás está el comando que la alimenta (0: ninguna). Es lo que
// hace falta para ver el script de `bash <<EOF` o `echo … | sh` (C-005). La
// distancia es relativa porque las sustituciones de un comando van justo
// antes que él y el resultado se anida dentro de otros sin reindexar.
type shellCommand struct {
	Tokens   []string
	Stdin    []string
	PipeBack int
	// Nested: sale de una sustitución, no es un comando de la línea.
	Nested bool
}

// SplitShellCommands parte una línea de shell en subcomandos tokenizados
// (feature 040, research R8). Reconoce comillas simples y dobles, escapes
// con barra invertida fuera de comillas simples, y los separadores `;`,
// `&&`, `||`, `|`, `(`, `)` y salto de línea. No expande nada: es un lexer
// mínimo para decidir sobre tokens en lugar de subcadenas, de modo que
// `echo "rm -rf /"` no se confunda con un `rm`. Una comilla sin cerrar se
// cierra al final. Barra invertida + salto de línea fuera de comillas
// simples es una continuación y desaparece (B-001), y el cuerpo de un
// heredoc es dato, no comandos (A-002). `$'…'` (ANSI-C, con sus escapes
// decodificados) y `$"…"` (localizado) son literales como cualquier otra
// comilla (R3-01). El contenido de `$(…)`, “ `…` “, `<(…)` y `>(…)` se
// ejecuta: se devuelve como subcomandos propios, antes del comando que los
// contiene porque el shell los ejecuta primero (C-004; C-003 del ACR
// acr_a635820b: `X=rm; echo $($X …); X=echo`), también dentro del
// cuerpo de un heredoc cuyo delimitador no va citado (H-02). Las
// redirecciones (`>/dev/null`, `2>&1`, `< in`, `<<EOF`) no son argumentos:
// el shell las quita del comando esté donde esté, así que no aparecen entre
// los tokens (H-01); el texto de un `<<<` es entrada estándar. Un `#` al
// principio de una palabra abre un comentario hasta el final de la línea
// (C-002 del ACR acr_a635820b).
//
// Devuelve primero los comandos de la línea y después los de sus
// sustituciones, para que quien mira el primero (la clave de un instinct) vea
// `git` en `git commit -m "$(cat msg)"`; el orden de ejecución solo lo
// necesita destructive-shell, que usa splitShell.
func SplitShellCommands(cmd string) [][]string {
	var top, nested [][]string
	for _, c := range splitShell(cmd, 0) {
		if c.Nested {
			nested = append(nested, c.Tokens)
		} else {
			top = append(top, c.Tokens)
		}
	}
	return append(top, nested...)
}

// maxSubstitutionNesting acota las sustituciones anidadas: cada nivel
// vuelve a recorrer su contenido. Más allá, la línea se marca con
// deepSubstitution y destructive-shell la bloquea en vez de seguir.
const maxSubstitutionNesting = 16

// deepSubstitution es el token que sustituye a una sustitución demasiado
// anidada para analizarla.
const deepSubstitution = "\x00sustitución-demasiado-anidada"

func splitShell(cmd string, depth int) []shellCommand {
	if depth > maxSubstitutionNesting {
		return []shellCommand{{Tokens: []string{deepSubstitution}}}
	}
	var (
		out      []shellCommand
		nested   []shellCommand
		current  []string
		token    strings.Builder
		inToken  bool
		quote    rune
		ansiC    bool
		pipeIn   bool
		heredocs []pendingHeredoc
		// redirect: el token en curso es una redirección; pendingOp: el
		// operador de una redirección que espera su destino en el token
		// siguiente (`> out`, `<<< 'texto'`); quoted: el token en curso tiene
		// algo citado o escapado, así que no es un descriptor (`"2">x`).
		redirect  bool
		pendingOp string
		quoted    bool
		herestr   []string
		// prevCmd es el índice en out del último comando de este nivel, el
		// que alimenta una tubería.
		prevCmd = -1
	)
	flushToken := func() {
		if !inToken {
			return
		}
		word := token.String()
		token.Reset()
		inToken = false
		switch {
		case pendingOp != "":
			if pendingOp == "<<<" {
				herestr = append(herestr, word)
			}
			pendingOp = ""
		case redirect:
			op, target := splitRedirect(word)
			switch {
			case target == "" && redirectOps[op]:
				pendingOp = op
			case op == "<<<":
				herestr = append(herestr, target)
			}
		default:
			current = append(current, word)
		}
		redirect, quoted = false, false
	}
	flushCommand := func(pipeNext bool) {
		flushToken()
		out = append(out, nested...)
		nested = nil
		if len(current) > 0 {
			idx := len(out)
			for k := range heredocs {
				if heredocs[k].owner < 0 {
					heredocs[k].owner = idx
				}
			}
			back := 0
			if pipeIn && prevCmd >= 0 {
				back = idx - prevCmd
			}
			out = append(out, shellCommand{Tokens: current, Stdin: herestr, PipeBack: back})
			prevCmd = idx
			current = nil
		}
		herestr, pendingOp = nil, ""
		pipeIn = pipeNext
	}
	runes := []rune(cmd)
	// substitution copia al token el texto de una sustitución que empieza en
	// i (su apertura ocupa open-i+1 runas) y analiza su contenido aparte.
	substitution := func(i, open int, closer rune) int {
		end := matchClose(runes, open, closer)
		inner := string(runes[open+1 : min(end, len(runes))])
		if closer == '`' {
			inner = unescapeBackticks(inner)
		}
		nested = append(nested, markNested(splitShell(inner, depth+1))...)
		token.WriteString(string(runes[i:min(end+1, len(runes))]))
		inToken, quoted = true, true
		return end
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch {
		case quote != 0:
			if r == quote {
				quote, ansiC = 0, false
				continue
			}
			if ansiC && r == '\\' && i+1 < len(runes) {
				var dec string
				dec, i = decodeANSIEscape(runes, i+1)
				token.WriteString(dec)
				continue
			}
			if quote == '"' && r == '$' && next == '(' {
				i = substitution(i, i+1, ')')
				continue
			}
			if quote == '"' && r == '`' {
				i = substitution(i, i, '`')
				continue
			}
			if quote == '"' && r == '\\' && i+1 < len(runes) {
				i++
				r = runes[i]
				if r == '\n' {
					continue
				}
			}
			token.WriteRune(r)
		case r == '$' && (next == '\'' || next == '"'):
			i++
			quote, ansiC = runes[i], runes[i] == '\''
			inToken, quoted = true, true
		case (r == '$' || r == '<' || r == '>') && next == '(':
			i = substitution(i, i+1, ')')
		case r == '`':
			i = substitution(i, i, '`')
		case r == '\'' || r == '"':
			quote = r
			inToken, quoted = true, true
		case r == '\\' && i+1 < len(runes):
			i++
			if runes[i] == '\n' {
				continue
			}
			token.WriteRune(runes[i])
			inToken, quoted = true, true
		case r == ' ' || r == '\t':
			flushToken()
		case r == '\n':
			flushCommand(false)
			var bodies []string
			i, bodies = skipHeredocBodies(runes, i, heredocs)
			for k, body := range bodies {
				if owner := heredocs[k].owner; owner >= 0 && owner < len(out) {
					out[owner].Stdin = append(out[owner].Stdin, body)
				}
				if !heredocs[k].quoted {
					out = append(out, heredocSubstitutions(body, depth)...)
				}
			}
			heredocs = nil
		case r == ';' || r == '(' || r == ')':
			flushCommand(false)
		case r == '&' && next == '&':
			i++
			flushCommand(false)
		case r == '&' && !isRedirectAmp(runes, i):
			// `cmd & otro`: segundo plano, el resto es otro comando (B-002).
			flushCommand(false)
		case r == '|' && redirect && runes[i-1] == '>':
			token.WriteRune(r) // `>|`: redirección que sobrescribe, no tubería.
		case r == '#' && !inToken:
			for i+1 < len(runes) && runes[i+1] != '\n' {
				i++
			}
		case r == '|':
			pipe := next != '|'
			if !pipe {
				i++
			}
			flushCommand(pipe)
		default:
			if !redirect && (r == '<' || r == '>' || (r == '&' && next == '>')) {
				// Una redirección empieza aquí: lo anterior es otro argumento
				// salvo que sea su descriptor (`2>`, `{fd}>`).
				if inToken && (quoted || !redirectFD.MatchString(token.String())) {
					flushToken()
				}
				redirect = true
			}
			if h, ok := parseHeredoc(runes, i); ok {
				heredocs = append(heredocs, pendingHeredoc{heredoc: h, owner: -1})
			}
			token.WriteRune(r)
			inToken = true
		}
	}
	flushCommand(false)
	return out
}

// matchClose devuelve la posición del cierre de la sustitución que se abre
// en open: el `)` que equilibra los paréntesis (fuera de comillas) o el
// siguiente “ ` “ sin escapar. Sin cierre, el final del texto.
func matchClose(runes []rune, open int, closer rune) int {
	depth := 1
	var quote rune
	for i := open + 1; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\\' && quote != '\'':
			i++
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case closer == '`':
			if r == '`' {
				return i
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '(':
			depth++
		case r == ')':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return len(runes)
}

// pendingHeredoc es un heredoc cuyo cuerpo empieza en la línea siguiente;
// owner es el índice en out de su comando (-1 hasta que el comando se cierra).
type pendingHeredoc struct {
	heredoc
	owner int
}

type heredoc struct {
	delim     string
	stripTabs bool
	// quoted: el delimitador lleva comillas o barras; el cuerpo es literal
	// y el shell no ejecuta sus sustituciones.
	quoted bool
}

// parseHeredoc reconoce `<<DELIM` o `<<-DELIM` empezando en i (no `<<<`).
// El delimitador puede ir entre comillas o con barras invertidas; el
// operador se sigue tokenizando como siempre, solo se anota su cuerpo.
func parseHeredoc(runes []rune, i int) (heredoc, bool) {
	if runes[i] != '<' || (i > 0 && runes[i-1] == '<') || i+2 >= len(runes) || runes[i+1] != '<' || runes[i+2] == '<' {
		return heredoc{}, false
	}
	j := i + 2
	h := heredoc{}
	if runes[j] == '-' {
		h.stripTabs = true
		j++
	}
	for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
		j++
	}
	var delim strings.Builder
	for ; j < len(runes) && !strings.ContainsRune(" \t\n;|&<>()", runes[j]); j++ {
		switch r := runes[j]; {
		case r == '\'' || r == '"':
			h.quoted = true
			for j++; j < len(runes) && runes[j] != r; j++ {
				delim.WriteRune(runes[j])
			}
		case r == '\\' && j+1 < len(runes):
			h.quoted = true
			j++
			delim.WriteRune(runes[j])
		default:
			delim.WriteRune(r)
		}
	}
	h.delim = delim.String()
	return h, h.delim != ""
}

// skipHeredocBodies salta, tras el salto de línea de la posición nl, los
// cuerpos de los heredocs pendientes en orden y devuelve la posición del
// último salto consumido y cada cuerpo. Un cuerpo sin delimitador de cierre
// llega al final.
func skipHeredocBodies(runes []rune, nl int, pending []pendingHeredoc) (int, []string) {
	bodies := make([]string, 0, len(pending))
	for _, h := range pending {
		var lines []string
		for nl < len(runes) {
			start := nl + 1
			end := start
			for end < len(runes) && runes[end] != '\n' {
				end++
			}
			line := string(runes[start:end])
			if h.stripTabs {
				line = strings.TrimLeft(line, "\t")
			}
			nl = end
			if line == h.delim {
				break
			}
			lines = append(lines, line)
		}
		bodies = append(bodies, strings.Join(lines, "\n"))
	}
	return nl, bodies
}

// heredocSubstitutions devuelve los subcomandos de las sustituciones `$(…)`
// y “ `…` “ de un cuerpo de heredoc expandible, que el shell ejecuta antes
// de entregar el texto (H-02). La barra invertida escapa el carácter
// siguiente, así `\$(rm -rf x)` queda como texto; el resto es dato.
func heredocSubstitutions(body string, depth int) []shellCommand {
	runes := []rune(body)
	var out []shellCommand
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == '\\':
			i++
		case runes[i] == '$' && i+1 < len(runes) && runes[i+1] == '(':
			end := matchClose(runes, i+1, ')')
			out = append(out, markNested(splitShell(string(runes[i+2:min(end, len(runes))]), depth+1))...)
			i = end
		case runes[i] == '`':
			end := matchClose(runes, i, '`')
			out = append(out, markNested(splitShell(unescapeBackticks(string(runes[i+1:min(end, len(runes))])), depth+1))...)
			i = end
		}
	}
	return out
}

// markNested marca los comandos de una sustitución.
func markNested(cmds []shellCommand) []shellCommand {
	for k := range cmds {
		cmds[k].Nested = true
	}
	return cmds
}

// redirectFD reconoce el descriptor que precede a una redirección: `2` en
// `2>&1` o `{fd}` en `{fd}>log`.
var redirectFD = regexp.MustCompile(`^([0-9]+|\{[A-Za-z_][A-Za-z0-9_]*\})$`)

// redirectOps son los operadores que, solos, llevan el destino en el token
// siguiente (`> out`, `<< EOF`, `<<< 'texto'`, `2>& 1`).
var redirectOps = map[string]bool{"<": true, ">": true, ">>": true, "<<": true, "<<-": true, "<<<": true, "<>": true, ">|": true, "&>": true, "&>>": true, "<&": true, ">&": true}

// splitRedirect separa una redirección en su operador y su destino, sin el
// descriptor: `2>&1` → `>&`, `1`; `<<-EOF` → `<<-`, `EOF`.
func splitRedirect(word string) (op, target string) {
	word = strings.TrimLeft(word, "0123456789")
	if strings.HasPrefix(word, "{") {
		if _, rest, ok := strings.Cut(word, "}"); ok {
			word = rest
		}
	}
	n := 0
	for n < len(word) && strings.IndexByte("<>&|", word[n]) >= 0 {
		n++
	}
	op, target = word[:n], word[n:]
	if op == "<<" && strings.HasPrefix(target, "-") {
		op, target = "<<-", target[1:]
	}
	return op, target
}

// isRedirectAmp dice si el & de la posición i forma parte de una
// redirección (`2>&1`, `>&2`, `<&3`, `&>archivo`, `&>>archivo`) y no separa
// comandos.
func isRedirectAmp(runes []rune, i int) bool {
	if i > 0 && (runes[i-1] == '>' || runes[i-1] == '<') {
		return true
	}
	return i+1 < len(runes) && runes[i+1] == '>'
}

// decodeANSIEscape decodifica el escape de `$'…'` cuyo carácter tras la
// barra está en i y devuelve el texto y la posición de su último carácter.
// Un escape desconocido se conserva tal cual, como hace bash.
func decodeANSIEscape(runes []rune, i int) (string, int) {
	simple := map[rune]string{'n': "\n", 't': "\t", 'r': "\r", 'a': "\a", 'b': "\b", 'f': "\f", 'v': "\v", 'e': "\x1b", 'E': "\x1b", '\\': "\\", '\'': "'", '"': "\"", '?': "?"}
	r := runes[i]
	if dec, ok := simple[r]; ok {
		return dec, i
	}
	digits := func(start, max, base int) (rune, int) {
		v, n := 0, 0
		for n < max && start+n < len(runes) {
			d := strings.IndexRune("0123456789abcdef"[:base], unicode.ToLower(runes[start+n]))
			if d < 0 {
				break
			}
			v = v*base + d
			n++
		}
		return rune(v), n
	}
	switch r {
	case 'x', 'u', 'U':
		max := map[rune]int{'x': 2, 'u': 4, 'U': 8}[r]
		if v, n := digits(i+1, max, 16); n > 0 {
			return string(v), i + n
		}
	case '0', '1', '2', '3', '4', '5', '6', '7':
		v, n := digits(i, 3, 8)
		return string(v), i + n - 1
	}
	return "\\" + string(r), i
}

// unescapeBackticks aplica al contenido de “ `…` “ lo que hace el shell
// antes de ejecutarlo: `\“, `\$` y `\\` pierden la barra, así un
// “ `echo \`rm -rf x\“ “ anidado vuelve a ser una sustitución.
func unescapeBackticks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("`$\\", s[i+1]) >= 0 {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
