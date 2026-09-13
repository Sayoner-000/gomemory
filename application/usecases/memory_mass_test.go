package usecases_test

import (
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestBuildAndRankMass(t *testing.T) {
	mems := []domain.Memory{{ID: 1, Type: domain.Learning}, {ID: 2, Type: domain.Learning}, {ID: 3, Type: domain.Checkpoint}}
	rels := []domain.Relation{{MemoryIDA: 1, MemoryIDB: 2, Relation: domain.Supersedes, Confidence: 1}, {MemoryIDA: 2, MemoryIDB: 3, Relation: domain.Related, Confidence: 1}}
	g := usecases.BuildMassGraph(mems, rels)
	if len(g.Nodes) != 2 || len(g.Edges) != 1 || g.Edges[0].From != 2 || g.Edges[0].To != 1 {
		t.Fatalf("%#v", g)
	}
	rank := usecases.RankMass(mems, rels, map[int64]float64{2: 1})
	if len(rank) != 2 || rank[0].ID != 2 {
		t.Fatalf("%#v", rank)
	}
}
