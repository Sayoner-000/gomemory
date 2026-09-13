package usecases

import (
	"sort"
	"strconv"

	"mem/application/ports"
	"mem/domain"
)

// MassEntry es la masa de una memoria en el ranking.
type MassEntry struct {
	ID   int64
	Mass float64
}

// MassDisclaimer acompaña a toda salida de masa: dice qué NO afirma el número.
const MassDisclaimer = "masa = centralidad en el grafo de memorias sembrado en %s; no mide importancia ni corrección"

// BuildMassGraph aplica las reglas de aristas de FR-016. Los veredictos no son
// transiciones, y los checkpoints o las memorias borradas no transportan masa.
func BuildMassGraph(mems []domain.Memory, rels []domain.Relation) domain.MassGraph {
	known := map[int64]bool{}
	graph := domain.MassGraph{}
	for _, m := range mems {
		if m.Type != domain.Checkpoint {
			known[m.ID] = true
			graph.Nodes = append(graph.Nodes, m.ID)
		}
	}
	for _, r := range rels {
		if r.Confidence <= 0 || !known[r.MemoryIDA] || !known[r.MemoryIDB] {
			continue
		}
		switch r.Relation {
		case domain.Related, domain.Compatible, domain.Scoped:
			graph.Edges = append(graph.Edges,
				domain.MassEdge{From: r.MemoryIDA, To: r.MemoryIDB, Weight: r.Confidence},
				domain.MassEdge{From: r.MemoryIDB, To: r.MemoryIDA, Weight: r.Confidence})
		case domain.Supersedes:
			// La masa del sustituido fluye al vigente, nunca al revés.
			graph.Edges = append(graph.Edges, domain.MassEdge{From: r.MemoryIDB, To: r.MemoryIDA, Weight: r.Confidence})
		}
	}
	return graph
}

// RankMass ordena las memorias no checkpoint por masa descendente y, en empate,
// por id ascendente.
func RankMass(mems []domain.Memory, rels []domain.Relation, seeds map[int64]float64) []MassEntry {
	mass := domain.ComputeMass(BuildMassGraph(mems, rels), seeds)
	out := make([]MassEntry, 0, len(mass))
	for id, value := range mass {
		out = append(out, MassEntry{id, value})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mass != out[j].Mass {
			return out[i].Mass > out[j].Mass
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ContextSeeds siembra la masa en lo que importa ahora: las memorias de la
// sesión activa y las ancladas a código activo. Sin ninguna devuelve nil, que
// ComputeMass trata como siembra uniforme.
func ContextSeeds(all []domain.Memory, active *domain.Session, providers []ports.CodeGraphProvider) (map[int64]float64, string) {
	seeds := make(map[int64]float64)
	typeByID := make(map[int64]domain.MemoryType, len(all))
	for _, m := range all {
		typeByID[m.ID] = m.Type
	}
	sessionN := 0
	if active != nil {
		for _, m := range all {
			if m.SessionID == active.ID && m.Type != domain.Checkpoint {
				seeds[m.ID] = 1
				sessionN++
			}
		}
	}
	hotspotN := 0
	for id := range hotspotMemoryIDs(all, providers) {
		if typeByID[id] == domain.Checkpoint {
			continue
		}
		seeds[id] = 1
		hotspotN++
	}
	if len(seeds) == 0 {
		return nil, DescribeSeeds(0, 0, true)
	}
	return seeds, DescribeSeeds(sessionN, hotspotN, false)
}

// DescribeSeeds es el texto {semillas} de la leyenda de masa.
func DescribeSeeds(sessionN, hotspotN int, uniform bool) string {
	switch {
	case uniform:
		return "todas las memorias (uniforme)"
	case sessionN > 0 && hotspotN > 0:
		return "sesión activa (" + strconv.Itoa(sessionN) + ") + anclas a hotspot (" + strconv.Itoa(hotspotN) + ")"
	case sessionN > 0:
		return "sesión activa (" + strconv.Itoa(sessionN) + ")"
	default:
		return "anclas a hotspot (" + strconv.Itoa(hotspotN) + ")"
	}
}
