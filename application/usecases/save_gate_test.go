package usecases_test

import (
	"errors"
	"reflect"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

type gateStoreFake struct {
	insertID  int64
	insertErr error
	listErr   error
	mems      []domain.Memory
	inserted  bool
}

func (f *gateStoreFake) Insert(*domain.Memory) (int64, error) {
	f.inserted = true
	return f.insertID, f.insertErr
}
func (f *gateStoreFake) ListAll(string) ([]domain.Memory, error) { return f.mems, f.listErr }

func TestGateSimilarity_TituloIdentico(t *testing.T) {
	title, _ := usecases.GateSimilarity(
		domain.Memory{Title: "misma decisión técnica"},
		domain.Memory{Title: "misma decisión técnica"},
	)
	if title != 1 {
		t.Fatalf("title similarity = %v, want 1", title)
	}
}

func TestGateSimilarity_SinTokensComunes(t *testing.T) {
	title, body := usecases.GateSimilarity(
		domain.Memory{Title: "caché redis", Content: "latencia"},
		domain.Memory{Title: "facturación anual", Content: "impuestos"},
	)
	if title != 0 || body != 0 {
		t.Fatalf("similarities = (%v, %v), want (0, 0)", title, body)
	}
}

func TestGateSizeCompatible(t *testing.T) {
	if usecases.GateSizeCompatible(10, 2, 0.3) {
		t.Fatal("incompatible sizes accepted")
	}
	if !usecases.GateSizeCompatible(10, 5, 0.3) {
		t.Fatal("compatible sizes rejected")
	}
	if usecases.GateSizeCompatible(0, 5, 0.3) {
		t.Fatal("empty size accepted")
	}
}

func TestCheckNearDuplicates(t *testing.T) {
	saved := domain.Memory{ID: 9, Type: domain.Learning, Title: "misma decisión técnica", Content: "contenido parecido"}
	result := usecases.CheckNearDuplicates([]domain.Memory{
		{ID: 2, Type: domain.Learning, Title: "misma decisión técnica", Content: "contenido parecido", TopicKey: "tema"},
		{ID: 3, Type: domain.Checkpoint, Title: saved.Title, Content: saved.Content},
		{ID: 4, Type: domain.Decision, Title: saved.Title, Content: saved.Content},
	}, saved, 9)
	if !result.Checked || len(result.Candidates) != 1 || result.Candidates[0].ID != 2 || result.Candidates[0].TopicKey != "tema" {
		t.Fatalf("unexpected gate result: %#v", result)
	}
	updated := usecases.CheckNearDuplicates([]domain.Memory{{ID: 9, Type: domain.Learning}}, saved, 9)
	if !updated.Updated || len(updated.Candidates) != 0 {
		t.Fatalf("upsert result: %#v", updated)
	}
}

func TestCheckNearDuplicates_OrdenaYLimita(t *testing.T) {
	saved := domain.Memory{Type: domain.Learning, Title: "caché redis compartida", Content: "latencia caché redis compartida"}
	snapshot := []domain.Memory{
		{ID: 5, Type: domain.Learning, Title: saved.Title, Content: saved.Content},
		{ID: 1, Type: domain.Learning, Title: saved.Title, Content: saved.Content},
		{ID: 3, Type: domain.Learning, Title: saved.Title, Content: saved.Content},
		{ID: 2, Type: domain.Learning, Title: saved.Title, Content: saved.Content},
	}
	result := usecases.CheckNearDuplicates(snapshot, saved, 99)
	if len(result.Candidates) != 3 {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
	for i, id := range []int64{1, 2, 3} {
		if result.Candidates[i].ID != id {
			t.Fatalf("candidate %d = %d, want %d", i, result.Candidates[i].ID, id)
		}
	}
	checkpoint := usecases.CheckNearDuplicates(snapshot, domain.Memory{Type: domain.Checkpoint}, 99)
	if len(checkpoint.Candidates) != 0 {
		t.Fatalf("checkpoint candidates = %#v", checkpoint.Candidates)
	}
}

func TestCheckNearDuplicates_ExclusionesYDeterminismo(t *testing.T) {
	saved := domain.Memory{Type: domain.Learning, Title: "caché redis compartida", Content: "latencia caché redis compartida"}
	a := []domain.Memory{{ID: 2, Type: domain.Learning, Title: saved.Title, Content: saved.Content, TopicKey: "reuse"}, {ID: 3, Type: domain.Decision, Title: saved.Title, Content: saved.Content}, {ID: 4, Type: domain.Checkpoint, Title: saved.Title, Content: saved.Content}}
	b := []domain.Memory{a[2], a[1], a[0]}
	gotA, gotB := usecases.CheckNearDuplicates(a, saved, 99), usecases.CheckNearDuplicates(b, saved, 99)
	if len(gotA.Candidates) != 1 || gotA.Candidates[0].TopicKey != "reuse" || !reflect.DeepEqual(gotA, gotB) {
		t.Fatalf("A=%#v B=%#v", gotA, gotB)
	}
}

func TestSaveWithGate(t *testing.T) {
	t.Run("list failure does not block save", func(t *testing.T) {
		store := &gateStoreFake{insertID: 7, listErr: errors.New("offline")}
		id, result, err := usecases.SaveWithGate(store, &domain.Memory{Project: "p", Type: domain.Learning, Content: "ok"})
		if err != nil || id != 7 || !store.inserted || result.Checked || result.Reason == "" {
			t.Fatalf("id=%d result=%#v err=%v", id, result, err)
		}
	})
	t.Run("insert failure", func(t *testing.T) {
		store := &gateStoreFake{insertErr: errors.New("write")}
		_, _, err := usecases.SaveWithGate(store, &domain.Memory{Project: "p", Content: "ok"})
		if err == nil {
			t.Fatal("expected insert error")
		}
	})
	t.Run("normal path delegates to candidate check", func(t *testing.T) {
		memory := domain.Memory{ID: 1, Project: "p", Type: domain.Learning, Title: "misma caché", Content: "redis compartido"}
		store := &gateStoreFake{insertID: 7, mems: []domain.Memory{memory}}
		_, result, err := usecases.SaveWithGate(store, &domain.Memory{Project: "p", Type: domain.Learning, Title: memory.Title, Content: memory.Content})
		if err != nil || len(result.Candidates) != 1 || result.Candidates[0].ID != 1 {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
}
