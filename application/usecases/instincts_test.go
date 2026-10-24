package usecases

import (
	"context"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"strings"
	"testing"
	"time"
)

type instinctQueueFake struct {
	batch     []domain.Observation
	removeErr bool
}

func (q *instinctQueueFake) Append(_ context.Context, o domain.Observation) error {
	q.batch = append(q.batch, o)
	return nil
}
func (q *instinctQueueFake) ReadBatch(_ context.Context, n int) ([]domain.Observation, error) {
	if n <= 0 || n > len(q.batch) {
		n = len(q.batch)
	}
	return q.batch[:n], nil
}
func (q *instinctQueueFake) Remove(_ context.Context, ids []string) error {
	if q.removeErr {
		return errors.New("remove")
	}
	q.batch = q.batch[len(ids):]
	return nil
}
func (q *instinctQueueFake) Sanitize(context.Context, func(string) string) (int, error) {
	return 0, nil
}

type instinctRepoFake struct {
	ports.InstinctRepository
	seen  map[string]bool
	count int
	fail  bool
	list  []domain.Instinct
}

func (r *instinctRepoFake) Fold(_ context.Context, b []domain.Observation, _ time.Time) error {
	if r.fail {
		return errors.New("fold")
	}
	if r.seen == nil {
		r.seen = map[string]bool{}
	}
	for _, o := range b {
		if !r.seen[o.ID] {
			r.count++
			r.seen[o.ID] = true
		}
	}
	return nil
}
func (r *instinctRepoFake) PurgeFolded(context.Context, time.Time) error { return nil }
func (r *instinctRepoFake) ForContext(context.Context, float64) ([]domain.Instinct, error) {
	return []domain.Instinct{{Slug: "cmd:go-test", Confidence: .7}}, nil
}
func (r *instinctRepoFake) List(context.Context, bool) ([]domain.Instinct, error) { return r.list, nil }

type instinctGlobalFake struct {
	ports.InstinctGlobalRepository
	fail     bool
	recorded int
	list     []domain.Instinct
}

func (g *instinctGlobalFake) RecordProject(context.Context, string, domain.Instinct, time.Time) error {
	if g.fail {
		return errors.New("global")
	}
	g.recorded++
	return nil
}
func (g *instinctGlobalFake) GlobalFor(context.Context, float64) ([]domain.Instinct, error) {
	return g.list, nil
}

func TestInstinctObserveRedactsAndFoldRecovers(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	q := &instinctQueueFake{}
	r := &instinctRepoFake{}
	g := &instinctGlobalFake{}
	u := &Instincts{Queue: q, Repo: r, Global: g, Project: "p", Now: func() time.Time { return now }, NewID: func() string { return "id" }}
	token := "ghp_" + strings.Repeat("a", 36)
	if err := u.Observe(ctx, domain.ExpandSignals{Event: domain.ExpandPostTool, Tool: domain.ToolShell, Command: "go test " + token + " <private>hidden</private>"}); err != nil {
		t.Fatal(err)
	}
	if len(q.batch) != 1 || strings.Contains(q.batch[0].Sample, token) || strings.Contains(q.batch[0].Sample, "hidden") {
		t.Fatalf("unsafe observation: %+v", q.batch)
	}
	r.fail = true
	if u.FoldPending(ctx, now) == nil || len(q.batch) != 1 {
		t.Fatal("fold failure lost queue")
	}
	r.fail = false
	q.removeErr = true
	if u.FoldPending(ctx, now) == nil || r.count != 1 {
		t.Fatal("commit/remove simulation")
	}
	q.removeErr = false
	g.fail = true
	if u.FoldPending(ctx, now) == nil || len(q.batch) != 0 || r.count != 1 {
		t.Fatal("recount after commit")
	}
	g.fail = false
	if err := u.FoldPending(ctx, now); err != nil || r.count != 1 || g.recorded == 0 {
		t.Fatal("global reconciliation failed", err)
	}
	for i := range 1200 {
		q.batch = append(q.batch, domain.Observation{ID: fmt.Sprint(i)})
	}
	before := r.count
	if err := u.FoldPending(ctx, now); err != nil || r.count-before != 500 || len(q.batch) != 700 {
		t.Fatal("unbounded batch", err)
	}
}

func TestInstinctFoldSkipsGlobalRecordForEphemeralProject(t *testing.T) {
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		ephemeral bool
		want      int
	}{{false, 1}, {true, 0}} {
		g := &instinctGlobalFake{}
		u := &Instincts{Queue: &instinctQueueFake{}, Repo: &instinctRepoFake{}, Global: g, Project: "p", Ephemeral: tc.ephemeral}
		if err := u.FoldPending(context.Background(), now); err != nil || g.recorded != tc.want {
			t.Fatalf("ephemeral=%v: recorded=%d err=%v, want %d", tc.ephemeral, g.recorded, err, tc.want)
		}
	}
}

func TestInstinctContextHonorsRejection(t *testing.T) {
	r := &instinctRepoFake{list: []domain.Instinct{{Slug: "rejected", Confidence: .9, Status: "rejected"}, {Slug: "low", Confidence: .6}, {Slug: "high", Confidence: .7, Description: "safe"}}}
	g := &instinctGlobalFake{list: []domain.Instinct{{Slug: "rejected", Confidence: .9}, {Slug: "global", Confidence: .8}}}
	list, err := InstinctContext(context.Background(), r, g, time.Now())
	if err != nil || len(list) != 2 || list[0].Slug != "high" || list[1].Slug != "global" {
		t.Fatalf("%+v %v", list, err)
	}
}

func TestInstinctContextSkipsStaleAndOldInstincts(t *testing.T) {
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	r := &instinctRepoFake{list: []domain.Instinct{
		{Slug: "cmd:go-test", Kind: "cmd", Description: "go test ./...", Confidence: .8, LastSeen: "2026-10-20T00:00:00Z"},
		{Slug: "cmd:sed", Kind: "cmd", Description: "sed -n '1,9p' a.go", Confidence: .9, LastSeen: "2026-10-23T00:00:00Z"},
		{Slug: "cmd:git-diff", Kind: "cmd", Description: "git diff", Confidence: .9, LastSeen: "2026-09-01T00:00:00Z"},
	}}
	list, err := InstinctContext(context.Background(), r, nil, now)
	if err != nil || len(list) != 1 || list[0].Slug != "cmd:go-test" {
		t.Fatalf("%+v %v", list, err)
	}
}
