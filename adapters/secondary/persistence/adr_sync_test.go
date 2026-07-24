package persistence

import (
	"testing"

	"mem/domain"
)

func TestMigrate_CreatesADRSyncRecordsTable(t *testing.T) {
	db := openTestDB(t)
	// Si la tabla/índices no existieran, este INSERT fallaría.
	memID, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "decisión"})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	_, err = InsertADRSyncRecord(db, &domain.ADRSyncRecord{
		Project: "proj", MemoryID: &memID, Provider: "codebase-memory-mcp",
		Section: domain.ADRSectionArchitecture, BlockKey: "id=1", Origin: domain.SyncOriginGomemory,
		Status: domain.SyncStatusOK, ContentHash: "abc",
	})
	if err != nil {
		t.Fatalf("insert adr sync record: %v", err)
	}
}

func TestADRSyncRecords_UniqueByMemoryID(t *testing.T) {
	db := openTestDB(t)
	memID, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "decisión"})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	rec := &domain.ADRSyncRecord{
		Project: "proj", MemoryID: &memID, Provider: "prov",
		Section: domain.ADRSectionArchitecture, BlockKey: "id=1", Origin: domain.SyncOriginGomemory,
		Status: domain.SyncStatusOK, ContentHash: "abc",
	}
	if _, err := InsertADRSyncRecord(db, rec); err != nil {
		t.Fatalf("primer insert: %v", err)
	}
	if _, err := InsertADRSyncRecord(db, rec); err == nil {
		t.Fatal("un segundo registro con el mismo (project, memory_id) debería violar el índice único")
	}
}

func TestADRSyncRecords_UniqueByBlockKey(t *testing.T) {
	db := openTestDB(t)
	memA, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "a"})
	memB, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "b"})

	rec1 := &domain.ADRSyncRecord{Project: "proj", MemoryID: &memA, Provider: "prov", Section: "ARCHITECTURE", BlockKey: "mismo-key", Origin: domain.SyncOriginProvider, Status: domain.SyncStatusOK, ContentHash: "h1"}
	rec2 := &domain.ADRSyncRecord{Project: "proj", MemoryID: &memB, Provider: "prov", Section: "ARCHITECTURE", BlockKey: "mismo-key", Origin: domain.SyncOriginProvider, Status: domain.SyncStatusOK, ContentHash: "h2"}

	if _, err := InsertADRSyncRecord(db, rec1); err != nil {
		t.Fatalf("primer insert: %v", err)
	}
	if _, err := InsertADRSyncRecord(db, rec2); err == nil {
		t.Fatal("un segundo registro con el mismo (project, provider, block_key) debería violar el índice único")
	}
}

func TestGetADRSyncByMemory(t *testing.T) {
	db := openTestDB(t)
	memID, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "x"})
	want := &domain.ADRSyncRecord{Project: "proj", MemoryID: &memID, Provider: "prov", Section: "ARCHITECTURE", BlockKey: "id=1", Origin: domain.SyncOriginGomemory, Status: domain.SyncStatusOK, ContentHash: "h"}
	if _, err := InsertADRSyncRecord(db, want); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := GetADRSyncByMemory(db, "proj", memID)
	if err != nil {
		t.Fatalf("get by memory: %v", err)
	}
	if got == nil {
		t.Fatal("esperaba encontrar el registro")
	}
	if got.BlockKey != "id=1" || got.Origin != domain.SyncOriginGomemory {
		t.Fatalf("registro inesperado: %+v", got)
	}
}

func TestGetADRSyncByMemory_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := GetADRSyncByMemory(db, "proj", 999)
	if err != nil {
		t.Fatalf("no debería error, solo nil: %v", err)
	}
	if got != nil {
		t.Fatalf("esperaba nil, hubo %+v", got)
	}
}

func TestGetADRSyncByBlockKey(t *testing.T) {
	db := openTestDB(t)
	memID, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "x"})
	rec := &domain.ADRSyncRecord{Project: "proj", MemoryID: &memID, Provider: "prov", Section: "TRADEOFFS", BlockKey: "hash-abc", Origin: domain.SyncOriginProvider, Status: domain.SyncStatusOK, ContentHash: "h"}
	if _, err := InsertADRSyncRecord(db, rec); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := GetADRSyncByBlockKey(db, "proj", "prov", "hash-abc")
	if err != nil {
		t.Fatalf("get by block key: %v", err)
	}
	if got == nil || got.MemoryID == nil || *got.MemoryID != memID {
		t.Fatalf("registro inesperado: %+v", got)
	}
}

func TestListADRSyncRecords(t *testing.T) {
	db := openTestDB(t)
	memA, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "a"})
	memB, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Decision, Content: "b"})

	if _, err := InsertADRSyncRecord(db, &domain.ADRSyncRecord{Project: "proj", MemoryID: &memA, Provider: "prov", Section: "ARCHITECTURE", BlockKey: "id=1", Origin: domain.SyncOriginGomemory, Status: domain.SyncStatusOK, ContentHash: "h1"}); err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	if _, err := InsertADRSyncRecord(db, &domain.ADRSyncRecord{Project: "proj", MemoryID: &memB, Provider: "prov", Section: "TRADEOFFS", BlockKey: "id=2", Origin: domain.SyncOriginGomemory, Status: domain.SyncStatusPending, ContentHash: "h2"}); err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	// De otro proyecto: no debería aparecer en el listado de "proj".
	memC, _ := InsertMemory(db, &domain.Memory{Project: "otro-proj", Type: domain.Architecture, Content: "c"})
	if _, err := InsertADRSyncRecord(db, &domain.ADRSyncRecord{Project: "otro-proj", MemoryID: &memC, Provider: "prov", Section: "ARCHITECTURE", BlockKey: "id=3", Origin: domain.SyncOriginGomemory, Status: domain.SyncStatusOK, ContentHash: "h3"}); err != nil {
		t.Fatalf("insert 3: %v", err)
	}

	recs, err := ListADRSyncRecords(db, "proj")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("esperaba 2 registros de 'proj', hubo %d", len(recs))
	}
}

func TestUpdateADRSyncStatus(t *testing.T) {
	db := openTestDB(t)
	memID, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "x"})
	id, err := InsertADRSyncRecord(db, &domain.ADRSyncRecord{Project: "proj", MemoryID: &memID, Provider: "prov", Section: "ARCHITECTURE", BlockKey: "id=1", Origin: domain.SyncOriginGomemory, Status: domain.SyncStatusPending, ContentHash: "h1"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := UpdateADRSyncStatus(db, id, domain.SyncStatusOK, "h2"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err := GetADRSyncByMemory(db, "proj", memID)
	if err != nil || got == nil {
		t.Fatalf("get after update: %v / %v", got, err)
	}
	if got.Status != domain.SyncStatusOK || got.ContentHash != "h2" {
		t.Fatalf("update no se aplicó: %+v", got)
	}
}
