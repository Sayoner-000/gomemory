package usecases_test

import (
	"context"
	"errors"
	"mem/domain"
	"testing"
)

func TestHCDDeliveryRegistersNextSliceAndRejectsTaskCycle(t *testing.T) {
	uc, repo, _, _, _ := newHCDFlow(t)
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	u.State = domain.StateReady
	u.ApprovedRev = u.DefinitionRev
	if e = repo.UpdateUnit(ctx, u, u.StateRevision); e != nil {
		t.Fatal(e)
	}
	slice := domain.Slice{ID: "VS-01", Observable: "Guardar datos", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Limits: "Guardar sin borrar", Verification: "Test de persistencia"}
	u, e = uc.Slice(ctx, u.ID, u.StateRevision, "slice-1", []domain.Slice{slice})
	if e != nil {
		t.Fatal(e)
	}
	if u.DeliveryRev == "" {
		t.Fatal("missing delivery revision")
	}
	task := domain.Task{ID: "T-001", Title: "Guardar datos", SliceID: slice.ID, Stories: []string{"US1"}, Criteria: []string{"C-01"}, Type: "implementation", Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test", Rules: []string{"HCD-TRACE-01"}}
	u, e = uc.Tasks(ctx, u.ID, u.StateRevision, "tasks-1", slice.ID, []domain.Task{task})
	if e != nil {
		t.Fatal(e)
	}
	before, e := uc.ReadDelivery(ctx, u.ID)
	if e != nil || len(before.Slices) != 1 || len(before.Tasks) != 1 {
		t.Fatalf("delivery=%+v err=%v", before, e)
	}
	task.Dependencies = []string{task.ID}
	if _, e = uc.Tasks(ctx, u.ID, u.StateRevision, "tasks-cycle", slice.ID, []domain.Task{task}); !errors.Is(e, domain.ErrAtomicity) {
		t.Fatalf("cycle err=%v", e)
	}
	after, e := uc.ReadDelivery(ctx, u.ID)
	if e != nil || len(after.Tasks[0].Dependencies) != 0 {
		t.Fatalf("failed mutation changed delivery: %+v %v", after, e)
	}
	links, e := repo.ListLinks(ctx, u.ID)
	if e != nil || len(links) < 3 {
		t.Fatalf("links=%+v err=%v", links, e)
	}
}

func TestHCDCheckDetectsOrphanAndMultipleResults(t *testing.T) {
	for _, tc := range []struct {
		code   string
		modify func(*domain.Task)
	}{{"ATOM-LINK", func(t *domain.Task) { t.Criteria = []string{"C-99"} }}, {"ATOM-RESULT", func(t *domain.Task) { t.Result = "1. Guardar\n2. Borrar" }}} {
		t.Run(tc.code, func(t *testing.T) {
			uc, u := executableHCD(t)
			ctx := context.Background()
			d, e := uc.ReadDelivery(ctx, u.ID)
			if e != nil {
				t.Fatal(e)
			}
			tc.modify(&d.Tasks[0])
			if _, e = uc.Tasks(ctx, u.ID, u.StateRevision, "change-task", "VS-01", d.Tasks); e != nil {
				t.Fatal(e)
			}
			a, e := uc.Check(ctx, u.ID, "T-001")
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, c := range a.Checks {
				found = found || c.Code == tc.code
			}
			if !found {
				t.Fatalf("checks=%+v", a.Checks)
			}
		})
	}
}

// Un archivo cuyos encabezados no siguen «## VS-NN.» se parseaba como entrega
// vacía y `slice` respondía ok sin registrar nada.
func TestHCDDeliveryRejectsEmptySliceAndTaskLists(t *testing.T) {
	uc, repo, _, _, _ := newHCDFlow(t)
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	u.State = domain.StateReady
	u.ApprovedRev = u.DefinitionRev
	if e = repo.UpdateUnit(ctx, u, u.StateRevision); e != nil {
		t.Fatal(e)
	}
	var fields *domain.HCDValidationError
	if _, e = uc.Slice(ctx, u.ID, u.StateRevision, "slice-empty", nil); !errors.As(e, &fields) || fields.Checks[0].Code != "SLICE-EMPTY" {
		t.Fatalf("empty slice err=%v", e)
	}
	if _, e = uc.Slice(ctx, u.ID, u.StateRevision, "slice-incomplete", []domain.Slice{{ID: "VS-01"}}); !errors.As(e, &fields) || fields.Checks[0].Code != "SLICE-FIELDS" {
		t.Fatalf("incomplete slice err=%v", e)
	}
	if _, e = uc.Tasks(ctx, u.ID, u.StateRevision, "tasks-empty", "VS-01", nil); !errors.As(e, &fields) || fields.Checks[0].Code != "TASK-EMPTY" {
		t.Fatalf("empty tasks err=%v", e)
	}
	got, e := repo.GetUnit(ctx, u.ID)
	if e != nil || got.StateRevision != u.StateRevision || got.DeliveryRev != "" {
		t.Fatalf("rejected input changed the unit: %+v %v", got, e)
	}
}
