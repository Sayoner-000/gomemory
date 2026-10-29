package usecases_test

import (
	"context"
	"fmt"
	"mem/application/usecases"
	"mem/domain"
	"testing"
	"time"
)

func TestHCDPerformanceFiftyTasks(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	d, e := uc.ReadDelivery(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	tasks := make([]domain.Task, 50)
	for i := range tasks {
		tasks[i] = d.Tasks[0]
		tasks[i].ID = fmt.Sprintf("T-%03d", i+1)
	}
	if _, e = uc.Tasks(ctx, u.ID, u.StateRevision, "fifty", "VS-01", tasks); e != nil {
		t.Fatal(e)
	}
	checks := []struct {
		name  string
		limit time.Duration
		run   func() error
	}{{"resume", 300 * time.Millisecond, func() error { _, e := uc.Resume(ctx, u.ID, "codex", "session"); return e }}, {"status", 300 * time.Millisecond, func() error { _, e := uc.Status(ctx, u.ID); return e }}, {"next", 300 * time.Millisecond, func() error { _, e := uc.Next(ctx, u.ID); return e }}, {"heartbeat", 200 * time.Millisecond, func() error {
		return uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: "codex", SessionID: "session"})
	}}}
	for _, check := range checks {
		start := time.Now()
		e = check.run()
		elapsed := time.Since(start)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("%s: %s", check.name, elapsed)
		if elapsed > check.limit {
			t.Fatalf("%s took %s; limit %s", check.name, elapsed, check.limit)
		}
	}
}
