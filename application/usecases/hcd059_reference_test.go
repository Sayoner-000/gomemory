package usecases

import (
	"mem/domain"
	"sort"
)

// HCD059ReferenceDuplicates freezes the pre-optimization exhaustive algorithm.
// Exported only in the test build so external benchmarks can compare identical
// corpora; the BFS oracle remains separate for correctness.
func HCD059ReferenceDuplicates(memories []domain.Memory, threshold float64) []DuplicateGroup {
	byType := map[domain.MemoryType][]domain.Memory{}
	for _, m := range memories {
		if m.Type != domain.Checkpoint {
			byType[m.Type] = append(byType[m.Type], m)
		}
	}
	var groups []DuplicateGroup
	for typ, mems := range byType {
		tokens := make([]map[string]struct{}, len(mems))
		parent := make([]int, len(mems))
		for i, m := range mems {
			tokens[i] = tokenize(m.Title + " " + m.Content)
			parent[i] = i
		}
		var find func(int) int
		find = func(x int) int {
			if parent[x] != x {
				parent[x] = find(parent[x])
			}
			return parent[x]
		}
		for i := range mems {
			for j := i + 1; j < len(mems); j++ {
				if jaccardSimilarity(tokens[i], tokens[j]) >= threshold {
					a, b := find(i), find(j)
					if a != b {
						parent[a] = b
					}
				}
			}
		}
		clusters := map[int][]domain.Memory{}
		for i, m := range mems {
			r := find(i)
			clusters[r] = append(clusters[r], m)
		}
		for _, cluster := range clusters {
			if len(cluster) < 2 {
				continue
			}
			sort.Slice(cluster, func(i, j int) bool { return cluster[i].ID < cluster[j].ID })
			groups = append(groups, DuplicateGroup{Type: typ, Memories: cluster, SuggestedKeepID: suggestKeep(cluster)})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Type != groups[j].Type {
			return groups[i].Type < groups[j].Type
		}
		return groups[i].Memories[0].ID < groups[j].Memories[0].ID
	})
	return groups
}
