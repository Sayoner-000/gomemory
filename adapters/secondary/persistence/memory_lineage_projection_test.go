package persistence

import (
	"testing"

	"mem/domain"
)

// acr_5836d32d, C-002: source_review_id se persistía pero ninguna proyección
// lo devolvía, así que el linaje solo era visible para CountPromotedMemories.
func TestProjections_DevuelvenSourceReviewID(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = db.Close() }()

	id, err := InsertMemory(db, &domain.Memory{Project: "p", Type: domain.Learning, Title: "linaje promovido",
		Content: "aprendizaje salido de una revisión", TopicKey: "tk-linaje", SourceReviewID: "acr_linaje"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	check := func(via string, got string) {
		t.Helper()
		if got != "acr_linaje" {
			t.Errorf("%s: SourceReviewID = %q, want \"acr_linaje\"", via, got)
		}
	}

	byID, err := GetMemoryByID(db, "p", id)
	if err != nil || byID == nil {
		t.Fatalf("GetMemoryByID: %v %v", byID, err)
	}
	check("GetMemoryByID", byID.SourceReviewID)

	byTopic, err := GetMemoryByTopicKey(db, "p", "tk-linaje")
	if err != nil || byTopic == nil {
		t.Fatalf("GetMemoryByTopicKey: %v %v", byTopic, err)
	}
	check("GetMemoryByTopicKey", byTopic.SourceReviewID)

	list, err := ListMemories(db, "p", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListMemories: %v %v", list, err)
	}
	check("ListMemories", list[0].SourceReviewID)

	all, err := ListAllMemories(db, "p")
	if err != nil || len(all) != 1 {
		t.Fatalf("ListAllMemories: %v %v", all, err)
	}
	check("ListAllMemories", all[0].SourceReviewID)

	fts, err := searchMemoriesFTS(db, "p", "linaje", 10)
	if err != nil || len(fts) != 1 {
		t.Fatalf("searchMemoriesFTS: %v %v", fts, err)
	}
	check("searchMemoriesFTS", fts[0].SourceReviewID)

	like, err := searchMemoriesLike(db, "p", "linaje", 10)
	if err != nil || len(like) != 1 {
		t.Fatalf("searchMemoriesLike: %v %v", like, err)
	}
	check("searchMemoriesLike", like[0].SourceReviewID)
}

// acr_5836d32d, C-001: ImportMemory descartaba topic_key y source_review_id.
func TestImportMemory_PreservaIdentidadYLinaje(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = db.Close() }()

	id, err := ImportMemory(db, &domain.Memory{Project: "p", Type: domain.Decision, Title: "fijada",
		Content: "documento fijado", TopicKey: " tk-fijada ", SourceReviewID: "acr_origen"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	got, err := GetMemoryByID(db, "p", id)
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	if got.TopicKey != "tk-fijada" || got.SourceReviewID != "acr_origen" {
		t.Fatalf("import perdió identidad o linaje: topic_key=%q source_review_id=%q", got.TopicKey, got.SourceReviewID)
	}
}
