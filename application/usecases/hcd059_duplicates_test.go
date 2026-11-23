package usecases

import (
	"fmt"
	"math"
	"math/rand"
	"mem/domain"
	"reflect"
	"sort"
	"testing"
)

// Exhaustive BFS reference independent of production's union-find and candidate
// enumeration. Tokenization/Jaccard are intentionally shared: this change only
// replaces pair enumeration, not lexical semantics.
func hcd059ExhaustiveGroups(mems []domain.Memory, threshold float64) []DuplicateGroup {
	byType := map[domain.MemoryType][]domain.Memory{}
	for _, m := range mems {
		if m.Type != domain.Checkpoint {
			byType[m.Type] = append(byType[m.Type], m)
		}
	}
	var out []DuplicateGroup
	for typ, items := range byType {
		tokens := make([]map[string]struct{}, len(items))
		for i, m := range items {
			tokens[i] = tokenize(m.Title + " " + m.Content)
		}
		seen := make([]bool, len(items))
		for start := range items {
			if seen[start] {
				continue
			}
			seen[start] = true
			queue := []int{start}
			var group []domain.Memory
			for len(queue) > 0 {
				i := queue[0]
				queue = queue[1:]
				group = append(group, items[i])
				for j := range items {
					if !seen[j] && jaccardSimilarity(tokens[i], tokens[j]) >= threshold {
						seen[j] = true
						queue = append(queue, j)
					}
				}
			}
			if len(group) < 2 {
				continue
			}
			sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
			best := group[0]
			for _, m := range group[1:] {
				if len(m.Content) > len(best.Content) || (len(m.Content) == len(best.Content) && m.CreatedAt > best.CreatedAt) {
					best = m
				}
			}
			out = append(out, DuplicateGroup{Type: typ, Memories: group, SuggestedKeepID: best.ID})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Memories[0].ID < out[j].Memories[0].ID
	})
	return out
}

func TestHCD059DuplicateCandidatePruning(t *testing.T) {
	tokens := make([]map[string]struct{}, 200)
	for i := range tokens {
		tokens[i] = tokenize(fmt.Sprintf("unique%d", i))
	}
	count := 0
	forEachDuplicateCandidate(tokens, 0.09, func(i, j int) {
		count++
		if i >= j {
			t.Fatal("invalid pair")
		}
	})
	if count != 0 {
		t.Fatalf("disjoint sets evaluated %d pairs", count)
	}
	count = 0
	forEachDuplicateCandidate(tokens, 0, func(i, j int) { count++ })
	if count != 19900 {
		t.Fatalf("zero threshold lost pairs: %d", count)
	}
	for i := range tokens {
		tokens[i] = tokenize("shared common")
	}
	seen := map[[2]int]bool{}
	forEachDuplicateCandidate(tokens, 0.09, func(i, j int) {
		p := [2]int{i, j}
		if seen[p] {
			t.Fatal("duplicate pair")
		}
		seen[p] = true
	})
	if len(seen) != 19900 {
		t.Fatalf("dense pairs=%d", len(seen))
	}
}

func TestHCD059DuplicateEquivalence(t *testing.T) {
	rng := rand.New(rand.NewSource(59))
	thresholds := []float64{-1, 0, 0.09, 1.0 / 3, 0.5, 1, 1.1, math.NaN(), math.Inf(1), math.Inf(-1)}
	fixtures := [][]domain.Memory{nil, {}, {
		{ID: 1, Type: domain.Learning, Content: "alpha beta", CreatedAt: "2026-01-01"},
		{ID: 2, Type: domain.Learning, Content: "beta gamma", CreatedAt: "2026-01-02"},
		{ID: 3, Type: domain.Learning, Content: "gamma delta"},
		{ID: 4, Type: domain.Decision, Content: "alpha beta"},
		{ID: 5, Type: domain.Checkpoint, Content: "alpha beta"},
		{ID: 6, Type: domain.Learning, Content: "el y a"},
		{ID: 7, Type: domain.Learning, Content: ""},
		{ID: 8, Type: domain.Learning, Content: "ÁRBOL 日本語 árbol"},
		{ID: 9, Type: domain.Learning, Content: "árbol 日本語", Title: "Unicode"},
	}}
	for trial := 0; trial < 50; trial++ {
		mems := make([]domain.Memory, 40)
		for i := range mems {
			content := ""
			for j := 0; j < rng.Intn(15); j++ {
				content += fmt.Sprintf("word%d ", rng.Intn(30))
			}
			types := []domain.MemoryType{domain.Learning, domain.Decision, domain.Checkpoint}
			mems[i] = domain.Memory{ID: int64(i + 1), Type: types[rng.Intn(len(types))], Content: content, CreatedAt: fmt.Sprint(rng.Intn(3))}
		}
		rng.Shuffle(len(mems), func(i, j int) { mems[i], mems[j] = mems[j], mems[i] })
		fixtures = append(fixtures, mems)
	}
	for i, mems := range fixtures {
		for _, threshold := range thresholds {
			want := hcd059ExhaustiveGroups(mems, threshold)
			got := DetectDuplicateGroups(mems, threshold)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("fixture=%d threshold=%v\ngot=%+v\nwant=%+v", i, threshold, got, want)
			}
		}
	}
}
