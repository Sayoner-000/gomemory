package usecases_test

import (
	"mem/application/usecases"
	"os"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"
)

// Five one-shot samples avoid testing.Benchmark adaptive calibration while
// preserving the acceptance criteria as ratios, not machine-specific times.
func hcd059Measure(f func()) (int64, uint64) {
	times := make([]int64, 5)
	bytes := make([]uint64, 5)
	for i := range times {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		f()
		times[i] = time.Since(start).Nanoseconds()
		runtime.ReadMemStats(&after)
		bytes[i] = after.TotalAlloc - before.TotalAlloc
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	sort.Slice(bytes, func(i, j int) bool { return bytes[i] < bytes[j] })
	return times[2], bytes[2]
}

func hcd059CheckPerformance(t *testing.T) {
	mems := hcd059DuplicateCorpus("Disperso", 10000)
	fast := usecases.DetectDuplicateGroups(mems, 0.09)
	ref := usecases.HCD059ReferenceDuplicates(mems, 0.09)
	if !reflect.DeepEqual(fast, ref) {
		t.Fatal("duplicate reference mismatch")
	}
	fastNS, fastBytes := hcd059Measure(func() { hcd059GroupSink = usecases.DetectDuplicateGroups(mems, 0.09) })
	refNS, refBytes := hcd059Measure(func() { hcd059GroupSink = usecases.HCD059ReferenceDuplicates(mems, 0.09) })
	ratio := float64(refNS) / float64(fastNS)
	t.Logf("duplicates reference=%dns/%dB optimized=%dns/%dB speedup=%.2fx", refNS, refBytes, fastNS, fastBytes, ratio)
	if ratio < 5 {
		t.Fatalf("duplicate speedup %.2f <5", ratio)
	}
	builder := hcd059ContextFixture(t, "SinRelaciones", 10000)
	legacy := hcd059LegacyBuilder(builder)
	got, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	want, err := legacy.Build()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("context mismatch")
	}
	fastNS, fastBytes = hcd059Measure(func() {
		out, e := builder.Build()
		if e != nil {
			t.Fatal(e)
		}
		hcd059ContextSink = out
	})
	refNS, refBytes = hcd059Measure(func() {
		out, e := legacy.Build()
		if e != nil {
			t.Fatal(e)
		}
		hcd059ContextSink = out
	})
	fraction := float64(fastBytes) / float64(refBytes)
	t.Logf("context reference=%dns/%dB optimized=%dns/%dB fraction=%.3f", refNS, refBytes, fastNS, fastBytes, fraction)
	if fraction > 0.7 {
		t.Fatalf("context allocation fraction %.3f >0.7", fraction)
	}
}

func TestHCD059PerformanceAcceptance(t *testing.T) {
	if os.Getenv("GOMEMORY_PERF_ACCEPTANCE") != "1" {
		t.Skip("explicit performance acceptance only")
	}
	hcd059CheckPerformance(t)
}
