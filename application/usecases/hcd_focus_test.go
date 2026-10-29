package usecases

import (
	"context"
	"mem/application/ports"
	"mem/domain"
	"testing"
	"time"
)

type hcdFocusRepoStub struct {
	ports.HCDRepository
	units    map[string]domain.Unit
	focus    domain.Focus
	hasFocus bool
}

func (s *hcdFocusRepoStub) GetUnit(_ context.Context, id string) (domain.Unit, error) {
	u, ok := s.units[id]
	if !ok {
		return domain.Unit{}, domain.ErrNotFound
	}
	return u, nil
}
func (s *hcdFocusRepoStub) SetFocus(_ context.Context, f domain.Focus) error {
	s.focus = f
	s.hasFocus = true
	return nil
}
func (s *hcdFocusRepoStub) GetFocus(context.Context) (domain.Focus, error) {
	if !s.hasFocus {
		return domain.Focus{}, domain.ErrNotFound
	}
	return s.focus, nil
}
func (s *hcdFocusRepoStub) ClearFocus(context.Context) error {
	s.hasFocus = false
	s.focus = domain.Focus{}
	return nil
}

type hcdFocusClock struct{ now time.Time }

func (c hcdFocusClock) Now() time.Time { return c.now }

func TestFocusSetReplacesCurrentAndClear(t *testing.T) {
	repo := &hcdFocusRepoStub{units: map[string]domain.Unit{"HCD-001": {ID: "HCD-001", State: domain.StateDraft}, "HCD-002": {ID: "HCD-002", State: domain.StateReady}}}
	uc := NewHCDFocus(repo, hcdFocusClock{now: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)}, "claude", "session-1")
	if _, err := uc.Current(context.Background()); err != domain.ErrNotFound {
		t.Fatalf("empty focus = %v", err)
	}
	if err := uc.Set(context.Background(), "HCD-001"); err != nil {
		t.Fatal(err)
	}
	if err := uc.Set(context.Background(), "HCD-002"); err != nil {
		t.Fatal(err)
	}
	got, err := uc.Current(context.Background())
	if err != nil || got.UnitID != "HCD-002" {
		t.Fatalf("focus=%+v err=%v", got, err)
	}
	if err := uc.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Current(context.Background()); err != domain.ErrNotFound {
		t.Fatalf("cleared focus=%v", err)
	}
}

func TestFocusRejectsTerminalUnit(t *testing.T) {
	repo := &hcdFocusRepoStub{units: map[string]domain.Unit{"HCD-001": {ID: "HCD-001", State: domain.StateCompleted}}}
	uc := NewHCDFocus(repo, hcdFocusClock{now: time.Now()}, "codex", "session-1")
	if err := uc.Set(context.Background(), "HCD-001"); err == nil {
		t.Fatal("terminal unit cannot be focused")
	}
	if repo.hasFocus {
		t.Fatal("terminal unit became active")
	}
}
