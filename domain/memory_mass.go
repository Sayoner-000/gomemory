package domain

import (
	"math"
	"sort"
)

// MassEdge es una arista dirigida y ponderada del grafo de memorias.
type MassEdge struct {
	From, To int64
	Weight   float64
}

// MassGraph es el grafo sobre el que se calcula la masa.
type MassGraph struct {
	Nodes []int64
	Edges []MassEdge
}

const (
	massDamping   = 0.85
	massTolerance = 1e-9
	massMaxIter   = 100
)

type indexedMassEdge struct {
	from, to int
	weight   float64
}

// ComputeMass es PageRank personalizado sobre g con reinicio a las semillas.
//
// Toda suma recorre nodos y aristas en orden de id: la suma de float64 no es
// asociativa, y sin un orden fijo el mismo almacén daría rankings distintos
// entre ejecuciones. La masa de los nodos sin salida vuelve a las semillas en
// lugar de repartirse por el grafo: si no, las memorias sin enlaces (la
// mayoría) regalarían su masa a nodos ajenos a lo que se está sembrando.
func ComputeMass(g MassGraph, seeds map[int64]float64) map[int64]float64 {
	nodes := uniqueSortedIDs(g.Nodes)
	result := make(map[int64]float64, len(nodes))
	if len(nodes) == 0 {
		return result
	}
	index := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		index[n] = i
	}

	p := make([]float64, len(nodes))
	total := 0.0
	for i, n := range nodes {
		if w := seeds[n]; w > 0 {
			p[i] = w
			total += w
		}
	}
	for i := range p {
		if total == 0 {
			p[i] = 1 / float64(len(nodes))
		} else {
			p[i] /= total
		}
	}

	edges := aggregateMassEdges(g.Edges, index)
	out := make([]float64, len(nodes))
	for _, e := range edges {
		out[e.from] += e.weight
	}

	rank := append([]float64(nil), p...)
	next := make([]float64, len(nodes))
	for it := 0; it < massMaxIter; it++ {
		dangling := 0.0
		for i := range rank {
			if out[i] == 0 {
				dangling += rank[i]
			}
		}
		restart := (1 - massDamping) + massDamping*dangling
		for i := range next {
			next[i] = restart * p[i]
		}
		for _, e := range edges {
			next[e.to] += massDamping * rank[e.from] * e.weight / out[e.from]
		}
		diff := 0.0
		for i := range next {
			diff += math.Abs(next[i] - rank[i])
		}
		rank, next = next, rank
		if diff < massTolerance {
			break
		}
	}
	for i, n := range nodes {
		result[n] = rank[i]
	}
	return result
}

func uniqueSortedIDs(ids []int64) []int64 {
	out := append([]int64(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	n := 0
	for i, id := range out {
		if i == 0 || id != out[n-1] {
			out[n] = id
			n++
		}
	}
	return out[:n]
}

// aggregateMassEdges descarta autolazos, pesos no positivos y extremos
// desconocidos, y suma las aristas repetidas por par dirigido con los pesos
// ordenados, para que el resultado no dependa del orden de entrada.
func aggregateMassEdges(edges []MassEdge, index map[int64]int) []indexedMassEdge {
	perPair := make(map[[2]int][]float64)
	for _, e := range edges {
		from, okFrom := index[e.From]
		to, okTo := index[e.To]
		if !okFrom || !okTo || from == to || e.Weight <= 0 {
			continue
		}
		key := [2]int{from, to}
		perPair[key] = append(perPair[key], e.Weight)
	}
	out := make([]indexedMassEdge, 0, len(perPair))
	for key, weights := range perPair {
		sort.Float64s(weights)
		sum := 0.0
		for _, w := range weights {
			sum += w
		}
		out = append(out, indexedMassEdge{from: key[0], to: key[1], weight: sum})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].from != out[j].from {
			return out[i].from < out[j].from
		}
		return out[i].to < out[j].to
	})
	return out
}
