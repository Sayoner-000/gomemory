package persistence

import (
	"testing"

	"mem/domain"
)

func TestHookGuardEvents_MigracionIdempotente(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		db, err := Init(dir)
		if err != nil {
			t.Fatalf("init %d: %v", i, err)
		}
		if err := RecordGuardEvent(db, "proj", "claude", domain.GuardDuplicateDropped, "user-prompt-submit"); err != nil {
			t.Fatalf("registrar tras init %d: %v", i, err)
		}
		_ = db.Close()
	}
}

func TestHookGuardEvents_SumaPorDiaYAgrega(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 3; i++ {
		if err := RecordGuardEvent(db, "proj", "claude", domain.GuardBudgetTrimmed, "orig=17777,emit=9990"); err != nil {
			t.Fatalf("registrar: %v", err)
		}
	}
	_ = RecordGuardEvent(db, "proj", "codex", domain.GuardDuplicateDropped, "turn-end")
	_ = RecordGuardEvent(db, "otro", "claude", domain.GuardBudgetTrimmed, "x")

	got, err := GuardEventsSince(db, "proj", 7)
	if err != nil {
		t.Fatalf("GuardEventsSince: %v", err)
	}
	want := map[string]int{"claude/" + domain.GuardBudgetTrimmed: 3, "codex/" + domain.GuardDuplicateDropped: 1}
	if len(got) != len(want) {
		t.Fatalf("filas inesperadas: %+v", got)
	}
	for _, g := range got {
		if want[g.Agent+"/"+g.Kind] != g.Count {
			t.Errorf("conteo de %s/%s = %d", g.Agent, g.Kind, g.Count)
		}
		if g.Kind == domain.GuardBudgetTrimmed && g.LastDetail != "orig=17777,emit=9990" {
			t.Errorf("detalle inesperado: %q", g.LastDetail)
		}
	}
}

func TestHookGuardEvents_ExcluyeDiasFueraDeVentana(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO hook_guard_events (project, agent, kind, day, count, last_detail, updated_at)
		 VALUES (?, ?, ?, '2000-01-01', 9, '', '2000-01-01 00:00:00')`,
		"proj", "claude", domain.GuardDuplicateDropped); err != nil {
		t.Fatalf("sembrar día viejo: %v", err)
	}
	got, err := GuardEventsSince(db, "proj", 7)
	if err != nil {
		t.Fatalf("GuardEventsSince: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("los días fuera de la ventana no cuentan: %+v", got)
	}
}

func TestChannelActivityRepository_ImplementaHookGuardRecorder(t *testing.T) {
	db := openTestDB(t)
	r := NewChannelActivityRepository(db, "proj")
	if err := r.RecordGuard("opencode", domain.GuardToolOutputExcluded, "bash"); err != nil {
		t.Fatalf("RecordGuard: %v", err)
	}
	got, err := r.GuardSince(7)
	if err != nil || len(got) != 1 || got[0].Count != 1 || got[0].LastDetail != "bash" {
		t.Errorf("GuardSince: %+v %v", got, err)
	}
}
