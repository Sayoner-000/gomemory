package domain

import (
	"fmt"
	"testing"
	"time"
)

// Compares scheduling policies against the real freshness predicate, not Pi calls.
func TestHCDSchedulingComparison(t *testing.T) {
	run := func(eager bool) int {
		s := HCDState{}
		count := 0
		verify := func(ids []string) {
			count++
			s.Evidence = append(s.Evidence, green(s, s.Fingerprint, t0.Add(time.Duration(count)*time.Second), ids...))
		}
		for i := 1; i <= 10; i++ {
			id := fmt.Sprint(i)
			s.Doc.Tasks = append(s.Doc.Tasks, Task{ID: id, Done: true})
			s.Fingerprint = id
			if eager {
				verify(s.Unverified())
			} else {
				verify([]string{id})
			}
		}
		// Count task-checks separately from grouped evidence records.
		if !eager {
			verify(s.Unverified())
		}
		if len(s.Unverified()) != 0 {
			t.Fatal("final evidence is stale")
		}
		s.Fingerprint = "changed-after-final"
		if len(s.Unverified()) != 10 {
			t.Fatal("freshness gate weakened")
		}
		executions := 0
		for _, e := range s.Evidence {
			executions += len(e.Tasks)
		}
		t.Logf("eager=%v records=%d task-checks=%d", eager, count, executions)
		return executions
	}
	before, after := run(true), run(false)
	if before != 55 || after != 19 {
		t.Fatalf("before=%d after=%d", before, after)
	}
}
