package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Ficha HCD (HCD-046): una memoria por unidad con lo decidido, derivada del
// documento y de las aprobaciones. La memoria nunca es la fuente: la ficha se
// regenera entera en cada paso y en el backfill.

// HCDDigestTopicPrefix agrupa las fichas HCD por topic_key.
const HCDDigestTopicPrefix = "hcd/"

// HCDDigestMaxContent es el tope de la ficha, para que el contexto la cite
// sin desplazar a lo demás.
const HCDDigestMaxContent = 2000

// Topes por parte de la ficha.
const (
	hcdDigestSectionRunes  = 400
	hcdDigestQuestionRunes = 140
	hcdDigestAnswerRunes   = 220
	hcdDigestMaxFiles      = 15
)

// HCDDigestTopic es la clave de upsert de la ficha de una unidad.
func HCDDigestTopic(unitID string) string { return HCDDigestTopicPrefix + unitID }

// HistoryDigest es una ficha del historial (unidad HCD o auditoría) lista
// para guardar, con los archivos que cita para enlazarla con ellos.
type HistoryDigest struct {
	Title   string
	Content string
	Files   []string
}

var unitStateLabels = map[UnitState]string{UnitActive: "activa", UnitClosed: "cerrada", UnitCancelled: "cancelada"}

// RenderHCDDigest arma la ficha de una unidad. No incluye las citas de las
// aprobaciones (son la respuesta literal de la persona, no la decisión) ni la
// evidencia (salidas de comandos).
func RenderHCDDigest(u Unit, doc UnitDoc, approvals []Approval) HistoryDigest {
	state := unitStateLabels[u.State]
	if state == "" {
		state = string(u.State)
	}
	var lines []string
	add := func(label, text string) {
		if text = digestText(text); text != "" && !isPending(text) {
			lines = append(lines, label+": "+truncRunes(text, hcdDigestSectionRunes))
		}
	}
	header := fmt.Sprintf("Tipo: %s · Carril: %s · Estado: %s", u.Kind, u.Lane, state)
	if !u.CreatedAt.IsZero() {
		header += " · Inicio: " + u.CreatedAt.Format("2006-01-02")
	}
	// Orden por valor, no por el del documento: lo decidido (aclaraciones,
	// alternativa, aprobaciones) va antes que el contexto (alcance,
	// reproducción), para que el tope recorte primero lo menos valioso.
	lines = append(lines, header)
	add("Intención", doc.Sections[SectionIntent])
	add("Motivo del carril", u.Why)
	if len(doc.Clarifications) > 0 {
		lines = append(lines, "Aclaraciones:")
		for _, c := range doc.Clarifications {
			answer := c.Answer
			if c.Open || strings.TrimSpace(answer) == "" {
				answer = "(abierta)"
			}
			lines = append(lines, "- "+c.ID+". "+truncRunes(digestText(c.Question), hcdDigestQuestionRunes)+" → "+truncRunes(digestText(answer), hcdDigestAnswerRunes))
		}
	}
	add("Alternativa descartada", doc.Sections[SectionCheapest])
	var decided []string
	for _, a := range approvals {
		switch a.Action {
		case ApprovalApprove:
			decided = append(decided, "definición aprobada "+a.Revision)
		case ApprovalClose:
			decided = append(decided, "cierre "+a.Revision)
		case ApprovalCancel:
			decided = append(decided, "cancelación "+a.Revision)
		}
	}
	if len(decided) > 0 {
		lines = append(lines, "Aprobaciones: "+strings.Join(decided, ", "))
	}
	add("Alcance", doc.Sections[SectionScope])
	add("Reproducción", doc.Sections[SectionReproduction])
	files := CitedPaths(doc)
	lines = appendFilesLine(lines, files)
	return HistoryDigest{
		Title:   fmt.Sprintf("%s · %s (%s)", u.ID, u.Title, state),
		Content: truncRunes(strings.Join(lines, "\n"), HCDDigestMaxContent),
		Files:   files,
	}
}

// appendFilesLine añade la línea «Archivos:» con un tope de rutas visibles;
// el enlace por archivo conserva la lista completa.
func appendFilesLine(lines, files []string) []string {
	if len(files) == 0 {
		return lines
	}
	shown, more := files, ""
	if len(shown) > hcdDigestMaxFiles {
		shown, more = shown[:hcdDigestMaxFiles], fmt.Sprintf(" y %d más", len(files)-hcdDigestMaxFiles)
	}
	return append(lines, "Archivos: "+strings.Join(shown, ", ")+more)
}

// digestText aplana una sección a una línea, sin comentarios HTML ni texto
// protegido: la ficha se protege en dominio, además de al persistirla.
func digestText(s string) string {
	return strings.Join(strings.Fields(ProtectText(stripComments(s))), " ")
}

var (
	unitDocID  = regexp.MustCompile(`^HCD-(\d{3,})$`)
	anyDocDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
)

// UnitFromDoc reconstruye una unidad que solo existe como documento (Q7 de
// HCD-046, p. ej. creada en otro worktree): id, título, tipo, carril y motivo
// de la cabecera; el estado de la sección Siguiente; la fecha de la primera
// evidencia. ok=false si la cabecera no trae un id HCD válido.
func UnitFromDoc(docPath string, doc UnitDoc) (Unit, bool) {
	id := strings.TrimSpace(doc.Fields["ID"])
	m := unitDocID.FindStringSubmatch(id)
	if m == nil {
		return Unit{}, false
	}
	var number int
	_, _ = fmt.Sscanf(m[1], "%d", &number)
	u := Unit{ID: id, Number: number, Title: doc.Title, Kind: Kind(doc.Fields["Tipo"]), Lane: Lane(doc.Fields["Carril"]), Why: doc.Fields["Motivo"], State: UnitActive, DocPath: docPath}
	next := strings.TrimSpace(stripComments(doc.Sections[SectionNext]))
	switch {
	case strings.HasPrefix(next, "Cerrada"):
		u.State = UnitClosed
	case strings.HasPrefix(next, "Cancelada"):
		u.State = UnitCancelled
	}
	if d := anyDocDate.FindString(doc.Sections[SectionEvidence]); d != "" {
		u.CreatedAt, _ = time.Parse("2006-01-02", d)
	}
	return u, true
}

var digestDate = regexp.MustCompile(`(?:Inicio: |^Auditoría )(\d{4}-\d{2}-\d{2})`)

// HistoryDigestDate es la fecha con la que se ordena una ficha: el inicio de
// la unidad o la fecha del informe. Vacía si la ficha no la lleva.
func HistoryDigestDate(m Memory) string {
	for _, s := range []string{m.Title, m.Content} {
		if d := digestDate.FindStringSubmatch(s); d != nil {
			return d[1]
		}
	}
	return ""
}

// hcdStepTitle reconoce los títulos exactos que escribía recordDecision antes
// de HCD-046. Solo esas memorias se consolidan en la ficha: cualquier otra que
// mencione una unidad (bugfix, discovery…) es conocimiento propio.
var hcdStepTitle = regexp.MustCompile(`^HCD-\d{3,}: (carril (fast|full)|aclaración Q\d+|definición aprobada|cerrada|cancelada|promovida a la ceremonia completa)$`)

// IsHCDStepMemoryTitle dice si un título es el de una memoria suelta de paso.
func IsHCDStepMemoryTitle(title string) bool { return hcdStepTitle.MatchString(title) }
