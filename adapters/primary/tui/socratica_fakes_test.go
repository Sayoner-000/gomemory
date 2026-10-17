package tui

import (
	"context"
	"sync/atomic"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// countingSocraticaRepo delega en el almacén SQLite real y cuenta cada
// llamada: la prueba de pureza exige cero llamadas desde View().
type countingSocraticaRepo struct {
	inner ports.SocraticaRepository
	calls atomic.Int64
}

func (r *countingSocraticaRepo) c() { r.calls.Add(1) }

func (r *countingSocraticaRepo) CreateFlow(ctx context.Context, f domain.RefinementFlow) error {
	r.c()
	return r.inner.CreateFlow(ctx, f)
}
func (r *countingSocraticaRepo) GetFlow(ctx context.Context, id string) (domain.RefinementFlow, error) {
	r.c()
	return r.inner.GetFlow(ctx, id)
}
func (r *countingSocraticaRepo) ActiveFlow(ctx context.Context, p, c string) (domain.RefinementFlow, bool, error) {
	r.c()
	return r.inner.ActiveFlow(ctx, p, c)
}
func (r *countingSocraticaRepo) SaveTransition(ctx context.Context, b, a domain.RefinementFlow) error {
	r.c()
	return r.inner.SaveTransition(ctx, b, a)
}
func (r *countingSocraticaRepo) Approve(ctx context.Context, id, d, p string) (domain.RefinementFlow, bool, error) {
	r.c()
	return r.inner.Approve(ctx, id, d, p)
}
func (r *countingSocraticaRepo) Ack(ctx context.Context, id, d, o, n string) (bool, bool, error) {
	r.c()
	return r.inner.Ack(ctx, id, d, o, n)
}
func (r *countingSocraticaRepo) ClaimDispatch(ctx context.Context, id, d, o string, t time.Time) (bool, error) {
	r.c()
	return r.inner.ClaimDispatch(ctx, id, d, o, t)
}
func (r *countingSocraticaRepo) ReleaseDispatch(ctx context.Context, id, o string) error {
	r.c()
	return r.inner.ReleaseDispatch(ctx, id, o)
}
func (r *countingSocraticaRepo) MarkDispatched(ctx context.Context, id, d string) error {
	r.c()
	return r.inner.MarkDispatched(ctx, id, d)
}
func (r *countingSocraticaRepo) ForceResend(ctx context.Context, id, o, n string) error {
	r.c()
	return r.inner.ForceResend(ctx, id, o, n)
}
func (r *countingSocraticaRepo) FlowExists(ctx context.Context, id string) (bool, error) {
	r.c()
	return r.inner.FlowExists(ctx, id)
}
func (r *countingSocraticaRepo) DeleteFlow(ctx context.Context, id string) error {
	r.c()
	return r.inner.DeleteFlow(ctx, id)
}
func (r *countingSocraticaRepo) FindByDispatchID(ctx context.Context, id string) (domain.RefinementFlow, bool, error) {
	r.c()
	return r.inner.FindByDispatchID(ctx, id)
}
func (r *countingSocraticaRepo) ConsumeDispatch(ctx context.Context, id, t string) (bool, error) {
	r.c()
	return r.inner.ConsumeDispatch(ctx, id, t)
}
func (r *countingSocraticaRepo) RedactFlow(ctx context.Context, id string) error {
	r.c()
	return r.inner.RedactFlow(ctx, id)
}
func (r *countingSocraticaRepo) UpsertHistory(ctx context.Context, h domain.RefinementHistory, l int) error {
	r.c()
	return r.inner.UpsertHistory(ctx, h, l)
}
func (r *countingSocraticaRepo) ListHistory(ctx context.Context, p, c string, l int) ([]domain.RefinementHistory, error) {
	r.c()
	return r.inner.ListHistory(ctx, p, c, l)
}
func (r *countingSocraticaRepo) PurgeStale(ctx context.Context, p string, t time.Time) ([]string, error) {
	r.c()
	return r.inner.PurgeStale(ctx, p, t)
}
func (r *countingSocraticaRepo) PurgeTerminal(ctx context.Context, p string, t time.Time) ([]string, error) {
	r.c()
	return r.inner.PurgeTerminal(ctx, p, t)
}
func (r *countingSocraticaRepo) PruneTerminal(ctx context.Context, p, c string, l int) ([]string, error) {
	r.c()
	return r.inner.PruneTerminal(ctx, p, c, l)
}

type tuiGen struct {
	calls     atomic.Int64
	questions []domain.Question
	scrutiny  *domain.ScrutinySynthesis
	fail      error
}

func (g *tuiGen) Name() string    { return "claude" }
func (g *tuiGen) Available() bool { return true }
func (g *tuiGen) GenerateQuestions(context.Context, ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	g.calls.Add(1)
	if g.fail != nil {
		return domain.GenerationResult{}, g.fail
	}
	return domain.GenerationResult{Questions: g.questions}, nil
}
func (g *tuiGen) Rewrite(context.Context, ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	g.calls.Add(1)
	return domain.RewriteResult{Proposed: "Envía el informe al cliente antes de [fecha]", Scrutiny: g.scrutiny}, nil
}

type tuiLauncher struct{ calls atomic.Int64 }

func (l *tuiLauncher) Launch(_ context.Context, _ ports.LaunchRequest, onStarted func() error) error {
	l.calls.Add(1)
	return onStarted()
}
