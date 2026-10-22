package domain

import "strings"

// Feature 040 — mem expand: supertools inspiradas en ECC (affaan-m/ECC, MIT),
// reimplementadas en Go. Este archivo es el núcleo puro: el registro de
// supertools, las señales de un evento de hook y el router que decide, una
// sola vez por evento, qué acciones corresponden (contracts/router.md).

// SupertoolName nombra una supertool del módulo expand.
type SupertoolName string

const (
	SupertoolRules     SupertoolName = "rules"
	SupertoolShield    SupertoolName = "shield"
	SupertoolFlow      SupertoolName = "flow"
	SupertoolInstincts SupertoolName = "instincts"
)

// Supertools es el registro cerrado, en el orden en que se presentan.
var Supertools = []SupertoolName{SupertoolRules, SupertoolShield, SupertoolFlow, SupertoolInstincts}

// ParseSupertoolName valida un nombre de supertool (FR-011).
func ParseSupertoolName(s string) (SupertoolName, bool) {
	for _, n := range Supertools {
		if string(n) == s {
			return n, true
		}
	}
	return "", false
}

// ExpandEvent es el evento normalizado de `mem hook expand <event>`.
type ExpandEvent string

const (
	ExpandSessionStart ExpandEvent = "session-start"
	ExpandPrompt       ExpandEvent = "prompt"
	ExpandPreTool      ExpandEvent = "pre-tool"
	ExpandPostTool     ExpandEvent = "post-tool"
	ExpandStop         ExpandEvent = "stop"
)

// ParseExpandEvent valida el argumento de `mem hook expand`. Un valor
// desconocido devuelve ok=false: el hook permite sin emitir nada.
func ParseExpandEvent(s string) (ExpandEvent, bool) {
	switch ExpandEvent(s) {
	case ExpandSessionStart, ExpandPrompt, ExpandPreTool, ExpandPostTool, ExpandStop:
		return ExpandEvent(s), true
	}
	return "", false
}

// ToolKind agrupa las herramientas de los tres harnesses por lo que hacen.
type ToolKind string

const (
	ToolShell    ToolKind = "shell"
	ToolFile     ToolKind = "file"
	ToolDelegate ToolKind = "delegate"
	ToolOther    ToolKind = "other"
)

// toolKinds es la tabla cerrada de nombres de herramienta. Los de Codex
// están verificados en una sesión interactiva real
// (specs/040-expand-supertools/evidence/codex-probe.md).
var toolKinds = map[string]ToolKind{
	// Claude Code
	"Bash": ToolShell, "Write": ToolFile, "Edit": ToolFile, "MultiEdit": ToolFile,
	"NotebookEdit": ToolFile, "Task": ToolDelegate, "Agent": ToolDelegate,
	// Codex
	"apply_patch": ToolFile, "collaborationspawn_agent": ToolDelegate,
	// OpenCode
	"bash": ToolShell, "write": ToolFile, "edit": ToolFile, "patch": ToolFile, "task": ToolDelegate,
}

// ClassifyTool clasifica un nombre de herramienta; los desconocidos son
// ToolOther, que no dispara ninguna acción en pre-tool.
func ClassifyTool(name string) ToolKind {
	if k, ok := toolKinds[name]; ok {
		return k
	}
	return ToolOther
}

// Nombres de las flow-skills que el router puede sugerir.
const (
	SkillVerifyLoop         = "verify-loop"
	SkillSearchFirst        = "search-first"
	SkillIterativeRetrieval = "iterative-retrieval"
	SkillStrategicCompact   = "strategic-compact"
	SkillInstincts          = "instincts"
)

// Umbral de herramientas interceptadas tras el que se sugiere
// strategic-compact. Fuera de [min, max] se usa el default (FR-023).
const (
	ExpandDefaultCompactThreshold = 50
	ExpandMinCompactThreshold     = 10
	ExpandMaxCompactThreshold     = 1000
)

// ExpandSignals son las entradas del router para un evento.
type ExpandSignals struct {
	Event            ExpandEvent
	Source           string // startup|resume|clear|compact (solo session-start)
	Tool             ToolKind
	Command          string
	FilePath         string
	Prompt           string
	Content          string // contenido nuevo de una escritura de archivo (o el parche de Codex)
	LastMessage      string // último mensaje del agente (stop)
	ToolCount        int
	CompactThreshold int
	ConfigHash       string
	LastShieldHash   string
}

// ExpandSwitches refleja el interruptor de emergencia (FR-011/FR-012).
type ExpandSwitches struct {
	Disabled bool
	Off      map[SupertoolName]bool
}

func (sw ExpandSwitches) on(n SupertoolName) bool { return !sw.Off[n] }

// ActionKind es el tipo de una acción de supertool.
type ActionKind string

const (
	ActionRulesEval        ActionKind = "rules.eval"
	ActionShieldScan       ActionKind = "shield.scan"
	ActionSuggest          ActionKind = "suggest"
	ActionInstinctsObserve ActionKind = "instincts.observe"
	ActionInstinctsFold    ActionKind = "instincts.fold"
	ActionCounterReset     ActionKind = "counter.reset"
	ActionEpisodeReset     ActionKind = "episode.reset"
)

// Objetivos de rules.eval.
const (
	RuleTargetBash   = "bash"
	RuleTargetFile   = "file"
	RuleTargetPrompt = "prompt"
	RuleTargetStop   = "stop"
)

// SupertoolAction es una acción decidida por el router. Target es el
// evento de reglas para rules.eval o el nombre de la skill para suggest.
type SupertoolAction struct {
	Kind   ActionKind
	Target string
}

// RouteSupertools decide qué acciones corresponden a un evento. Es pura y
// determinista; la invoca el adaptador una sola vez por evento. El orden es
// estable (contracts/router.md, I4): rules, shield, sugerencias, observación,
// plegado y resets. Ante la duda no sugiere nada (I6). La inyección de
// instincts no pasa por aquí: la hace build_context (research R9).
func RouteSupertools(sig ExpandSignals, sw ExpandSwitches) []SupertoolAction {
	if sw.Disabled {
		return nil
	}
	var rules, shield, suggest, observe, fold, resets []SupertoolAction
	evalRules := func(target string) {
		if sw.on(SupertoolRules) {
			rules = append(rules, SupertoolAction{Kind: ActionRulesEval, Target: target})
		}
	}
	suggestSkill := func(skill string) {
		if sw.on(SupertoolFlow) {
			suggest = append(suggest, SupertoolAction{Kind: ActionSuggest, Target: skill})
		}
	}
	observeInstinct := func() {
		if sw.on(SupertoolInstincts) {
			observe = append(observe, SupertoolAction{Kind: ActionInstinctsObserve})
		}
	}
	foldInstincts := func() {
		if sw.on(SupertoolInstincts) {
			fold = append(fold, SupertoolAction{Kind: ActionInstinctsFold})
		}
	}

	switch sig.Event {
	case ExpandSessionStart:
		if sw.on(SupertoolShield) && sig.ConfigHash != "" && sig.ConfigHash != sig.LastShieldHash {
			shield = append(shield, SupertoolAction{Kind: ActionShieldScan})
		}
		foldInstincts()
		if sig.Source == "compact" {
			resets = append(resets, SupertoolAction{Kind: ActionCounterReset}, SupertoolAction{Kind: ActionEpisodeReset})
		}
	case ExpandPrompt:
		evalRules(RuleTargetPrompt)
		if MatchesDependencyIntent(sig.Prompt) {
			suggestSkill(SkillSearchFirst)
		}
		observeInstinct()
	case ExpandPreTool:
		switch sig.Tool {
		case ToolShell:
			evalRules(RuleTargetBash)
			if IsCommitCommand(sig.Command) {
				suggestSkill(SkillVerifyLoop)
			}
		case ToolFile:
			evalRules(RuleTargetFile)
		case ToolDelegate:
			suggestSkill(SkillIterativeRetrieval)
		}
		if sig.Tool != ToolOther && sig.ToolCount >= compactThreshold(sig.CompactThreshold) {
			suggestSkill(SkillStrategicCompact)
		}
	case ExpandPostTool:
		if sig.Tool == ToolShell {
			observeInstinct()
		}
	case ExpandStop:
		evalRules(RuleTargetStop)
		foldInstincts()
	default:
		return nil
	}

	var out []SupertoolAction
	for _, group := range [][]SupertoolAction{rules, shield, suggest, observe, fold, resets} {
		out = append(out, group...)
	}
	return out
}

func compactThreshold(n int) int {
	if n < ExpandMinCompactThreshold || n > ExpandMaxCompactThreshold {
		return ExpandDefaultCompactThreshold
	}
	return n
}

// IsCommitCommand reconoce `git commit`, `gh pr create` y `glab mr create`
// en cualquier subcomando de una línea de shell, por tokens (no subcadenas).
func IsCommitCommand(cmd string) bool {
	for _, toks := range SplitShellCommands(cmd) {
		toks = stripEnvAssignments(toks)
		if len(toks) == 0 {
			continue
		}
		switch toks[0] {
		case "git":
			if gitSubcommand(toks[1:]) == "commit" {
				return true
			}
		case "gh":
			if len(toks) >= 3 && toks[1] == "pr" && toks[2] == "create" {
				return true
			}
		case "glab":
			if len(toks) >= 3 && toks[1] == "mr" && toks[2] == "create" {
				return true
			}
		}
	}
	return false
}

// stripEnvAssignments quita las asignaciones VAR=valor iniciales y `sudo`.
func stripEnvAssignments(toks []string) []string {
	for len(toks) > 0 {
		t := toks[0]
		if t == "sudo" || isEnvAssignment(t) {
			toks = toks[1:]
			continue
		}
		break
	}
	return toks
}

// gitSubcommand devuelve el subcomando de git saltando sus opciones globales
// (`-C dir`, `-c k=v` y cualquier otra que empiece por guion).
func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

var (
	dependencyVerbs = map[string]bool{
		"añade": true, "añadir": true, "agrega": true, "agregar": true, "instala": true, "instalar": true,
		"integra": true, "integrar": true, "incorpora": true, "incorporar": true,
		"add": true, "install": true, "integrate": true, "adopt": true,
	}
	dependencyObjects = map[string]bool{
		"dependencia": true, "dependencias": true, "librería": true, "libreria": true, "librerías": true,
		"paquete": true, "paquetes": true, "sdk": true, "integración": true, "integracion": true, "mcp": true,
		"dependency": true, "dependencies": true, "library": true, "libraries": true, "package": true,
		"packages": true, "integration": true,
	}
	installCommands = []string{"go get", "npm install", "npm i ", "pnpm add", "yarn add", "pip install", "cargo add", "uv add"}
)

// MatchesDependencyIntent reconoce un prompt que pide añadir una
// dependencia o una integración (research R9): un verbo de la lista cerrada
// junto a un objeto de la lista cerrada, o un comando de instalación
// explícito. Ante la duda devuelve false.
func MatchesDependencyIntent(prompt string) bool {
	p := strings.ToLower(prompt)
	for _, c := range installCommands {
		if strings.Contains(p, c) {
			return true
		}
	}
	var verb, object bool
	for _, w := range strings.FieldsFunc(p, func(r rune) bool {
		return !(r == 'ñ' || r == 'á' || r == 'é' || r == 'í' || r == 'ó' || r == 'ú' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if dependencyVerbs[w] {
			verb = true
		}
		if dependencyObjects[w] {
			object = true
		}
	}
	return verb && object
}

// ExpandDecision es el veredicto agregado de un evento.
type ExpandDecision string

const (
	DecisionAllow ExpandDecision = "allow"
	DecisionWarn  ExpandDecision = "warn"
	DecisionBlock ExpandDecision = "block"
)

// ExpandOutcome es lo que el caso de uso entrega al adaptador para un
// evento: la decisión, su motivo, el contexto a inyectar (avisos y
// sugerencias ya filtrados) y las claves de lo que se activó, para el
// registro de activaciones. Por defecto: allow sin contexto.
type ExpandOutcome struct {
	Decision ExpandDecision
	Reason   string
	Rule     string
	Context  []string
	Fired    []string
	// Suggestions son las skills que el router propuso; el adaptador las
	// filtra por episodio y las convierte en texto con SuggestionText.
	Suggestions []string
	// Errors nombra las acciones que fallaron (sin contenido del usuario);
	// TimedOut indica que el plazo venció antes de terminar.
	Errors   []string
	TimedOut bool
}

// suggestionTexts es el texto fijo de cada sugerencia
// (contracts/hook-expand.md «Formato de la sugerencia»).
var suggestionTexts = map[string]string{
	SkillVerifyLoop:         "Antes de confirmar cambios, aplica la skill `verify-loop` (build, vet, test -race, lint, diff y bugs previos sobre los archivos tocados).",
	SkillSearchFirst:        "Antes de añadir una dependencia o integración, aplica la skill `search-first` (memoria, grafo, skills instaladas y, al final, fuera; decide Adopt/Extend/Build).",
	SkillIterativeRetrieval: "Antes de delegar, aplica la skill `iterative-retrieval` para entregar al subagente un contexto acotado (como mucho 3 ciclos).",
	SkillStrategicCompact:   "Llevas muchas herramientas desde la última compactación: valora la skill `strategic-compact` (guarda el resumen de sesión antes de compactar; nunca a mitad de implementación).",
}

// SuggestionText devuelve el texto de la sugerencia de una skill, o "" si
// la skill no tiene sugerencia.
func SuggestionText(skill string) string { return suggestionTexts[skill] }
