package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Secciones fijas que expone manage_adr del proveedor externo (verificado en
// vivo, ver specs/010-codegraph-integration-evolution/research.md §2): un
// documento único por proyecto, no un CRUD de múltiples ADR con ID.
const (
	ADRSectionPurpose      = "PURPOSE"
	ADRSectionStack        = "STACK"
	ADRSectionArchitecture = "ARCHITECTURE"
	ADRSectionPatterns     = "PATTERNS"
	ADRSectionTradeoffs    = "TRADEOFFS"
	ADRSectionPhilosophy   = "PHILOSOPHY"
)

var adrSectionOrder = []string{
	ADRSectionPurpose, ADRSectionStack, ADRSectionArchitecture,
	ADRSectionPatterns, ADRSectionTradeoffs, ADRSectionPhilosophy,
}

// ADRBlock es una entrada dentro de una sección del documento único de ADR.
// MemoryID no-nil ⇒ el bloque se originó en una memoria de gomemory (vive
// marcado con un comentario `<!-- gomemory:id=N -->` justo antes del
// heading); nil ⇒ el bloque se escribió directo del lado del proveedor (sin
// marcador) o es texto libre sin heading dentro de la sección.
type ADRBlock struct {
	MemoryID *int64
	Heading  string
	Body     string
}

// ADRSection es una de las 6 secciones fijas del documento.
type ADRSection struct {
	Name   string
	Blocks []ADRBlock
}

// ADRDocument es el documento único de ADR de un proyecto, parseado en
// secciones/bloques. Dominio puro: sin I/O, el adaptador solo sabe leer/
// escribir el `content` completo (ver ports.ADRSyncProvider).
type ADRDocument struct {
	Sections []ADRSection
}

// Section devuelve la sección por nombre, o una ADRSection vacía si no
// existe (ParseADRDocument siempre inicializa las 6 fijas, así que en la
// práctica esto no debería fallar salvo un nombre mal escrito).
func (d ADRDocument) Section(name string) ADRSection {
	for _, s := range d.Sections {
		if s.Name == name {
			return s
		}
	}
	return ADRSection{Name: name}
}

// UpsertBlock crea o actualiza (por MemoryID) el bloque de una memoria
// dentro de la sección indicada. Idempotente: llamarlo dos veces con el
// mismo memoryID actualiza el mismo bloque en vez de duplicarlo.
func (d *ADRDocument) UpsertBlock(section string, memoryID int64, heading, body string) {
	idx := -1
	for i, s := range d.Sections {
		if s.Name == section {
			idx = i
			break
		}
	}
	if idx == -1 {
		d.Sections = append(d.Sections, ADRSection{Name: section})
		idx = len(d.Sections) - 1
	}
	for i := range d.Sections[idx].Blocks {
		if b := d.Sections[idx].Blocks[i].MemoryID; b != nil && *b == memoryID {
			d.Sections[idx].Blocks[i].Heading = heading
			d.Sections[idx].Blocks[i].Body = body
			return
		}
	}
	id := memoryID
	d.Sections[idx].Blocks = append(d.Sections[idx].Blocks, ADRBlock{MemoryID: &id, Heading: heading, Body: body})
}

// Render reserializa el documento completo, siempre con las 6 secciones
// fijas en orden canónico (aunque estén vacías) — así el `content` que se
// manda de vuelta a manage_adr(mode='update') es estructuralmente estable
// entre sincronizaciones sucesivas.
func (d ADRDocument) Render() string {
	var b strings.Builder
	for _, name := range adrSectionOrder {
		b.WriteString("## " + name + "\n\n")
		for _, blk := range d.Section(name).Blocks {
			if blk.MemoryID != nil {
				fmt.Fprintf(&b, "<!-- gomemory:id=%d -->\n", *blk.MemoryID)
			}
			if blk.Heading != "" {
				b.WriteString("### " + blk.Heading + "\n")
			}
			if blk.Body != "" {
				b.WriteString(blk.Body + "\n")
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// ParseADRDocument interpreta el `content` crudo devuelto por
// manage_adr(mode='get'). Tolerante: contenido vacío o sin alguna de las 6
// secciones igual devuelve un ADRDocument con las 6 presentes (vacías). Los
// encabezados de sección desconocidos (fuera de las 6 fijas) se ignoran —
// mismo criterio degradado-en-silencio del resto de esta feature.
func ParseADRDocument(content string) ADRDocument {
	raw := make(map[string][]string, len(adrSectionOrder))
	for _, name := range adrSectionOrder {
		raw[name] = nil
	}

	current := ""
	for _, line := range strings.Split(content, "\n") {
		if name, ok := sectionHeader(line); ok {
			current = name
			if _, known := raw[current]; !known {
				current = "" // sección desconocida: descartar su contenido
			}
			continue
		}
		if current != "" {
			raw[current] = append(raw[current], line)
		}
	}

	doc := ADRDocument{Sections: make([]ADRSection, 0, len(adrSectionOrder))}
	for _, name := range adrSectionOrder {
		doc.Sections = append(doc.Sections, ADRSection{Name: name, Blocks: parseBlocks(raw[name])})
	}
	return doc
}

// sectionHeader reconoce una línea "## NOMBRE" (header de sección) sin
// confundirla con "### heading" (header de bloque): "## " y "### " difieren
// en el tercer carácter, así que HasPrefix no colisiona entre ambos.
func sectionHeader(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "## ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), true
}

// parseBlocks interpreta las líneas de una sección en bloques: cada
// "### heading" abre un bloque nuevo (heredando el MemoryID pendiente de un
// marcador `<!-- gomemory:id=N -->` inmediatamente anterior); texto sin
// heading precedente se agrupa en un único bloque anónimo (Heading vacío)
// para no perder contenido escrito a mano fuera de la convención de bloques.
func parseBlocks(lines []string) []ADRBlock {
	var blocks []ADRBlock
	var cur *ADRBlock
	var bodyLines []string
	var pendingID *int64

	flush := func() {
		if cur == nil {
			return
		}
		body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		if cur.Heading != "" || body != "" {
			cur.Body = body
			blocks = append(blocks, *cur)
		}
		cur, bodyLines = nil, nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if cur != nil {
				bodyLines = append(bodyLines, line)
			}
			continue
		}
		if id, ok := memoryIDMarker(trimmed); ok {
			flush()
			pendingID = &id
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			flush()
			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			cur = &ADRBlock{MemoryID: pendingID, Heading: heading}
			pendingID = nil
			continue
		}
		if cur == nil {
			cur = &ADRBlock{}
		}
		bodyLines = append(bodyLines, line)
	}
	flush()
	return blocks
}

// memoryIDMarker reconoce `<!-- gomemory:id=N -->` y devuelve N. Cualquier
// otra forma (incluidos otros comentarios HTML) no matchea.
func memoryIDMarker(trimmed string) (int64, bool) {
	const prefix, suffix = "<!-- gomemory:id=", "-->"
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, suffix) {
		return 0, false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), suffix))
	id, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
