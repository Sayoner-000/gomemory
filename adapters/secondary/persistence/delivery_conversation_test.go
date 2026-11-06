package persistence

import (
	"context"
	"testing"
	"time"
)

type relojEntregas struct{}

func (relojEntregas) Now() time.Time { return time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC) }

// HCD-036 C-01: lo que se le entregó a un agente se anota en SU sesión; el
// arranque, la compactación o el cierre de otro agente no lo borran.
func TestEntregasPorConversacion(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	convA, convB := conversacionFija("agente-A"), conversacionFija("agente-B")
	for _, c := range []conversacionFija{convA, convB} {
		if _, err := NewSessionRepository(db).(*SessionRepository).WithConversation(c).Start("p"); err != nil {
			t.Fatal(err)
		}
	}
	logA := NewDeliveryLogRepository(db, "p").WithConversation(convA)
	logB := NewDeliveryLogRepository(db, "p").WithConversation(convB)
	bloquesA := NewDeliveredBlocksRepository(db, "p", relojEntregas{}).WithConversation(convA)
	bloquesB := NewDeliveredBlocksRepository(db, "p", relojEntregas{}).WithConversation(convB)

	if err := logA.Record("context", "hash-A"); err != nil {
		t.Fatal(err)
	}
	if err := bloquesA.Mark(ctx, []string{"b1", "b2"}); err != nil {
		t.Fatal(err)
	}
	// B arranca: reinicia lo SUYO.
	if err := logB.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := bloquesB.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if h, ok := logA.Last("context"); !ok || h != "hash-A" {
		t.Fatalf("A conserva su entrega: %q %v", h, ok)
	}
	if seen, _ := bloquesA.Seen(ctx, []string{"b1", "b2"}); !seen["b1"] || !seen["b2"] {
		t.Fatalf("A conserva sus bloques: %v", seen)
	}
	if _, ok := logB.Last("context"); ok {
		t.Fatal("B no ve como suya la entrega de A")
	}
	if seen, _ := bloquesB.Seen(ctx, []string{"b1"}); seen["b1"] {
		t.Fatal("B no ve como suyos los bloques de A")
	}
}
