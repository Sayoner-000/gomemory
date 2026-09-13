package domain

import (
	"math"
	"reflect"
	"testing"
)

func sumMass(m map[int64]float64) float64 {
	s := 0.0
	for _, v := range m {
		s += v
	}
	return s
}

func TestComputeMass_SumaUnoConColgantes(t *testing.T) {
	g := MassGraph{Nodes: []int64{1, 2, 3, 4, 5}, Edges: []MassEdge{{1, 2, 1}, {2, 3, 0.5}, {3, 1, 0.2}, {4, 1, 1}}}
	m := ComputeMass(g, map[int64]float64{2: 1})
	if math.Abs(sumMass(m)-1) > 1e-9 {
		t.Fatalf("Σ masa = %v, want 1", sumMass(m))
	}
}

func TestComputeMass_ColganteDevuelveMasaASemilla(t *testing.T) {
	m := ComputeMass(MassGraph{Nodes: []int64{1, 4}, Edges: []MassEdge{{4, 1, 1}}}, map[int64]float64{1: 1})
	if math.Abs(m[1]-1) > 1e-12 || m[4] != 0 {
		t.Fatalf("masa = %#v, want {1:1, 4:0}", m)
	}
}

func TestComputeMass_SupersedesSoloFluyeAlVigente(t *testing.T) {
	// A=1 sustituye a B=2: el grafo solo tiene la arista B→A.
	g := MassGraph{Nodes: []int64{1, 2}, Edges: []MassEdge{{2, 1, 1}}}
	if m := ComputeMass(g, map[int64]float64{2: 1}); m[1] <= 0 {
		t.Fatalf("sembrado en el sustituido, el vigente debe recibir masa: %#v", m)
	}
	if m := ComputeMass(g, map[int64]float64{1: 1}); m[2] != 0 {
		t.Fatalf("sembrado en el vigente, el sustituido no debe recibir masa: %#v", m)
	}
}

func TestComputeMass_PermutarEntradaNoCambiaSalida(t *testing.T) {
	nodes := []int64{1, 2, 3, 4}
	edges := []MassEdge{{1, 2, 0.1}, {1, 3, 0.2}, {1, 4, 0.7}, {2, 1, 0.3}, {3, 1, 0.6}, {4, 2, 0.9}}
	seeds := map[int64]float64{1: 0.1, 2: 0.2, 3: 0.7}
	want := ComputeMass(MassGraph{Nodes: nodes, Edges: edges}, seeds)
	permNodes := []int64{4, 2, 1, 3}
	permEdges := []MassEdge{edges[5], edges[2], edges[0], edges[4], edges[1], edges[3]}
	for i := 0; i < 50; i++ {
		got := ComputeMass(MassGraph{Nodes: permNodes, Edges: permEdges}, seeds)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iteración %d: la salida depende del orden de entrada\n got=%#v\nwant=%#v", i, got, want)
		}
	}
}

func TestComputeMass_SemillasVaciasEsUniforme(t *testing.T) {
	g := MassGraph{Nodes: []int64{1, 2, 3}, Edges: []MassEdge{{1, 2, 1}, {2, 3, 1}}}
	a := ComputeMass(g, nil)
	b := ComputeMass(g, map[int64]float64{1: 1, 2: 1, 3: 1})
	for id := range b {
		if math.Abs(a[id]-b[id]) > 1e-15 {
			t.Fatalf("nodo %d: sin semillas=%v, uniforme=%v", id, a[id], b[id])
		}
	}
}

func TestComputeMass_IgnoraAristasYSemillasInvalidas(t *testing.T) {
	base := MassGraph{Nodes: []int64{1, 2}, Edges: []MassEdge{{1, 2, 1}}}
	noisy := MassGraph{Nodes: []int64{2, 1, 2}, Edges: []MassEdge{{1, 2, 1}, {1, 1, 1}, {2, 1, 0}, {2, 1, -1}, {1, 99, 1}}}
	want := ComputeMass(base, map[int64]float64{1: 1})
	got := ComputeMass(noisy, map[int64]float64{1: 1, 99: 5, 2: 0})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestComputeMass_AgregaAristasRepetidas(t *testing.T) {
	a := ComputeMass(MassGraph{Nodes: []int64{1, 2, 3}, Edges: []MassEdge{{1, 2, 0.5}, {1, 2, 0.5}, {1, 3, 1}}}, map[int64]float64{1: 1})
	b := ComputeMass(MassGraph{Nodes: []int64{1, 2, 3}, Edges: []MassEdge{{1, 2, 1}, {1, 3, 1}}}, map[int64]float64{1: 1})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("repetidas=%#v agregadas=%#v", a, b)
	}
}

func TestComputeMass_GrafoVacio(t *testing.T) {
	if m := ComputeMass(MassGraph{}, map[int64]float64{1: 1}); len(m) != 0 {
		t.Fatalf("masa = %#v, want vacío", m)
	}
}
