package domain

import (
	"path"
	"regexp"
	"strings"
)

// Ficha de auditoría (HCD-046): una memoria por informe de docs/audits con su
// veredicto, sus hallazgos y su estado de cierre. El parser es tolerante: los
// informes no comparten plantilla, y uno sin tabla reconocible conserva al
// menos título y resumen.

// AuditDigestTopicPrefix agrupa las fichas de auditoría por topic_key.
const AuditDigestTopicPrefix = "audit/"

// AuditDigestTopic es la clave de upsert de la ficha de un informe.
func AuditDigestTopic(file string) string { return AuditDigestTopicPrefix + path.Base(file) }

// AuditFinding es una fila de la tabla de hallazgos.
type AuditFinding struct {
	ID, Severity, Problem, Status string
}

// AuditReport es la lectura de un informe.
type AuditReport struct {
	File, Title, Date, Verdict, Summary string
	Findings                            []AuditFinding
	Files                               []string
}

var (
	auditDate    = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)
	auditVerdict = regexp.MustCompile(`VERDICT:\s*(.+)$`)
	auditHeading = regexp.MustCompile(`^#{2,4}\s+`)
	auditTableID = regexp.MustCompile(`^[A-Z]{1,3}-\d+`)
)

// ParseAuditReport lee un informe de auditoría. file es su ruta relativa.
func ParseAuditReport(file string, data []byte) AuditReport {
	r := AuditReport{File: file}
	if m := auditDate.FindStringSubmatch(path.Base(file)); m != nil {
		r.Date = m[1]
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	section := ""
	status := map[string]string{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case r.Title == "" && strings.HasPrefix(line, "# "):
			r.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		case auditHeading.MatchString(line):
			section = strings.ToLower(line)
		case strings.HasPrefix(line, "|") && (strings.Contains(section, "hallazgos") || strings.Contains(section, "estado de cierre")):
			header, rows, next := readTable(lines, i)
			i = next - 1
			if strings.Contains(section, "estado de cierre") {
				for _, row := range rows {
					if id := auditTableID.FindString(row[0]); id != "" && len(row) > 1 {
						status[id] = row[1]
					}
				}
			} else if len(r.Findings) == 0 {
				r.Findings = findingsFrom(header, rows)
			}
		case r.Verdict == "" && auditVerdict.MatchString(line):
			r.Verdict = strings.Trim(auditVerdict.FindStringSubmatch(line)[1], "* ")
		case r.Summary == "" && r.Title != "" && line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "|"):
			r.Summary = line
		}
	}
	for i, f := range r.Findings {
		if st, ok := status[f.ID]; ok {
			r.Findings[i].Status = st
		}
	}
	seen := map[string]bool{}
	addCitedPaths(seen, text)
	r.Files = sortedKeys(seen)
	return r
}

// readTable lee una tabla markdown desde la línea start y devuelve su
// cabecera, sus filas (sin la de separación) y la línea siguiente a la tabla.
func readTable(lines []string, start int) ([]string, [][]string, int) {
	var header []string
	var rows [][]string
	i := start
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}
		if strings.Trim(strings.Join(cells, ""), "-: ") == "" {
			continue
		}
		if header == nil {
			header = cells
			continue
		}
		rows = append(rows, cells)
	}
	return header, rows, i
}

// findingsFrom toma el ID de la primera columna y localiza severidad,
// problema y estado por el nombre de su cabecera.
func findingsFrom(header []string, rows [][]string) []AuditFinding {
	col := func(name string) int {
		for i, h := range header {
			if strings.Contains(strings.ToLower(h), name) {
				return i
			}
		}
		return -1
	}
	sev, prob, st := col("severidad"), col("problema"), col("estado")
	if prob < 0 {
		prob = len(header) - 1
	}
	cell := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return row[i]
	}
	var out []AuditFinding
	for _, row := range rows {
		id := auditTableID.FindString(row[0])
		if id == "" {
			continue
		}
		out = append(out, AuditFinding{ID: id, Severity: cell(row, sev), Problem: cell(row, prob), Status: cell(row, st)})
	}
	return out
}

// RenderAuditDigest arma la ficha de un informe con el mismo tope que la de
// una unidad HCD.
func RenderAuditDigest(r AuditReport) HistoryDigest {
	title := r.Title
	if title == "" {
		title = path.Base(r.File)
	}
	lines := []string{"Informe: " + r.File}
	if r.Verdict != "" {
		lines = append(lines, "Veredicto: "+digestText(r.Verdict))
	}
	if len(r.Findings) == 0 && r.Summary != "" {
		lines = append(lines, "Resumen: "+truncRunes(digestText(r.Summary), hcdDigestSectionRunes))
	}
	if len(r.Findings) > 0 {
		lines = append(lines, "Hallazgos:")
		for _, f := range r.Findings {
			line := "- " + f.ID
			if f.Severity != "" {
				line += " [" + f.Severity + "]"
			}
			line += " " + truncRunes(digestText(f.Problem), hcdDigestSectionRunes/2)
			if f.Status != "" {
				line += " → " + truncRunes(digestText(f.Status), hcdDigestSectionRunes/4)
			}
			lines = append(lines, line)
		}
	}
	lines = appendFilesLine(lines, r.Files)
	prefix := "Auditoría"
	if r.Date != "" {
		prefix += " " + r.Date
	}
	return HistoryDigest{
		Title:   prefix + " · " + ProtectHCDField(title),
		Content: truncRunes(strings.Join(lines, "\n"), HCDDigestMaxContent),
		Files:   r.Files,
	}
}
