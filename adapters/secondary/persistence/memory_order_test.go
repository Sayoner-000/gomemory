package persistence

import (
	"fmt"
	"testing"

	"mem/domain"
)

// Feature 035: created_at tiene resolución de segundo. Con el desempate a
// criterio de SQLite, qué memorias entraban en la ventana dependía de si los
// inserts cruzaban un segundo, y los tests pasaban o fallaban según la carga.
// El id (AUTOINCREMENT) es único y monótono: desempata de forma determinista.
func TestListMemories_EmpateDeSegundoSeDesempataPorID(t *testing.T) {
	db := openTestDB(t)
	var ids []int64
	for i := 0; i < 5; i++ {
		id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Learning, Title: fmt.Sprintf("m%d", i), Content: fmt.Sprintf("contenido distinto %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if _, err := db.Exec(`UPDATE memories SET created_at = '2026-01-01 10:00:00'`); err != nil {
		t.Fatal(err)
	}
	got, err := ListMemories(db, "proj", 3)
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range got {
		if want := ids[len(ids)-1-i]; m.ID != want {
			t.Fatalf("posición %d: id %d, se esperaba %d (id DESC ante el empate)", i, m.ID, want)
		}
	}
}
