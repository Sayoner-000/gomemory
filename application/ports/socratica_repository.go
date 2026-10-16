package ports

import (
	"context"
	"errors"
	"mem/domain"
	"time"
)

// ErrAckRejected: the ack names another owner or a superseded dispatch_id.
var ErrAckRejected = errors.New("socratica ack rejected")

// ErrSocraticaNoMatch: a conditional mutation affected no row (unknown flow,
// foreign owner or a state that changed meanwhile). Never reported as success.
var ErrSocraticaNoMatch = errors.New("socratica: no matching flow")

// Purge and prune methods return the deleted flow ids so callers can drop
// per-flow host resources (dispatch lock files) after the commit.
type SocraticaRepository interface {
	CreateFlow(context.Context, domain.RefinementFlow) error
	GetFlow(context.Context, string) (domain.RefinementFlow, error)
	ActiveFlow(context.Context, string, string) (domain.RefinementFlow, bool, error)
	SaveTransition(context.Context, domain.RefinementFlow, domain.RefinementFlow) error
	Approve(context.Context, string, string, string) (domain.RefinementFlow, bool, error)
	Ack(context.Context, string, string, string, string) (bool, bool, error)
	ClaimDispatch(context.Context, string, string, string, time.Time) (bool, error)
	ReleaseDispatch(context.Context, string, string) error
	MarkDispatched(context.Context, string, string) error
	ForceResend(context.Context, string, string, string) error
	DeleteFlow(context.Context, string) error
	FindByDispatchID(context.Context, string) (domain.RefinementFlow, bool, error)
	ConsumeDispatch(context.Context, string, string) (bool, error)
	RedactFlow(context.Context, string) error
	UpsertHistory(context.Context, domain.RefinementHistory, int) error
	ListHistory(context.Context, string, string, int) ([]domain.RefinementHistory, error)
	PurgeStale(context.Context, string, time.Time) ([]string, error)
	PurgeTerminal(context.Context, string, time.Time) ([]string, error)
	PruneTerminal(context.Context, string, string, int) ([]string, error)
}
