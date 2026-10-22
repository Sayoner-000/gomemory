package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/expandstate"
	"mem/application/usecases"
	"mem/domain"
)

// slowInstinctRepo simula una base lenta: Fold espera a que venza el plazo.
type slowInstinctRepo struct{}

func (slowInstinctRepo) Fold(ctx context.Context, _ []domain.Observation, _ time.Time) error {
	<-ctx.Done()
	return ctx.Err()
}
func (slowInstinctRepo) PurgeFolded(context.Context, time.Time) error          { return nil }
func (slowInstinctRepo) List(context.Context, bool) ([]domain.Instinct, error) { return nil, nil }
func (slowInstinctRepo) SetStatus(context.Context, string, string) error       { return nil }
func (slowInstinctRepo) Adopt(context.Context, domain.Instinct, string, time.Time) error {
	return nil
}
func (slowInstinctRepo) ForContext(context.Context, float64) ([]domain.Instinct, error) {
	return nil, nil
}

type foldRecord struct{ kind, detail string }

// H-01: un plegado que excede su plazo conserva la cola y deja constancia.
func TestDeferredFoldTimeoutKeepsQueueAndRecords(t *testing.T) {
	q := expandstate.NewQueue(t.TempDir())
	ctx := context.Background()
	if err := q.Append(ctx, domain.Observation{ID: "u1", T: time.Now(), Kind: "cmd", Key: "go-test"}); err != nil {
		t.Fatal(err)
	}
	uc := &usecases.Instincts{Queue: q, Repo: slowInstinctRepo{}}
	var got []foldRecord
	runDeferredFold(uc.Fold, 50*time.Millisecond, func(k, d string) { got = append(got, foldRecord{k, d}) })

	if len(got) != 1 || got[0].kind != "expand.timeout" || got[0].detail != string(domain.ActionInstinctsFold) {
		t.Fatalf("registro: %+v", got)
	}
	if batch, err := q.ReadBatch(ctx, 10); err != nil || len(batch) != 1 {
		t.Fatalf("la cola debe conservarse tras el timeout: %v %v", batch, err)
	}
}

// H-01: sin base el plegado falla y se registra como expand.error, no como
// «no había nada que plegar».
func TestDeferredFoldOpenErrorIsRecorded(t *testing.T) {
	deps := &Deps{ExpandFull: func() (*Deps, error) { return nil, errors.New("disk I/O error") }}
	var got []foldRecord
	runDeferredFold(lazyExpandInstincts{deps}.Fold, time.Second, func(k, d string) { got = append(got, foldRecord{k, d}) })
	if len(got) != 1 || got[0].kind != "expand.error" || !strings.Contains(got[0].detail, "disk I/O error") {
		t.Fatalf("registro: %+v", got)
	}
}

// H-01: dentro del plazo de la decisión, Fold solo anota la petición.
func TestDeferFoldOnlyRequests(t *testing.T) {
	inner := &countingObserver{}
	h := usecases.ExpandHookDeps{Instincts: inner}
	req := deferFold(&h)
	if err := h.Instincts.Fold(context.Background()); err != nil || inner.folds != 0 || !req.requested.Load() {
		t.Fatalf("fold diferido: err=%v folds=%d requested=%t", err, inner.folds, req.requested.Load())
	}
	if err := h.Instincts.Observe(context.Background(), domain.ExpandSignals{}); err != nil || inner.observes != 1 {
		t.Fatalf("Observe sigue siendo directo: %v %d", err, inner.observes)
	}
	empty := usecases.ExpandHookDeps{}
	if r := deferFold(&empty); empty.Instincts != nil || r.requested.Load() {
		t.Fatal("sin instincts no hay petición")
	}
}

type countingObserver struct{ folds, observes int }

func (c *countingObserver) Observe(context.Context, domain.ExpandSignals) error {
	c.observes++
	return nil
}
func (c *countingObserver) Fold(context.Context) error {
	c.folds++
	return nil
}
