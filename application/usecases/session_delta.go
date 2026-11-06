package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// SessionDelta evita reenviar en la misma sesión lo que el agente ya recibió
// (feature 033, FR-018, práctica live-zone de Headroom). Trabaja por unidades:
// cada entrada de memoria de un documento de contexto o cada resultado de una
// búsqueda. Una unidad ya entregada e igual se sustituye por una línea
// compacta; una nueva o cambiada viaja completa y queda anotada.
//
// Nunca toca el protocolo, las reglas fijadas, los conflictos ni ninguna
// sección fuera de deltaSections: lo que no es una lista de memorias se
// entrega siempre entero. La excepción es el baseline universal, que cuenta como
// una sola unidad para no reenviarse en cada turno.
type SessionDelta struct {
	Blocks ports.DeliveredBlocksRepository
	// KeepBaseline entrega el baseline universal siempre entero. Lo usa el canal
	// que reconstruye el system en cada turno, donde lo elidido se pierde.
	KeepBaseline bool
}

// deltaSections son las secciones de get_context cuyas entradas son memorias.
var deltaSections = []string{
	"## HCD activo",
	"## Preferencias del Usuario",
	"## Instincts",
	"## Decisiones de Arquitectura",
	"## Decisiones Técnicas",
	"## Patrones y Convenciones",
	"## Bugfixes",
	"## Aprendizajes Recientes",
	"## Actividad Reciente (auto)",
}

var entryTitle = regexp.MustCompile(`^- \*\*(.+?)\*\*`)

func unitHash(s string) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(s, "\n")))
	return hex.EncodeToString(sum[:])
}

// ApplyContext aplica el delta a un documento de contexto (get_context,
// get_plan_context, contexto de arranque).
func (d SessionDelta) ApplyContext(ctx context.Context, doc string) string {
	if d.Blocks == nil || doc == "" {
		return doc
	}
	lines := strings.SplitAfter(doc, "\n")
	units := d.contextUnits(lines)
	if len(units) == 0 {
		return doc
	}
	hashes := make([]string, len(units))
	for k, u := range units {
		hashes[k] = unitHash(strings.Join(lines[u.start:u.end], ""))
	}
	seen, err := d.Blocks.Seen(ctx, hashes)
	if err != nil {
		return doc // sin registro fiable, se entrega completo
	}

	var out strings.Builder
	next := 0
	replaced := false
	for k := 0; k < len(units); {
		if !seen[hashes[k]] {
			k++
			continue
		}
		// Serie de unidades consecutivas ya entregadas.
		run := k
		for run < len(units) && seen[hashes[run]] && (run == k || units[run].start == units[run-1].end) {
			run++
		}
		out.WriteString(strings.Join(lines[next:units[k].start], ""))
		if run-k == 1 {
			title := "entrada"
			if m := entryTitle.FindStringSubmatch(lines[units[k].start]); m != nil {
				title = "**" + m[1] + "**"
			}
			fmt.Fprintf(&out, "- %s %s\n", title, domain.DeliveredTag)
		} else {
			fmt.Fprintf(&out, "- %s %d entradas ya entregadas en esta sesión, sin cambios\n", domain.MarkerTag, run-k)
		}
		replaced = true
		next = units[run-1].end
		k = run
	}
	out.WriteString(strings.Join(lines[next:], ""))
	if replaced {
		out.WriteString(DeliveredRecoveryHint)
	}

	var fresh []string
	for _, h := range hashes {
		if !seen[h] {
			fresh = append(fresh, h)
		}
	}
	_ = d.Blocks.Mark(ctx, fresh)
	return out.String()
}

// deltaUnit son los índices [inicio, fin) de una entrada dentro de una sección
// elegible del documento partido en líneas.
type deltaUnit struct{ start, end int }

// contextUnits localiza las unidades de un documento de contexto: cada entrada
// de las secciones de deltaSections y, salvo con KeepBaseline, el baseline.
func (d SessionDelta) contextUnits(lines []string) []deltaUnit {
	var units []deltaUnit
	inSection := false
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, baselineHeading) && !d.KeepBaseline {
			if end := lineWithPrefix(lines, i+1, baselineEnd); end != -1 {
				units = append(units, deltaUnit{i + 1, end})
				inSection = false
				i = end
				continue
			}
		}
		if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "<!-- mem:volatile") || strings.HasPrefix(l, "# ") {
			inSection = false
			for _, h := range deltaSections {
				if strings.HasPrefix(l, h) {
					inSection = true
					break
				}
			}
			continue
		}
		if !inSection || !strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "- (+") {
			continue
		}
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(lines[j], "- ") && !strings.HasPrefix(lines[j], "## ") &&
			!strings.HasPrefix(lines[j], "<!-- mem:volatile") && strings.TrimSpace(lines[j]) != "" {
			j++
		}
		units = append(units, deltaUnit{i, j})
		i = j - 1
	}
	return units
}

// MarkVisible anota como entregadas solo las unidades de doc que aparecen
// completas en visible, el texto que de verdad llegó al agente (HCD-037). Lo
// usa el arranque cuando el canal recorta la salida: lo que quedó fuera, o
// llegó cortado a medias, no se anota y la siguiente entrega lo envía entero.
//
// visible es la salida ya comprimida y doc el documento crudo: ambos se comparan
// con los espacios normalizados igual que el compresor estructural (colapsa
// espacios repetidos y recorta los extremos de línea). Una unidad a la que el
// motor omitió contenido no aparece completa y no se anota. El hash sigue
// siendo el de la unidad cruda, que es la que verá el delta después.
func (d SessionDelta) MarkVisible(ctx context.Context, doc, visible string) {
	if d.Blocks == nil || doc == "" || visible == "" {
		return
	}
	lines := strings.SplitAfter(doc, "\n")
	shown := normalizeSpaces(visible)
	var vistos []string
	for _, u := range d.contextUnits(lines) {
		text := strings.TrimRight(strings.Join(lines[u.start:u.end], ""), "\n")
		if text != "" && containsWholeLines(shown, normalizeSpaces(text)) {
			vistos = append(vistos, unitHash(text))
		}
	}
	_ = d.Blocks.Mark(ctx, vistos)
}

var repeatedSpaces = regexp.MustCompile(`[ \t]{2,}`)

// normalizeSpaces colapsa los espacios repetidos y recorta los extremos de cada
// línea, como hace la compresión estructural con el texto que se entrega.
func normalizeSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(repeatedSpaces.ReplaceAllString(l, " "))
	}
	return strings.Join(lines, "\n")
}

// containsWholeLines dice si text aparece en s empezando en un inicio de línea
// y terminando en un fin de línea: así una entrada no cuenta como vista porque
// otra más larga la contenga, ni porque el recorte la cortara a media línea.
func containsWholeLines(s, text string) bool {
	for from := 0; ; {
		i := strings.Index(s[from:], text)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(text)
		if (start == 0 || s[start-1] == '\n') && (end == len(s) || s[end] == '\n') {
			return true
		}
		from = start + 1
	}
}

func lineWithPrefix(lines []string, from int, prefix string) int {
	for i := from; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], prefix) {
			return i
		}
	}
	return -1
}

// searchUnitHead reconoce la cabecera de un resultado: "[id] tipo | título".
var searchUnitHead = regexp.MustCompile(`^\[\d+\] `)

// ApplySearch aplica el delta a una lista de resultados de search_memories.
// Un resultado ya entregado conserva su cabecera (id, tipo y título, para que
// el agente sepa cuál es) y pierde el extracto.
func (d SessionDelta) ApplySearch(ctx context.Context, doc string) string {
	if d.Blocks == nil || doc == "" {
		return doc
	}
	blocks := strings.SplitAfter(doc, "\n\n")
	hashes := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if searchUnitHead.MatchString(b) {
			hashes = append(hashes, unitHash(b))
		}
	}
	seen, err := d.Blocks.Seen(ctx, hashes)
	if err != nil {
		return doc
	}
	var out strings.Builder
	var fresh []string
	for _, b := range blocks {
		if !searchUnitHead.MatchString(b) {
			out.WriteString(b)
			continue
		}
		h := unitHash(b)
		if !seen[h] {
			out.WriteString(b)
			fresh = append(fresh, h)
			continue
		}
		head := strings.SplitN(b, "\n", 2)[0]
		out.WriteString(head + " " + domain.DeliveredTag + "\n\n")
	}
	_ = d.Blocks.Mark(ctx, fresh)
	return out.String()
}

// MarkDelivered anota como entregadas todas las unidades de un documento que
// se entrega completo (el contexto de arranque), para que las entregas
// siguientes de la sesión sí apliquen el delta. Descarta la salida: solo
// interesa el registro.
func (d SessionDelta) MarkDelivered(ctx context.Context, doc string) {
	if d.Blocks == nil {
		return
	}
	_ = d.ApplyContext(ctx, doc)
}

// ApplyWhole trata el documento como una sola unidad: si ya se entregó igual
// en la sesión, devuelve notice; si no, lo anota y lo devuelve entero. Lo usa
// el método de planificación, que se reenviaba completo en cada llamada a
// get_plan_context aunque el agente ya lo tuviera.
func (d SessionDelta) ApplyWhole(ctx context.Context, doc, notice string) string {
	if d.Blocks == nil || strings.TrimSpace(doc) == "" {
		return doc
	}
	h := unitHash(doc)
	seen, err := d.Blocks.Seen(ctx, []string{h})
	if err != nil {
		return doc
	}
	if seen[h] {
		return notice
	}
	_ = d.Blocks.Mark(ctx, []string{h})
	return doc
}

// DeliveredRecoveryHint acompaña a todo documento en el que el delta sustituyó
// entradas. "Entregado" significa enviado a la sesión de gomemory, no que esté
// en el contexto de quien llama: un subagente comparte la sesión sin compartir
// el contexto, y el host puede truncar una salida larga. Por eso cada marcador
// tiene que llevar a una vía que devuelva el contenido de verdad.
const DeliveredRecoveryHint = "\n> ⟦ya entregado⟧ = enviado antes en esta sesión. Si no lo tienes (eres un subagente, " +
	"la salida se truncó o hubo compactación), pídelo completo: get_context(full=true) o " +
	"get_plan_context(full=true) (CLI: mem context --full, mem plan-context --full).\n"

// MethodAlreadyDelivered es el aviso que sustituye al método de planificación
// ya entregado en la sesión.
const MethodAlreadyDelivered = "> El método de descomposición atómica ya se entregó en esta sesión y no ha cambiado: aplícalo igual.\n" +
	"> Si no lo tienes (eres un subagente, la salida se truncó o hubo compactación), pídelo completo con " +
	"get_plan_context(full=true) (CLI: mem plan-context --full)."
