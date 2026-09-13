package usecases_test

import (
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

func hasMassEdge(g domain.MassGraph, from, to int64, w float64) bool {
	for _, e := range g.Edges {
		if e.From == from && e.To == to && e.Weight == w {
			return true
		}
	}
	return false
}

func TestBuildMassGraph_ReglasDeAristas(t *testing.T) {
	mems := []domain.Memory{
		{ID: 1, Type: domain.Decision}, {ID: 2, Type: domain.Learning},
		{ID: 3, Type: domain.Bugfix}, {ID: 4, Type: domain.Checkpoint},
	}
	rels := []domain.Relation{
		{MemoryIDA: 1, MemoryIDB: 2, Relation: domain.Compatible, Confidence: 0.8},
		{MemoryIDA: 2, MemoryIDB: 3, Relation: domain.Scoped, Confidence: 0.4},
		{MemoryIDA: 1, MemoryIDB: 3, Relation: domain.ConflictsWith, Confidence: 1},
		{MemoryIDA: 3, MemoryIDB: 1, Relation: domain.NotConflict, Confidence: 1},
		{MemoryIDA: 1, MemoryIDB: 4, Relation: domain.Related, Confidence: 1},
		{MemoryIDA: 2, MemoryIDB: 99, Relation: domain.Related, Confidence: 1},
		{MemoryIDA: 3, MemoryIDB: 2, Relation: domain.Related, Confidence: 0},
	}
	g := usecases.BuildMassGraph(mems, rels)
	if len(g.Nodes) != 3 {
		t.Fatalf("nodos = %v, want 3 sin el checkpoint", g.Nodes)
	}
	for _, e := range [][2]int64{{1, 2}, {2, 1}} {
		if !hasMassEdge(g, e[0], e[1], 0.8) {
			t.Fatalf("falta la arista compatible %v con peso 0.8: %#v", e, g.Edges)
		}
	}
	for _, e := range [][2]int64{{2, 3}, {3, 2}} {
		if !hasMassEdge(g, e[0], e[1], 0.4) {
			t.Fatalf("falta la arista scoped %v con peso 0.4: %#v", e, g.Edges)
		}
	}
	if len(g.Edges) != 4 {
		t.Fatalf("aristas = %#v, want solo las 4 de compatible/scoped", g.Edges)
	}
}

func TestRankMass_OrdenEstableSinCheckpoints(t *testing.T) {
	mems := []domain.Memory{{ID: 3, Type: domain.Decision}, {ID: 1, Type: domain.Decision}, {ID: 2, Type: domain.Checkpoint}}
	rank := usecases.RankMass(mems, nil, nil)
	if len(rank) != 2 || rank[0].ID != 1 || rank[1].ID != 3 {
		t.Fatalf("rank = %#v, want [1 3] con empate resuelto por id", rank)
	}
}

func massOf031(entries []usecases.MassEntry, id int64) float64 {
	for _, e := range entries {
		if e.ID == id {
			return e.Mass
		}
	}
	return 0
}

// SC-006 (enmendado el 2026-09-13): con tarea, una memoria enlazada a los
// resultados gana masa respecto del ranking sin tarea, y una aislada nunca
// supera su cuota de semilla.
func TestRankMass_TareaSubeEnlazadasYAisladaConservaSuCuota(t *testing.T) {
	mems := []domain.Memory{
		{ID: 1, Type: domain.Decision}, {ID: 2, Type: domain.Learning},
		{ID: 3, Type: domain.Decision}, {ID: 4, Type: domain.Decision}, {ID: 5, Type: domain.Decision},
		{ID: 6, Type: domain.Bugfix},
	}
	rels := []domain.Relation{
		{MemoryIDA: 1, MemoryIDB: 2, Relation: domain.Related, Confidence: 0.5},
		{MemoryIDA: 3, MemoryIDB: 4, Relation: domain.Related, Confidence: 1},
		{MemoryIDA: 4, MemoryIDB: 5, Relation: domain.Related, Confidence: 1},
		{MemoryIDA: 5, MemoryIDB: 3, Relation: domain.Related, Confidence: 1},
	}
	without := usecases.RankMass(mems, rels, nil)
	// Resultados de la tarea: 1 en el puesto 0 (peso 1) y 6, aislada, en el 1 (peso 1/2).
	with := usecases.RankMass(mems, rels, map[int64]float64{1: 1, 6: 0.5})

	if massOf031(with, 2) <= massOf031(without, 2) {
		t.Fatalf("la vecina enlazada debe ganar masa con la tarea: con=%v sin=%v", massOf031(with, 2), massOf031(without, 2))
	}
	share := 0.5 / 1.5
	if got := massOf031(with, 6); got > share+1e-12 {
		t.Fatalf("la semilla aislada supera su cuota: masa=%v cuota=%v", got, share)
	}
}

func TestDescribeSeeds(t *testing.T) {
	for _, tc := range []struct {
		session, hotspot int
		uniform          bool
		want             string
	}{
		{0, 0, true, "todas las memorias (uniforme)"},
		{2, 0, false, "sesión activa (2)"},
		{0, 3, false, "anclas a hotspot (3)"},
		{2, 3, false, "sesión activa (2) + anclas a hotspot (3)"},
	} {
		if got := usecases.DescribeSeeds(tc.session, tc.hotspot, tc.uniform); got != tc.want {
			t.Fatalf("DescribeSeeds(%d,%d,%t) = %q, want %q", tc.session, tc.hotspot, tc.uniform, got, tc.want)
		}
	}
}
