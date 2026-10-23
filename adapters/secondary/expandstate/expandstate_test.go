package expandstate

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"mem/domain"
)

func obs(id, sample string) domain.Observation {
	return domain.Observation{ID: id, T: time.Unix(0, 0).UTC(), Kind: "cmd", Key: "cmd:go-test", Sample: sample}
}

func TestQueueAppendReadRemove(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(t.TempDir())
	if got, err := q.ReadBatch(ctx, 10); err != nil || len(got) != 0 {
		t.Fatalf("cola vacía: %v %v", got, err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := q.Append(ctx, obs(id, "s")); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := q.ReadBatch(ctx, 2)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("lote: %+v", got)
	}
	if err := q.Remove(ctx, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	got, _ = q.ReadBatch(ctx, 10)
	if len(got) != 1 || got[0].ID != "c" {
		t.Fatalf("tras remove: %+v", got)
	}
	info, err := os.Stat(q.path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("permisos: %v %v", info.Mode().Perm(), err)
	}
}

func TestQueueIgnoresCorruptLines(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(t.TempDir())
	_ = q.Append(ctx, obs("a", "s"))
	f, _ := os.OpenFile(q.path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString("{no json\n\n{\"kind\":\"cmd\"}\n")
	_ = f.Close()
	_ = q.Append(ctx, obs("b", "s"))
	got, err := q.ReadBatch(ctx, 10)
	if err != nil || len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("líneas corruptas: %+v %v", got, err)
	}
}

func TestQueueRotatesKeepingNewestHalf(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(t.TempDir())
	big := strings.Repeat("x", 1024)
	n := 0
	for {
		if err := q.Append(ctx, obs(fmt.Sprintf("id-%04d", n), big)); err != nil {
			t.Fatal(err)
		}
		n++
		info, _ := os.Stat(q.path)
		if n > 400 || info.Size() < int64(n*1000) {
			break
		}
	}
	info, _ := os.Stat(q.path)
	if info.Size() > QueueMaxBytes {
		t.Fatalf("la cola supera el tope: %d", info.Size())
	}
	got, _ := q.ReadBatch(ctx, 1000)
	if len(got) == 0 || got[len(got)-1].ID != fmt.Sprintf("id-%04d", n-1) {
		t.Fatalf("la rotación debe conservar las más recientes; última = %+v", got[len(got)-1].ID)
	}
	if got[0].ID == "id-0000" {
		t.Fatal("la rotación debe descartar las más antiguas")
	}
}

func TestQueueSanitize(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(t.TempDir())
	_ = q.Append(ctx, obs("a", "token=SECRETO"))
	_ = q.Append(ctx, obs("b", "limpio"))
	n, err := q.Sanitize(ctx, func(s string) string { return strings.ReplaceAll(s, "SECRETO", "[REDACTED]") })
	if err != nil || n != 1 {
		t.Fatalf("sanitize: %d %v", n, err)
	}
	data, _ := os.ReadFile(q.path)
	if strings.Contains(string(data), "SECRETO") {
		t.Fatal("el secreto sigue en la cola")
	}
}

func TestQueueRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q := NewQueue(t.TempDir())
	if err := q.Append(ctx, obs("a", "s")); err == nil {
		t.Fatal("Append con ctx cancelado debe fallar")
	}
}

func TestShieldStateRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewShieldState(t.TempDir())
	if s.LastHash(ctx) != "" {
		t.Fatal("sin estado previo, huella vacía")
	}
	if _, ok := s.Last(ctx); ok {
		t.Fatal("sin estado previo, sin informe")
	}
	r := domain.ShieldReport{Hash: "h1", ScannedAt: time.Unix(10, 0).UTC(),
		Findings: []domain.ShieldFinding{{File: "~/.claude/settings.json", Category: "secret", Severity: "critical", Detail: "token"}},
		Skipped:  []string{"~/big.json"}}
	if err := s.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	if s.LastHash(ctx) != "h1" {
		t.Fatal("huella no guardada")
	}
	got, ok := s.Last(ctx)
	if !ok || got.Hash != "h1" || len(got.Findings) != 1 || got.Skipped[0] != "~/big.json" {
		t.Fatalf("informe: %+v", got)
	}
	for _, p := range []string{s.hashPath, s.reportPath} {
		if info, err := os.Stat(p); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
			t.Fatalf("permisos de %s: %v", p, err)
		}
	}
}

// A-001: Remove reemplaza la cola entera; un Append concurrente no puede
// perderse. Se conservan todos los IDs no eliminados expresamente.
func TestQueueConcurrentAppendAndRemoveKeepsObservations(t *testing.T) {
	q := NewQueue(t.TempDir())
	ctx := context.Background()
	if err := q.Append(ctx, domain.Observation{ID: "drop", T: time.Now(), Kind: "cmd", Key: "x"}); err != nil {
		t.Fatal(err)
	}
	const n = 200
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := q.Append(ctx, domain.Observation{ID: fmt.Sprintf("o%d", i), T: time.Now(), Kind: "cmd", Key: "x"}); err != nil {
				t.Error(err)
			}
		}()
	}
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := q.Remove(ctx, []string{"drop"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	all, err := q.ReadBatch(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range all {
		seen[o.ID] = true
	}
	if seen["drop"] {
		t.Error("el ID eliminado sigue en la cola")
	}
	for i := range n {
		if !seen[fmt.Sprintf("o%d", i)] {
			t.Fatalf("se perdió o%d: quedan %d de %d", i, len(all), n)
		}
	}
}
