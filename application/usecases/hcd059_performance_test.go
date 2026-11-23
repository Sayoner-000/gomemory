package usecases_test

import (
	"fmt"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCD059ContextBenchmarkFixture(t *testing.T) {
	for _, kind := range []string{"SinRelaciones", "SinTitulo", "Relaciones", "Anclas"} {
		t.Run(kind, func(t *testing.T) {
			builder := hcd059ContextFixture(t, kind, 120)
			out, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			if out == "" {
				t.Fatal("empty context")
			}
			all, err := builder.Lister.(interface {
				ListAll(string) ([]domain.Memory, error)
			}).ListAll("perf")
			if err != nil || len(all) != 120 {
				t.Fatalf("count=%d err=%v", len(all), err)
			}
			if len(all[0].Content) != 1040 {
				t.Fatalf("body bytes=%d", len(all[0].Content))
			}
			if kind == "SinTitulo" && all[0].Title != "" {
				t.Fatal("expected empty title")
			}
			if kind == "Anclas" && !strings.Contains(out, "Anclas sin evidencia") {
				t.Fatal("missing anchors")
			}
			if kind == "Relaciones" && !strings.Contains(out, "Sinapsis") {
				t.Fatal("missing relations")
			}
		})
	}
}

// Fixtures use isolated SQLite databases and explicit dates/IDs. No user data or
// normal insert side effects (dedup, exports, FTS updates) enter measured work.
func hcd059ContextFixture(tb testing.TB, kind string, n int) *usecases.Builder {
	tb.Helper()
	tb.Setenv("GOMEMORY_NO_CODE_REFRESH", "1")
	root := tb.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := db.Close(); err != nil {
			tb.Error(err)
		}
	})
	tx, err := db.Begin()
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT INTO memories(project,session_id,type,title,content,filepath,origin_prompt,created_at,updated_at) VALUES('perf','','learning',?, ?, ?, '', ?, ?)`)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = stmt.Close() }()
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("item%d", i)
		if kind == "SinTitulo" {
			title = ""
		}
		path := ""
		if kind == "Anclas" {
			path = fmt.Sprintf("missing/file%d.go", i)
		}
		// Strict order across the recent window, independent of wall clock.
		date := fmt.Sprintf("2026-01-01T%08d", i)
		if _, err := stmt.Exec(title, strings.Repeat("measured technical content ", 40)[:1040], path, date, date); err != nil {
			tb.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
	if kind == "Relaciones" {
		rels := persistence.NewRelationRepository(db)
		for i := 1; i < n; i++ {
			_, err := rels.Insert(&domain.Relation{Project: "perf", MemoryIDA: int64(i), MemoryIDB: int64(i + 1), Relation: domain.Related, Confidence: 0.5})
			if err != nil {
				tb.Fatal(err)
			}
		}
	}
	builder := usecases.New(persistence.NewMemoryRepository(db), persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "perf")
	builder.Budget = 12000
	return builder
}

type hcd059NoopRecorder struct{}

func (hcd059NoopRecorder) Record(string, int, int) {}

var hcd059ContextSink string

func BenchmarkHCD059Context(b *testing.B)          { hcd059BenchmarkContext(b, false) }
func BenchmarkHCD059ContextReference(b *testing.B) { hcd059BenchmarkContext(b, true) }
func hcd059BenchmarkContext(b *testing.B, reference bool) {
	for _, kind := range []string{"SinRelaciones", "SinTitulo", "Relaciones", "Anclas"} {
		b.Run(kind, func(b *testing.B) {
			for _, n := range []int{100, 1000, 10000} {
				b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
					builder := hcd059ContextFixture(b, kind, n)
					if reference {
						builder = hcd059LegacyBuilder(builder)
					}
					for _, metrics := range []bool{false, true} {
						b.Run(fmt.Sprintf("Metrics%v", metrics), func(b *testing.B) {
							local := *builder
							if metrics {
								local.Counter = tokens.ApproximateTokenCounter{}
								local.Recorder = hcd059NoopRecorder{}
							}
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								out, err := local.Build()
								if err != nil {
									b.Fatal(err)
								}
								hcd059ContextSink = out
							}
						})
					}
				})
			}
		})
	}
}

var hcd059GroupSink []usecases.DuplicateGroup

func hcd059DuplicateCorpus(kind string, n int) []domain.Memory {
	mems := make([]domain.Memory, n)
	for i := range mems {
		var content strings.Builder
		for j := 0; j < 24; j++ {
			tokenID := i
			if kind != "Disperso" {
				tokenID = 0
			}
			fmt.Fprintf(&content, "term%dword%d ", tokenID, j)
		}
		typ := domain.Learning
		if kind == "Multitipo" && i%2 == 1 {
			typ = domain.Decision
		}
		mems[i] = domain.Memory{ID: int64(i + 1), Type: typ, Title: fmt.Sprintf("item%d", i), Content: content.String(), CreatedAt: "2026-01-01"}
	}
	return mems
}

func BenchmarkHCD059Duplicates(b *testing.B)          { hcd059BenchmarkDuplicates(b, false) }
func BenchmarkHCD059DuplicatesReference(b *testing.B) { hcd059BenchmarkDuplicates(b, true) }
func hcd059BenchmarkDuplicates(b *testing.B, reference bool) {
	for _, kind := range []string{"Disperso", "Denso", "Multitipo"} {
		b.Run(kind, func(b *testing.B) {
			for _, n := range []int{100, 1000, 10000} {
				b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
					mems := hcd059DuplicateCorpus(kind, n)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if reference {
							hcd059GroupSink = usecases.HCD059ReferenceDuplicates(mems, 0.09)
						} else {
							hcd059GroupSink = usecases.DetectDuplicateGroups(mems, 0.09)
						}
					}
				})
			}
		})
	}
}

func TestHCD059DuplicateBenchmarkMatrix(t *testing.T) {
	for _, kind := range []string{"Disperso", "Denso", "Multitipo"} {
		mems := hcd059DuplicateCorpus(kind, 100)
		if len(mems) != 100 {
			t.Fatalf("%s: count=%d", kind, len(mems))
		}
		groups := usecases.DetectDuplicateGroups(mems, 0.09)
		switch kind {
		case "Disperso":
			if len(groups) != 0 {
				t.Fatalf("unexpected dispersed groups: %d", len(groups))
			}
		case "Denso":
			if len(groups) != 1 || len(groups[0].Memories) != 100 {
				t.Fatal("dense corpus must form one group")
			}
		case "Multitipo":
			if len(groups) != 2 {
				t.Fatalf("want two types, got %d", len(groups))
			}
			for _, g := range groups {
				for _, m := range g.Memories {
					if m.Type != g.Type || m.Type == domain.Checkpoint {
						t.Fatal("cross-type group")
					}
				}
			}
		}
	}
}
