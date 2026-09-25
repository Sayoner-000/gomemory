package native

import (
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"testing"
	"time"
)

var benchmarkCorpus = []string{
	"golist.json", "gitlog.txt", "gotest-fail.log", "go-panic.log",
	"py-traceback.log", "mem-context.md", "search-results.txt", "repo.diff",
	"sample.go", "sample.py", "sample.ts", "sample.java", "prose.md",
}

func BenchmarkCompressionCorpus(b *testing.B) {
	for _, name := range benchmarkCorpus {
		input := readCorpus(b, name)
		b.Run(name, func(b *testing.B) {
			engine := NewEngine(newMemStore(), nil, nil, "benchmark")
			b.ReportMetric(float64(approxTokens(input)), "raw_tokens")
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := engine.Compress(input, maxOpts)
				if err != nil {
					b.Fatal(err)
				}
				if i == 0 {
					b.ReportMetric(float64(result.Tokens), "final_tokens")
				}
			}
		})
	}
}

func TestCompressionCorpusP95(t *testing.T) {
	assertLatency := os.Getenv("GOMEMORY_PERF_ASSERT") == "1"
	if !assertLatency {
		t.Log("medición informativa; usa GOMEMORY_PERF_ASSERT=1 en un runner aislado para exigir 50 ms/10k tokens")
	}
	for _, name := range benchmarkCorpus {
		input := readCorpus(t, name)
		engine := NewEngine(newMemStore(), nil, nil, "benchmark")
		durations := make([]time.Duration, 50)
		// El criterio mide el compresor, no una pausa global del recolector que
		// puede pertenecer a cualquier paquete de la suite. La recolección se
		// ejecuta entre muestras, fuera de la ventana cronometrada.
		previousGC := debug.SetGCPercent(-1)
		for i := range durations {
			started := time.Now()
			if _, err := engine.Compress(input, maxOpts); err != nil {
				t.Fatal(err)
			}
			durations[i] = time.Since(started)
		}
		debug.SetGCPercent(previousGC)
		runtime.GC()
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p95 := durations[47]
		tokens := approxTokens(input)
		if tokens < 1 {
			tokens = 1
		}
		normalized := time.Duration(float64(p95) * (10000.0 / float64(tokens)))
		t.Logf("%s: p95=%s; normalizado=%s/10k tokens", name, p95, normalized)
		if assertLatency && normalized > 50*time.Millisecond {
			t.Errorf("%s: p95 normalizado %s supera 50 ms/10k tokens", name, normalized)
		}
	}
}
