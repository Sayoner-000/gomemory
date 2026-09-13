package persistence

import (
	"fmt"
	"sync"
	"testing"

	"mem/domain"
)

// C-002 (acr_c1622428): insertMemory lee (dedup) y luego escribe dentro de la
// misma transacción. Con BEGIN diferido, en WAL SQLite no invoca el busy
// handler al promover la lectura a escritura y las inserciones concurrentes
// fallaban con SQLITE_BUSY pese al busy_timeout.
func TestInsertMemory_ConcurrentInsertsDoNotFailWithBusy(t *testing.T) {
	db := openTestDB(t)
	const n = 64
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := InsertMemory(db, &domain.Memory{
				Project: "proj",
				Type:    domain.Learning,
				Title:   fmt.Sprintf("titulo %d", i),
				Content: fmt.Sprintf("contenido %d", i),
			}); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	fallos := 0
	for err := range errs {
		if fallos == 0 {
			t.Logf("primer error: %v", err)
		}
		fallos++
	}
	if fallos > 0 {
		t.Fatalf("%d de %d inserciones concurrentes fallaron", fallos, n)
	}
}

// C-003: una promoción posterior con la misma topic_key reescribe el contenido;
// el linaje debe apuntar a la revisión cuyo contenido quedó guardado.
func TestInsertMemory_DedupUpdatesSourceReviewID(t *testing.T) {
	db := openTestDB(t)
	var id int64
	for _, rid := range []string{"1", "2"} {
		got, err := InsertMemory(db, &domain.Memory{
			Project: "proj", Type: domain.Learning, TopicKey: "review-learning:x:y",
			Title: "Revisión: x en y", Content: "origen revisión " + rid, SourceReviewID: rid,
		})
		if err != nil {
			t.Fatal(err)
		}
		id = got
	}
	var rid string
	if err := db.QueryRow(`SELECT COALESCE(source_review_id,'') FROM memories WHERE id = ?`, id).Scan(&rid); err != nil {
		t.Fatal(err)
	}
	if rid != "2" {
		t.Fatalf("source_review_id = %q, quiero 2 (el contenido es de la revisión 2)", rid)
	}

	reviews := NewReviewRepository(db).(*ReviewRepository)
	promovidas, deduplicadas, err := reviews.CountPromotedMemories("proj", "2")
	if err != nil {
		t.Fatal(err)
	}
	if promovidas != 1 {
		t.Errorf("revisión 2: promovidas = %d, quiero 1", promovidas)
	}
	_ = deduplicadas // updated_at y created_at comparten segundo en el test
}

// C-003: un guardado sin revisión con la misma topic_key no borra el linaje.
func TestInsertMemory_DedupWithoutReviewKeepsSourceReviewID(t *testing.T) {
	db := openTestDB(t)
	base := domain.Memory{Project: "proj", Type: domain.Learning, TopicKey: "tk", Title: "t"}
	primera := base
	primera.Content, primera.SourceReviewID = "uno", "7"
	id, err := InsertMemory(db, &primera)
	if err != nil {
		t.Fatal(err)
	}
	segunda := base
	segunda.Content = "dos"
	if _, err := InsertMemory(db, &segunda); err != nil {
		t.Fatal(err)
	}
	var rid string
	if err := db.QueryRow(`SELECT COALESCE(source_review_id,'') FROM memories WHERE id = ?`, id).Scan(&rid); err != nil {
		t.Fatal(err)
	}
	if rid != "7" {
		t.Fatalf("source_review_id = %q, quiero 7 conservado", rid)
	}
}

// C-004: GetMemoryByID y las búsquedas proyectan topic_key como List/ListAll.
func TestMemoryProjections_IncludeTopicKey(t *testing.T) {
	db := openTestDB(t)
	id, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Decision, TopicKey: "clave-x",
		Title: "decision proyectada", Content: "contenido proyectado",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := GetMemoryByID(db, "proj", id)
	if err != nil || got == nil {
		t.Fatalf("GetMemoryByID: %v %v", got, err)
	}
	if got.TopicKey != "clave-x" {
		t.Errorf("GetMemoryByID TopicKey = %q", got.TopicKey)
	}

	fts, err := searchMemoriesFTS(db, "proj", "proyectada", 10)
	if err != nil || len(fts) != 1 {
		t.Fatalf("searchMemoriesFTS: %v %v", fts, err)
	}
	if fts[0].TopicKey != "clave-x" {
		t.Errorf("searchMemoriesFTS TopicKey = %q", fts[0].TopicKey)
	}

	like, err := searchMemoriesLike(db, "proj", "proyectada", 10)
	if err != nil || len(like) != 1 {
		t.Fatalf("searchMemoriesLike: %v %v", like, err)
	}
	if like[0].TopicKey != "clave-x" {
		t.Errorf("searchMemoriesLike TopicKey = %q", like[0].TopicKey)
	}
}

// C-005: % y _ en la consulta son literales, no comodines de LIKE.
func TestSearchMemoriesLike_EscapesWildcards(t *testing.T) {
	db := openTestDB(t)
	for _, c := range []string{"avance del 100% listo", "sin porcentaje", "snake_case aquí", "snakeXcase aquí", `ruta C:\tmp`} {
		if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Learning, Title: c, Content: c}); err != nil {
			t.Fatal(err)
		}
	}
	for query, want := range map[string]int{"%": 1, "snake_case": 1, `C:\tmp`: 1} {
		got, err := searchMemoriesLike(db, "proj", query, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want {
			t.Errorf("searchMemoriesLike(%q) = %d resultados, quiero %d", query, len(got), want)
		}
	}
}
