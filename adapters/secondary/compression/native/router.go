package native

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"mem/domain"
)

// block es un fragmento contiguo de la entrada con su tipo detectado. fenced
// indica que venía entre cercas Markdown: open y close son las líneas de cerca
// literales y content lo que hay entre ellas.
type block struct {
	content string
	typ     domain.ContentType
	lang    string
	fenced  bool
	open    string
	close   string
}

// fenceLine reconoce la línea de apertura o cierre de una cerca Markdown.
var fenceLine = regexp.MustCompile("^\\s*```([A-Za-z0-9_+-]*)\\s*$")

// segment divide la entrada en bloques (research.md R2): cada cerca ``` es un
// bloque con el tipo de su etiqueta y el texto entre cercas es otro bloque. Una
// cerca sin cerrar deja el resto como texto normal: nunca se inventa un cierre.
func segment(input string) []block {
	lines := strings.SplitAfter(input, "\n")
	var blocks []block
	var text strings.Builder
	flushText := func() {
		if text.Len() > 0 {
			c := text.String()
			typ, lang := classify(c)
			blocks = append(blocks, block{content: c, typ: typ, lang: lang})
			text.Reset()
		}
	}
	for i := 0; i < len(lines); i++ {
		m := fenceLine.FindStringSubmatch(strings.TrimRight(lines[i], "\n"))
		if m == nil {
			text.WriteString(lines[i])
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if fenceLine.MatchString(strings.TrimRight(lines[j], "\n")) {
				end = j
				break
			}
		}
		if end < 0 {
			text.WriteString(lines[i])
			continue
		}
		flushText()
		inner := strings.Join(lines[i+1:end], "")
		typ, lang := classifyFenced(m[1], inner)
		blocks = append(blocks, block{content: inner, typ: typ, lang: lang, fenced: true, open: lines[i], close: lines[end]})
		i = end
	}
	flushText()
	return blocks
}

// classifyFenced usa la etiqueta de lenguaje de la cerca y, si no hay o no se
// reconoce, detecta por contenido.
func classifyFenced(tag, content string) (domain.ContentType, string) {
	switch strings.ToLower(tag) {
	case "json", "jsonl", "ndjson":
		if t, _ := classify(content); t == domain.ContentJSON {
			return domain.ContentJSON, ""
		}
		return domain.ContentProse, ""
	case "go", "golang":
		return domain.ContentCode, domain.LangGo
	case "python", "py":
		return domain.ContentCode, domain.LangPython
	case "java", "kotlin":
		return domain.ContentCode, domain.LangJava
	case "ts", "typescript", "tsx":
		return domain.ContentCode, domain.LangTS
	case "js", "javascript", "jsx":
		return domain.ContentCode, domain.LangJS
	case "sql":
		return domain.ContentCode, domain.LangSQL
	case "diff", "patch":
		return domain.ContentDiff, ""
	case "log", "text", "txt", "console", "shell", "sh", "bash":
		if t, l := classify(content); t != domain.ContentProse {
			return t, l
		}
		return domain.ContentLog, ""
	}
	return classify(content)
}

var (
	gitCommitLine = regexp.MustCompile(`^commit [0-9a-f]{7,40}\b`)
	diffHeader    = regexp.MustCompile(`(?m)^(diff --git |@@ -\d+(,\d+)? \+\d+(,\d+)? @@)`)
	logLevel      = regexp.MustCompile(`(?i)\b(ERROR|WARN|WARNING|INFO|DEBUG|TRACE|FATAL|PANIC)\b`)
	logTimestamp  = regexp.MustCompile(`^\s*(\[)?\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}`)
	traceMarker   = regexp.MustCompile(`^(goroutine \d+ \[|Traceback \(most recent call last\)|panic: |\s+at [\w.$]+\(|Exception in thread |\s+File ".*", line \d+)`)
	testLogLine   = regexp.MustCompile(`^(=== (RUN|PAUSE|CONT)|--- (PASS|FAIL|SKIP)|\s+\S+_test\.go:\d+:|(ok|FAIL)\s+\S+\s)`)
)

// classify detecta el tipo de un bloque sin cercas, por precedencia (R2):
// JSON → JSON Lines → diff → log/traza → código → tabla → prosa.
func classify(content string) (domain.ContentType, string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return domain.ContentProse, ""
	}
	if (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
		return domain.ContentJSON, ""
	}
	lines := nonBlankLines(trimmed)
	if isJSONLines(lines) {
		return domain.ContentJSON, "jsonl"
	}
	if diffHeader.MatchString(trimmed) {
		return domain.ContentDiff, ""
	}
	if isLog(lines) {
		return domain.ContentLog, ""
	}
	if lang := detectCode(trimmed, lines); lang != "" {
		return domain.ContentCode, lang
	}
	if isTable(lines) {
		return domain.ContentTable, ""
	}
	return domain.ContentProse, ""
}

func nonBlankLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// isJSONLines: al menos 3 líneas y el 90 % son objetos JSON válidos.
func isJSONLines(lines []string) bool {
	if len(lines) < 3 {
		return false
	}
	ok := 0
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "{") && json.Valid([]byte(t)) {
			ok++
		}
	}
	return ok*10 >= len(lines)*9
}

// isLog: el 30 % de las líneas tiene nivel, marca de tiempo inicial, forma de
// salida de pruebas o de traza; o es la salida de `git log`.
func isLog(lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	if gitCommitLine.MatchString(lines[0]) {
		return true
	}
	hits := 0
	for _, l := range lines {
		head := l
		if len(head) > 120 {
			head = head[:120]
		}
		if logLevel.MatchString(head) || logTimestamp.MatchString(l) || traceMarker.MatchString(l) || testLogLine.MatchString(l) {
			hits++
		}
	}
	return hits*10 >= len(lines)*3
}

var codePatterns = map[string]*regexp.Regexp{
	domain.LangPython: regexp.MustCompile(`^\s*(def |class |import |from \S+ import |return\b|elif |except\b|@\w+|if .*:\s*$|for .*:\s*$|with .*:\s*$)`),
	domain.LangJava:   regexp.MustCompile(`^\s*(public |private |protected |package [\w.]+;|import [\w.*]+;|@\w+|return\b.*;|\}\s*$|.*;\s*$)`),
	domain.LangTS:     regexp.MustCompile(`^\s*(export |import .* from |const |let |interface |type \w+ =|function |async |return\b|\}\s*$|.*;\s*$|.*=>)`),
	domain.LangSQL:    regexp.MustCompile(`(?i)^\s*(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|WITH)\b`),
}

// detectCode devuelve el lenguaje si el bloque es código con suficiente
// confianza, o "" si no lo es. Go se confirma parseándolo (go/parser); el
// resto, por proporción de líneas con forma de código del lenguaje.
func detectCode(trimmed string, lines []string) string {
	if strings.HasPrefix(trimmed, "package ") {
		if _, err := parser.ParseFile(token.NewFileSet(), "", trimmed, parser.ParseComments); err == nil {
			return domain.LangGo
		}
	}
	if len(lines) < 4 {
		return ""
	}
	best, bestScore := "", 0.0
	for _, lang := range []string{domain.LangPython, domain.LangJava, domain.LangTS, domain.LangSQL} {
		n := 0
		for _, l := range lines {
			if codePatterns[lang].MatchString(l) {
				n++
			}
		}
		score := float64(n) / float64(len(lines))
		if score > bestScore {
			best, bestScore = lang, score
		}
	}
	if bestScore < 0.35 {
		return ""
	}
	// Java y TS comparten llaves y punto y coma: desempata por palabras
	// propias de cada uno.
	if best == domain.LangJava || best == domain.LangTS {
		if strings.Contains(trimmed, "public class ") || strings.Contains(trimmed, "import java.") || strings.Contains(trimmed, "package ") && strings.Contains(trimmed, ";") {
			return domain.LangJava
		}
		return domain.LangTS
	}
	return best
}

var alignedColumns = regexp.MustCompile(`\S {2,}\S.* {2,}\S`)

// isTable: el 60 % de las líneas comparte el mismo número de separadores | o
// tabuladores, o columnas alineadas con dos o más espacios.
func isTable(lines []string) bool {
	if len(lines) < 4 {
		return false
	}
	counts := map[int]int{}
	tabs, aligned := 0, 0
	for _, l := range lines {
		if n := strings.Count(l, "|"); n > 0 {
			counts[n]++
		}
		if strings.Count(l, "\t") >= 2 {
			tabs++
		}
		if alignedColumns.MatchString(l) {
			aligned++
		}
	}
	threshold := len(lines) * 6
	for _, c := range counts {
		if c*10 >= threshold {
			return true
		}
	}
	return tabs*10 >= threshold || aligned*10 >= threshold
}
