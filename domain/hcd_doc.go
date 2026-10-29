package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Secciones del documento de la unidad.
const (
	SectionIntent         = "Intención"
	SectionReproduction   = "Reproducción"
	SectionScope          = "Alcance"
	SectionInaction       = "¿Qué pasa si no lo hacemos?"
	SectionCheapest       = "Alternativa más barata"
	SectionStories        = "Historias"
	SectionClarifications = "Aclaraciones"
	SectionSlices         = "Slices"
	SectionChecklist      = "Checklist"
	SectionEvidence       = "Evidencia"
	SectionNext           = "Siguiente"
)

// definitionSections forman la revisión que aprueba approve; workSections
// añaden lo que se entrega y forman la revisión de la evidencia y el cierre.
var (
	definitionSections = []string{SectionIntent, SectionReproduction, SectionScope, SectionStories, SectionClarifications}
	workSections       = append(append([]string{}, definitionSections...), SectionSlices, SectionChecklist)
)

// UnitDoc es la lectura estructurada de hcd/HCD-NNN-<slug>.md.
type UnitDoc struct {
	Title          string
	Fields         map[string]string
	Sections       map[string]string
	Stories        []Story
	Clarifications []Clarification
	// Markers son las secciones con [NEEDS CLARIFICATION: …]; cuentan como
	// aclaraciones abiertas.
	Markers []string
	Slices  []Slice
	Tasks   []Task
}

// OpenClarifications lista las aclaraciones abiertas, incluidos los marcadores.
func (d UnitDoc) OpenClarifications() []string {
	var open []string
	for _, c := range d.Clarifications {
		if c.Open {
			open = append(open, c.ID)
		}
	}
	return append(open, d.Markers...)
}

var (
	htmlComment       = regexp.MustCompile(`(?s)<!--.*?-->`)
	checklistLine     = regexp.MustCompile(`^- \[([ xX])\] (\d+)\.\s+(.*)$`)
	clarificationLine = regexp.MustCompile(`^- \[([ xX])\] (Q\d+)\.\s+(.*)$`)
	storyHeading      = regexp.MustCompile(`^### (US\d+)\.\s*(.*)$`)
	sliceHeading      = regexp.MustCompile(`^### (VS-\d+)\.\s*(.*)$`)
	criterionLine     = regexp.MustCompile(`^- (C-\d+):\s*(.*)$`)
	checkboxMark      = regexp.MustCompile(`^- \[[xX]\]`)
	needsClarify      = regexp.MustCompile(`(?i)\[needs clarification`)
)

// ParseUnitDoc lee el documento y enumera los faltantes. Solo comprueba que el
// contenido esté presente y sin marcadores de pendiente, no que sea correcto.
func ParseUnitDoc(data []byte, kind Kind, lane Lane) (UnitDoc, []PendingCheck) {
	doc := UnitDoc{Fields: map[string]string{}, Sections: map[string]string{}}
	current := ""
	var body []string
	flush := func() {
		if current != "" {
			doc.Sections[current] = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		case current == "" && strings.HasPrefix(line, "# "):
			doc.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		case current == "":
			if key, value, ok := strings.Cut(line, ":"); ok {
				doc.Fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		default:
			body = append(body, line)
		}
	}
	flush()

	var pending []PendingCheck
	missing := func(field, message string) {
		pending = append(pending, PendingCheck{Code: "DOC-MISSING", Field: field, Message: message})
	}
	required := []string{SectionIntent, SectionScope, SectionInaction, SectionCheapest}
	if kind == KindFix {
		required = append(required, SectionReproduction)
	}
	for _, name := range required {
		text := stripComments(doc.Sections[name])
		if needsClarify.MatchString(text) {
			doc.Markers = append(doc.Markers, name)
			continue
		}
		if isPending(text) {
			missing(name, "falta la sección «"+name+"»")
		}
	}
	if scope, ok := doc.Sections[SectionScope]; ok && !needsClarify.MatchString(scope) {
		for _, key := range []string{"Incluido", "Excluido"} {
			if isPending(lineValue(scope, key)) {
				missing(SectionScope+"."+key, "falta «"+key+"» en el alcance")
			}
		}
	}
	doc.Clarifications = parseClarifications(stripComments(doc.Sections[SectionClarifications]))
	if lane == LaneFull {
		var checks []PendingCheck
		doc.Stories, checks = parseStories(stripComments(doc.Sections[SectionStories]))
		pending = append(pending, checks...)
		doc.Slices = parseSlices(stripComments(doc.Sections[SectionSlices]))
	}
	tasks, taskChecks := parseChecklist(stripComments(doc.Sections[SectionChecklist]))
	doc.Tasks = tasks
	pending = append(pending, taskChecks...)
	return doc, pending
}

func parseStories(section string) ([]Story, []PendingCheck) {
	var stories []Story
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if m := storyHeading.FindStringSubmatch(line); m != nil {
			stories = append(stories, Story{ID: m[1], Title: m[2]})
			continue
		}
		if len(stories) == 0 || line == "" {
			continue
		}
		s := &stories[len(stories)-1]
		if m := criterionLine.FindStringSubmatch(line); m != nil {
			s.Criteria = append(s.Criteria, Criterion{ID: m[1], Text: strings.TrimSpace(m[2])})
		} else if s.Text == "" {
			s.Text = line
		}
	}
	var pending []PendingCheck
	if len(stories) == 0 {
		pending = append(pending, PendingCheck{Code: "STORIES-EMPTY", Field: SectionStories, Message: "el carril completo necesita al menos una historia con criterios"})
	}
	for _, s := range stories {
		if isPending(s.Text) {
			pending = append(pending, PendingCheck{Code: "STORY-TEXT", Field: s.ID, Message: "la historia " + s.ID + " no dice «Como…, quiero…, para…»"})
		}
		if len(s.Criteria) == 0 {
			pending = append(pending, PendingCheck{Code: "STORY-CRITERIA", Field: s.ID, Message: "la historia " + s.ID + " no tiene criterios"})
		}
		for _, c := range s.Criteria {
			if isPending(c.Text) {
				pending = append(pending, PendingCheck{Code: "CRITERION-TEXT", Field: c.ID, Message: "el criterio " + c.ID + " no dice «Dado…, cuando…, entonces…»"})
			}
		}
	}
	return stories, pending
}

func parseClarifications(section string) []Clarification {
	var out []Clarification
	for _, raw := range strings.Split(section, "\n") {
		m := clarificationLine.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		question, answer, _ := strings.Cut(m[3], "→")
		out = append(out, Clarification{ID: m[2], Question: strings.TrimSpace(question), Answer: strings.TrimSpace(answer), Open: m[1] == " "})
	}
	return out
}

func parseSlices(section string) []Slice {
	var slices []Slice
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if m := sliceHeading.FindStringSubmatch(line); m != nil {
			slices = append(slices, Slice{ID: m[1], Title: m[2]})
			continue
		}
		if len(slices) == 0 {
			continue
		}
		s := &slices[len(slices)-1]
		if v, ok := strings.CutPrefix(line, "Tareas:"); ok {
			s.Tasks = splitRefs(v)
		} else if v, ok := strings.CutPrefix(line, "Criterios:"); ok {
			s.Criteria = splitRefs(v)
		}
	}
	return slices
}

func parseChecklist(section string) ([]Task, []PendingCheck) {
	var tasks []Task
	var pending []PendingCheck
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- [") {
			continue
		}
		m := checklistLine.FindStringSubmatch(line)
		if m == nil {
			pending = append(pending, PendingCheck{Code: "CHECKLIST-FORMAT", Field: SectionChecklist, Message: "línea sin el formato «- [ ] N. tarea → comprobación»: " + line})
			continue
		}
		text, check, ok := strings.Cut(m[3], "→")
		if !ok {
			text, check, ok = strings.Cut(m[3], "->")
		}
		task := Task{ID: m[2], Text: strings.TrimSpace(text), Check: strings.TrimSpace(check), Done: m[1] != " "}
		if !ok || isPending(task.Check) {
			pending = append(pending, PendingCheck{Code: "CHECK-MISSING", Field: SectionChecklist + "." + task.ID, Message: "la tarea " + task.ID + " no dice qué la demuestra"})
		}
		tasks = append(tasks, task)
	}
	return tasks, pending
}

// DefinitionRevision identifica lo que aprueba approve.
func DefinitionRevision(doc UnitDoc) string { return sectionsRevision(doc, definitionSections) }

// WorkRevision identifica lo que se evalúa: la definición más slices y tareas.
// Las casillas del checklist se normalizan para que marcar una tarea no
// invalide la evidencia que la marcó.
func WorkRevision(doc UnitDoc) string { return sectionsRevision(doc, workSections) }

func sectionsRevision(doc UnitDoc, names []string) string {
	var b strings.Builder
	for _, name := range names {
		b.WriteString("## " + name + "\n")
		for _, line := range strings.Split(stripComments(doc.Sections[name]), "\n") {
			line = strings.TrimSpace(line)
			if name == SectionChecklist {
				line = checkboxMark.ReplaceAllString(line, "- [ ]")
			}
			b.WriteString(line + "\n")
		}
	}
	return ContentRevision([]byte(b.String()))
}

// RenderUnitTemplate rellena la plantilla. Los bloques <!-- fix --> solo
// quedan en unidades fix y los <!-- full --> solo en el carril completo.
func RenderUnitTemplate(template string, unit Unit, intent string) string {
	var out []string
	block := ""
	for _, line := range strings.Split(template, "\n") {
		switch strings.TrimSpace(line) {
		case "<!-- fix -->", "<!-- full -->":
			block = strings.Trim(strings.TrimSpace(line), "<!-> ")
			continue
		case "<!-- /fix -->", "<!-- /full -->":
			block = ""
			continue
		}
		if (block == "fix" && unit.Kind != KindFix) || (block == "full" && unit.Lane != LaneFull) {
			continue
		}
		out = append(out, line)
	}
	if strings.TrimSpace(intent) == "" {
		intent = "pendiente"
	}
	return strings.NewReplacer(
		"{TITLE}", ProtectHCDField(unit.Title),
		"{ID}", unit.ID,
		"{KIND}", string(unit.Kind),
		"{LANE}", string(unit.Lane),
		"{WHY}", ProtectHCDField(unit.Why),
		"{INTENT}", strings.TrimSpace(ProtectText(intent)),
	).Replace(strings.Join(out, "\n"))
}

// FullSectionsTemplate es lo que promote añade a un documento fast.
const FullSectionsTemplate = "### US1. pendiente\npendiente\n- C-01: pendiente"

// ReplaceSection sustituye el cuerpo de una sección `## name`; si no existe,
// la inserta antes de la sección `before` o, sin ella, al final.
func ReplaceSection(data []byte, name, body, before string) []byte {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	start, end, insertAt := -1, len(lines), -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 && trimmed == "## "+name {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(line, "## ") {
			end = i
			break
		}
		if insertAt < 0 && before != "" && trimmed == "## "+before {
			insertAt = i
		}
	}
	block := []string{"## " + name, strings.TrimRight(body, "\n"), ""}
	switch {
	case start >= 0:
		return []byte(strings.Join(append(append(append([]string{}, lines[:start]...), block...), lines[end:]...), "\n"))
	case insertAt >= 0:
		return []byte(strings.Join(append(append(append([]string{}, lines[:insertAt]...), block...), lines[insertAt:]...), "\n"))
	default:
		return []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n\n" + strings.Join(block, "\n"))
	}
}

// MarkTasksDone marca [x] las tareas del checklist indicadas.
func MarkTasksDone(data []byte, ids []string) []byte {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if m := checklistLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil && want[m[2]] && m[1] == " " {
			lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// AddClarification añade una pregunta abierta con el siguiente Qn libre.
func AddClarification(data []byte, doc UnitDoc, question string) ([]byte, string) {
	id := fmt.Sprintf("Q%d", len(doc.Clarifications)+1)
	body := stripComments(doc.Sections[SectionClarifications])
	if isPending(body) {
		body = ""
	}
	body = strings.TrimSpace(body + "\n- [ ] " + id + ". " + ProtectHCDField(question))
	return ReplaceSection(data, SectionClarifications, body, SectionSlices), id
}

// AnswerClarification resuelve la pregunta id con la respuesta dada.
func AnswerClarification(data []byte, id, answer string) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		m := clarificationLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || m[2] != id {
			continue
		}
		if m[1] != " " {
			return nil, fmt.Errorf("%w: la aclaración %s ya está resuelta", ErrNotAllowed, id)
		}
		question, _, _ := strings.Cut(m[3], "→")
		lines[i] = "- [x] " + id + ". " + strings.TrimSpace(question) + " → " + ProtectHCDField(answer)
		return []byte(strings.Join(lines, "\n")), nil
	}
	return nil, fmt.Errorf("%w: no existe la aclaración %s", ErrNotFound, id)
}

func stripComments(s string) string { return strings.TrimSpace(htmlComment.ReplaceAllString(s, "")) }

func lineValue(section, key string) string {
	for _, line := range strings.Split(section, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func splitRefs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" && !isPending(part) {
			out = append(out, part)
		}
	}
	return out
}

// isPending: vacío o marcador de pendiente cuenta como faltante.
func isPending(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "pendiente", "[pendiente]", "todo", "…", "...":
		return true
	}
	return false
}

var slugFold = map[rune]rune{'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e', 'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i', 'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u', 'ñ': 'n', 'ç': 'c'}

// Slugify deriva el nombre de archivo de un título: minúsculas ASCII y guiones.
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if plain, ok := slugFold[r]; ok {
			r = plain
		}
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		return "unidad"
	}
	return slug
}
