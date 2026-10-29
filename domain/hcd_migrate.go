package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Clases de un elemento de origen en una propuesta de migración (FR-054).
const (
	MigrationMigrated     = "migrated"
	MigrationConsolidated = "consolidated"
	MigrationReference    = "reference"
	MigrationDiscarded    = "discarded"
	MigrationPending      = "pending"
)

// Estados de una migración (data-model §2).
const (
	MigrationProposed  = "proposed"
	MigrationStale     = "stale"
	MigrationActivated = "activated"
)

var MigrationSourceFiles = []string{"spec.md", "plan.md", "tasks.md"}

type MigrationItem struct {
	Source        string `json:"source"`
	Kind          string `json:"kind"`
	Text          string `json:"text"`
	Class         string `json:"class"`
	Justification string `json:"justification,omitempty"`
}

type Migration struct {
	ID          string          `json:"id"`
	UnitID      string          `json:"unit_id"`
	SourceDir   string          `json:"source_dir"`
	SourceRev   string          `json:"source_rev"`
	Status      string          `json:"status"`
	Items       []MigrationItem `json:"items"`
	Counts      map[string]int  `json:"counts"`
	Revision    string          `json:"revision"`
	CreatedAt   string          `json:"created_at,omitempty"`
	ActivatedAt string          `json:"activated_at,omitempty"`
}

// MigrationSourceRevision identifica el contenido exacto de las fuentes: si
// cambia tras la propuesta, la activación exige una propuesta nueva (FR-055).
func MigrationSourceRevision(sources map[string][]byte) string {
	var all []byte
	for _, name := range MigrationSourceFiles {
		if content, ok := sources[name]; ok {
			all = append(all, []byte("\x00"+name+"\x00")...)
			all = append(all, content...)
		}
	}
	return ContentRevision(all)
}

// ProposalRevision es la revisión de la propuesta que la persona aprueba. Se
// calcula solo con el contenido clasificado, no con fechas ni estado.
func (m Migration) ProposalRevision() string {
	encoded, _ := json.Marshal(struct {
		Dir   string          `json:"dir"`
		Rev   string          `json:"rev"`
		Items []MigrationItem `json:"items"`
	}{m.SourceDir, m.SourceRev, m.Items})
	return ContentRevision(encoded)
}

func (m *Migration) Recount() {
	m.Counts = map[string]int{}
	for _, item := range m.Items {
		m.Counts[item.Class]++
	}
	m.Revision = m.ProposalRevision()
}

var (
	migrationHeading = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
	migrationList    = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*)$`)
	migrationTask    = regexp.MustCompile(`^\s*[-*]\s+\[([ xX])\]\s+(T\d+)\b`)
	migrationTable   = regexp.MustCompile(`^\s*\|`)
	migrationTableHr = regexp.MustCompile(`^\s*\|[\s:|-]+\|?\s*$`)
)

// ClassifyMigration clasifica cada encabezado, elemento de lista, fila de
// tabla, bloque de código y párrafo de las fuentes. Ninguna clase equivale a
// una decisión aprobada: la propuesta solo ordena el material para revisarlo.
func ClassifyMigration(sources map[string][]byte) []MigrationItem {
	var out []MigrationItem
	for _, file := range MigrationSourceFiles {
		content, ok := sources[file]
		if !ok {
			continue
		}
		section := ""
		fenced := false
		paragraph := -1
		for i, line := range strings.Split(string(content), "\n") {
			line = strings.TrimRight(line, "\r")
			emit := func(kind, text string) {
				class, why := classifyMigrationItem(file, section, kind, text)
				out = append(out, MigrationItem{Source: fmt.Sprintf("%s:%d", file, i+1), Kind: kind, Text: migrationText(text), Class: class, Justification: why})
			}
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				if !fenced {
					emit("code", line)
				}
				fenced = !fenced
				paragraph = -1
				continue
			}
			if fenced {
				continue
			}
			trimmed := strings.TrimSpace(line)
			switch {
			case trimmed == "":
				paragraph = -1
			case migrationHeading.MatchString(trimmed):
				section = migrationHeading.FindStringSubmatch(trimmed)[1]
				emit("heading", section)
				paragraph = -1
			case migrationList.MatchString(line):
				emit("item", trimmed)
				paragraph = -1
			case migrationTable.MatchString(line):
				if !migrationTableHr.MatchString(line) {
					emit("row", trimmed)
				}
				paragraph = -1
			default:
				if paragraph < 0 {
					emit("paragraph", trimmed)
					paragraph = i
				}
			}
		}
	}
	return out
}

func migrationText(text string) string {
	text = ProtectText(strings.TrimSpace(text))
	if r := []rune(text); len(r) > 200 {
		text = string(r[:200]) + "…"
	}
	return text
}

func inSection(section string, words ...string) bool {
	lower := strings.ToLower(section)
	for _, w := range words {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func classifyMigrationItem(file, section, kind, text string) (string, string) {
	if strings.Contains(text, "NEEDS CLARIFICATION") {
		return MigrationPending, "ambigüedad abierta: resolver con hcd-clarify antes de aprobar"
	}
	switch file {
	case "spec.md":
		if kind == "item" {
			switch {
			case strings.Contains(text, "**FR-"), strings.Contains(text, "**SC-"), strings.Contains(text, "**Given**"):
				return MigrationMigrated, ""
			case inSection(section, "assumption", "edge case", "out of scope", "supuesto"):
				return MigrationConsolidated, ""
			}
			return MigrationPending, "sin equivalente directo en la definición HCD: requiere revisión humana"
		}
		return MigrationConsolidated, ""
	case "plan.md":
		if inSection(section, "constitution check", "complexity tracking") {
			return MigrationDiscarded, "control propio de Spec Kit sin equivalente en HCD"
		}
		return MigrationReference, ""
	default:
		if inSection(section, "dependencies", "parallel", "notes", "format", "strategy") {
			return MigrationDiscarded, "guía de ejecución de Spec Kit sin equivalente en HCD"
		}
		if m := migrationTask.FindStringSubmatch(text); m != nil {
			if strings.EqualFold(m[1], "x") {
				return MigrationReference, "terminada en Spec Kit: no se promueve a verificada (FR-056)"
			}
			return MigrationPending, "tarea abierta: requiere slice y tareas atómicas HCD"
		}
		return MigrationConsolidated, ""
	}
}
