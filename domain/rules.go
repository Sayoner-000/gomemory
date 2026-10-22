package domain

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Reglas de la supertool rules (feature 040, contracts/rules-format.md):
// reglas propias en .gomemory/rules/*.md más la regla de serie
// destructive-shell. Todo es puro; la carga desde disco la hace el
// adaptador rulesfs.

// RuleEvent es el evento sobre el que actúa una regla.
type RuleEvent string

const (
	RuleEventBash   RuleEvent = "bash"
	RuleEventFile   RuleEvent = "file"
	RuleEventPrompt RuleEvent = "prompt"
	RuleEventStop   RuleEvent = "stop"
	RuleEventAny    RuleEvent = "any"
)

// MatchKind es el tipo de coincidencia del patrón.
type MatchKind string

const (
	MatchContains MatchKind = "contains"
	MatchRegex    MatchKind = "regex"
)

// RuleAction es lo que hace una regla al coincidir (y el veredicto).
type RuleAction string

const (
	RuleAllow RuleAction = "allow"
	RuleWarn  RuleAction = "warn"
	RuleBlock RuleAction = "block"
)

// Origen de una regla.
const (
	RuleOriginBuiltin = "builtin"
	RuleOriginProject = "project"
)

// Topes de una regla (contracts/rules-format.md).
const (
	RuleFileMaxBytes    = 16 * 1024
	RuleMaxCount        = 64
	RulePatternMaxRunes = 512
	RuleMessageMaxBytes = 2 * 1024
)

var ruleNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Rule es una regla ya validada. re se compila al parsear (o con Compile
// tras leerla de la caché, que no guarda expresiones compiladas).
type Rule struct {
	Name      string     `json:"name"`
	Event     RuleEvent  `json:"event"`
	Match     MatchKind  `json:"match"`
	Pattern   string     `json:"pattern"`
	Action    RuleAction `json:"action"`
	Enabled   bool       `json:"enabled"`
	Overrides string     `json:"overrides,omitempty"`
	Message   string     `json:"message"`
	Origin    string     `json:"origin"`
	Source    string     `json:"source,omitempty"`

	re      *regexp.Regexp
	builtin func(RuleInput) (RuleAction, string)
}

// RuleLoadError describe una regla que no se pudo cargar. No impide cargar
// las demás.
type RuleLoadError struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

func (e *RuleLoadError) Error() string { return e.Source + ": " + e.Reason }

// RuleInput es la entrada que se evalúa, según el evento.
type RuleInput struct {
	Command     string
	FilePaths   []string
	Content     string
	Prompt      string
	LastMessage string
}

// RuleVerdict es el resultado de evaluar las reglas para un evento.
type RuleVerdict struct {
	Action RuleAction
	Reason string
	Rule   string
}

// ParseRule valida una regla propia. source es el nombre del archivo; si el
// frontmatter no trae name, se deriva de él.
func ParseRule(source string, raw []byte) (Rule, error) {
	fail := func(format string, args ...any) (Rule, error) {
		return Rule{}, &RuleLoadError{Source: source, Reason: fmt.Sprintf(format, args...)}
	}
	if len(raw) > RuleFileMaxBytes {
		return fail("supera %d bytes", RuleFileMaxBytes)
	}
	fm, body, ok := ParseFrontmatter(raw)
	if !ok {
		return fail("falta el frontmatter (--- … ---)")
	}
	r := Rule{Origin: RuleOriginProject, Source: source, Enabled: true, Match: MatchContains}
	r.Name = strings.ToLower(fm["name"])
	if r.Name == "" {
		r.Name = strings.ToLower(strings.TrimSuffix(filepath.Base(source), ".md"))
	}
	if !ruleNameRe.MatchString(r.Name) {
		return fail("nombre inválido %q (minúsculas, dígitos y guiones, hasta 64)", r.Name)
	}
	switch e := RuleEvent(fm["event"]); e {
	case RuleEventBash, RuleEventFile, RuleEventPrompt, RuleEventStop, RuleEventAny:
		r.Event = e
	default:
		return fail("event inválido %q (bash|file|prompt|stop|any)", fm["event"])
	}
	if m := fm["match"]; m != "" {
		switch MatchKind(m) {
		case MatchContains, MatchRegex:
			r.Match = MatchKind(m)
		default:
			return fail("match inválido %q (contains|regex)", m)
		}
	}
	switch a := RuleAction(fm["action"]); a {
	case RuleWarn, RuleBlock:
		r.Action = a
	default:
		return fail("action inválida %q (warn|block)", fm["action"])
	}
	if v, set := fm["enabled"]; set {
		switch strings.ToLower(v) {
		case "false", "no", "0":
			r.Enabled = false
		case "true", "yes", "1", "":
		default:
			return fail("enabled inválido %q", v)
		}
	}
	r.Pattern = fm["pattern"]
	if strings.TrimSpace(r.Pattern) == "" {
		return fail("pattern vacío")
	}
	if len([]rune(r.Pattern)) > RulePatternMaxRunes {
		return fail("pattern supera %d caracteres", RulePatternMaxRunes)
	}
	r.Overrides = strings.ToLower(fm["overrides"])
	r.Message = capBytes(strings.TrimSpace(StripTerminalControls(body)), RuleMessageMaxBytes)
	if err := r.Compile(); err != nil {
		return fail("%v", err)
	}
	return r, nil
}

// Compile prepara la expresión regular de una regla leída de la caché.
// RE2 garantiza tiempo lineal: no hay backtracking catastrófico.
func (r *Rule) Compile() error {
	if r.Match != MatchRegex {
		return nil
	}
	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return fmt.Errorf("regex inválida: %v", err)
	}
	r.re = re
	return nil
}

func capBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// EvaluateRules evalúa rules (la de serie primero, después las propias en
// el orden recibido) y devuelve la decisión más restrictiva: block > warn
// > allow; con empate gana la primera. Una regla propia con
// `overrides: <nombre>` sustituye a la regla de serie de ese nombre.
func EvaluateRules(target RuleEvent, in RuleInput, rules []Rule) RuleVerdict {
	overridden := map[string]bool{}
	for _, r := range rules {
		if r.Origin != RuleOriginBuiltin && r.Overrides != "" {
			overridden[r.Overrides] = true
		}
	}
	best := RuleVerdict{Action: RuleAllow}
	for _, r := range rules {
		if !r.Enabled || (r.Origin == RuleOriginBuiltin && overridden[r.Name]) {
			continue
		}
		if r.Event != target && r.Event != RuleEventAny {
			continue
		}
		action, detail := r.evaluate(target, in)
		if rank(action) <= rank(best.Action) {
			continue
		}
		msg := r.Message
		if msg == "" {
			msg = detail
		}
		best = RuleVerdict{Action: action, Rule: r.Name, Reason: ruleReason(action, r.Name, msg)}
	}
	return best
}

func rank(a RuleAction) int {
	switch a {
	case RuleBlock:
		return 2
	case RuleWarn:
		return 1
	}
	return 0
}

func ruleReason(action RuleAction, name, msg string) string {
	if action == RuleBlock {
		return fmt.Sprintf("gomemory expand · regla %s: %s. Si es intencionado, ejecútalo tú en tu terminal o desactiva la regla (mem expand off rules).", name, strings.TrimSuffix(msg, "."))
	}
	return fmt.Sprintf("regla %s: %s", name, msg)
}

func (r Rule) evaluate(target RuleEvent, in RuleInput) (RuleAction, string) {
	if r.builtin != nil {
		return r.builtin(in)
	}
	if r.Match == MatchRegex && r.re == nil && r.Compile() != nil {
		return RuleAllow, ""
	}
	for _, text := range ruleTexts(target, in) {
		if text == "" {
			continue
		}
		if r.Match == MatchRegex {
			if r.re.MatchString(text) {
				return r.Action, ""
			}
		} else if strings.Contains(strings.ToLower(text), strings.ToLower(r.Pattern)) {
			return r.Action, ""
		}
	}
	return RuleAllow, ""
}

// ruleTexts devuelve los textos que se comparan con el patrón. En file,
// cada ruta y el contenido se evalúan por separado, para que `\.env$` se
// refiera a la ruta y no al final de todo el texto.
func ruleTexts(target RuleEvent, in RuleInput) []string {
	switch target {
	case RuleEventBash:
		return []string{in.Command}
	case RuleEventFile:
		return append(append([]string{}, in.FilePaths...), in.Content)
	case RuleEventPrompt:
		return []string{in.Prompt}
	case RuleEventStop:
		return []string{in.LastMessage}
	}
	return nil
}

// BuiltinRules devuelve las reglas de serie. Hoy solo destructive-shell.
func BuiltinRules() []Rule {
	return []Rule{{
		Name:    "destructive-shell",
		Event:   RuleEventBash,
		Action:  RuleBlock,
		Enabled: true,
		Origin:  RuleOriginBuiltin,
		Message: "",
		builtin: evalDestructiveShell,
	}}
}

// evalDestructiveShell decide sobre tokens, no subcadenas (research R8):
// `echo "rm -rf /"` no es un rm. Se evalúa cada subcomando de la línea.
func evalDestructiveShell(in RuleInput) (RuleAction, string) {
	return destructiveLine(in.Command, 0, nil)
}

// maxShellNesting acota `sh -c "sh -c ..."`, `eval` y similares. Más allá
// no hay uso legítimo y no se sigue analizando: se bloquea (C-002).
const maxShellNesting = 4

func destructiveLine(cmd string, depth int, vars map[string]string) (RuleAction, string) {
	if depth > maxShellNesting {
		return RuleBlock, "anidamiento de shell demasiado profundo para analizarlo"
	}
	// Las asignaciones literales de la línea (`R=rm; $R -rf x`) se resuelven
	// en los comandos siguientes y en las líneas anidadas; las variables del
	// entorno no se conocen y no se interpretan.
	vars = maps.Clone(vars)
	if vars == nil {
		vars = map[string]string{}
	}
	worst, detail := RuleAllow, ""
	cmds := splitShell(cmd, 0)
	// Una sustitución demasiado anidada bloquea la línea pase lo que pase.
	// Decidirlo antes de evaluar los demás comandos evita volver a recorrer
	// cada nivel: con 20 000 niveles, eso tardaba segundos.
	for _, c := range cmds {
		if len(c.Tokens) > 0 && c.Tokens[0] == deepSubstitution {
			return destructiveVerdict(c.Tokens)
		}
	}
	for i, c := range cmds {
		if recordAssignments(c.Tokens, vars) {
			continue
		}
		var stdin []string
		if c.PipeBack > 0 && i >= c.PipeBack {
			stdin = pipedText(cmds[i-c.PipeBack])
		}
		a, d := destructiveCommand(c.Tokens, append(stdin, c.Stdin...), depth, vars)
		if rank(a) > rank(worst) {
			worst, detail = a, d
		}
	}
	return worst, detail
}

// destructiveCommand decide sobre un subcomando: lo desenvuelve y, si
// ejecuta otra línea (`sh -c`, `eval`, `su -c`, `trap`, `find -exec`, un
// shell que lee su script de la entrada), la analiza también. stdin es lo
// que el subcomando recibe por la entrada estándar (heredoc o tubería).
func destructiveCommand(toks, stdin []string, depth int, vars map[string]string) (RuleAction, string) {
	// El lexer no conserva si una expansión iba entre comillas (`"$ARG"` no
	// se divide en palabras; `$ARG`, sí): se evalúan ambas lecturas y
	// prevalece la más restrictiva.
	split, whole := expandKnown(toks, vars, 0, true), expandKnown(toks, vars, 0, false)
	a, d := destructiveTokens(split, stdin, depth, vars)
	if !slices.Equal(split, whole) {
		if a2, d2 := destructiveTokens(whole, stdin, depth, vars); rank(a2) > rank(a) {
			a, d = a2, d2
		}
	}
	return a, d
}

func destructiveTokens(toks, stdin []string, depth int, vars map[string]string) (RuleAction, string) {
	toks = unwrapCommand(toks)
	if len(toks) == 0 {
		return RuleAllow, ""
	}
	var lines []string
	switch {
	case toks[0] == "eval":
		lines = append(lines, strings.Join(toks[1:], " "))
	case toks[0] == "trap":
		// `trap 'cmd' EXIT` ejecuta cmd más tarde; `trap -`, `trap ''` y
		// `trap -p` no ejecutan nada.
		args := toks[1:]
		for len(args) > 0 && (args[0] == "-p" || args[0] == "-l" || args[0] == "--") {
			args = args[1:]
		}
		if len(args) > 1 && args[0] != "-" {
			lines = append(lines, args[0])
		}
	case toks[0] == "su" || toks[0] == "runuser":
		if inner, ok := suCommand(toks[1:]); ok {
			lines = append(lines, inner)
		} else if toks[0] == "runuser" && (hasFlag(toks[1:], 'u', "--user") || hasLongPrefix(toks[1:], "--user=")) {
			// `runuser -u usuario [--] cmd` ejecuta cmd sin pasar por -c.
			rest := skipOptions(toks[1:], "ugGsw", []string{"--user", "--group", "--supp-group", "--shell", "--whitelist-environment"})
			return destructiveCommand(rest, stdin, depth, vars)
		}
	case toks[0] == "find":
		if contains(toks[1:], "-delete") {
			return RuleBlock, "find -delete borra los archivos encontrados"
		}
		worst, detail := RuleAllow, ""
		for _, exec := range findExecCommands(toks[1:]) {
			if depth >= maxShellNesting {
				return RuleBlock, "anidamiento de shell demasiado profundo para analizarlo"
			}
			if a, d := destructiveCommand(exec, nil, depth+1, vars); rank(a) > rank(worst) {
				worst, detail = a, d
			}
		}
		return worst, detail
	default:
		if inner, ok := shellDashC(toks); ok {
			lines = append(lines, inner)
		} else if shellReadsStdin(toks) {
			lines = append(lines, stdin...)
		}
	}
	if len(lines) == 0 {
		return destructiveVerdict(toks)
	}
	worst, detail := RuleAllow, ""
	for _, line := range lines {
		if a, d := destructiveLine(line, depth+1, vars); rank(a) > rank(worst) {
			worst, detail = a, d
		}
	}
	return worst, detail
}

// declarationWords preceden a asignaciones que siguen siendo asignaciones.
var declarationWords = map[string]bool{"export": true, "readonly": true, "declare": true, "local": true, "typeset": true}

// recordAssignments anota en vars un comando que solo asigna (`R=rm`,
// `export F=-rf`) y dice si lo era.
func recordAssignments(toks []string, vars map[string]string) bool {
	if len(toks) > 0 && declarationWords[toks[0]] {
		toks = toks[1:]
		for len(toks) > 0 && isShortFlags(toks[0]) {
			toks = toks[1:]
		}
	}
	if len(toks) == 0 || !slices.ContainsFunc(toks, isEnvAssignment) {
		return false
	}
	for _, t := range toks {
		if !isEnvAssignment(t) {
			return false
		}
	}
	for _, t := range toks {
		name, value, _ := strings.Cut(t, "=")
		if shellVarName.MatchString(name) {
			vars[name] = strings.Join(expandKnown([]string{value}, vars, 0, false), " ")
		}
	}
	return true
}

var (
	shellVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	shellVarRef  = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
)

// expandKnown sustituye lo que se conoce sin ejecutar nada: las variables
// asignadas en la propia línea y `$(echo …)`/“ `printf …` “, cuya salida
// se calcula, también pegadas a otro texto (`r$(printf m)`, C-001 del ACR
// acr_a635820b). Un token que era solo una expansión se divide en palabras
// como haría el shell. Lo desconocido queda tal cual.
func expandKnown(toks []string, vars map[string]string, depth int, split bool) []string {
	if depth > maxShellNesting {
		return toks
	}
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if inner, ok := wholeSubstitution(t); ok {
			if text, ok := echoOutput(inner, vars, depth+1); ok {
				if split {
					out = append(out, strings.Fields(text)...)
				} else {
					out = append(out, text)
				}
				continue
			}
			out = append(out, t)
			continue
		}
		inline := false
		if strings.Contains(t, "$(") || strings.Contains(t, "`") {
			if t, inline = expandInlineSubstitutions(t, vars, depth); t == unresolvedOutput {
				out = append(out, t)
				continue
			}
		}
		whole := t != "" && shellVarRef.FindString(t) == t
		known := true
		expanded := shellVarRef.ReplaceAllStringFunc(t, func(ref string) string {
			name := strings.Trim(ref, "${}")
			if v, ok := vars[name]; ok {
				return v
			}
			known = false
			return ref
		})
		if (whole && known || inline) && split {
			out = append(out, strings.Fields(expanded)...)
		} else {
			out = append(out, expanded)
		}
	}
	return out
}

// expandInlineSubstitutions reemplaza dentro de t cada `$(…)` o “ `…` “
// cuya salida se conoce y dice si reemplazó alguna. Si una no se puede
// calcular (un printf no resoluble), el token entero lo es.
func expandInlineSubstitutions(t string, vars map[string]string, depth int) (string, bool) {
	runes := []rune(t)
	var b strings.Builder
	changed := false
	for i := 0; i < len(runes); i++ {
		open, closer, skip := -1, rune(0), 0
		switch {
		case runes[i] == '$' && i+1 < len(runes) && runes[i+1] == '(' && (i+2 >= len(runes) || runes[i+2] != '('):
			open, closer, skip = i+1, ')', 2
		case runes[i] == '`':
			open, closer, skip = i, '`', 1
		}
		if open < 0 {
			b.WriteRune(runes[i])
			continue
		}
		end := matchClose(runes, open, closer)
		if end >= len(runes) {
			b.WriteString(string(runes[i:]))
			break
		}
		inner := string(runes[i+skip : end])
		if closer == '`' {
			inner = unescapeBackticks(inner)
		}
		text, ok := echoOutput(inner, vars, depth+1)
		switch {
		case ok && text == unresolvedOutput:
			return unresolvedOutput, true
		case ok:
			b.WriteString(text)
			changed = true
		default:
			b.WriteString(string(runes[i : end+1]))
		}
		i = end
	}
	return b.String(), changed
}

func isWholeSubstitution(t string) bool {
	_, ok := wholeSubstitution(t)
	return ok
}

// wholeSubstitution devuelve el contenido de un token que es entero una
// sustitución `$(…)` o “ `…` “.
func wholeSubstitution(t string) (string, bool) {
	switch {
	case len(t) > 3 && strings.HasPrefix(t, "$(") && strings.HasSuffix(t, ")") && !strings.HasPrefix(t, "$(("):
		return t[2 : len(t)-1], true
	case len(t) > 2 && strings.HasPrefix(t, "`") && strings.HasSuffix(t, "`"):
		return unescapeBackticks(t[1 : len(t)-1]), true
	}
	return "", false
}

// echoOutput es la salida de una sustitución que solo ejecuta echo o printf.
func echoOutput(inner string, vars map[string]string, depth int) (string, bool) {
	cmds := splitShell(inner, 0)
	if len(cmds) != 1 {
		return "", false
	}
	toks := unwrapCommand(expandKnown(cmds[0].Tokens, vars, depth, true))
	if len(toks) == 0 || (toks[0] != "echo" && toks[0] != "printf") {
		return "", false
	}
	text := pipedText(shellCommand{Tokens: toks})
	return strings.Join(text, " "), len(text) > 0
}

// pipedText es lo que un subcomando escribe por la tubería cuando se puede
// saber sin ejecutarlo: los argumentos de echo/printf y los heredocs de un
// cat sin archivos (`echo 'rm -rf /' | sh`, `cat <<EOF | bash`).
func pipedText(c shellCommand) []string {
	toks := unwrapCommand(c.Tokens)
	if len(toks) == 0 {
		return nil
	}
	args := toks[1:]
	switch toks[0] {
	case "echo":
		return []string{echoText(args)}
	case "printf":
		if out, ok := printfOutput(args); ok {
			return []string{out}
		}
		return []string{unresolvedOutput}
	case "cat":
		if len(args) == 0 {
			return c.Stdin
		}
	}
	return nil
}

// unwrapCommand quita los lanzadores que ejecutan su argumento como comando
// (`sudo -u root`, `env -i`, `command`, `nohup`, `nice -n 5`, `timeout 5`,
// `xargs -n1`…) y normaliza la ruta del ejecutable: `/bin/rm` sigue siendo
// un rm (H-02). Las palabras clave de control (`then`, `do`, `{`, `!`…) no
// son el comando. `command -v`/`-V` solo consulta, no ejecuta.
func unwrapCommand(toks []string) []string {
	for {
		for len(toks) > 0 && (isEnvAssignment(toks[0]) || shellKeywords[toks[0]]) {
			toks = toks[1:]
		}
		if len(toks) > 0 && toks[0] == "--" {
			toks = toks[1:]
		}
		if len(toks) == 0 {
			return toks
		}
		name := path.Base(strings.TrimPrefix(toks[0], `\`))
		launcher, ok := launchers[name]
		if !ok {
			return append([]string{name}, toks[1:]...)
		}
		if name == "env" {
			// `env -S 'cmd args'` ejecuta su argumento como una línea de
			// comando: se vuelve a tokenizar y se sigue desenvolviendo (R3-02).
			if embedded, ok := envSplitString(toks[1:]); ok {
				toks = embedded
				continue
			}
		}
		if name == "command" && len(toks) > 1 && (hasFlag(toks[1:2], 'v', "") || hasFlag(toks[1:2], 'V', "")) {
			return nil
		}
		toks = skipOptions(toks[1:], launcher.shortArgs, launcher.longArgs)
		for range launcher.positional {
			if len(toks) > 0 {
				toks = toks[1:]
			}
		}
	}
}

// launcher describe un programa que ejecuta su argumento como comando:
// opciones cortas y largas que consumen el token siguiente y cuántos
// argumentos posicionales van antes del comando (`timeout 5 cmd`).
type launcher struct {
	shortArgs  string
	longArgs   []string
	positional int
}

var launchers = map[string]launcher{
	"sudo":    {shortArgs: "ugpChDrtTUa", longArgs: []string{"--user", "--group", "--host", "--prompt", "--chdir", "--role", "--type", "--close-from", "--other-user", "--command-timeout", "--login-class", "--chroot"}},
	"doas":    {shortArgs: "uaC"},
	"pkexec":  {longArgs: []string{"--user"}},
	"env":     {shortArgs: "uCa", longArgs: []string{"--unset", "--chdir", "--argv0", "--block-signal", "--default-signal", "--ignore-signal"}},
	"nice":    {shortArgs: "n", longArgs: []string{"--adjustment"}},
	"exec":    {shortArgs: "a"},
	"command": {},
	"nohup":   {},
	"time":    {shortArgs: "fo", longArgs: []string{"--format", "--output"}},
	"builtin": {},
	"busybox": {},
	"setsid":  {},
	"xargs":   {shortArgs: "IaEdLnPs", longArgs: []string{"--arg-file", "--delimiter", "--max-lines", "--max-args", "--max-procs", "--max-chars", "--process-slot-var", "--eof", "--replace"}},
	"timeout": {shortArgs: "sk", longArgs: []string{"--signal", "--kill-after"}, positional: 1},
	"stdbuf":  {shortArgs: "ioe", longArgs: []string{"--input", "--output", "--error"}},
	"ionice":  {shortArgs: "cnp", longArgs: []string{"--class", "--classdata", "--pid", "--pgid", "--uid"}},
	"chrt":    {shortArgs: "TPD", longArgs: []string{"--sched-runtime", "--sched-period", "--sched-deadline"}, positional: 1},
	"taskset": {positional: 1},
}

// shellKeywords son palabras reservadas que preceden al comando real en
// `if …; then rm -rf x; fi`, `{ rm -rf x; }` o `! rm -rf x`.
var shellKeywords = map[string]bool{"!": true, "{": true, "}": true, "if": true, "then": true, "else": true, "elif": true, "fi": true, "do": true, "done": true, "while": true, "until": true}

func hasLongPrefix(args []string, prefix string) bool {
	return slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, prefix) })
}

// suCommand devuelve el comando de `su`/`runuser` (-c, --command, -cCMD,
// --command=CMD), con sus opciones antes o después del usuario.
func suCommand(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "--command" || a == "--session-command":
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		case strings.HasPrefix(a, "--command=") || strings.HasPrefix(a, "--session-command="):
			_, v, _ := strings.Cut(a, "=")
			return v, true
		case a == "-s" || a == "-g" || a == "-G" || a == "-w" || a == "--shell" || a == "--group" || a == "--supp-group" || a == "--whitelist-environment":
			i++
		case isShortFlags(a) && strings.ContainsRune(a, 'c'):
			_, v, _ := strings.Cut(a, "c")
			if v != "" {
				return v, true
			}
			if i+1 < len(args) {
				return args[i+1], true
			}
		}
	}
	return "", false
}

// findExecCommands devuelve los comandos de -exec/-execdir/-ok/-okdir de
// find, hasta su `;` o `+`.
func findExecCommands(args []string) [][]string {
	var out [][]string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-exec", "-execdir", "-ok", "-okdir":
			j := i + 1
			for j < len(args) && args[j] != ";" && args[j] != "+" {
				j++
			}
			if j > i+1 {
				out = append(out, args[i+1:j])
			}
			i = j
		}
	}
	return out
}

// envSplitString busca -S/--split-string entre las opciones de env (también
// agrupada, `-iS`, pegada, `-Scmd`, o con `=`) y devuelve la línea embebida
// tokenizada seguida de los argumentos restantes.
func envSplitString(args []string) ([]string, bool) {
	expand := func(line string, rest []string) ([]string, bool) {
		var embedded []string
		if cmds := SplitShellCommands(line); len(cmds) > 0 {
			embedded = cmds[0]
		}
		return append(embedded, rest...), true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case isEnvAssignment(a):
			// Una asignación antes de -S no termina la búsqueda: ante la
			// duda sobre el orden que acepta cada env, se falla cerrado (R4-01).
		case a == "--" || !strings.HasPrefix(a, "-") || a == "-":
			return nil, false
		case strings.HasPrefix(a, "--split-string="):
			return expand(strings.TrimPrefix(a, "--split-string="), args[i+1:])
		case a == "--split-string":
			if i+1 < len(args) {
				return expand(args[i+1], args[i+2:])
			}
			return nil, false
		case a == "--unset" || a == "--chdir" || a == "--argv0":
			i++
		case strings.HasPrefix(a, "--"):
		default:
			for j := 1; j < len(a); j++ {
				if !strings.ContainsRune("uCSa", rune(a[j])) {
					continue
				}
				value, next := a[j+1:], i+1
				if value == "" && i+1 < len(args) {
					value, next = args[i+1], i+2
				}
				if a[j] == 'S' {
					return expand(value, args[min(next, len(args)):])
				}
				i = next - 1
				break
			}
		}
	}
	return nil, false
}

func isEnvAssignment(t string) bool {
	return strings.Contains(t, "=") && !strings.HasPrefix(t, "-") && !strings.HasPrefix(t, "=")
}

// skipOptions salta las opciones iniciales de un lanzador; withArg lista las
// opciones cortas que consumen el token siguiente (o el resto del grupo,
// `-oL`) y longArgs, las largas que lo consumen cuando no llevan `=`.
func skipOptions(toks []string, withArg string, longArgs []string) []string {
	for len(toks) > 0 {
		t := toks[0]
		if t == "--" {
			return toks[1:]
		}
		if !strings.HasPrefix(t, "-") || t == "-" {
			return toks
		}
		toks = toks[1:]
		if strings.HasPrefix(t, "--") {
			if !strings.Contains(t, "=") && slices.Contains(longArgs, t) && len(toks) > 0 {
				toks = toks[1:]
			}
			continue
		}
		for k := 1; k < len(t); k++ {
			if strings.ContainsRune(withArg, rune(t[k])) {
				if k == len(t)-1 && len(toks) > 0 {
					toks = toks[1:]
				}
				break
			}
		}
	}
	return toks
}

// shellNames son los shells cuyo -c o entrada estándar se analizan.
var shellNames = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "mksh": true, "yash": true, "posh": true, "csh": true, "tcsh": true, "fish": true}

// shellReadsStdin dice si un shell sin -c ejecuta como script lo que le
// llega por la entrada estándar (sin archivo de script, o con -s): heredocs,
// herestrings (`bash <<< 'cmd'`) y tuberías (C-005).
func shellReadsStdin(toks []string) bool {
	if !shellNames[toks[0]] {
		return false
	}
	args := toks[1:]
	dashS := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return dashS || i+1 >= len(args)
		case a == "--rcfile" || a == "--init-file":
			i++
		case strings.HasPrefix(a, "--"):
		case a == "-":
			return true
		case len(a) > 1 && (a[0] == '-' || a[0] == '+'):
			dashS = dashS || (a[0] == '-' && strings.ContainsRune(a, 's'))
			if strings.ContainsAny(a[1:], "oO") && i+1 < len(args) {
				i++
			}
		case isWholeSubstitution(a):
			// `sh $(true)`: una sustitución desconocida puede quedar vacía, y
			// entonces el shell lee su script de la entrada; no cuenta como
			// archivo de script.
		default:
			// Un archivo de script: la entrada estándar es suya, no del shell.
			return dashS
		}
	}
	return true
}

// shellDashC reconoce `sh|bash|zsh|dash|ksh -c <cmd>` (también agrupado:
// -lc, -ec) y devuelve el comando anidado. Como el propio shell, recorre
// todas las opciones antes del primer argumento: las largas (`--norc`,
// `--rcfile <f>`), `-o|+o|-O|+O <opción>` y las cortas agrupadas, aunque
// vayan antes o después de -c (R2-01). Con -c, ese primer argumento es el
// comando.
func shellDashC(toks []string) (string, bool) {
	if !shellNames[toks[0]] {
		return "", false
	}
	dashC := false
	for i := 1; i < len(toks); i++ {
		a := toks[i]
		switch {
		case a == "--" || a == "-":
			if dashC && i+1 < len(toks) {
				return toks[i+1], true
			}
			return "", false
		case strings.HasPrefix(a, "--"):
			if (a == "--rcfile" || a == "--init-file") && i+1 < len(toks) {
				i++
			}
		case len(a) > 1 && (a[0] == '-' || a[0] == '+'):
			if a[0] == '-' && strings.ContainsRune(a, 'c') {
				dashC = true
			}
			// -o/+o/-O/+O consumen el nombre de la opción siguiente.
			if strings.ContainsAny(a[1:], "oO") && i+1 < len(toks) {
				i++
			}
		default:
			if dashC {
				return a, true
			}
			return "", false
		}
	}
	return "", false
}

func destructiveVerdict(toks []string) (RuleAction, string) {
	switch toks[0] {
	case deepSubstitution:
		return RuleBlock, "sustituciones demasiado anidadas para analizarlas"
	case unresolvedOutput:
		return RuleBlock, "la salida de printf que se ejecuta no se puede calcular sin ejecutarla"
	case "rm":
		if hasFlag(toks[1:], 'r', "--recursive") && hasFlag(toks[1:], 'f', "--force") {
			return RuleBlock, "borrado recursivo forzado (rm -rf)"
		}
		if hasFlag(toks[1:], 'R', "--recursive") && hasFlag(toks[1:], 'f', "--force") {
			return RuleBlock, "borrado recursivo forzado (rm -rf)"
		}
	case "git":
		sub, rest := gitSubcommandArgs(toks[1:])
		switch sub {
		case "reset":
			if contains(rest, "--hard") {
				return RuleBlock, "git reset --hard descarta cambios sin confirmar"
			}
		case "push":
			refs := gitPushRefspecs(rest)
			for _, a := range rest {
				if a == "--force" || a == "-f" || a == "--mirror" || (isShortFlags(a) && strings.ContainsRune(a, 'f')) {
					return RuleBlock, "git push --force reescribe el historial remoto"
				}
			}
			// `+ref` fuerza esa actualización igual que --force (C-007).
			for _, r := range refs {
				if strings.HasPrefix(r, "+") {
					return RuleBlock, "git push +ref reescribe el historial remoto"
				}
			}
			for _, a := range rest {
				if a == "--force-with-lease" || strings.HasPrefix(a, "--force-with-lease=") {
					return RuleWarn, "git push --force-with-lease reescribe el historial remoto"
				}
			}
			for _, r := range refs {
				if len(r) > 1 && strings.HasPrefix(r, ":") {
					return RuleWarn, "git push :rama borra la rama remota"
				}
			}
			if contains(rest, "--delete") || hasFlag(rest, 'd', "") {
				return RuleWarn, "git push --delete borra la rama remota"
			}
		case "checkout":
			// `git checkout .`, `-- ./` o `HEAD -- :/` restauran todo el árbol
			// desde el índice o un commit (C-009). `git checkout -` cambia de rama.
			for _, a := range rest {
				if a != "--" && wholeTreePathspec(a) {
					return RuleBlock, "git checkout -- . descarta todos los cambios del árbol"
				}
			}
		case "restore":
			// --staged solo deshace el indexado; el árbol se toca sin él o
			// con --worktree explícito (H-06).
			staged := hasFlag(rest, 'S', "--staged")
			worktree := hasFlag(rest, 'W', "--worktree")
			if staged && !worktree {
				break
			}
			for _, a := range rest {
				if wholeTreePathspec(a) {
					return RuleBlock, "git restore . descarta los cambios sin confirmar del árbol de trabajo"
				}
			}
		case "clean":
			// -f basta para borrar los archivos sin seguimiento; -n o
			// --dry-run solo los listan.
			if hasFlag(rest, 'f', "--force") && !hasFlag(rest, 'n', "--dry-run") {
				return RuleBlock, "git clean -f borra archivos sin seguimiento"
			}
		}
	}
	return RuleAllow, ""
}

// hasFlag reconoce una opción corta (sola o agrupada: -rf) o su forma larga.
func hasFlag(args []string, short rune, long string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if long != "" && a == long {
			return true
		}
		if isShortFlags(a) && strings.ContainsRune(a, short) {
			return true
		}
	}
	return false
}

func isShortFlags(a string) bool {
	return len(a) > 1 && a[0] == '-' && a[1] != '-'
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// wholeTreePathspec dice si una ruta de git abarca todo el árbol: `.`,
// `./`, `*`, `:/` y sus variantes con magia de pathspec.
func wholeTreePathspec(a string) bool {
	switch strings.TrimSuffix(strings.TrimSuffix(a, "*"), "/") {
	case "", ".", ":", ":(top)", ":(top).", ":/.":
		return a != ""
	}
	return path.Clean(a) == "."
}

// gitPushRefspecs devuelve los argumentos posicionales de git push tras el
// remoto (los refspecs), saltando las opciones que llevan valor aparte.
func gitPushRefspecs(args []string) []string {
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		case a == "-o" || a == "--push-option" || a == "--repo" || a == "--receive-pack" || a == "--exec":
			i++
		case strings.HasPrefix(a, "-") && a != "-":
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) > 0 {
		pos = pos[1:]
	}
	return pos
}

// gitSubcommandArgs devuelve el subcomando de git y sus argumentos.
func gitSubcommandArgs(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree" || a == "--namespace" || a == "--super-prefix" || a == "--config-env" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a, args[i+1:]
	}
	return "", nil
}

// ParseApplyPatch extrae de un parche de Codex (apply_patch) las rutas de
// sus cabeceras y el contenido añadido (líneas `+`), para evaluar reglas
// file en Codex (research R7; parches reales en evidence/codex-probe.md).
func ParseApplyPatch(patch string) (paths []string, body string) {
	if !strings.Contains(patch, "*** Begin Patch") {
		return nil, ""
	}
	var added []string
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			paths = append(paths, strings.TrimPrefix(line, "*** Add File: "))
		case strings.HasPrefix(line, "*** Update File: "):
			paths = append(paths, strings.TrimPrefix(line, "*** Update File: "))
		case strings.HasPrefix(line, "*** Delete File: "):
			paths = append(paths, strings.TrimPrefix(line, "*** Delete File: "))
		case strings.HasPrefix(line, "*** Move to: "):
			paths = append(paths, strings.TrimPrefix(line, "*** Move to: "))
		case strings.HasPrefix(line, "+"):
			added = append(added, strings.TrimPrefix(line, "+"))
		}
	}
	return paths, strings.Join(added, "\n")
}
