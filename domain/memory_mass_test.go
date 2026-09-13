package domain

import "testing"

func TestComputeMass(t *testing.T) {
	mass := ComputeMass(MassGraph{Nodes: []int64{1, 2, 3}, Edges: []MassEdge{{1, 2, 1}, {2, 1, 1}, {2, 3, 1}, {3, 2, 1}}}, map[int64]float64{1: 1})
	if mass[2] <= mass[1] || mass[1] <= mass[3] {
		t.Fatalf("unexpected mass: %#v", mass)
	}
	sum := 0.
	for _, v := range mass {
		sum += v
	}
	if sum < .999999999 || sum > 1.000000001 {
		t.Fatalf("sum=%v", sum)
	}
}
