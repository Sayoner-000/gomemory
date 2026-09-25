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
// entrega siempre entero.
type SessionDelta struct {
	Blocks ports.DeliveredBlocksRepository
}

// deltaSections son las secciones de get_context cuyas entradas son memorias.
var deltaSections = []string{
	"## Preferencias del Usuario",
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
	// units: índices [inicio, fin) de cada entrada dentro de una sección elegible.
	type unit struct{ start, end int }
	var units []unit
	inSection := false
	for i := 0; i < len(lines); i++ {
		l := lines[i]
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
		units = append(units, unit{i, j})
		i = j - 1
	}
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
			fmt.Fprintf(&out, "- %s %s\n", title, domain.MarkerOpen+"ya entregado"+domain.MarkerClose)
		} else {
			fmt.Fprintf(&out, "- %s %d entradas ya entregadas en esta sesión, sin cambios · get_memory(id) o search_memories para reconsultar\n", domain.MarkerTag, run-k)
		}
		next = units[run-1].end
		k = run
	}
	out.WriteString(strings.Join(lines[next:], ""))

	var fresh []string
	for _, h := range hashes {
		if !seen[h] {
			fresh = append(fresh, h)
		}
	}
	_ = d.Blocks.Mark(ctx, fresh)
	return out.String()
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
		out.WriteString(head + " " + domain.MarkerOpen + "ya entregado" + domain.MarkerClose + "\n\n")
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

// MethodAlreadyDelivered es el aviso que sustituye al método de planificación
// ya entregado en la sesión.
const MethodAlreadyDelivered = "> El método de descomposición atómica ya se entregó en esta sesión y no ha cambiado: aplícalo igual.\n" +
	"> Si ya no lo tienes (por ejemplo, tras una compactación), gomemory lo reenvía completo en la siguiente llamada."
